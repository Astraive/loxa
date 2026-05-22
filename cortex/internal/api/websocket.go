package api

import (
	"context"
	"fmt"
	"net/http"

	transportcontracts "github.com/astraive/loxa/spec/transport/contracts"
	"github.com/astraive/loxa/loxa-cortex/internal/eventconv"
	"github.com/gorilla/websocket"
)

var cortexWSUpgrader = websocket.Upgrader{
	ReadBufferSize:  16 * 1024,
	WriteBufferSize: 16 * 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) WebSocketHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := cortexWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

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
	switch req.Action {
	case "healthz":
		return map[string]any{"status": "OK", "ready": s.ready}, nil
	case "ingest_event":
		event, err := eventconv.FromRawMap(req.Event, "loxa")
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
		return s.graph.GetServiceGraph(ctx, req.Service, depth)
	case "incident_graph":
		depth := req.Depth
		if depth <= 0 {
			depth = 3
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
