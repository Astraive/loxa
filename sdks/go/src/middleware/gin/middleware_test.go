package gin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	loza "github.com/astraive/loza/sdks/go"
	ginpkg "github.com/gin-gonic/gin"
)

func configureGin(t *testing.T, panicRecovery bool) *loza.MemorySinkStore {
	t.Helper()
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink).WithPanicRecovery(panicRecovery)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	t.Cleanup(func() { _ = loza.Shutdown(context.Background()) })
	return store
}

func TestMiddlewareEmitsSuccessAndErrorEvents(t *testing.T) {
	store := configureGin(t, false)
	ginpkg.SetMode(ginpkg.TestMode)
	r := ginpkg.New()
	r.Use(Middleware())
	r.GET("/ok", func(c *ginpkg.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/error", func(c *ginpkg.Context) { _ = c.Error(errors.New("handler failure")) })
	for _, path := range []string{"/ok", "/error"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
	}
	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if store.Len() != 2 {
		t.Fatalf("events = %d, want 2", store.Len())
	}
}

func TestMiddlewareRecoversPanicAndUsesConfiguredEvent(t *testing.T) {
	store := configureGin(t, true)
	r := ginpkg.New()
	r.Use(Middleware(Config{Event: "gin.custom"}))
	r.GET("/panic", func(*ginpkg.Context) { panic("boom") })
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("panic status = %d", rr.Code)
	}
	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if store.Len() != 1 {
		t.Fatalf("events = %d, want 1", store.Len())
	}
	ev := store.Events()[0]
	if ev.Event != "gin.custom" || ev.Error == nil {
		t.Fatalf("panic event = %+v", ev)
	}
	if got := (panicErr{value: "x"}).Error(); got != "panic recovered: x" {
		t.Fatalf("panicErr = %q", got)
	}
}
