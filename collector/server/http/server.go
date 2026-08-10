package httpserver

import (
	"net/http"
	"os"
	"strings"

	internalserver "github.com/astraive/loza/collector/internal/server"
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
	HandleValidate(http.ResponseWriter, *http.Request)
	HandleSinks(http.ResponseWriter, *http.Request)
	HandleSink(http.ResponseWriter, *http.Request)
	HandleSinkTest(http.ResponseWriter, *http.Request)
	HandleSchemaList(http.ResponseWriter, *http.Request)
	HandleSchemaDiff(http.ResponseWriter, *http.Request)
	HandleSchemaCheck(http.ResponseWriter, *http.Request)
	HandleSchemaPublish(http.ResponseWriter, *http.Request)
	HandleBlueprintPublish(http.ResponseWriter, *http.Request)
	HandleBlueprintList(http.ResponseWriter, *http.Request)
	HandleQuery(http.ResponseWriter, *http.Request)
	HandleLQLQuery(http.ResponseWriter, *http.Request)
	HandlePIIAudit(http.ResponseWriter, *http.Request)
	HandlePolicyValidate(http.ResponseWriter, *http.Request)
	HandleRetentionApply(http.ResponseWriter, *http.Request)
	HandleKeyCreate(http.ResponseWriter, *http.Request)
	HandleKeyRevoke(http.ResponseWriter, *http.Request)
	HandleKeyRotate(http.ResponseWriter, *http.Request)
	HandleDeleteEvents(http.ResponseWriter, *http.Request)
	HandleDLQList(http.ResponseWriter, *http.Request)
	HandleDLQReplayAll(http.ResponseWriter, *http.Request)
	HandleDLQShow(http.ResponseWriter, *http.Request)
	HandleDLQReplay(http.ResponseWriter, *http.Request)
	HandleDLQDelete(http.ResponseWriter, *http.Request)
	HandleTail(http.ResponseWriter, *http.Request)
	HandleReplay(http.ResponseWriter, *http.Request)
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
	if ingestPath != "" && ingestPath != "/events" && ingestPath != "/ingest" {
		route("POST", ingestPath, handlers.HandleIngest, "events:write")
	}
	route("POST", "/events", handlers.HandleIngest, "events:write")
	route("POST", "/ingest", handlers.HandleIngest, "events:write")
	route("POST", "/events/batch", handlers.HandleIngest, "events:write")
	route("POST", "/events/ndjson", handlers.HandleIngest, "events:write")
	route("POST", "/validate", handlers.HandleValidate, "schema:read")
	route("POST", "/otlp/logs", handlers.HandleOTLPLogs, "logs:write")

	// ── Public (no auth) ─────────────────────────────────────────────────
	if healthPath != "" && healthPath != "/health" && healthPath != "/healthz" {
		mux.HandleFunc("GET "+healthPath, handlers.HandleHealth)
	}
	mux.HandleFunc("GET /health", handlers.HandleHealth)
	mux.HandleFunc("GET /healthz", handlers.HandleHealth)
	if readyPath != "" && readyPath != "/ready" && readyPath != "/readyz" {
		mux.HandleFunc("GET "+readyPath, handlers.HandleReady)
	}
	mux.HandleFunc("GET /ready", handlers.HandleReady)
	mux.HandleFunc("GET /readyz", handlers.HandleReady)
	mux.HandleFunc("GET /version", handlers.HandleVersion)
	route("GET", "/status", handlers.HandleStatus, "status:read")

	// ── Read endpoints ───────────────────────────────────────────────────
	route("GET", "/sinks", handlers.HandleSinks, "events:read")
	route("GET", "/sinks/{name}", handlers.HandleSink, "events:read")
	route("POST", "/sinks/{name}/test", handlers.HandleSinkTest, "events:write")
	route("GET", "/schema", handlers.HandleSchemaList, "schema:read")
	route("POST", "/schema/check", handlers.HandleSchemaCheck, "schema:read")
	route("POST", "/schema/diff", handlers.HandleSchemaDiff, "schema:read")
	route("POST", "/query", handlers.HandleQuery, "events:read")
	route("POST", "/lql/query", handlers.HandleLQLQuery, "events:read")
	route("GET", "/schema/blueprint", handlers.HandleBlueprintList, "schema:read")

	// ── Tail (events:read) ───────────────────────────────────────────────
	route("GET", "/tail", handlers.HandleTail, "events:read")

	// ── Write endpoints ──────────────────────────────────────────────────
	route("POST", "/schema/publish", handlers.HandleSchemaPublish, "schema:write")
	route("POST", "/schema/blueprint", handlers.HandleBlueprintPublish, "schema:write")

	// ── Replay ───────────────────────────────────────────────────────────
	route("POST", "/replay", handlers.HandleReplay, "events:write")

	// ── Admin endpoints ──────────────────────────────────────────────────
	route("POST", "/audit/pii", handlers.HandlePIIAudit, "pii_audit:read")
	route("POST", "/policy/validate", handlers.HandlePolicyValidate, "schema:read")
	route("POST", "/retention/apply", handlers.HandleRetentionApply, "project:admin")
	route("POST", "/keys", handlers.HandleKeyCreate, "project:admin")
	route("POST", "/keys/{id}/revoke", handlers.HandleKeyRevoke, "project:admin")
	route("DELETE", "/keys/{id}", handlers.HandleKeyRevoke, "project:admin")
	route("POST", "/keys/{id}/rotate", handlers.HandleKeyRotate, "project:admin")
	route("DELETE", "/events", handlers.HandleDeleteEvents, "events:delete")
	route("DELETE", "/events/by-tenant/{tenant_id}", handlers.HandleDeleteEvents, "events:delete")
	route("DELETE", "/events/by-user/{user_id}", handlers.HandleDeleteEvents, "events:delete")
	route("DELETE", "/events/{event_id}", handlers.HandleDeleteEvents, "events:delete")

	// ── DLQ ──────────────────────────────────────────────────────────────
	route("GET", "/dlq", handlers.HandleDLQList, "events:read")
	route("POST", "/dlq/replay", handlers.HandleDLQReplayAll, "events:write")
	route("GET", "/dlq/{id}", handlers.HandleDLQShow, "events:read")
	route("POST", "/dlq/{id}/replay", handlers.HandleDLQReplay, "events:write")
	route("DELETE", "/dlq/{id}", handlers.HandleDLQDelete, "events:delete")

	// ── WebSocket tail ───────────────────────────────────────────────────
	if tailWebSocketHandler != nil {
		if protect != nil {
			mux.Handle("GET /ws/tail", protect(tailWebSocketHandler, "events:read"))
		} else {
			mux.Handle("GET /ws/tail", tailWebSocketHandler)
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

	if v := strings.ToLower(strings.TrimSpace(os.Getenv("LOZA_ENABLE_PPROF"))); v == "1" || v == "true" {
		registerPprof(mux)
	}
	return mux
}
