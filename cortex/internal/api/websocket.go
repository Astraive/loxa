package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/astraive/loza/cortex/internal/eventconv"
	"github.com/astraive/loza/cortex/internal/middleware"
	transportcontracts "github.com/astraive/loza/spec/transport/contracts"
	"github.com/gorilla/websocket"
)

// newCortexWSUpgrader creates a websocket upgrader with origin allowlist.
func newCortexWSUpgrader(allowedOrigins []string) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  16 * 1024,
		WriteBufferSize: 16 * 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // Non-browser clients (curl, SDKs)
			}
			// Check configured allowlist
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			// Allow localhost for development (exact match with port, not prefix)
			if origin == "http://localhost" || strings.HasPrefix(origin, "http://localhost:") ||
				origin == "http://127.0.0.1" || strings.HasPrefix(origin, "http://127.0.0.1:") ||
				origin == "http://[::1]" || strings.HasPrefix(origin, "http://[::1]:") {
				return true
			}
			return false
		},
	}
}

func (s *Server) WebSocketHandler() http.Handler {
	upgrader := newCortexWSUpgrader(s.config.Server.AllowedOrigins)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Limit maximum frame size to prevent memory exhaustion
		conn.SetReadLimit(1 * 1024 * 1024) // 1MB max frame

		for {
			var req transportcontracts.WebSocketRequest
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			resp, err := s.executeWebSocketAction(r.Context(), req)
			if err != nil {
				_ = conn.WriteJSON(transportcontracts.WebSocketResponse{Type: "error", Error: err.Error()})
				continue
			}
			_ = conn.WriteJSON(transportcontracts.WebSocketResponse{Type: "result", Data: resp})
		}
	})
}

func (s *Server) executeWebSocketAction(ctx context.Context, req transportcontracts.WebSocketRequest) (any, error) {
	// Write operations require writer role (defense-in-depth; HTTP middleware is primary gate)
	if req.Action == "ingest_event" || req.Action == "ingest_batch" {
		if !hasWriterRole(ctx) {
			return nil, fmt.Errorf("writer role required for ingest operations")
		}
	}

	switch req.Action {
	case "healthz":
		return map[string]any{"status": "OK", "ready": s.ready}, nil
	case "ingest_event":
		event, err := eventconv.FromRawMap(req.Event, "loza")
		if err != nil {
			return nil, err
		}
		if err := s.processor.ProcessEvent(ctx, event); err != nil {
			return nil, err
		}
		return map[string]any{"status": "accepted", "id": event.ID}, nil
	case "reconstruct":
		if req.Mode == "deep" {
			return s.recon.ReconstructDeep(ctx, req.IncidentID)
		}
		return s.recon.ReconstructFast(ctx, req.IncidentID)
	case "service_graph":
		depth := req.Depth
		if depth <= 0 {
			depth = 3
		}
		if depth > 100 {
			depth = 100
		}
		return s.graph.GetServiceGraph(ctx, req.Service, depth)
	case "incident_graph":
		depth := req.Depth
		if depth <= 0 {
			depth = 3
		}
		if depth > 100 {
			depth = 100
		}
		return s.graph.GetIncidentGraph(ctx, req.IncidentID, depth)
	case "graphql":
		query := req.Query
		vars := req.Variables
		if query == "" && vars != nil {
			if candidate, ok := vars["query"].(string); ok {
				query = candidate
			}
		}
		return s.graphql.executeQuery(ctx, query, vars)
	default:
		return nil, fmt.Errorf("unknown websocket action %q", req.Action)
	}
}

// hasWriterRole checks the auth context for writer-level permission.
func hasWriterRole(ctx context.Context) bool {
	result := middleware.GetAuthResult(ctx)
	if result == nil {
		// No auth context means auth is disabled; allow (matches HTTP middleware behavior)
		return true
	}
	roleHierarchy := map[string]int{
		"reader": 1,
		"writer": 2,
		"admin":  3,
	}
	currentLevel := roleHierarchy[result.Role]
	return currentLevel >= roleHierarchy["writer"]
}
