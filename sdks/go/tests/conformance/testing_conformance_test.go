package conformance

import (
	"context"
	"testing"
	"time"

	"github.com/astraive/loxa/sdks/go"
)

func TestTestingAndConformanceHelpers(t *testing.T) {
	captured, err := loxa.Capture(func() {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "capture.event"})
		_ = loxa.Enrich(ctx, loxa.String("family", "testkit"))
		_ = loxa.Checkpoint(ctx, "captured")
		_ = loxa.Finish(ctx, "success")
		_ = loxa.Emit(ctx)
	})
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if len(captured) == 0 {
		t.Fatalf("expected captured events")
	}
	loxa.AssertEvent(t, captured[0], "family", "testkit")
	loxa.ExpectAttr(t, captured[0], "family", "testkit")
	_ = loxa.SnapshotEvent(t, captured[0])
	if loxa.NewMockSink() == nil {
		t.Fatalf("expected mock sink")
	}
	if loxa.NewFakeClock(time.Unix(0, 0)) == nil {
		t.Fatalf("expected fake clock")
	}
}
