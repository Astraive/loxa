package loza_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/astraive/loza/sdks/go"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type closeFailSink struct{}

func (closeFailSink) Name() string { return "close-fail" }
func (closeFailSink) WriteEvent(context.Context, []byte, *loza.Event) error {
	return nil
}
func (closeFailSink) Flush(context.Context) error { return nil }
func (closeFailSink) Close(context.Context) error { return context.DeadlineExceeded }

func TestShutdownHelpers(t *testing.T) {
	orig := loza.Default()
	l, err := loza.New(loza.Test().WithSink(closeFailSink{}))
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	loza.SetDefault(l)
	defer loza.SetDefault(orig)

	if err := loza.ShutdownTimeout(50 * time.Millisecond); err == nil {
		t.Fatalf("expected shutdown timeout error")
	}
	defer func() {
		if recover() == nil {
			t.Fatalf("expected MustShutdown panic")
		}
	}()
	loza.MustShutdown(50 * time.Millisecond)
}

func TestShutdownTimeoutRejectsNonPositive(t *testing.T) {
	if err := loza.ShutdownTimeout(0); err == nil {
		t.Fatalf("expected invalid timeout error")
	}
}

func TestStrictConfigureValidationSemantics(t *testing.T) {
	err := loza.Configure(loza.Test().WithStrict(true))
	if err == nil {
		t.Fatalf("expected strict config validation error")
	}
	if !errors.Is(err, loza.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
	var cfgErr *loza.ConfigValidationError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigValidationError, got %T", err)
	}
	if cfgErr.Field != "Service" {
		t.Fatalf("expected Service validation field, got %q", cfgErr.Field)
	}
}

func TestHTTPContextHelpers(t *testing.T) {
	sink, _ := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithService("svc").WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	ctx := loza.StartEvent(req.Context(), loza.Params{
		Event:     "http.request",
		RequestID: "req-1",
		TraceID:   "trace-1",
		SpanID:    "span-1",
	})
	req = req.WithContext(ctx)

	loza.InjectHTTPHeaders(req)
	if got := loza.RequestIDFromHTTP(req); got != "req-1" {
		t.Fatalf("request id mismatch: %q", got)
	}
	attrs := loza.ExtractHTTPHeaders(req)
	if len(attrs) < 2 {
		t.Fatalf("expected extracted attrs, got %d", len(attrs))
	}
}

func TestStartHTTPEventFromRequest(t *testing.T) {
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithService("svc").WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com/checkout", nil)
	req.Header.Set("X-Request-ID", "req-123")

	ctx := loza.StartHTTPEventFromRequest(req, loza.Params{
		Event: "checkout.request",
	})
	if err := loza.Finish(ctx, "success"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if err := loza.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if store.Len() != 1 {
		t.Fatalf("expected 1 event, got %d", store.Len())
	}

	var payload map[string]any
	if err := json.Unmarshal(store.Raw()[0], &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["method"]; got != "POST" {
		t.Fatalf("expected method POST, got %#v", got)
	}
	if got := payload["path"]; got != "/checkout" {
		t.Fatalf("expected path /checkout, got %#v", got)
	}
	if got := payload["request_id"]; got != "req-123" {
		t.Fatalf("expected request_id req-123, got %#v", got)
	}
}

func TestHTTPContextHeaderCarrierHelpers(t *testing.T) {
	sink, _ := loza.MemorySink()
	cfg := loza.ApplyConfig(
		loza.Test(),
		loza.WithService("svc"),
		loza.WithSink(sink),
		loza.WithAsyncQueue(16),
		loza.WithWorkers(2),
		loza.WithAsyncFlushInterval(10*time.Millisecond),
		loza.WithAsyncMaxBatchBytes(1024),
		loza.WithBackpressure(loza.DropNewest),
		loza.WithStrict(true),
	)
	if err := loza.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	ctx := loza.StartEvent(context.Background(), loza.Params{
		Event:     "http.request",
		RequestID: "req-2",
		TraceID:   "trace-2",
		SpanID:    "span-2",
	})
	header := loza.InjectHTTPHeaderCarrier(ctx, nil)
	if got := header.Get("X-Request-ID"); got != "req-2" {
		t.Fatalf("request id mismatch: %q", got)
	}
	if got := header.Get("X-Trace-ID"); got != "trace-2" {
		t.Fatalf("trace id mismatch: %q", got)
	}
	if got := header.Get("X-Span-ID"); got != "span-2" {
		t.Fatalf("span id mismatch: %q", got)
	}
}

func TestExtractHTTPHeaderAttrsTraceparentFallback(t *testing.T) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	header := make(http.Header)
	propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(header))

	attrs := loza.ExtractHTTPHeaderAttrs(header)
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attrs from traceparent fallback, got %d", len(attrs))
	}
}
