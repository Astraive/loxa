package httpserver

import (
	"net/http"
	"os"
	"strings"

	internalserver "github.com/astraive/loxa-collector/internal/server"
)

func New(cfg internalserver.HTTPConfig, state internalserver.State) *internalserver.HTTPServer {
	return internalserver.NewHTTPServer(cfg, state)
}

type PublicHandlerSet interface {
	HandleIngest(http.ResponseWriter, *http.Request)
	HandleOTLPLogs(http.ResponseWriter, *http.Request)
	HandleHealth(http.ResponseWriter, *http.Request)
	HandleReady(http.ResponseWriter, *http.Request)
	HandleVersion(http.ResponseWriter, *http.Request)
	HandleStatus(http.ResponseWriter, *http.Request)
	HandleSinks(http.ResponseWriter, *http.Request)
	HandleSink(http.ResponseWriter, *http.Request)
	HandleSchemaList(http.ResponseWriter, *http.Request)
	HandleSchemaDiff(http.ResponseWriter, *http.Request)
	HandleSchemaPublish(http.ResponseWriter, *http.Request)
	HandleBlueprintPublish(http.ResponseWriter, *http.Request)
	HandleBlueprintList(http.ResponseWriter, *http.Request)
	HandleQuery(http.ResponseWriter, *http.Request)
	HandlePIIAudit(http.ResponseWriter, *http.Request)
	HandleDeleteEvents(http.ResponseWriter, *http.Request)
	HandleDLQList(http.ResponseWriter, *http.Request)
	HandleDLQReplayAll(http.ResponseWriter, *http.Request)
	HandleDLQShow(http.ResponseWriter, *http.Request)
	HandleDLQReplay(http.ResponseWriter, *http.Request)
	HandleDLQDelete(http.ResponseWriter, *http.Request)
	HandleTail(http.ResponseWriter, *http.Request)
}

// RouteProtector wraps an http.Handler with a permission check.
// The perm string is the required permission name (e.g., "events:write").
type RouteProtector func(next http.Handler, perm string) http.Handler

// BuildMux constructs the HTTP route table. When protect is non-nil, protected
// routes are wrapped with per-route permission checks. Public routes (health,
// ready, version, status) are never wrapped.
func BuildMux(ingestPath, healthPath, readyPath, metricsPath string, metricsEnabled bool, metricsHandler http.Handler, tailWebSocketHandler http.Handler, handlers PublicHandlerSet, protect RouteProtector) *http.ServeMux {
	mux := http.NewServeMux()

	// Helper: register a route, optionally protected
	route := func(method, pattern string, h http.HandlerFunc, perm string) {
		if protect != nil && perm != "" {
			mux.Handle(method+" "+pattern, protect(h, perm))
		} else {
			mux.HandleFunc(method+" "+pattern, h)
		}
	}

	// ── Ingest (events:write) ────────────────────────────────────────────
	if ingestPath != "" {
		route("POST", ingestPath, handlers.HandleIngest, "events:write")
	}
	route("POST", "/v1/events", handlers.HandleIngest, "events:write")
	route("POST", "/v1/events/batch", handlers.HandleIngest, "events:write")
	route("POST", "/v1/events/ndjson", handlers.HandleIngest, "events:write")
	route("POST", "/v1/otlp/logs", handlers.HandleOTLPLogs, "logs:write")
	route("POST", "/otlp/v1/logs", handlers.HandleOTLPLogs, "logs:write")

	// ── Public (no auth) ─────────────────────────────────────────────────
	if healthPath != "" {
		mux.HandleFunc("GET "+healthPath, handlers.HandleHealth)
	}
	mux.HandleFunc("GET /health", handlers.HandleHealth)
	if readyPath != "" {
		mux.HandleFunc("GET "+readyPath, handlers.HandleReady)
	}
	mux.HandleFunc("GET /ready", handlers.HandleReady)
	mux.HandleFunc("GET /version", handlers.HandleVersion)
	mux.HandleFunc("GET /v1/status", handlers.HandleStatus)
	mux.HandleFunc("GET /status", handlers.HandleStatus)

	// ── Read endpoints ───────────────────────────────────────────────────
	route("GET", "/v1/sinks", handlers.HandleSinks, "events:read")
	route("GET", "/sinks", handlers.HandleSinks, "events:read")
	route("GET", "/v1/sinks/{name}", handlers.HandleSink, "events:read")
	route("GET", "/v1/schema", handlers.HandleSchemaList, "schema:read")
	route("POST", "/v1/schema/diff", handlers.HandleSchemaDiff, "schema:read")
	route("POST", "/v1/query", handlers.HandleQuery, "events:read")
	route("POST", "/query", handlers.HandleQuery, "events:read")
	route("GET", "/v1/schema/blueprint", handlers.HandleBlueprintList, "schema:read")

	// ── Tail (events:read) ───────────────────────────────────────────────
	route("GET", "/tail", handlers.HandleTail, "events:read")
	route("GET", "/v1/tail", handlers.HandleTail, "events:read")

	// ── Write endpoints ──────────────────────────────────────────────────
	route("POST", "/v1/schema/publish", handlers.HandleSchemaPublish, "schema:write")
	route("POST", "/v1/schema/blueprint", handlers.HandleBlueprintPublish, "schema:write")

	// ── Admin endpoints ──────────────────────────────────────────────────
	route("POST", "/v1/audit/pii", handlers.HandlePIIAudit, "pii_audit:read")
	route("DELETE", "/v1/events", handlers.HandleDeleteEvents, "events:delete")
	route("DELETE", "/v1/events/by-tenant/{tenant_id}", handlers.HandleDeleteEvents, "events:delete")
	route("DELETE", "/v1/events/by-user/{user_id}", handlers.HandleDeleteEvents, "events:delete")
	route("DELETE", "/v1/events/{event_id}", handlers.HandleDeleteEvents, "events:delete")

	// ── DLQ ──────────────────────────────────────────────────────────────
	route("GET", "/v1/dlq", handlers.HandleDLQList, "events:read")
	route("GET", "/dlq", handlers.HandleDLQList, "events:read")
	route("POST", "/v1/dlq/replay", handlers.HandleDLQReplayAll, "events:write")
	route("GET", "/v1/dlq/{id}", handlers.HandleDLQShow, "events:read")
	route("POST", "/v1/dlq/{id}/replay", handlers.HandleDLQReplay, "events:write")
	route("DELETE", "/v1/dlq/{id}", handlers.HandleDLQDelete, "events:delete")

	// ── WebSocket tail ───────────────────────────────────────────────────
	if tailWebSocketHandler != nil {
		if protect != nil {
			mux.Handle("GET /ws/tail", protect(tailWebSocketHandler, "events:read"))
			mux.Handle("GET /v1/ws/tail", protect(tailWebSocketHandler, "events:read"))
		} else {
			mux.Handle("GET /ws/tail", tailWebSocketHandler)
			mux.Handle("GET /v1/ws/tail", tailWebSocketHandler)
		}
	}

	// ── Metrics (protected by default) ───────────────────────────────────
	if metricsEnabled && metricsHandler != nil {
		if protect != nil {
			mux.Handle("GET "+metricsPath, protect(metricsHandler, "events:read"))
		} else {
			mux.Handle("GET "+metricsPath, metricsHandler)
		}
	}

	if v := strings.ToLower(strings.TrimSpace(os.Getenv("LOXA_ENABLE_PPROF"))); v == "1" || v == "true" {
		registerPprof(mux)
	}
	return mux
}
