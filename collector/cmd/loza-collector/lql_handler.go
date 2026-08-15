package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	publichttp "github.com/astraive/loza/collector/server/http"
	"github.com/rs/zerolog/log"
)

// QueryValue is a typed value supplied to an LQL compiler. The compiler owns
// conversion to driver values; the Collector never interpolates these values
// into SQL.
type QueryValue struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// LQLRequest is the source-oriented body for POST /lql/query.
type LQLRequest struct {
	Query      string                `json:"query"`
	Parameters map[string]QueryValue `json:"parameters,omitempty"`
	Limit      int                   `json:"limit,omitempty"`
}

// LQLScope is server-owned authorization context. It is never read from the
// request body and is passed to the compiler for mandatory ownership filters.
type LQLScope struct {
	Collector   string `json:"collector"`
	Environment string `json:"environment"`
}

// LQLCompileRequest is the only input accepted by the Collector compiler
// boundary. Callers cannot provide a target table or SQL.
type LQLCompileRequest struct {
	Source     string                `json:"source"`
	Parameters map[string]QueryValue `json:"parameters,omitempty"`
	Limit      int                   `json:"limit"`
	Target     string                `json:"target"`
	Scope      LQLScope              `json:"scope"`
}

// ParameterizedPlan is the compiler output executed by DuckDB.
type ParameterizedPlan struct {
	SQL          string      `json:"sql"`
	Args         []any       `json:"-"`
	OutputSchema []LQLColumn `json:"output_schema,omitempty"`
}

type LQLColumn struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	FieldType string `json:"field_type,omitempty"`
	Nullable  bool   `json:"nullable"`
}

// LQLCompiler is small so tests can inject a fake while the production
// process adapter is developed independently.
type LQLCompiler interface {
	Compile(context.Context, LQLCompileRequest) (ParameterizedPlan, error)
	Close(context.Context) error
}

// LQLDiagnostic is a structured compiler diagnostic safe to return to clients.
type LQLDiagnostic struct {
	Code        string            `json:"code"`
	Severity    string            `json:"severity,omitempty"`
	Message     string            `json:"message"`
	PrimarySpan json.RawMessage   `json:"primary_span,omitempty"`
	Labels      []json.RawMessage `json:"labels,omitempty"`
}

// LQLCompileError distinguishes source diagnostics from compiler outages.
type LQLCompileError struct {
	Diagnostics []LQLDiagnostic
	Unavailable bool
	Err         error
}

func (e *LQLCompileError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	if len(e.Diagnostics) > 0 {
		return e.Diagnostics[0].Message
	}
	return "lql compiler failed"
}

func (e *LQLCompileError) Unwrap() error { return e.Err }

func NewLQLCompileError(diagnostics ...LQLDiagnostic) error {
	return &LQLCompileError{Diagnostics: diagnostics}
}

func NewLQLCompilerUnavailable(err error) error {
	return &LQLCompileError{Unavailable: true, Err: err}
}

// HandleLQLQuery handles source-only POST /lql/query. It never executes the
// caller's source as SQL; only a plan returned by LQLCompiler reaches DuckDB.
func (s *collectorState) HandleLQLQuery(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	scope, ok := publichttp.AuthorizedCollectorFromContext(r.Context())
	if !ok || strings.TrimSpace(scope.Name) == "" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "collector_scope_required"})
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
	if req.Limit <= 0 || req.Limit > 1000 {
		req.Limit = 1000
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if s.lqlCompiler == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "lql_compiler_unavailable"})
		return
	}
	plan, err := s.lqlCompiler.Compile(ctx, LQLCompileRequest{
		Source:     query,
		Parameters: req.Parameters,
		Limit:      req.Limit,
		Target:     "duckdb",
		Scope: LQLScope{
			Collector:   scope.Name,
			Environment: scope.Environment,
		},
	})
	if err != nil {
		var compileErr *LQLCompileError
		if errors.As(err, &compileErr) && !compileErr.Unavailable {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":       "lql_compile_failed",
				"diagnostics": compileErr.Diagnostics,
			})
		} else {
			log.Error().Err(err).Str("request_id", requestID).Msg("lql compiler unavailable")
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "lql_compiler_unavailable"})
		}
		return
	}
	if strings.TrimSpace(plan.SQL) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "lql_compile_failed",
			"diagnostics": []LQLDiagnostic{{
				Code:     "empty_plan",
				Severity: "error",
				Message:  "compiler returned an empty parameterized plan",
			}},
		})
		return
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

	conn, err := db.Conn(ctx)
	if err != nil {
		log.Error().Err(err).Str("request_id", requestID).Msg("lql query conn acquire failed")
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "query_unavailable", "request_id": requestID})
		return
	}
	defer conn.Close()

	// Disable external access to block read_csv, read_json, etc.
	if _, err := conn.ExecContext(ctx, "SET enable_external_access=false"); err != nil {
		log.Error().Err(err).Str("request_id", requestID).Msg("failed to disable external access in DuckDB")
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "query_safety_check_failed", "request_id": requestID})
		return
	}

	start := time.Now()
	rows, err := conn.QueryContext(ctx, plan.SQL, plan.Args...)
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
	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			log.Error().Err(err).Str("request_id", requestID).Msg("lql query scan failed")
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "query_failed", "request_id": requestID})
			return
		}
		row := make(map[string]any, len(columns))
		for i, col := range columns {
			if b, ok := values[i].([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = values[i]
			}
		}
		result = append(result, row)
		if len(result) >= req.Limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Str("request_id", requestID).Msg("lql query rows iteration failed")
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "query_failed", "request_id": requestID})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"columns":     columns,
		"rows":        result,
		"duration_ms": time.Since(start).Milliseconds(),
		"row_count":   len(result),
	})
}
