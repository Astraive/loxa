package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/astraive/loza/collector/internal/database"
	publichttp "github.com/astraive/loza/collector/server/http"
)

type databaseConnectionResponse struct {
	database.Metadata
	Health      string `json:"health"`
	LastTestAt  string `json:"last_test_at,omitempty"`
	LastTestErr string `json:"last_test_error,omitempty"`
}

type databaseQueryRequest struct {
	Connection string                `json:"connection"`
	Query      string                `json:"query"`
	Parameters map[string]QueryValue `json:"parameters,omitempty"`
	Limit      int                   `json:"limit,omitempty"`
}

func (s *collectorState) databaseConnection(name string) (database.Connection, time.Duration, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		if s.cfg.storageConnection != "" {
			name = s.cfg.storageConnection
		} else if s.queryConnection != nil {
			return s.queryConnection, 10 * time.Second, nil
		}
	}
	connection := s.databaseConnections[name]
	if connection == nil {
		return nil, 0, fmt.Errorf("database connection %q not found", name)
	}
	for _, configured := range s.cfg.databaseConnections {
		if configured.name == name {
			return connection, configured.queryTimeout, nil
		}
	}
	return connection, 10 * time.Second, nil
}

func (s *collectorState) databaseScope(w http.ResponseWriter, r *http.Request) bool {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return false
	}
	if scope, ok := publichttp.AuthorizedCollectorFromContext(r.Context()); !ok || strings.TrimSpace(scope.Name) == "" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "collector_scope_required"})
		return false
	}
	return true
}

func (s *collectorState) HandleDatabaseConnections(w http.ResponseWriter, r *http.Request) {
	if !s.databaseScope(w, r) {
		return
	}
	result := make([]databaseConnectionResponse, 0, len(s.databaseMetadata)+1)
	for _, info := range s.databaseMetadata {
		entry := databaseConnectionResponse{Metadata: info, Health: "unknown"}
		ctx, cancel := database.BoundedContext(r.Context(), 2*time.Second)
		if connection := s.databaseConnections[info.Name]; connection != nil {
			if err := connection.Ping(ctx); err == nil {
				entry.Health = "healthy"
			} else {
				entry.Health = "unhealthy"
				entry.LastTestErr = "connection health check failed"
			}
		}
		cancel()
		result = append(result, entry)
	}
	if len(result) == 0 && s.queryConnection != nil {
		info := s.queryConnection.Metadata()
		info.Name = "primary"
		info.Primary = true
		result = append(result, databaseConnectionResponse{Metadata: info, Health: "healthy"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"connections": result})
}

func (s *collectorState) HandleDatabaseConnectionTest(w http.ResponseWriter, r *http.Request) {
	if !s.databaseScope(w, r) {
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	connection, timeout, err := s.databaseConnection(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "connection_not_found"})
		return
	}
	effectiveName := connection.Metadata().Name
	if effectiveName == "" {
		effectiveName = name
	}
	ctx, cancel := database.BoundedContext(r.Context(), timeout)
	defer cancel()
	start := time.Now()
	err = connection.Ping(ctx)
	if err == nil {
		_, err = connection.Query(ctx, "SELECT 1")
	}
	response := map[string]any{
		"connection":  effectiveName,
		"backend":     connection.Backend(),
		"healthy":     err == nil,
		"duration_ms": time.Since(start).Milliseconds(),
	}
	if err != nil {
		response["error"] = "connection_test_failed"
		writeJSON(w, http.StatusServiceUnavailable, response)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *collectorState) HandleDatabaseQuery(w http.ResponseWriter, r *http.Request) {
	if !s.databaseScope(w, r) {
		return
	}
	var req databaseQueryRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
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
	connection, timeout, err := s.databaseConnection(req.Connection)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "connection_not_found"})
		return
	}
	effectiveName := connection.Metadata().Name
	if effectiveName == "" {
		effectiveName = req.Connection
	}
	if s.lqlCompiler == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "lql_compiler_unavailable"})
		return
	}
	scope, _ := publichttp.AuthorizedCollectorFromContext(r.Context())
	ctx, cancel := database.BoundedContext(r.Context(), timeout)
	defer cancel()
	plan, err := s.lqlCompiler.Compile(ctx, LQLCompileRequest{
		Source: query, Parameters: req.Parameters, Limit: req.Limit,
		Target: connection.Backend(), Scope: LQLScope{Collector: scope.Name, Environment: scope.Environment},
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "lql_compile_failed"})
		return
	}
	start := time.Now()
	result, err := connection.Query(ctx, plan.SQL, plan.Args...)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "query_failed"})
		return
	}
	if len(result.Rows) > req.Limit {
		result.Rows = result.Rows[:req.Limit]
	}
	rows := make([]map[string]any, 0, len(result.Rows))
	for _, values := range result.Rows {
		row := make(map[string]any, len(result.Columns))
		for i, column := range result.Columns {
			if i < len(values) {
				row[column] = values[i]
			}
		}
		rows = append(rows, row)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"connection": effectiveName, "backend": connection.Backend(),
		"columns": result.Columns, "rows": rows, "row_count": len(rows),
		"duration_ms": time.Since(start).Milliseconds(),
	})
}
