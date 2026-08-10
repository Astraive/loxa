package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	collectorconfig "github.com/astraive/loza/collector/internal/config"
)

type schemaEntryRequest struct {
	SchemaVersion  string   `json:"schema_version"`
	EventVersion   string   `json:"event_version"`
	RequiredFields []string `json:"required_fields"`
}

type piiFindingKey struct {
	Path           string
	Classification string
}

func (s *collectorState) handleSchemaList(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mode":            s.cfg.schemaMode,
		"schema_version":  s.cfg.schemaSchemaVersion,
		"event_version":   s.cfg.schemaEventVersion,
		"registry":        s.cfg.schemaRegistry,
		"quarantine_path": s.cfg.schemaQuarantinePath,
	})
}

func (s *collectorState) handleSchemaDiff(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	entry, ok := decodeSchemaEntryRequest(w, r)
	if !ok {
		return
	}
	current := s.findSchemaRegistryEntry(entry.SchemaVersion, entry.EventVersion)
	currentFields := []string(nil)
	if current != nil {
		currentFields = append(currentFields, current.RequiredFields...)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schema_version": entry.SchemaVersion,
		"event_version":  entry.EventVersion,
		"current":        currentFields,
		"proposed":       entry.RequiredFields,
		"added":          setDifference(entry.RequiredFields, currentFields),
		"removed":        setDifference(currentFields, entry.RequiredFields),
		"unchanged":      intersection(entry.RequiredFields, currentFields),
	})
}

func (s *collectorState) handleSchemaPublish(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	entry, ok := decodeSchemaEntryRequest(w, r)
	if !ok {
		return
	}

	updated := false
	for i := range s.cfg.schemaRegistry {
		current := &s.cfg.schemaRegistry[i]
		if current.SchemaVersion == entry.SchemaVersion && current.EventVersion == entry.EventVersion {
			current.RequiredFields = append([]string(nil), entry.RequiredFields...)
			updated = true
			break
		}
	}
	if !updated {
		s.cfg.schemaRegistry = append(s.cfg.schemaRegistry, collectorconfig.SchemaRegistryEntry{
			SchemaVersion:  entry.SchemaVersion,
			EventVersion:   entry.EventVersion,
			RequiredFields: append([]string(nil), entry.RequiredFields...),
		})
	}
	if err := saveSchemaRegistryFile(s.cfg.schemaRegistryFile, s.cfg.schemaRegistry, s.cfg.storageEncryptionKey); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "schema_registry_save_failed", "message": err.Error()})
		return
	}

	s.processorMu.Lock()
	if s.processor != nil {
		_ = s.processor.Close()
		s.processor = nil
	}
	s.processorMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"published":       true,
		"updated":         updated,
		"schema_version":  entry.SchemaVersion,
		"event_version":   entry.EventVersion,
		"required_fields": entry.RequiredFields,
	})
}

func decodeSchemaEntryRequest(w http.ResponseWriter, r *http.Request) (schemaEntryRequest, bool) {
	var entry schemaEntryRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&entry); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_schema_request", "message": err.Error()})
		return schemaEntryRequest{}, false
	}
	entry.SchemaVersion = strings.TrimSpace(entry.SchemaVersion)
	entry.EventVersion = strings.TrimSpace(entry.EventVersion)
	entry.RequiredFields = normalizeFields(entry.RequiredFields)
	if entry.SchemaVersion == "" || entry.EventVersion == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "schema_version_and_event_version_required"})
		return schemaEntryRequest{}, false
	}
	return entry, true
}

func (s *collectorState) findSchemaRegistryEntry(schemaVersion, eventVersion string) *collectorconfig.SchemaRegistryEntry {
	for i := range s.cfg.schemaRegistry {
		entry := &s.cfg.schemaRegistry[i]
		if entry.SchemaVersion == schemaVersion && entry.EventVersion == eventVersion {
			return entry
		}
	}
	return nil
}

type piiAuditRequest struct {
	Limit int `json:"limit"`
}

