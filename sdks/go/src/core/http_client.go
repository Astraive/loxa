package core

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type loxaRoundTripper struct {
	base http.RoundTripper
}

// NewRoundTripper wraps a base transport and enriches active events with outbound HTTP metadata.
func NewRoundTripper(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &loxaRoundTripper{base: base}
}

// WrapHTTPClient wraps an existing client with LOXA outbound instrumentation.
func WrapHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	cp := *client
	cp.Transport = NewRoundTripper(client.Transport)
	return &cp
}

func (rt *loxaRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	ctx := req.Context()
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		req = req.Clone(ctx)
		if req.Header == nil {
			req.Header = make(http.Header)
		}
		propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(req.Header))
	}
	if HasEvent(ctx) {
		_ = Default().Checkpoint(ctx, "http.client.started",
			String("http.client.method", req.Method),
			String("http.client.url", req.URL.String()),
		)
	}
	resp, err := rt.base.RoundTrip(req)
	if HasEvent(ctx) {
		attrs := []Attr{
			Int64("http.client.duration_ms", time.Since(start).Milliseconds()),
		}
		if resp != nil {
			attrs = append(attrs, Int("http.client.status_code", resp.StatusCode))
		}
		if err != nil {
			attrs = append(attrs, String("http.client.error", err.Error()))
		}
		_ = Default().Checkpoint(ctx, "http.client.finished", attrs...)
	}
	return resp, err
}
