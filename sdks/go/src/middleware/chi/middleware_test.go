package chi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astraive/loza/sdks/go"
	chipkg "github.com/go-chi/chi/v5"
)

func TestMiddlewareExtractsChiRoutePattern(t *testing.T) {
	store := configureMemoryStore(t)

	r := chipkg.NewRouter()
	r.Use(Middleware(Config{}))
	r.Get("/users/{id}", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	ev := singleEvent(t, store)
	route, ok := ev.Get("route")
	if !ok || route != "/users/{id}" {
		t.Fatalf("expected chi route pattern, got %v", route)
	}
}

func TestMiddlewareUsesFallbackRouteExtractorWhenChiPatternUnavailable(t *testing.T) {
	store := configureMemoryStore(t)

	h := MiddlewareWithConfig(Config{
		RouteExtractor: func(*http.Request) string { return "/fallback" },
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	ev := singleEvent(t, store)
	route, ok := ev.Get("route")
	if !ok || route != "/fallback" {
		t.Fatalf("expected fallback route extractor to be used, got %v", route)
	}
}

func configureMemoryStore(t *testing.T) *loza.MemorySinkStore {
	t.Helper()
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	return store
}

func singleEvent(t *testing.T, store *loza.MemorySinkStore) *loza.Event {
	t.Helper()
	if store.Len() != 1 {
		t.Fatalf("expected 1 event, got %d", store.Len())
	}
	return store.Events()[0]
}
