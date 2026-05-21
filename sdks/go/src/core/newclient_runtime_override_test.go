package core

import (
	"context"
	"testing"
)

func TestNewClient_CodeCompressionOverrideWinsOverEnv(t *testing.T) {
	t.Setenv("LOXA_ENABLE_COMPRESSION", "true")

	logger, err := NewClient(
		ApplyConfig(
			Config{},
			WithCollectorURL("http://collector.example:8080"),
			WithService("checkout"),
			WithCompression(false),
		),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if logger.cfg.EnableCompression {
		t.Fatalf("expected code-level compression=false to override env=true")
	}
}

func TestNewClient_WithAsyncFalseDisablesDefaultPipeline(t *testing.T) {
	logger, err := NewClient(
		ApplyConfig(
			Config{},
			WithCollectorURL("http://collector.example:8080"),
			WithService("checkout"),
			WithAsync(false),
		),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if logger.cfg.Async.Enabled {
		t.Fatalf("expected async to be disabled")
	}
	if logger.pipeline != nil {
		t.Fatalf("expected no async pipeline when async is disabled")
	}
}

func TestNewClient_DropOversizedOverrideWinsOverDefaults(t *testing.T) {
	sink, store := MemorySink()

	logger, err := NewClient(
		Config{}.
			WithCollectorEndpoint("http://collector.example:8080").
			WithService("checkout").
			WithSink(sink).
			WithAsync(false).
			WithMaxEventBytes(1).
			WithDropOversizedEvents(false),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := logger.StartEvent(context.Background(), Params{Event: "oversized.runtime.override"})
	if err := logger.Set(ctx, String("payload", "oversized message that should still be delivered")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := logger.Finish(ctx, "success"); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if err := logger.Emit(ctx); err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	if store.Len() == 0 {
		t.Fatalf("expected oversized event to be delivered when drop_oversized_events=false")
	}
}
