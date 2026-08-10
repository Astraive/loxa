package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/astraive/loza/sdks/go"
)

func TestProcessGroupTimerAndStopwatchHelpers(t *testing.T) {
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink).WithAsync(false)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	ctx := loza.StartEvent(context.Background(), loza.Params{Event: "helper.lifecycle"})

	p1, err := loza.StartProcess(ctx, "validate")
	if err != nil {
		t.Fatalf("start process: %v", err)
	}
	if err := loza.FinishProcess(p1, loza.String("phase", "ok")); err != nil {
		t.Fatalf("finish process: %v", err)
	}
	p2, err := loza.Process(ctx, "charge")
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if err := loza.FinishProcessError(p2, errors.New("charge failed"), 500, loza.String("gateway", "stripe")); err != nil {
		t.Fatalf("finish process error: %v", err)
	}
	if err := loza.Step(ctx, "step.wrap", func() error { return nil }); err != nil {
		t.Fatalf("step: %v", err)
	}

	g1, err := loza.StartGroup(ctx, "checkout")
	if err != nil {
		t.Fatalf("start group: %v", err)
	}
	if err := loza.FinishGroup(g1, loza.String("result", "ok")); err != nil {
		t.Fatalf("finish group: %v", err)
	}
	g2, err := loza.StartGroup(ctx, "shipment")
	if err != nil {
		t.Fatalf("start group: %v", err)
	}
	if err := loza.FinishGroupError(g2, errors.New("carrier timeout"), loza.HTTPStatus(503)); err != nil {
		t.Fatalf("finish group error: %v", err)
	}
	if err := loza.Phase(ctx, "phase.wrap", func() error { return nil }); err != nil {
		t.Fatalf("phase: %v", err)
	}

	t1, err := loza.Timer(ctx, "db")
	if err != nil {
		t.Fatalf("timer: %v", err)
	}
	if err := loza.StopTimer(t1, loza.HTTPStatus(204)); err != nil {
		t.Fatalf("stop timer: %v", err)
	}
	t2, err := loza.StartTimer(ctx, "cache")
	if err != nil {
		t.Fatalf("start timer: %v", err)
	}
	if err := loza.StopTimer(t2); err != nil {
		t.Fatalf("stop timer alias: %v", err)
	}
	if err := loza.Span(ctx, "span.wrap", func() error { return nil }); err != nil {
		t.Fatalf("span: %v", err)
	}

	sw := loza.Stopwatch()
	time.Sleep(2 * time.Millisecond)
	if sw.Elapsed() <= 0 {
		t.Fatalf("expected stopwatch elapsed > 0")
	}
	measure := loza.Measure("encode", func() {})

	if err := loza.Finish(ctx, "success", measure); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if err := loza.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if store.Len() == 0 {
		t.Fatalf("expected emitted helper lifecycle event")
	}

	var lifecycle map[string]any
	for _, raw := range store.Raw() {
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if payload["event"] == "helper.lifecycle" {
			lifecycle = payload
			break
		}
	}
	if lifecycle == nil {
		t.Fatalf("helper lifecycle event not found")
	}
	if got := len(asSlice(t, lifecycle["processes"])); got < 3 {
		t.Fatalf("expected at least 3 processes, got %d", got)
	}
	if got := len(asSlice(t, lifecycle["groups"])); got < 3 {
		t.Fatalf("expected at least 3 groups, got %d", got)
	}
	if got := len(asSlice(t, lifecycle["timers"])); got < 3 {
		t.Fatalf("expected at least 3 timers, got %d", got)
	}
}

func asSlice(t *testing.T, value any) []any {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected JSON array, got %T", value)
	}
	return items
}
