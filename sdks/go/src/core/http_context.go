package core

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var strictAttrKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// RequestIDFromHTTP resolves request id from header, then active event context.
func RequestIDFromHTTP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if v := strings.TrimSpace(r.Header.Get("X-Request-ID")); v != "" {
		return v
	}
	return RequestIDFromContext(r.Context())
}

// TraceFromOTel returns trace and span ids from the current OTel span context.
func TraceFromOTel(ctx context.Context) (traceID string, spanID string) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return "", ""
	}
	return sc.TraceID().String(), sc.SpanID().String()
}

// InjectHTTPHeaders injects LOXA + trace context headers into an outbound request.
func InjectHTTPHeaders(req *http.Request) {
	if req == nil {
		return
	}
	req.Header = InjectHTTPHeaderCarrier(req.Context(), req.Header)
}

// InjectHTTPHeaderCarrier injects LOXA and trace context into headers.
func InjectHTTPHeaderCarrier(ctx context.Context, header http.Header) http.Header {
	if header == nil {
		header = make(http.Header)
	}
	if rid := RequestIDFromContext(ctx); rid != "" {
		header.Set("X-Request-ID", rid)
	}
	if tid := TraceIDFromContext(ctx); tid != "" {
		header.Set("X-Trace-ID", tid)
	}
	if sid := SpanIDFromContext(ctx); sid != "" {
		header.Set("X-Span-ID", sid)
	}
	propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(header))
	return header
}

// ExtractHTTPHeaders converts common tracing/request headers into attrs.
func ExtractHTTPHeaders(r *http.Request) []Attr {
	if r == nil {
		return nil
	}
	return ExtractHTTPHeaderAttrsWithContext(r.Context(), r.Header)
}

// ExtractHTTPHeaderAttrs converts common tracing/request headers into attrs.
func ExtractHTTPHeaderAttrs(header http.Header) []Attr {
	return ExtractHTTPHeaderAttrsWithContext(context.Background(), header)
}

// ExtractHTTPHeaderAttrsWithContext converts common tracing/request headers into attrs.
func ExtractHTTPHeaderAttrsWithContext(ctx context.Context, header http.Header) []Attr {
	if header == nil {
		return nil
	}
	attrs := make([]Attr, 0, 3)
	if rid := strings.TrimSpace(header.Get("X-Request-ID")); rid != "" {
		attrs = append(attrs, RequestID(rid))
	}
	traceID := strings.TrimSpace(header.Get("X-Trace-ID"))
	spanID := strings.TrimSpace(header.Get("X-Span-ID"))
	if traceID == "" || spanID == "" {
		otelCtx := propagation.TraceContext{}.Extract(ctx, propagation.HeaderCarrier(header))
		otelTraceID, otelSpanID := TraceFromOTel(otelCtx)
		if traceID == "" {
			traceID = otelTraceID
		}
		if spanID == "" {
			spanID = otelSpanID
		}
	}
	if traceID != "" {
		attrs = append(attrs, TraceID(traceID))
	}
	if spanID != "" {
		attrs = append(attrs, SpanID(spanID))
	}
	return attrs
}
