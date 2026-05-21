package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

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

// handleDeleteEvents deletes events based on query parameters
// Supports: by-tenant/{tenant_id}, by-user/{user_id}, by-event/{event_id}
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

	// Get path parameters
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/events"), "/")

	var deletedCount int64
	var err error
	var deletionType string

	switch {
	case strings.Contains(r.URL.Path, "/by-tenant/"):
		// DELETE /v1/events/by-tenant/{tenant_id}
		tenantID := getTenantIDFromPath(r.URL.Path)
		if tenantID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "missing_tenant_id",
			})
			return
		}
		deletedCount, err = s.deleteEventsByTenant(r.Context(), tenantID)
		deletionType = "by_tenant"

	case strings.Contains(r.URL.Path, "/by-user/"):
		// DELETE /v1/events/by-user/{user_id}
		userID := getUserIDFromPath(r.URL.Path)
		if userID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "missing_user_id",
			})
			return
		}
		deletedCount, err = s.deleteEventsByUser(r.Context(), userID)
		deletionType = "by_user"

	default:
		// DELETE /v1/events/{event_id}
		if len(pathParts) > 1 && pathParts[len(pathParts)-1] != "" {
			eventID := pathParts[len(pathParts)-1]
			deletedCount, err = s.deleteEvent(r.Context(), eventID)
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
	var req DeletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Reason != "" {
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

// deleteEventsByTenant deletes all events for a specific tenant
func (s *collectorState) deleteEventsByTenant(ctx context.Context, tenantID string) (int64, error) {
	// Build safe SQL query with parameterized input
	query := fmt.Sprintf(`DELETE FROM "%s" WHERE tenant_id = ?`, s.cfg.duckDBTable)

	result, err := s.queryDB.ExecContext(ctx, query, tenantID)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

// deleteEventsByUser deletes all events for a specific user
func (s *collectorState) deleteEventsByUser(ctx context.Context, userID string) (int64, error) {
	// Delete events by user_id field in the raw JSON or via schema projection
	query := fmt.Sprintf(`DELETE FROM "%s" WHERE user_id = ? OR raw LIKE ?`, s.cfg.duckDBTable)

	result, err := s.queryDB.ExecContext(ctx, query, userID, fmt.Sprintf("%%\"user_id\":\"%s\"%%", escapeSQL(userID)))
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

// deleteEvent deletes a specific event by ID
func (s *collectorState) deleteEvent(ctx context.Context, eventID string) (int64, error) {
	query := fmt.Sprintf(`DELETE FROM "%s" WHERE event_id = ?`, s.cfg.duckDBTable)

	result, err := s.queryDB.ExecContext(ctx, query, eventID)
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
