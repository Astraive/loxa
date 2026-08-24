package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	publichttp "github.com/astraive/loza/collector/server/http"
	"github.com/rs/zerolog/log"
)

func (s *collectorState) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":            collectorVersion,
		"ingest_api_version": "v1",
		"schema_version":     s.cfg.schemaSchemaVersion,
		"event_version":      s.cfg.schemaEventVersion,
	})
}

func (s *collectorState) handleStatus(w http.ResponseWriter, r *http.Request) {
	// Auth is handled by the multi-key middleware in BuildMux.
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           statusString(s.isReady()),
		"version":          collectorVersion,
		"uptime_seconds":   s.uptimeSeconds(time.Now()),
		"reliability_mode": s.cfg.reliabilityMode,
		"ingest": map[string]any{
			"accepted":   s.metrics.eventsAccepted.Load(),
			"rejected":   s.metrics.eventsRejected.Load(),
			"invalid":    s.metrics.eventsInvalid.Load(),
			"duplicates": s.metrics.eventsDeduped.Load(),
		},
		"queue": map[string]any{
			"mode":        s.cfg.reliabilityMode,
			"depth":       deliveryQueueDepth(s.deliveryQueue),
			"spool_bytes": s.metrics.spoolBytes.Load(),
			"queue_bytes": s.metrics.queueBytes.Load(),
		},
		"inflight": map[string]any{
			"requests": s.metrics.inflightRequests.Load(),
			"events":   s.metrics.inflightEvents.Load(),
		},
		"limits": map[string]any{
			"max_event_bytes":       s.cfg.maxEventBytes,
			"max_attr_count":        s.cfg.maxAttrCount,
			"max_attr_depth":        s.cfg.maxAttrDepth,
			"max_string_length":     s.cfg.maxStringLength,
			"max_inflight_requests": s.cfg.maxInflightRequests,
			"max_inflight_events":   s.cfg.maxInflightEvents,
			"max_queue_bytes":       s.cfg.maxQueueBytes,
		},
	})
}

func (s *collectorState) handleSinks(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	sinks := s.sinksForShutdown()
	out := make([]map[string]any, 0, len(sinks))
	for _, sink := range sinks {
		out = append(out, sinkStatus(sink.Name, s.effectiveSinkHealthy(), ""))
	}
	if len(out) == 0 && s.ingestSink != nil {
		out = append(out, sinkStatus(s.ingestSink.Name(), s.effectiveSinkHealthy(), ""))
	}
	writeJSON(w, http.StatusOK, map[string]any{"sinks": out})
}

func (s *collectorState) handleSink(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	name := r.PathValue("name")
	if sink, ok := s.findSinkByName(name); ok {
		writeJSON(w, http.StatusOK, sinkStatus(sink.Name, s.effectiveSinkHealthy(), ""))
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "sink_not_found"})
}

func (s *collectorState) handleSinkTest(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	if rejectUnsupportedScopedOperation(w, r) {
		return
	}
	name := r.PathValue("name")
	sink, ok := s.findSinkByName(name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "sink_not_found"})
		return
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	payload := []byte(`{"event":"collector.sink.test","service":"loza.collector","attributes":{"kind":"sink_test","source":"collector"}}`)
	err := sink.Sink.WriteEvent(ctx, payload, nil)
	if err == nil {
		err = sink.Sink.Flush(ctx)
	}
	result := sinkStatus(sink.Name, err == nil, "")
	result["latency_ms"] = time.Since(start).Milliseconds()
	result["tested_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	result["check"] = "writeability"
	if err != nil {
		s.sinkHealthy.Store(false)
		s.metrics.sinkWriteErrors.Add(1)
		result["last_error"] = err.Error()
		writeJSON(w, http.StatusServiceUnavailable, result)
		return
	}
	s.sinkHealthy.Store(true)
	writeJSON(w, http.StatusOK, result)
}