func (s *collectorState) handlePIIAudit(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}

	var req piiAuditRequest
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req)
	if req.Limit <= 0 || req.Limit > 5000 {
		req.Limit = 500
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	findings, scanned, err := s.auditPII(ctx, req.Limit)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "pii_audit_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"scanned":  scanned,
		"findings": findings,
		"count":    len(findings),
	})
}

func (s *collectorState) auditPII(ctx context.Context, limit int) ([]map[string]any, int, error) {
	db := s.queryDB
	var closeDB func()
	if db == nil {
		var err error
		db, err = sql.Open(s.cfg.duckDBDriver, s.cfg.duckDBPath)
		if err != nil {
			return nil, 0, err
		}
		closeDB = func() { _ = db.Close() }
	}
	if closeDB != nil {
		defer closeDB()
	}

	query := fmt.Sprintf("SELECT %s FROM %s LIMIT ?", mustQuoteSQLIdent(s.cfg.duckDBRawColumn), mustQuoteSQLIdent(s.cfg.duckDBTable))
	rows, err := db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	blocked := make(map[string]struct{}, len(s.cfg.privacyBlocklist))
	for _, key := range s.cfg.privacyBlocklist {
		blocked[normalizeSensitiveKey(key)] = struct{}{}
	}

	agg := map[piiFindingKey]int{}
	scanned := 0
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, scanned, err
		}
		scanned++
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var payload any
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			continue
		}
		walkPII(payload, "", blocked, agg)
	}
	if err := rows.Err(); err != nil {
		return nil, scanned, err
	}

	findings := make([]map[string]any, 0, len(agg))
	for key, count := range agg {
		findings = append(findings, map[string]any{
			"path":           key.Path,
			"classification": key.Classification,
			"count":          count,
		})
	}
	slices.SortFunc(findings, func(a, b map[string]any) int {
		return strings.Compare(fmt.Sprint(a["path"]), fmt.Sprint(b["path"]))
	})
	return findings, scanned, nil
}

func walkPII(value any, path string, blocked map[string]struct{}, agg map[piiFindingKey]int) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			nextPath := key
			if path != "" {
				nextPath = path + "." + key
			}
			if _, ok := blocked[normalizeSensitiveKey(key)]; ok {
				agg[piiFindingKey{Path: nextPath, Classification: "restricted"}]++
			}
			if text, ok := child.(string); ok && secretValuePattern.MatchString(text) {
				agg[piiFindingKey{Path: nextPath, Classification: "confidential"}]++
			}
			walkPII(child, nextPath, blocked, agg)
		}
	case []any:
		for i, child := range typed {
			walkPII(child, fmt.Sprintf("%s[%d]", path, i), blocked, agg)
		}
	}
}

func normalizeFields(fields []string) []string {
	if len(fields) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
	}
	slices.Sort(out)
	return out
}

func setDifference(left, right []string) []string {
	rightSet := map[string]struct{}{}
	for _, item := range right {
		rightSet[item] = struct{}{}
	}
	out := make([]string, 0, len(left))
	for _, item := range left {
		if _, ok := rightSet[item]; !ok {
			out = append(out, item)
		}
	}
	return out
}

func intersection(left, right []string) []string {
	rightSet := map[string]struct{}{}
	for _, item := range right {
		rightSet[item] = struct{}{}
	}
	out := make([]string, 0, len(left))
	for _, item := range left {
		if _, ok := rightSet[item]; ok {
			out = append(out, item)
		}
	}
	return out
}

func mustQuoteSQLIdent(value string) string {
	quoted, err := quoteSQLIdent(value)
	if err != nil {
		return `""`
	}
	return quoted
}

// Blueprint represents a user-defined schema blueprint
type Blueprint struct {
	Name    string                     `json:"name"`
	Columns map[string]BlueprintColumn `json:"columns"`
}

// BlueprintColumn defines a single column in a blueprint
type BlueprintColumn struct {
	JSONPath   string `json:"json_path"`
	DuckDBType string `json:"duckdb_type"`
}

