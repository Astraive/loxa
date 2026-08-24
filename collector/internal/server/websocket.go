package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/astraive/loza/collector/internal/auth"
	"github.com/gorilla/websocket"
)

type TailFilters struct {
	Since        time.Time
	AfterEventID string
	Service      string
	Kind         string
	TraceID      string
	IncidentID   string
	Collector    string
	Environment  string
	Limit        int
}

type TailState interface {
	TailHistory(context.Context, TailFilters) ([][]byte, error)
	TailMatches(context.Context, []byte, TailFilters) bool
	AddTailSubscriber(chan []byte)
	RemoveTailSubscriber(chan []byte)
}

// NewWebSocketUpgrader creates a websocket upgrader with origin allowlist checking.
// If allowedOrigins is empty, only localhost and non-browser clients are permitted.
func NewWebSocketUpgrader(allowedOrigins []string) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  16 * 1024,
		WriteBufferSize: 16 * 1024,
		Subprotocols:    []string{auth.WebSocketTailProtocol},
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // Non-browser clients (no Origin header)
			}
			// Check configured allowlist
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			// Allow localhost for development (exact host match, not suffix)
			if origin == "http://localhost" || strings.HasPrefix(origin, "http://localhost:") ||
				origin == "http://127.0.0.1" || strings.HasPrefix(origin, "http://127.0.0.1:") ||
				origin == "http://[::1]" || strings.HasPrefix(origin, "http://[::1]:") {
				return true
			}
			return false
		},
	}
}

var websocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  16 * 1024,
	WriteBufferSize: 16 * 1024,
	Subprotocols:    []string{auth.WebSocketTailProtocol},
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Non-browser clients (no Origin header)
		}
		// Allow localhost for development (exact host match, not suffix)
		if origin == "http://localhost" || strings.HasPrefix(origin, "http://localhost:") ||
			origin == "http://127.0.0.1" || strings.HasPrefix(origin, "http://127.0.0.1:") ||
			origin == "http://[::1]" || strings.HasPrefix(origin, "http://[::1]:") {
			return true
		}
		return false
	},
}

func ParseTailFilters(r *http.Request) (TailFilters, error) {
	filters := TailFilters{Limit: 1000}
	q := r.URL.Query()
	if raw := strings.TrimSpace(q.Get("since")); raw != "" {
		ts, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return TailFilters{}, fmt.Errorf("invalid since: %w", err)
		}
		filters.Since = ts.UTC()
	}
	filters.AfterEventID = strings.TrimSpace(q.Get("after_event_id"))
	filters.Service = strings.TrimSpace(q.Get("service"))
	filters.Kind = strings.TrimSpace(q.Get("kind"))
	filters.TraceID = strings.TrimSpace(q.Get("trace_id"))
	filters.IncidentID = strings.TrimSpace(q.Get("incident_id"))
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return TailFilters{}, fmt.Errorf("invalid limit %q", raw)
		}
		if n > 10000 {
			n = 10000
		}
		filters.Limit = n
	}
	return filters, nil
}

func NewTailWebSocketHandler(cfg HTTPConfig, state TailState) http.Handler {
	upgrader := NewWebSocketUpgrader(cfg.AllowedOrigins)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth.PrepareWebSocketRequest(r)
		if cfg.AuthEnabled {
			provided := strings.TrimSpace(r.Header.Get(cfg.AuthHeader))
			if credential := auth.WebSocketCredential(r); credential != "" {
				provided = credential
			}
			if subtle.ConstantTimeCompare([]byte(provided), []byte(cfg.AuthValue)) != 1 {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
		filters, err := ParseTailFilters(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetReadLimit(1 * 1024 * 1024) // 1MB max frame
		clientClosed := make(chan struct{})
		go func() {
			defer close(clientClosed)
			for {
				if _, _, err := conn.NextReader(); err != nil {
					return
				}
			}
		}()

		history, err := state.TailHistory(r.Context(), filters)
		if err != nil {
			_ = conn.WriteJSON(map[string]any{"error": err.Error()})
			return
		}
		for _, raw := range history {
			if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
				return
			}
		}

		ch := make(chan []byte, 128)
		state.AddTailSubscriber(ch)
		defer state.RemoveTailSubscriber(ch)
		for {
			select {
			case <-r.Context().Done():
				return
			case <-clientClosed:
				return
			case raw, ok := <-ch:
				if !ok {
					return
				}
				if !state.TailMatches(r.Context(), raw, filters) {
					continue
				}
				if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
					return
				}
			}
		}
	})
}

func RawMatchesTailFilters(raw []byte, filters TailFilters) bool {
	if filters.Collector == "" && filters.Environment == "" && filters.Service == "" &&
		filters.Kind == "" && filters.TraceID == "" && filters.IncidentID == "" {
		return true
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	if filters.Collector != "" && stringValue(payload["collector"]) != filters.Collector {
		return false
	}
	if filters.Environment != "" && stringValue(payload["environment"]) != filters.Environment {
		return false
	}
	if filters.Service != "" && stringValue(payload["service"]) != filters.Service {
		return false
	}
	if filters.Kind != "" && stringValue(payload["kind"]) != filters.Kind {
		return false
	}
	if filters.TraceID != "" && stringValue(payload["trace_id"]) != filters.TraceID {
		return false
	}
	if filters.IncidentID != "" && stringValue(payload["incident_id"]) != filters.IncidentID {
		return false
	}
	return true
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}
