package core

import (
	"context"
	"testing"
)

type contextCaptureSink struct {
	val any
}

func (s *contextCaptureSink) Name() string { return "context-capture" }

func (s *contextCaptureSink) WriteEvent(ctx context.Context, _ []byte, _ *Event) error {
	s.val = ctx.Value("ctx-key")
	return nil
}

func (s *contextCaptureSink) Flush(context.Context) error { return nil }
func (s *contextCaptureSink) Close(context.Context) error { return nil }

func TestInfoContextUsesCallerContextForSyncWrites(t *testing.T) {
	sink := &contextCaptureSink{}
	cfg := Test()
	cfg.Sinks = []Sink{sink}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := context.WithValue(context.Background(), "ctx-key", "ctx-value")
	l.InfoContext(ctx, "msg", "event")

	if sink.val != "ctx-value" {
		t.Fatalf("expected sink to receive caller context value, got %#v", sink.val)
	}
}
