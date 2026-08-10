package conformance

import (
	"context"
	"testing"
	"time"

	"github.com/astraive/loza/sdks/go"
)

func TestTestingAndConformanceHelpers(t *testing.T) {
	captured, err := loza.Capture(func() {
		ctx := loza.StartEvent(context.Background(), loza.Params{Event: "capture.event"})
		_ = loza.Enrich(ctx, loza.String("family", "testkit"))
		_ = loza.Checkpoint(ctx, "captured")
		_ = loza.Finish(ctx, "success")
		_ = loza.Emit(ctx)
	})
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if len(captured) == 0 {
		t.Fatalf("expected captured events")
	}
	loza.AssertEvent(t, captured[0], "family", "testkit")
	loza.ExpectAttr(t, captured[0], "family", "testkit")
	_ = loza.SnapshotEvent(t, captured[0])
	if loza.NewMockSink() == nil {
		t.Fatalf("expected mock sink")
	}
	if loza.NewFakeClock(time.Unix(0, 0)) == nil {
		t.Fatalf("expected fake clock")
	}
}