func (s *collectorState) handleBlueprintPublish(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}

	var bp Blueprint
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&bp); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json", "message": err.Error()})
		return
	}
	if bp.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name_required"})
		return
	}
	if len(bp.Columns) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "columns_required"})
		return
	}

	// Apply blueprint columns to the database
	if err := s.applyBlueprint(bp); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "apply_failed", "message": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "accepted",
		"name":    bp.Name,
		"columns": len(bp.Columns),
	})
}

func (s *collectorState) handleBlueprintList(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}

	db := s.queryDB
	if db == nil {
		var err error
		db, err = sql.Open(s.cfg.duckDBDriver, s.cfg.duckDBPath)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "db_open_failed", "message": err.Error()})
			return
		}
		defer db.Close()
	}

	rows, err := db.Query("SELECT column_name, data_type FROM information_schema.columns WHERE table_name = ? ORDER BY ordinal_position", s.cfg.duckDBTable)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "query_failed", "message": err.Error()})
		return
	}
	defer rows.Close()

	columns := []map[string]string{}
	for rows.Next() {
		var name, dataType string
		if err := rows.Scan(&name, &dataType); err != nil {
			continue
		}
		columns = append(columns, map[string]string{"name": name, "type": dataType})
	}

	writeJSON(w, http.StatusOK, map[string]any{"columns": columns})
}

func (s *collectorState) applyBlueprint(bp Blueprint) error {
	db := s.queryDB
	if db == nil {
		var err error
		db, err = sql.Open(s.cfg.duckDBDriver, s.cfg.duckDBPath)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer db.Close()
	}

	allowedDuckDBTypes := map[string]bool{
		"TEXT": true, "VARCHAR": true, "STRING": true,
		"INTEGER": true, "INT": true, "BIGINT": true, "SMALLINT": true, "TINYINT": true, "HUGEINT": true,
		"DOUBLE": true, "FLOAT": true, "REAL": true,
		"BOOLEAN": true, "BOOL": true,
		"TIMESTAMP": true, "DATE": true, "TIME": true, "INTERVAL": true,
		"BLOB": true, "BYTEA": true,
		"JSON": true, "UUID": true, "BIT": true, "NULL": true,
		"DECIMAL": true, "NUMERIC": true,
	}

	tableIdent, err := quoteSQLIdent(s.cfg.duckDBTable)
	if err != nil {
		return fmt.Errorf("invalid table name %q: %w", s.cfg.duckDBTable, err)
	}

	for colName, colDef := range bp.Columns {
		colIdent, err := quoteSQLIdent(colName)
		if err != nil {
			return fmt.Errorf("invalid column name %q: %w", colName, err)
		}
		typ := colDef.DuckDBType
		if typ == "" {
			typ = "TEXT"
		}
		// Validate the COMPLETE DuckDB type string against the allowlist.
		// Only the base type (before parentheses) is in the allowlist, but
		// we must also validate that any parameters contain only safe chars
		// (digits, commas, whitespace) to prevent SQL injection via types
		// like "DECIMAL(10); DROP TABLE events--".
		baseType := strings.ToUpper(typ)
		params := ""
		if idx := strings.Index(baseType, "("); idx > 0 {
			baseType = baseType[:idx]
			params = typ[idx:] // keep original case for params
		}
		if !allowedDuckDBTypes[baseType] {
			return fmt.Errorf("unsupported DuckDB type %q for column %q", colDef.DuckDBType, colName)
		}
		// If there are parameters, validate they contain only safe chars
		if params != "" {
			safeParams := true
			for _, c := range params {
				if !((c >= '0' && c <= '9') || c == '(' || c == ')' || c == ',' || c == ' ') {
					safeParams = false
					break
				}
			}
			if !safeParams {
				return fmt.Errorf("invalid characters in DuckDB type parameters %q for column %q", colDef.DuckDBType, colName)
			}
		}
		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s", tableIdent, colIdent, typ)
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("add column %s: %w", colName, err)
		}
	}
	return nil
}