func (s *collectorState) findSinkByName(name string) (namedSink, bool) {
	for _, sink := range s.sinksForShutdown() {
		if sink.Name == name || sink.Sink.Name() == name {
			return sink, true
		}
	}
	return namedSink{}, false
}

func (s *collectorState) uptimeSeconds(now time.Time) int64 {
	if s.startedAt.IsZero() || now.Before(s.startedAt) {
		return 0
	}
	return int64(now.Sub(s.startedAt) / time.Second)
}

func sinkStatus(name string, healthy bool, lastErr string) map[string]any {
	status := "healthy"
	if !healthy {
		status = "down"
	}
	result := map[string]any{
		"name":          name,
		"status":        status,
		"circuit_state": "not_configured",
	}
	if lastErr != "" {
		result["last_error"] = lastErr
	}
	return result
}

type queryRequest struct {
	Engine string `json:"engine"`
	Query  string `json:"query"`
	SQL    string `json:"sql"`
	Limit  int    `json:"limit"`
}

func (s *collectorState) handleQuery(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	if isCanonicalCollectorRoute(r) {
		s.handleScopedQuery(w, r)
		return
	}
	requestID := fmt.Sprintf("q_%d", time.Now().UTC().UnixNano())
	var req queryRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_query_request", "message": err.Error()})
		return
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		query = strings.TrimSpace(req.SQL)
	}
	if !isReadOnlyQuery(query) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query_must_be_read_only"})
		return
	}
	if !isSafeQuery(query) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query_contains_blocked_operations"})
		return
	}
	if req.Limit <= 0 || req.Limit > 1000 {
		req.Limit = 1000
	}

	db := s.queryDB
	var closeDB func()
	if db == nil {
		var err error
		db, err = sql.Open(s.cfg.duckDBDriver, s.cfg.duckDBPath)
		if err != nil {
			log.Error().Err(err).Str("request_id", requestID).Msg("query db open failed")
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "query_unavailable", "request_id": requestID})
			return
		}
		closeDB = func() { _ = db.Close() }
	}
	if closeDB != nil {
		defer closeDB()
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Use a dedicated connection so SET and query run on the same connection.
	// DuckDB SET is connection-scoped; using the pool directly means the SET
	// might apply to one connection while the query runs on another.
	conn, err := db.Conn(ctx)
	if err != nil {
		log.Error().Err(err).Str("request_id", requestID).Msg("query conn acquire failed")
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "query_unavailable", "request_id": requestID})
		return
	}
	defer conn.Close()

	// Disable external access to block read_csv, read_json, etc.
	// Reject the query if this fails — proceeding without the safety guard is unsafe.
	if _, err := conn.ExecContext(ctx, "SET enable_external_access=false"); err != nil {
		log.Error().Err(err).Str("request_id", requestID).Msg("failed to disable external access in DuckDB")
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "query_safety_check_failed", "request_id": requestID})
		return
	}

	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		log.Error().Err(err).Str("request_id", requestID).Msg("query failed")
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "query_failed", "request_id": requestID})
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		log.Error().Err(err).Str("request_id", requestID).Msg("query columns failed")
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "query_failed", "request_id": requestID})
		return
	}
	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			log.Error().Err(err).Str("request_id", requestID).Msg("query scan failed")
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "query_failed", "request_id": requestID})
			return
		}
		row := make(map[string]any, len(columns))
		for i, col := range columns {
			switch v := values[i].(type) {
			case []byte:
				row[col] = string(v)
			default:
				row[col] = v
			}
		}
		result = append(result, row)
		if len(result) >= req.Limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Str("request_id", requestID).Msg("query rows iteration failed")
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "query_failed", "request_id": requestID})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"columns": columns, "rows": result, "row_count": len(result)})
}

