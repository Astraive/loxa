package core

import (
	"context"
	"testing"
	"time"
)

type testStatsHandler struct{}

func (testStatsHandler) OnEmit(*Event)           {}
func (testStatsHandler) OnDrop(string)           {}
func (testStatsHandler) OnError(error)           {}
func testEnricher(context.Context) []Attr        { return nil }
func testFallbackSink() (Sink, *MemorySinkStore) { return MemorySink() }

func TestApplyConfigAsyncOptionHelpers(t *testing.T) {
	cfg := ApplyConfig(
		Test(),
		WithAsyncQueue(64),
		WithWorkers(3),
		WithAsyncFlushInterval(250*time.Millisecond),
		WithAsyncMaxBatchBytes(2048),
		WithBackpressure(DropNewest),
		WithStrict(true),
	)

	if !cfg.Async.Enabled {
		t.Fatalf("expected async to be enabled")
	}
	if cfg.Async.QueueSize != 64 {
		t.Fatalf("expected queue size 64, got %d", cfg.Async.QueueSize)
	}
	if cfg.Async.Workers != 3 {
		t.Fatalf("expected workers 3, got %d", cfg.Async.Workers)
	}
	if cfg.Async.FlushInterval != 250*time.Millisecond {
		t.Fatalf("expected flush interval 250ms, got %s", cfg.Async.FlushInterval)
	}
	if cfg.Async.MaxBatchBytes != 2048 {
		t.Fatalf("expected max batch bytes 2048, got %d", cfg.Async.MaxBatchBytes)
	}
	if cfg.Async.Backpressure != DropNewest {
		t.Fatalf("expected DropNewest backpressure, got %d", cfg.Async.Backpressure)
	}
	if !cfg.Strict {
		t.Fatalf("expected strict mode enabled")
	}
}

func TestApplyConfigExtendedHelpers(t *testing.T) {
	sink, _ := testFallbackSink()
	enc := JSONEncoder()
	stats := testStatsHandler{}
	cfg := ApplyConfig(
		Test(),
		WithEncoder(enc),
		WithEnricher(testEnricher),
		WithFallbackSink(sink),
		WithStatsHandler(stats),
		nil,
	)

	if cfg.Encoder == nil {
		t.Fatalf("expected encoder to be set")
	}
	if cfg.Enricher == nil {
		t.Fatalf("expected enricher to be set")
	}
	if cfg.FallbackSink == nil {
		t.Fatalf("expected fallback sink to be set")
	}
	if cfg.StatsHandler == nil {
		t.Fatalf("expected stats handler to be set")
	}
}
