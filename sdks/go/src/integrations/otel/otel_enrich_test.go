package otel

import (
	"context"
	"testing"

	loza "github.com/astraive/loza/sdks/go"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
)

func TestEnrichHelpersAddAllowListedTraceAndBaggageAttrs(t *testing.T) {
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	t.Cleanup(func() { _ = loza.Shutdown(context.Background()) })
	traceID := trace.TraceID{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	spanID := trace.SpanID{2, 2, 2, 2, 2, 2, 2, 2}
	span := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID})
	tenant, err := baggage.NewMember("tenant", "acme")
	if err != nil {
		t.Fatalf("baggage member: %v", err)
	}
	bg, err := baggage.New(tenant)
	if err != nil {
		t.Fatalf("baggage: %v", err)
	}
	ctx := trace.ContextWithSpanContext(baggage.ContextWithBaggage(context.Background(), bg), span)
	traceAttrs := TraceAttrs(ctx)
	if len(traceAttrs) != 2 || traceAttrs[0].Value != traceID.String() || traceAttrs[1].Value != spanID.String() {
		t.Fatalf("trace attrs = %#v", traceAttrs)
	}
	ctx = loza.StartEvent(ctx, loza.Params{Event: "otel.enrich"})
	EnrichTrace(ctx)
	EnrichBaggage(ctx, " tenant ")
	if err := loza.Finish(ctx, "success"); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := loza.Emit(ctx); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if store.Len() != 1 {
		t.Fatalf("events = %d", store.Len())
	}
	ev := store.Events()[0]
	if _, ok := ev.Get("trace_id"); !ok {
		t.Fatal("trace attr was not present")
	}
	if got, ok := ev.Get("baggage.tenant"); !ok || got != "acme" {
		t.Fatalf("baggage attr = %v, %v", got, ok)
	}
}

func TestEnrichHelpersIgnoreMissingTraceBaggageAndAllowlist(t *testing.T) {
	if attrs := BaggageAttrs(context.Background(), " "); attrs != nil {
		t.Fatalf("empty allowlist attrs = %#v", attrs)
	}
	if attrs := TraceAttrs(context.Background()); attrs != nil {
		t.Fatalf("invalid trace attrs = %#v", attrs)
	}
	EnrichTrace(nil)
	EnrichBaggage(nil, "tenant")
}
