package otel

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
)

func TestTraceAttrsNilContext(t *testing.T) {
	if attrs := TraceAttrs(nil); attrs != nil {
		t.Fatalf("expected nil attrs for nil context, got %+v", attrs)
	}
}

func TestTraceAttrsFromSpanContext(t *testing.T) {
	traceID := trace.TraceID{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	spanID := trace.SpanID{2, 2, 2, 2, 2, 2, 2, 2}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  true,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	attrs := TraceAttrs(ctx)
	if len(attrs) != 2 {
		t.Fatalf("expected 2 trace attrs, got %d", len(attrs))
	}
	if attrs[0].Key != "trace_id" || attrs[0].Value != traceID.String() {
		t.Fatalf("unexpected trace_id attr: %+v", attrs[0])
	}
	if attrs[1].Key != "span_id" || attrs[1].Value != spanID.String() {
		t.Fatalf("unexpected span_id attr: %+v", attrs[1])
	}
}

func TestBaggageAttrsHandlesNilAndTrimmedAllowlist(t *testing.T) {
	if attrs := BaggageAttrs(nil, "tenant"); attrs != nil {
		t.Fatalf("expected nil attrs for nil context, got %+v", attrs)
	}

	tenant, err := baggage.NewMember("tenant", "acme")
	if err != nil {
		t.Fatalf("new tenant member: %v", err)
	}
	role, err := baggage.NewMember("role", "admin")
	if err != nil {
		t.Fatalf("new role member: %v", err)
	}
	bg, err := baggage.New(tenant, role)
	if err != nil {
		t.Fatalf("new baggage: %v", err)
	}

	attrs := BaggageAttrs(baggage.ContextWithBaggage(context.Background(), bg), " tenant ", "role", "", "tenant")
	if len(attrs) != 2 {
		t.Fatalf("expected 2 baggage attrs, got %d", len(attrs))
	}
	found := map[string]string{}
	for _, a := range attrs {
		if v, ok := a.Value.(string); ok {
			found[a.Key] = v
		}
	}
	if found["baggage.tenant"] != "acme" {
		t.Fatalf("missing or wrong tenant baggage attr: %v", found)
	}
	if found["baggage.role"] != "admin" {
		t.Fatalf("missing or wrong role baggage attr: %v", found)
	}
}
