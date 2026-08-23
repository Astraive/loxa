package fiber

import (
	"context"
	"errors"
	"net/http"
	"testing"

	loza "github.com/astraive/loza/sdks/go"
	fiberpkg "github.com/gofiber/fiber/v2"
)

func configureFiber(t *testing.T, panicRecovery bool) *loza.MemorySinkStore {
	t.Helper()
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink).WithPanicRecovery(panicRecovery)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	t.Cleanup(func() { _ = loza.Shutdown(context.Background()) })
	return store
}

func TestMiddlewareEmitsSuccessAndErrorEvents(t *testing.T) {
	store := configureFiber(t, false)
	app := fiberpkg.New()
	app.Use(Middleware())
	app.Get("/ok", func(c *fiberpkg.Ctx) error { return c.SendString("ok") })
	app.Get("/error", func(*fiberpkg.Ctx) error { return errors.New("handler failure") })
	for _, path := range []string{"/ok", "/error"} {
		req, err := http.NewRequest(http.MethodGet, path, nil)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		if _, err := app.Test(req); err != nil {
			t.Fatalf("app.Test %s: %v", path, err)
		}
	}
	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if store.Len() != 2 {
		t.Fatalf("events = %d, want 2", store.Len())
	}
}

func TestMiddlewareRecoversPanicAndAddsRoute(t *testing.T) {
	store := configureFiber(t, true)
	app := fiberpkg.New()
	app.Use(Middleware(Config{Event: "fiber.custom"}))
	app.Get("/panic", func(*fiberpkg.Ctx) error { panic("boom") })
	req, err := http.NewRequest(http.MethodGet, "/panic", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("panic status = %d", resp.StatusCode)
	}
	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if store.Len() != 1 {
		t.Fatalf("events = %d, want 1", store.Len())
	}
	ev := store.Events()[0]
	if ev.Event != "fiber.custom" || ev.Error == nil {
		t.Fatalf("panic event = %+v", ev)
	}
	if got := (panicErr{value: "x"}).Error(); got != "panic recovered: x" {
		t.Fatalf("panicErr = %q", got)
	}
}
