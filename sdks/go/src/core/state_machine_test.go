package core

import (
	"context"
	"errors"
	"testing"
)

func TestEventStateMachineIdempotentEmitAndClosedFinish(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "state.machine"})
	if err := l.Finish(ctx, "success"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("second emit should be idempotent, got %v", err)
	}
	if err := l.Finish(ctx, "success"); err == nil {
		t.Fatalf("expected event closed error")
	} else {
		var closed *EventClosedError
		if !errors.As(err, &closed) {
			t.Fatalf("expected EventClosedError, got %T %v", err, err)
		}
	}
}

func TestValidationFailureDoesNotMarkEmitted(t *testing.T) {
	cfg := Test()
	cfg.DuplicateFieldPolicy = ErrorOnDuplicate
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ctx := l.StartEvent(context.Background(), Params{Event: "invalid"})
	ev, _ := FromContext(ctx)
	if err := l.Enrich(ctx, String("service", "shadow")); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if err := l.Emit(ctx); err == nil {
		t.Fatalf("expected validation error")
	}
	if ev.IsEmitted() {
		t.Fatalf("validation failure must not mark emitted")
	}
	if got := ev.State(); got != EventStateFailedValidation {
		t.Fatalf("expected failed_validation, got %s", got)
	}
}