// handleScopedQuery exposes a fixed, parameterized event-listing query. Raw
// client SQL cannot prove collector ownership and is deliberately unavailable
// on canonical collector routes.
func (s *collectorState) handleScopedQuery(w http.ResponseWriter, r *http.Request) {
	scope, ok := publichttp.AuthorizedCollectorFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "collector_scope_required"})
		return
	}
	var req queryRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_query_request", "message": err.Error()})
		return
	}
	if strings.TrimSpace(req.Query) != "" || strings.TrimSpace(req.SQL) != "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "scoped_raw_sql_unsupported"})
		return
	}
	if req.Limit <= 0 || req.Limit > 1000 {
		req.Limit = 1000
	}
	table, err := quoteSQLIdent(s.cfg.duckDBTable)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "query_unavailable"})
		return
	}
	db := s.queryDB
	var closeDB func()
	if db == nil {
		db, err = sql.Open(s.cfg.duckDBDriver, s.cfg.duckDBPath)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "query_unavailable"})
			return
		}
		closeDB = func() { _ = db.Close() }
	}
	if closeDB != nil {
		defer closeDB()
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ? AND %s = ? LIMIT ?", table, collectorOwnershipColumn, environmentOwnershipColumn)
	rows, err := db.QueryContext(ctx, query, scope.Name, scope.Environment, req.Limit)
	if err != nil {
		log.Error().Err(err).Msg("scoped query failed")
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "query_failed"})
		return
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "query_failed"})
		return
	}
	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "query_failed"})
			return
		}
		row := make(map[string]any, len(columns))
		for i, column := range columns {
			if value, isBytes := values[i].([]byte); isBytes {
				row[column] = string(value)
			} else {
				row[column] = values[i]
			}
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "query_failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"columns": columns, "rows": result, "row_count": len(result)})
}

func (s *collectorState) handleReplay(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	if rejectUnsupportedScopedOperation(w, r) {
		return
	}
	var req struct {
		Events [][]byte `json:"events"`
		Filter string   `json:"filter"`
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "read_failed", "message": err.Error()})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_replay_request", "message": err.Error()})
		return
	}
	if len(req.Events) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "empty_events", "message": "events array required"})
		return
	}
	accepted, err := s.handleIngestBatch(r.Context(), req.Events)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "replay_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": accepted, "replayed": len(req.Events)})
}

func (s *collectorState) handleDLQList(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	if rejectUnsupportedScopedOperation(w, r) {
		return
	}
	events, err := s.readDLQRecords()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"events": []any{}, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events, "count": len(events)})
}

func (s *collectorState) handleDLQShow(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	if rejectUnsupportedScopedOperation(w, r) {
		return
	}
	id := r.PathValue("id")
	events, err := s.readDLQRecords()
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	for _, event := range events {
		if fmt.Sprint(event["dlq_id"]) == id || fmt.Sprint(event["event_id"]) == id {
			writeJSON(w, http.StatusOK, event)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "dlq_event_not_found"})
}

func (s *collectorState) handleDLQReplay(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	if rejectUnsupportedScopedOperation(w, r) {
		return
	}
	id := r.PathValue("id")
	events, err := s.readDLQRecords()
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	for _, event := range events {
		if fmt.Sprint(event["dlq_id"]) == id || fmt.Sprint(event["event_id"]) == id {
			raw, ok := event["raw_event"].(string)
			if !ok {
				raw, _ = event["raw"].(string)
			}
			accepted, err := s.handleIngestBatch(r.Context(), [][]byte{[]byte(raw)})
			if err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "replay_failed", "message": err.Error()})
				return
			}
			writeJSON(w, http.StatusAccepted, map[string]any{"accepted": accepted, "replayed": 1})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "dlq_event_not_found"})
}

func (s *collectorState) handleDLQReplayAll(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	if rejectUnsupportedScopedOperation(w, r) {
		return
	}
	events, err := s.readDLQRecords()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"accepted": 0, "replayed": 0, "error": err.Error()})
		return
	}
	rawEvents := make([][]byte, 0, len(events))
	for _, event := range events {
		raw, ok := event["raw_event"].(string)
		if !ok {
			raw, _ = event["raw"].(string)
		}
		if strings.TrimSpace(raw) != "" {
			rawEvents = append(rawEvents, []byte(raw))
		}
	}
	accepted, err := s.handleIngestBatch(r.Context(), rawEvents)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "replay_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": accepted, "replayed": len(rawEvents)})
}

