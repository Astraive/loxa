package loza_test

import (
	"context"
	"errors"
	"testing"

	"github.com/astraive/loza/sdks/go"
)

func TestRunEventSuccess(t *testing.T) {
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	err := loza.RunEvent(context.Background(), loza.Params{Event: "run.success"}, func(ctx context.Context) error {
		_ = loza.Enrich(ctx, loza.String("k", "v"))
		return nil
	})
	if err != nil {
		t.Fatalf("run event: %v", err)
	}
	if store.Len() != 1 {
		t.Fatalf("expected 1 event, got %d", store.Len())
	}
	ev := store.Events()[0]
	if ev.Outcome != "success" {
		t.Fatalf("expected success outcome, got %q", ev.Outcome)
	}
}

func TestRunEventError(t *testing.T) {
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	want := errors.New("boom")
	err := loza.RunEvent(context.Background(), loza.Params{Event: "run.error"}, func(context.Context) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
	if store.Len() != 1 {
		t.Fatalf("expected 1 event, got %d", store.Len())
	}
	ev := store.Events()[0]
	if ev.Outcome != "error" {
		t.Fatalf("expected error outcome, got %q", ev.Outcome)
	}
	if ev.Error == nil {
		t.Fatalf("expected structured error")
	}
}

func TestRunEventPanicRecovered(t *testing.T) {
	sink, store := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	cfg.IncludeSource = true
	cfg.PanicRecovery = true
	if err := loza.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	err := loza.RunEvent(context.Background(), loza.Params{Event: "run.panic"}, func(context.Context) error {
		panic("panic value")
	})
	if err == nil {
		t.Fatalf("expected panic to be returned as error")
	}
	if store.Len() != 1 {
		t.Fatalf("expected 1 event, got %d", store.Len())
	}
	ev := store.Events()[0]
	if ev.Error == nil || ev.Error.Stack == "" {
		t.Fatalf("expected panic stack in event error")
	}
}

func TestRunEventPanicRecoveryDisabledRepanics(t *testing.T) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	cfg.PanicRecovery = false
	if err := loza.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatalf("expected panic to propagate when PanicRecovery=false")
		}
	}()
	_ = loza.RunEvent(context.Background(), loza.Params{Event: "run.panic"}, func(context.Context) error {
		panic("panic value")
	})
}
