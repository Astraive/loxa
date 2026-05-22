package loxa_test

import (
	"context"
	"errors"
	"testing"

	"github.com/astraive/loxa/sdks/go"
)

func TestRunEventSuccess(t *testing.T) {
	sink, store := loxa.MemorySink()
	if err := loxa.Configure(loxa.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	err := loxa.RunEvent(context.Background(), loxa.Params{Event: "run.success"}, func(ctx context.Context) error {
		loxa.Enrich(ctx, loxa.String("k", "v"))
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
	sink, store := loxa.MemorySink()
	if err := loxa.Configure(loxa.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	want := errors.New("boom")
	err := loxa.RunEvent(context.Background(), loxa.Params{Event: "run.error"}, func(context.Context) error {
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
	sink, store := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink)
	cfg.IncludeSource = true
	cfg.PanicRecovery = true
	if err := loxa.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	err := loxa.RunEvent(context.Background(), loxa.Params{Event: "run.panic"}, func(context.Context) error {
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
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink)
	cfg.PanicRecovery = false
	if err := loxa.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatalf("expected panic to propagate when PanicRecovery=false")
		}
	}()
	_ = loxa.RunEvent(context.Background(), loxa.Params{Event: "run.panic"}, func(context.Context) error {
		panic("panic value")
	})
}
