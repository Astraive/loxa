package testkit

import (
	"context"
	"testing"

	loza "github.com/astraive/loza/sdks/go"
)

func TestTestKitCreatesCapturingLogger(t *testing.T) {
	logger, store, err := TestKit()
	if err != nil {
		t.Fatalf("TestKit: %v", err)
	}
	ctx := logger.StartEvent(context.Background(), loza.Params{Event: "testkit.event"})
	if err := logger.Finish(ctx, "success", loza.String("key", "value")); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := logger.Emit(ctx); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if err := logger.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if store.Len() != 1 {
		t.Fatalf("captured events = %d, want 1", store.Len())
	}
	ev := store.Events()[0]
	AssertEvent(t, ev, "key", "value")
	_ = SanitizeEvent(ev)
	ResetForTest()
}

func TestCaptureRunsFunctionAndReturnsEvents(t *testing.T) {
	events, err := Capture(func() {
		ctx := loza.StartEvent(context.Background(), loza.Params{Event: "captured"})
		_ = loza.Finish(ctx, "success")
		_ = loza.Emit(ctx)
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("captured event count = %d, want 1", len(events))
	}
}
