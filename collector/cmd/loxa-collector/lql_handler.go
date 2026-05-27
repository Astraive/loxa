package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// LQLRequest is the body for POST /lql/query.
type LQLRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// HandleLQLQuery handles POST /lql/query — accepts LQL or SQL, returns results.
// For MVP, the Loxana client compiles LQL to SQL via WASM, so this passes through.
// When the lql binary is available server-side, it will compile LQL to SQL here.
func (s *collectorState) HandleLQLQuery(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	requestID := fmt.Sprintf("lql_%d", time.Now().UTC().UnixNano())
	var req LQLRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request", "message": err.Error()})
		return
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query_required"})
		return
	}

	// Future: detect LQL syntax and compile server-side via lql binary
	// For now, pass through as SQL (Loxana compiles LQL client-side via WASM)
	sqlQuery := query

	if !isReadOnlyQuery(sqlQuery) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query_must_be_read_only"})
		return
	}
	if !isSafeQuery(sqlQuery) {
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
			log.Error().Err(err).Str("request_id", requestID).Msg("lql query db open failed")
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

	// Disable external access to block read_csv, read_json, etc.
	// Reject the query if this fails — proceeding without the safety guard is unsafe.
	if _, err := db.ExecContext(ctx, "SET enable_external_access=false"); err != nil {
		log.Error().Err(err).Str("request_id", requestID).Msg("failed to disable external access in DuckDB")
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "query_safety_check_failed", "request_id": requestID})
		return
	}

	start := time.Now()
	rows, err := db.QueryContext(ctx, sqlQuery)
	if err != nil {
		log.Error().Err(err).Str("request_id", requestID).Msg("lql query failed")
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "query_failed", "request_id": requestID})
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "columns_error"})
		return
	}

	var result []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}
		row := make(map[string]any, len(columns))
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		result = append(result, row)
		if len(result) >= req.Limit {
			break
		}
	}

	duration := time.Since(start).Milliseconds()
	writeJSON(w, http.StatusOK, map[string]any{
		"columns":     columns,
		"rows":        result,
		"duration_ms": duration,
		"row_count":   len(result),
	})
}
