package echo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	loza "github.com/astraive/loza/sdks/go"
	echopkg "github.com/labstack/echo/v4"
)

func configureEcho(t *testing.T, panicRecovery bool) *loza.MemorySinkStore {
	t.Helper()
	sink, store := loza.MemorySink()
	cfg := loza.Test().WithSink(sink).WithPanicRecovery(panicRecovery)
	if err := loza.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}
	t.Cleanup(func() { _ = loza.Shutdown(context.Background()) })
	return store
}

func TestMiddlewareEmitsSuccessAndErrorEvents(t *testing.T) {
	store := configureEcho(t, false)
	e := echopkg.New()
	e.Use(Middleware())
	e.GET("/ok", func(c echopkg.Context) error { return c.String(http.StatusOK, "ok") })
	e.GET("/error", func(echopkg.Context) error { return errors.New("handler failure") })

	for _, path := range []string{"/ok", "/error"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		e.ServeHTTP(rr, req)
	}
	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if store.Len() != 2 {
		t.Fatalf("events = %d, want 2", store.Len())
	}
}

func TestMiddlewareRecoversPanicAndUsesConfiguredEvent(t *testing.T) {
	store := configureEcho(t, true)
	e := echopkg.New()
	e.Use(Middleware(Config{Event: "echo.custom"}))
	e.GET("/panic", func(echopkg.Context) error { panic("boom") })
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rr := httptest.NewRecorder()
	e.ServeHTTP(rr, req)
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
	if ev.Event != "echo.custom" || ev.Error == nil {
		t.Fatalf("panic event = %+v", ev)
	}
	if got := (panicErr{value: "x"}).Error(); got != "panic recovered: x" {
		t.Fatalf("panicErr = %q", got)
	}
}
