package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
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

	if req.Limit <= 0 || req.Limit > 10000 {
		req.Limit = 1000
	}

	db := s.queryDB
	var closeDB func()
	if db == nil {
		var err error
		db, err = sql.Open(s.cfg.duckDBDriver, s.cfg.duckDBPath)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "query_db_open_failed", "message": err.Error()})
			return
		}
		closeDB = func() { _ = db.Close() }
	}
	if closeDB != nil {
		defer closeDB()
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	start := time.Now()
	rows, err := db.QueryContext(ctx, sqlQuery)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "query_error", "message": err.Error(), "sql": sqlQuery})
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
		"sql":         sqlQuery,
		"duration_ms": duration,
		"row_count":   len(result),
	})
}
