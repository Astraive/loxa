package otel

import (
	"context"

	"github.com/astraive/loxa/sdks/go"
	"go.opentelemetry.io/otel/trace"
)

// TraceAttrs extracts trace context identifiers from ctx as loxa attrs.
func TraceAttrs(ctx context.Context) []loxa.Attr {
	if ctx == nil {
		return nil
	}
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return nil
	}
	return []loxa.Attr{
		loxa.TraceID(sc.TraceID().String()),
		loxa.SpanID(sc.SpanID().String()),
	}
}

// EnrichTrace extracts and appends trace attrs to an active LOXA event in ctx.
func EnrichTrace(ctx context.Context) {
	attrs := TraceAttrs(ctx)
	if len(attrs) == 0 {
		return
	}
	loxa.Enrich(ctx, attrs...)
}
