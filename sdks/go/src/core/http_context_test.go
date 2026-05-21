package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestInjectAndExtractHTTPHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com", nil)
	cfg := Test().WithService("svc")
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ctx := l.StartEvent(req.Context(), Params{
		Event:     "http.request",
		RequestID: "req-1",
		TraceID:   "trace-1",
		SpanID:    "span-1",
	})
	req = req.WithContext(ctx)

	InjectHTTPHeaders(req)
	if got := req.Header.Get("X-Request-ID"); got != "req-1" {
		t.Fatalf("request id mismatch: %q", got)
	}
	if got := req.Header.Get("X-Trace-ID"); got != "trace-1" {
		t.Fatalf("trace id mismatch: %q", got)
	}
	if got := req.Header.Get("X-Span-ID"); got != "span-1" {
		t.Fatalf("span id mismatch: %q", got)
	}

	attrs := ExtractHTTPHeaders(req)
	if len(attrs) != 3 {
		t.Fatalf("expected 3 attrs, got %d", len(attrs))
	}
}

func TestInjectHTTPHeaderCarrierCreatesHeader(t *testing.T) {
	cfg := Test().WithService("svc")
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3},
		SpanID:     trace.SpanID{4, 4, 4, 4, 4, 4, 4, 4},
		TraceFlags: trace.FlagsSampled,
	})
	baseCtx := trace.ContextWithSpanContext(context.Background(), spanCtx)
	ctx := l.StartEvent(baseCtx, Params{
		Event:     "http.request",
		RequestID: "req-1",
		TraceID:   "trace-1",
		SpanID:    "span-1",
	})

	header := InjectHTTPHeaderCarrier(ctx, nil)
	if got := header.Get("X-Request-ID"); got != "req-1" {
		t.Fatalf("request id mismatch: %q", got)
	}
	if got := header.Get("X-Trace-ID"); got != "trace-1" {
		t.Fatalf("trace id mismatch: %q", got)
	}
	if got := header.Get("X-Span-ID"); got != "span-1" {
		t.Fatalf("span id mismatch: %q", got)
	}
	if got := header.Get("traceparent"); got == "" {
		t.Fatalf("expected traceparent header to be injected")
	}
}

func TestExtractHTTPHeaderAttrsTraceparentFallback(t *testing.T) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		SpanID:     trace.SpanID{2, 2, 2, 2, 2, 2, 2, 2},
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	header := make(http.Header)
	propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(header))

	attrs := ExtractHTTPHeaderAttrs(header)
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attrs from traceparent fallback, got %d", len(attrs))
	}

	gotTrace := ""
	gotSpan := ""
	for _, a := range attrs {
		if a.Key == "trace_id" {
			gotTrace = a.Value.(string)
		}
		if a.Key == "span_id" {
			gotSpan = a.Value.(string)
		}
	}
	if gotTrace == "" || gotSpan == "" {
		t.Fatalf("expected trace_id and span_id attrs, got trace=%q span=%q", gotTrace, gotSpan)
	}
}

func TestTraceFromOTel(t *testing.T) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	traceID, spanID := TraceFromOTel(ctx)
	if traceID == "" || spanID == "" {
		t.Fatalf("expected non-empty trace/span ids")
	}
}