func (s *collectorState) handleDLQDelete(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	if rejectUnsupportedScopedOperation(w, r) {
		return
	}
	id := r.PathValue("id")
	events, err := s.readDLQRecords()
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	remaining := make([]map[string]any, 0, len(events))
	deleted := 0
	for _, event := range events {
		if fmt.Sprint(event["dlq_id"]) == id || fmt.Sprint(event["event_id"]) == id {
			deleted++
			continue
		}
		remaining = append(remaining, event)
	}
	if deleted == 0 {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "dlq_event_not_found"})
		return
	}
	if err := s.writeDLQRecords(remaining); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "dlq_delete_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
}

func (s *collectorState) readDLQRecords() ([]map[string]any, error) {
	if strings.TrimSpace(s.cfg.dlqPath) == "" {
		return nil, os.ErrNotExist
	}
	file, err := os.Open(s.cfg.dlqPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	out := []map[string]any{}
	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	i := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			record = map[string]any{"raw_event": line, "error": "dlq_record_invalid_json"}
		}
		if _, ok := record["dlq_id"]; !ok {
			record["dlq_id"] = fmt.Sprintf("dlq_%d", i)
		}
		if raw, ok := record["raw"].(string); ok {
			record["raw_event"] = raw
		}
		out = append(out, record)
		i++
	}
	return out, sc.Err()
}

func (s *collectorState) writeDLQRecords(records []map[string]any) error {
	if strings.TrimSpace(s.cfg.dlqPath) == "" {
		return os.ErrNotExist
	}
	if err := os.MkdirAll(filepath.Dir(s.cfg.dlqPath), 0o755); err != nil {
		return err
	}
	tmp := s.cfg.dlqPath + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	for _, record := range records {
		raw, err := json.Marshal(record)
		if err != nil {
			_ = file.Close()
			return err
		}
		if _, err := file.Write(append(raw, '\n')); err != nil {
			_ = file.Close()
			return err
		}
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, s.cfg.dlqPath)
}

func isReadOnlyQuery(query string) bool {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, ";") || strings.Contains(lower, "--") || strings.Contains(lower, "/*") || strings.Contains(lower, "*/") {
		return false
	}
	for _, prefix := range []string{"select", "with", "show", "describe", "pragma table_info", "pragma database_list"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// isSafeQuery performs a deeper check beyond isReadOnlyQuery, blocking dangerous
// DuckDB functions and DML operations that can be hidden inside CTEs or subqueries.
func isSafeQuery(query string) bool {
	lower := strings.ToLower(strings.TrimSpace(query))
	if strings.Contains(lower, ";") || strings.Contains(lower, "--") || strings.Contains(lower, "/*") || strings.Contains(lower, "*/") {
		return false
	}
	// Block dangerous DuckDB functions and DDL/DML keywords
	dangerous := []string{
		"chr(", "unicode(",
		"read_", "scan(", "glob(", "iceberg_scan", "delta_scan",
		"httpfs", "spatial", "sqlite", "postgres", "mysql", "s3", "http://", "https://", "file:",
		"install", "load ", "load\t", "attach", "copy ", "export",
		"call ", "prepare", "execute",
		"query(", "printf(", "format(",
		"create ", "drop ", "alter ", "truncate",
		"insert ", "update ", "delete ", "merge ",
		"grant ", "revoke",
	}
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return false
		}
	}
	return true
}

func deliveryQueueDepth(ch chan spoolDelivery) int {
	if ch == nil {
		return 0
	}
	return len(ch)
}

func statusString(ok bool) string {
	if ok {
		return "ok"
	}
	return "degraded"
}
