package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type routeTestHandlers struct {
	scope AuthorizedCollector
	seen  bool
}

func (h *routeTestHandlers) handle(w http.ResponseWriter, r *http.Request) {
	h.scope, h.seen = AuthorizedCollectorFromContext(r.Context())
	w.WriteHeader(http.StatusOK)
}

func (h *routeTestHandlers) HandleIngest(w http.ResponseWriter, r *http.Request)      { h.handle(w, r) }
func (h *routeTestHandlers) HandleOTLPLogs(w http.ResponseWriter, r *http.Request)    { h.handle(w, r) }
func (h *routeTestHandlers) HandleHealth(w http.ResponseWriter, r *http.Request)      { h.handle(w, r) }
func (h *routeTestHandlers) HandleReady(w http.ResponseWriter, r *http.Request)       { h.handle(w, r) }
func (h *routeTestHandlers) HandleVersion(w http.ResponseWriter, r *http.Request)     { h.handle(w, r) }
func (h *routeTestHandlers) HandleStatus(w http.ResponseWriter, r *http.Request)      { h.handle(w, r) }
func (h *routeTestHandlers) HandleValidate(w http.ResponseWriter, r *http.Request)    { h.handle(w, r) }
func (h *routeTestHandlers) HandleSinks(w http.ResponseWriter, r *http.Request)       { h.handle(w, r) }
func (h *routeTestHandlers) HandleSink(w http.ResponseWriter, r *http.Request)        { h.handle(w, r) }
func (h *routeTestHandlers) HandleSinkTest(w http.ResponseWriter, r *http.Request)    { h.handle(w, r) }
func (h *routeTestHandlers) HandleSchemaList(w http.ResponseWriter, r *http.Request)  { h.handle(w, r) }
func (h *routeTestHandlers) HandleSchemaDiff(w http.ResponseWriter, r *http.Request)  { h.handle(w, r) }
func (h *routeTestHandlers) HandleSchemaCheck(w http.ResponseWriter, r *http.Request) { h.handle(w, r) }
func (h *routeTestHandlers) HandleSchemaPublish(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r)
}
func (h *routeTestHandlers) HandleBlueprintPublish(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r)
}
func (h *routeTestHandlers) HandleBlueprintList(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r)
}
func (h *routeTestHandlers) HandleQuery(w http.ResponseWriter, r *http.Request)    { h.handle(w, r) }
func (h *routeTestHandlers) HandleLQLQuery(w http.ResponseWriter, r *http.Request) { h.handle(w, r) }
func (h *routeTestHandlers) HandlePIIAudit(w http.ResponseWriter, r *http.Request) { h.handle(w, r) }
func (h *routeTestHandlers) HandlePolicyValidate(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r)
}
func (h *routeTestHandlers) HandleRetentionApply(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r)
}
func (h *routeTestHandlers) HandleKeyCreate(w http.ResponseWriter, r *http.Request) { h.handle(w, r) }
func (h *routeTestHandlers) HandleKeyRevoke(w http.ResponseWriter, r *http.Request) { h.handle(w, r) }
func (h *routeTestHandlers) HandleKeyRotate(w http.ResponseWriter, r *http.Request) { h.handle(w, r) }
func (h *routeTestHandlers) HandleDeleteEvents(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r)
}
func (h *routeTestHandlers) HandleDLQList(w http.ResponseWriter, r *http.Request) { h.handle(w, r) }
func (h *routeTestHandlers) HandleDLQReplayAll(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r)
}
func (h *routeTestHandlers) HandleDLQShow(w http.ResponseWriter, r *http.Request)   { h.handle(w, r) }
func (h *routeTestHandlers) HandleDLQReplay(w http.ResponseWriter, r *http.Request) { h.handle(w, r) }
func (h *routeTestHandlers) HandleDLQDelete(w http.ResponseWriter, r *http.Request) { h.handle(w, r) }
func (h *routeTestHandlers) HandleTail(w http.ResponseWriter, r *http.Request)      { h.handle(w, r) }
func (h *routeTestHandlers) HandleReplay(w http.ResponseWriter, r *http.Request)    { h.handle(w, r) }

