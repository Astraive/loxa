package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

type captureRoundTripper struct {
	req  *http.Request
	resp *http.Response
	err  error
}

func (rt *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.req = req
	if rt.resp != nil || rt.err != nil {
		return rt.resp, rt.err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("ok")),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func TestRoundTripperInjectsTraceContext(t *testing.T) {
	ctx := traceContextWithState(t, "")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	base := &captureRoundTripper{}
	rt := NewRoundTripper(base)
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	if base.req == nil {
		t.Fatalf("expected request to be forwarded")
	}
	if base.req.Header.Get("traceparent") == "" {
		t.Fatalf("expected traceparent header injection")
	}
}

func TestRoundTripperInjectsTraceState(t *testing.T) {
	ctx := traceContextWithState(t, "vendor=value")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	base := &captureRoundTripper{}
	rt := NewRoundTripper(base)
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	if base.req == nil {
		t.Fatalf("expected request to be forwarded")
	}
	if base.req.Header.Get("tracestate") != "vendor=value" {
		t.Fatalf("expected tracestate header injection")
	}
}

func TestRoundTripperCheckpointsIncludeHTTPAttrs(t *testing.T) {
	cfg := Test()
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	prev := Default()
	SetDefault(l)
	defer SetDefault(prev)

	ctx := Default().StartEvent(context.Background(), Params{Event: "http.client.test"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://example.com/api", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	base := &captureRoundTripper{
		resp: &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
			Request:    req,
		},
	}
	if _, err := NewRoundTripper(base).RoundTrip(req); err != nil {
		t.Fatalf("roundtrip: %v", err)
	}

	started := mustCheckpointByName(t, ctx, "http.client.started")
	if got, ok := attrValue(started.Attrs, "http.client.method"); !ok || got != http.MethodPost {
		t.Fatalf("expected checkpoint method attr")
	}
	if got, ok := attrValue(started.Attrs, "http.client.url"); !ok || got != "https://example.com/api" {
		t.Fatalf("expected checkpoint url attr")
	}

	finished := mustCheckpointByName(t, ctx, "http.client.finished")
	if got, ok := attrValue(finished.Attrs, "http.client.status_code"); !ok || got != http.StatusCreated {
		t.Fatalf("expected checkpoint status_code attr")
	}
	if got, ok := attrValue(finished.Attrs, "http.client.duration_ms"); !ok {
		t.Fatalf("expected checkpoint duration attr")
	} else if durationMS, ok := got.(int64); !ok || durationMS < 0 {
		t.Fatalf("expected non-negative int64 duration attr")
	}
}

func TestRoundTripperCheckpointsIncludeErrorAttr(t *testing.T) {
	cfg := Test()
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	prev := Default()
	SetDefault(l)
	defer SetDefault(prev)

	ctx := Default().StartEvent(context.Background(), Params{Event: "http.client.test"})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/fail", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	callErr := errors.New("dial failed")
	base := &captureRoundTripper{err: callErr}
	if _, err := NewRoundTripper(base).RoundTrip(req); !errors.Is(err, callErr) {
		t.Fatalf("expected roundtrip error")
	}

	finished := mustCheckpointByName(t, ctx, "http.client.finished")
	if got, ok := attrValue(finished.Attrs, "http.client.error"); !ok || got != callErr.Error() {
		t.Fatalf("expected checkpoint error attr")
	}
	if got, ok := attrValue(finished.Attrs, "http.client.duration_ms"); !ok {
		t.Fatalf("expected checkpoint duration attr")
	} else if durationMS, ok := got.(int64); !ok || durationMS < 0 {
		t.Fatalf("expected non-negative int64 duration attr")
	}
}

func traceContextWithState(t *testing.T, tracestate string) context.Context {
	t.Helper()

	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatalf("trace id: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatalf("span id: %v", err)
	}

	cfg := trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	}
	if tracestate != "" {
		state, err := trace.ParseTraceState(tracestate)
		if err != nil {
			t.Fatalf("parse tracestate: %v", err)
		}
		cfg.TraceState = state
	}
	return trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(cfg))
}

func mustCheckpointByName(t *testing.T, ctx context.Context, name string) EventCheckpoint {
	t.Helper()

	ev, ok := FromContext(ctx)
	if !ok {
		t.Fatalf("expected event in context")
	}
	ev.MuLock()
	defer ev.MuUnlock()

	for _, cp := range ev.Checkpoints {
		if cp.Name == name {
			return cp
		}
	}
	t.Fatalf("expected checkpoint %q", name)
	return EventCheckpoint{}
}

func attrValue(attrs []Attr, key string) (any, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value, true
		}
	}
	return nil, false
}
