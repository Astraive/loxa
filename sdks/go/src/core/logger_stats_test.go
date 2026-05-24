package core

import (
	"context"
	"errors"
	"testing"
)

type statsProbe struct {
	emitCount           int
	deliveryFailedCount int
	drops               []string
	errors              []error
}

func (s *statsProbe) OnEmit(_ *Event) {
	s.emitCount++
}

func (s *statsProbe) OnDrop(reason string) {
	s.drops = append(s.drops, reason)
}

func (s *statsProbe) OnError(err error) {
	s.errors = append(s.errors, err)
}

func (s *statsProbe) OnDeliveryFailed(_ *Event, _ error) {
	s.deliveryFailedCount++
}

type failingSink struct {
	err error
}

func (s failingSink) Name() string { return "failing" }

func (s failingSink) WriteEvent(_ context.Context, _ []byte, _ *Event) error { return s.err }

func (s failingSink) Flush(_ context.Context) error { return nil }

func (s failingSink) Close(_ context.Context) error { return nil }

func TestEmitUsesFallbackSinkOnPrimaryFailure(t *testing.T) {
	fallback := &captureSink{}
	primaryErr := errors.New("primary write failed")

	cfg := Test()
	cfg.Sinks = []Sink{failingSink{err: primaryErr}}
	cfg.FallbackSink = fallback

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ctx := l.StartEvent(context.Background(), Params{Event: "fallback.test"})
	_ = l.Finish(ctx, "success")
	if err := l.Emit(ctx); !errors.Is(err, primaryErr) {
		t.Fatalf("expected primary error, got: %v", err)
	}
	if len(fallback.last) == 0 {
		t.Fatalf("expected fallback sink to receive event")
	}
}

func TestStatsHandlerReceivesEmitDropAndError(t *testing.T) {
	t.Run("drop callback", func(t *testing.T) {
		stats := &statsProbe{}
		cfg := Test()
		cfg.Sampler = SampleNone()
		cfg.StatsHandler = stats

		l, err := New(cfg)
		if err != nil {
			t.Fatalf("new logger: %v", err)
		}
		ctx := l.StartEvent(context.Background(), Params{Event: "drop.test"})
		_ = l.Finish(ctx, "success")
		if err := l.Emit(ctx); err != nil {
			t.Fatalf("emit: %v", err)
		}
		if len(stats.drops) == 0 || stats.drops[0] != "sampled_out" {
			t.Fatalf("expected sampled_out drop callback")
		}
	})

	t.Run("emit callback", func(t *testing.T) {
		stats := &statsProbe{}
		cfg := Test()
		cfg.Sinks = []Sink{NoopSink()}
		cfg.StatsHandler = stats

		l, err := New(cfg)
		if err != nil {
			t.Fatalf("new logger: %v", err)
		}
		ctx := l.StartEvent(context.Background(), Params{Event: "emit.test"})
		_ = l.Finish(ctx, "success")
		if err := l.Emit(ctx); err != nil {
			t.Fatalf("emit: %v", err)
		}
		if stats.emitCount != 1 {
			t.Fatalf("expected one emit callback, got %d", stats.emitCount)
		}
	})

	t.Run("error callback", func(t *testing.T) {
		stats := &statsProbe{}
		expectedErr := errors.New("sink failed")
		cfg := Test()
		cfg.Sinks = []Sink{failingSink{err: expectedErr}}
		cfg.StatsHandler = stats

		l, err := New(cfg)
		if err != nil {
			t.Fatalf("new logger: %v", err)
		}
		ctx := l.StartEvent(context.Background(), Params{Event: "error.test"})
		_ = l.Finish(ctx, "success")
		_ = l.Emit(ctx)
		if len(stats.errors) == 0 || !errors.Is(stats.errors[0], expectedErr) {
			t.Fatalf("expected error callback with sink failure")
		}
		if stats.emitCount != 0 {
			t.Fatalf("expected no emit callback on failed delivery, got %d", stats.emitCount)
		}
		if stats.deliveryFailedCount != 1 {
			t.Fatalf("expected one delivery failure callback, got %d", stats.deliveryFailedCount)
		}
	})
}
