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

func BuildMux(ingestPath, healthPath, readyPath, metricsPath string, metricsEnabled bool, metricsHandler http.Handler, tailWebSocketHandler http.Handler, handlers PublicHandlerSet) *http.ServeMux {
	mux := http.NewServeMux()
	if ingestPath != "" {
		mux.HandleFunc("POST "+ingestPath, handlers.HandleIngest)
	}
	mux.HandleFunc("POST /v1/events", handlers.HandleIngest)
	mux.HandleFunc("POST /v1/events/batch", handlers.HandleIngest)
	mux.HandleFunc("POST /v1/events/ndjson", handlers.HandleIngest)
	mux.HandleFunc("POST /v1/otlp/logs", handlers.HandleOTLPLogs)
	mux.HandleFunc("POST /otlp/v1/logs", handlers.HandleOTLPLogs)
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
	mux.HandleFunc("GET /v1/sinks", handlers.HandleSinks)
	mux.HandleFunc("GET /sinks", handlers.HandleSinks)
	mux.HandleFunc("GET /v1/sinks/{name}", handlers.HandleSink)
	mux.HandleFunc("GET /v1/schema", handlers.HandleSchemaList)
	mux.HandleFunc("POST /v1/schema/diff", handlers.HandleSchemaDiff)
	mux.HandleFunc("POST /v1/schema/publish", handlers.HandleSchemaPublish)
	mux.HandleFunc("POST /v1/schema/blueprint", handlers.HandleBlueprintPublish)
	mux.HandleFunc("GET /v1/schema/blueprint", handlers.HandleBlueprintList)
	mux.HandleFunc("POST /v1/query", handlers.HandleQuery)
	mux.HandleFunc("POST /query", handlers.HandleQuery)
	mux.HandleFunc("POST /v1/audit/pii", handlers.HandlePIIAudit)
	mux.HandleFunc("DELETE /v1/events", handlers.HandleDeleteEvents)
	mux.HandleFunc("DELETE /v1/events/by-tenant/{tenant_id}", handlers.HandleDeleteEvents)
	mux.HandleFunc("DELETE /v1/events/by-user/{user_id}", handlers.HandleDeleteEvents)
	mux.HandleFunc("DELETE /v1/events/{event_id}", handlers.HandleDeleteEvents)
	mux.HandleFunc("GET /v1/dlq", handlers.HandleDLQList)
	mux.HandleFunc("GET /dlq", handlers.HandleDLQList)
	mux.HandleFunc("POST /v1/dlq/replay", handlers.HandleDLQReplayAll)
	mux.HandleFunc("GET /v1/dlq/{id}", handlers.HandleDLQShow)
	mux.HandleFunc("POST /v1/dlq/{id}/replay", handlers.HandleDLQReplay)
	mux.HandleFunc("DELETE /v1/dlq/{id}", handlers.HandleDLQDelete)
	mux.HandleFunc("GET /tail", handlers.HandleTail)
	mux.HandleFunc("GET /v1/tail", handlers.HandleTail)
	if tailWebSocketHandler != nil {
		mux.Handle("GET /ws/tail", tailWebSocketHandler)
		mux.Handle("GET /v1/ws/tail", tailWebSocketHandler)
	}
	if metricsEnabled && metricsHandler != nil {
		mux.Handle("GET "+metricsPath, metricsHandler)
	}
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("LOXA_ENABLE_PPROF"))); v == "1" || v == "true" {
		registerPprof(mux)
	}
	return mux
}
