package loxa_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/astraive/loxa/sdks/go"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type closeFailSink struct{}

func (closeFailSink) Name() string { return "close-fail" }
func (closeFailSink) WriteEvent(context.Context, []byte, *loxa.Event) error {
	return nil
}
func (closeFailSink) Flush(context.Context) error { return nil }
func (closeFailSink) Close(context.Context) error { return context.DeadlineExceeded }

func TestShutdownHelpers(t *testing.T) {
	orig := loxa.Default()
	l, err := loxa.New(loxa.Test().WithSink(closeFailSink{}))
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	loxa.SetDefault(l)
	defer loxa.SetDefault(orig)

	if err := loxa.ShutdownTimeout(50 * time.Millisecond); err == nil {
		t.Fatalf("expected shutdown timeout error")
	}
	defer func() {
		if recover() == nil {
			t.Fatalf("expected MustShutdown panic")
		}
	}()
	loxa.MustShutdown(50 * time.Millisecond)
}

func TestShutdownTimeoutRejectsNonPositive(t *testing.T) {
	if err := loxa.ShutdownTimeout(0); err == nil {
		t.Fatalf("expected invalid timeout error")
	}
}

func TestStrictConfigureValidationSemantics(t *testing.T) {
	err := loxa.Configure(loxa.Test().WithStrict(true))
	if err == nil {
		t.Fatalf("expected strict config validation error")
	}
	if !errors.Is(err, loxa.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
	var cfgErr *loxa.ConfigValidationError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigValidationError, got %T", err)
	}
	if cfgErr.Field != "Service" {
		t.Fatalf("expected Service validation field, got %q", cfgErr.Field)
	}
}

func TestHTTPContextHelpers(t *testing.T) {
	sink, _ := loxa.MemorySink()
	if err := loxa.Configure(loxa.Test().WithService("svc").WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	ctx := loxa.StartEvent(req.Context(), loxa.Params{
		Event:     "http.request",
		RequestID: "req-1",
		TraceID:   "trace-1",
		SpanID:    "span-1",
	})
	req = req.WithContext(ctx)

	loxa.InjectHTTPHeaders(req)
	if got := loxa.RequestIDFromHTTP(req); got != "req-1" {
		t.Fatalf("request id mismatch: %q", got)
	}
	attrs := loxa.ExtractHTTPHeaders(req)
	if len(attrs) < 2 {
		t.Fatalf("expected extracted attrs, got %d", len(attrs))
	}
}

func TestStartHTTPEventFromRequest(t *testing.T) {
	sink, store := loxa.MemorySink()
	if err := loxa.Configure(loxa.Test().WithService("svc").WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com/v1/checkout", nil)
	req.Header.Set("X-Request-ID", "req-123")

	ctx := loxa.StartHTTPEventFromRequest(req, loxa.Params{
		Event: "checkout.request",
	})
	if err := loxa.Finish(ctx, "success"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if err := loxa.Emit(ctx); err != nil {
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
	if got := payload["path"]; got != "/v1/checkout" {
		t.Fatalf("expected path /v1/checkout, got %#v", got)
	}
	if got := payload["request_id"]; got != "req-123" {
		t.Fatalf("expected request_id req-123, got %#v", got)
	}
}

func TestHTTPContextHeaderCarrierHelpers(t *testing.T) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.ApplyConfig(
		loxa.Test(),
		loxa.WithService("svc"),
		loxa.WithSink(sink),
		loxa.WithAsyncQueue(16),
		loxa.WithWorkers(2),
		loxa.WithAsyncFlushInterval(10*time.Millisecond),
		loxa.WithAsyncMaxBatchBytes(1024),
		loxa.WithBackpressure(loxa.DropNewest),
		loxa.WithStrict(true),
	)
	if err := loxa.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	ctx := loxa.StartEvent(context.Background(), loxa.Params{
		Event:     "http.request",
		RequestID: "req-2",
		TraceID:   "trace-2",
		SpanID:    "span-2",
	})
	header := loxa.InjectHTTPHeaderCarrier(ctx, nil)
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

	attrs := loxa.ExtractHTTPHeaderAttrs(header)
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attrs from traceparent fallback, got %d", len(attrs))
	}
}