func TestBuildMuxCollectorScopedAuthorization(t *testing.T) {
	handlers := &routeTestHandlers{}
	protect := func(next http.Handler, _ string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer test" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	collectorProtect := func(next http.Handler, _ string, resolve CollectorResolver, mode CollectorRouteMode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if mode == CanonicalCollectorRoute && r.Header.Get("Authorization") != "Bearer test" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			collector, ok := resolve(r)
			if mode == CanonicalCollectorRoute && (!ok || collector != "orders" || r.Header.Get("X-Loza-Env") != "dev") {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithAuthorizedCollector(r.Context(), collector, "dev")))
		})
	}
	mux := BuildMux("", "/health", "/ready", "/metrics", false, nil, nil, handlers, protect, collectorProtect, "")

	request := func(path string, headers map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		for name, value := range headers {
			req.Header.Set(name, value)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}

	response := request("/collectors/orders/events", map[string]string{"Authorization": "Bearer test", "X-Loza-Env": "dev"})
	if response.Code != http.StatusOK {
		t.Fatalf("authorized scoped write status = %d, want %d", response.Code, http.StatusOK)
	}
	if !handlers.seen || handlers.scope.Name != "orders" || handlers.scope.Environment != "dev" {
		t.Fatalf("handler scope = %#v (seen %t), want authorized orders/dev", handlers.scope, handlers.seen)
	}
	if response = request("/collectors/payments/events", map[string]string{"Authorization": "Bearer test", "X-Loza-Env": "dev"}); response.Code != http.StatusForbidden {
		t.Fatalf("wrong collector status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if response = request("/collectors/orders/events", map[string]string{"Authorization": "Bearer test", "X-Loza-Env": "prod"}); response.Code != http.StatusForbidden {
		t.Fatalf("wrong environment status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if response = request("/collectors/orders/events", nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("missing canonical credentials status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if response = request("/events", map[string]string{"Authorization": "Bearer test", "X-Loza-Env": "dev"}); response.Code != http.StatusNotFound {
		t.Fatalf("unscoped write without default status = %d, want %d", response.Code, http.StatusNotFound)
	}

	health := httptest.NewRecorder()
	mux.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("public health status = %d, want %d", health.Code, http.StatusOK)
	}
}

func TestBuildMuxRoutesScopedWebSocketTail(t *testing.T) {
	handlers := &routeTestHandlers{}
	tail := http.HandlerFunc(handlers.handle)
	collectorProtect := func(next http.Handler, _ string, resolve CollectorResolver, mode CollectorRouteMode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			collector, ok := resolve(r)
			if mode != CanonicalCollectorRoute || !ok {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithAuthorizedCollector(r.Context(), collector, "prod")))
		})
	}
	mux := BuildMux("", "/health", "/ready", "/metrics", false, nil, tail, handlers, nil, collectorProtect, "")

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/collectors/orders/ws/tail", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("scoped websocket tail status = %d, want %d", response.Code, http.StatusOK)
	}
	if !handlers.seen || handlers.scope.Name != "orders" || handlers.scope.Environment != "prod" {
		t.Fatalf("handler scope = %#v (seen %t), want authorized orders/prod", handlers.scope, handlers.seen)
	}
}

func TestBuildMuxUsesLogWriteScopeForCanonicalOTLPLogs(t *testing.T) {
	handlers := &routeTestHandlers{}
	var requiredPermission string
	collectorProtect := func(next http.Handler, permission string, _ CollectorResolver, _ CollectorRouteMode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requiredPermission = permission
			next.ServeHTTP(w, r)
		})
	}
	mux := BuildMux("", "/health", "/ready", "/metrics", false, nil, nil, handlers, nil, collectorProtect, "")

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/collectors/logs/otlp/logs", nil))
	if requiredPermission != "logs:write" {
		t.Fatalf("canonical OTLP log permission = %q, want logs:write", requiredPermission)
	}
}
