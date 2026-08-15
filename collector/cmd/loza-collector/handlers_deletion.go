package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	publichttp "github.com/astraive/loza/collector/server/http"
)

// escapeLIKE escapes LIKE metacharacters to prevent wildcard injection.
func escapeLIKE(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// DeletionRequest represents a request to delete data
type DeletionRequest struct {
	Reason string `json:"reason,omitempty"`
}

// DeletionResponse represents the response to a deletion request
type DeletionResponse struct {
	Deleted   int64     `json:"deleted"`
	Reason    string    `json:"reason,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func ownershipDeletePredicate(scope *publichttp.AuthorizedCollector) (string, []any) {
	if scope == nil {
		return "", nil
	}
	return ` AND collector = ? AND environment = ?`, []any{scope.Name, scope.Environment}
}

// handleDeleteEvents deletes events based on query parameters.
// Supports: /events/by-tenant/{tenant_id}, /events/by-user/{user_id}, /events/{event_id}.
func (s *collectorState) handleDeleteEvents(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		logJSON("warn", "collector_auth_failed", map[string]any{
			"path":        r.URL.Path,
			"method":      r.Method,
			"remote_addr": r.RemoteAddr,
		})
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": "unauthorized",
		})
		return
	}

	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"error": "method_not_allowed",
		})
		return
	}

	var scope *publichttp.AuthorizedCollector
	if isCanonicalCollectorRoute(r) {
		authorizedScope, ok := canonicalCollectorScope(r)
		if !ok {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "collector_scope_missing"})
			return
		}
		scope = &authorizedScope
	}

	// Verify that queryDB is available
	if s.queryDB == nil {
		logJSON("error", "deletion_unavailable", map[string]any{
			"reason": "duckdb_not_available",
		})
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "database_unavailable",
		})
		return
	}

	// Read the request body BEFORE executing the DELETE for audit logging.
	var req DeletionRequest
	bodyRead := false
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
		bodyRead = true
	}

	// Get path parameters relative to the canonical /events prefix.
	path := strings.TrimPrefix(r.URL.Path, "/events")
	pathParts := strings.Split(path, "/")

	var deletedCount int64
	var err error
	var deletionType string

	switch {
	case strings.Contains(r.URL.Path, "/by-tenant/"):
		// DELETE /events/by-tenant/{tenant_id}
		tenantID := getTenantIDFromPath(r.URL.Path)
		if tenantID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "missing_tenant_id",
			})
			return
		}
		deletedCount, err = s.deleteEventsByTenant(r.Context(), tenantID, scope)
		deletionType = "by_tenant"

	case strings.Contains(r.URL.Path, "/by-user/"):
		// DELETE /events/by-user/{user_id}
		userID := getUserIDFromPath(r.URL.Path)
		if userID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "missing_user_id",
			})
			return
		}
		deletedCount, err = s.deleteEventsByUser(r.Context(), userID, scope)
		deletionType = "by_user"

	default:
		// DELETE /events/{event_id}
		if len(pathParts) > 1 && pathParts[len(pathParts)-1] != "" {
			eventID := pathParts[len(pathParts)-1]
			deletedCount, err = s.deleteEvent(r.Context(), eventID, scope)
			deletionType = "by_event_id"
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid_path",
			})
			return
		}
	}

	if err != nil {
		logJSON("error", "deletion_failed", map[string]any{
			"type":  deletionType,
			"error": err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "deletion_failed",
		})
		return
	}

	// Log the deletion for audit trail
	if bodyRead && req.Reason != "" {
		logJSON("info", "deletion_executed", map[string]any{
			"type":    deletionType,
			"deleted": deletedCount,
			"reason":  req.Reason,
		})
	} else {
		logJSON("info", "deletion_executed", map[string]any{
			"type":    deletionType,
			"deleted": deletedCount,
		})
	}

	writeJSON(w, http.StatusOK, DeletionResponse{
		Deleted:   deletedCount,
		Timestamp: time.Now().UTC(),
	})
}

// deleteEventsByTenant deletes all events for a specific tenant.
func (s *collectorState) deleteEventsByTenant(ctx context.Context, tenantID string, scope *publichttp.AuthorizedCollector) (int64, error) {
	tableIdent, err := quoteSQLIdent(s.cfg.duckDBTable)
	if err != nil {
		return 0, err
	}
	predicate, ownershipArgs := ownershipDeletePredicate(scope)
	query := fmt.Sprintf(`DELETE FROM %s WHERE tenant_id = ?%s`, tableIdent, predicate)
	args := append([]any{tenantID}, ownershipArgs...)

	result, err := s.queryDB.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

// deleteEventsByUser deletes all events for a specific user.
func (s *collectorState) deleteEventsByUser(ctx context.Context, userID string, scope *publichttp.AuthorizedCollector) (int64, error) {
	tableIdent, err := quoteSQLIdent(s.cfg.duckDBTable)
	if err != nil {
		return 0, err
	}
	rawIdent, err := quoteSQLIdent(s.cfg.duckDBRawColumn)
	if err != nil {
		rawIdent, err = quoteSQLIdent("raw")
		if err != nil {
			return 0, err
		}
	}
	predicate, ownershipArgs := ownershipDeletePredicate(scope)
	query := fmt.Sprintf(`DELETE FROM %s WHERE (user_id = ? OR %s LIKE ? ESCAPE '\')%s`, tableIdent, rawIdent, predicate)
	args := append([]any{userID, fmt.Sprintf("%%\"user_id\":\"%s\"%%", escapeLIKE(userID))}, ownershipArgs...)

	result, err := s.queryDB.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

// deleteEvent deletes a specific event by ID.
func (s *collectorState) deleteEvent(ctx context.Context, eventID string, scope *publichttp.AuthorizedCollector) (int64, error) {
	tableIdent, err := quoteSQLIdent(s.cfg.duckDBTable)
	if err != nil {
		return 0, err
	}
	predicate, ownershipArgs := ownershipDeletePredicate(scope)
	query := fmt.Sprintf(`DELETE FROM %s WHERE event_id = ?%s`, tableIdent, predicate)
	args := append([]any{eventID}, ownershipArgs...)

	result, err := s.queryDB.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

// getTenantIDFromPath extracts the tenant ID from the URL path
func getTenantIDFromPath(path string) string {
	parts := strings.Split(path, "/by-tenant/")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

// getUserIDFromPath extracts the user ID from the URL path
func getUserIDFromPath(path string) string {
	parts := strings.Split(path, "/by-user/")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

// escapeSQL escapes SQL string values
func escapeSQL(s string) string {
	return strings.ReplaceAll(s, `'`, `''`)
}
