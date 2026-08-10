package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/astraive/loza/sdks/go"
)

func TestAsyncPipelineDrain(t *testing.T) {
	sink, store := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	cfg.Async.Enabled = true
	cfg.Async.QueueSize = 128
	cfg.Async.Workers = 2
	cfg.Async.FlushInterval = 10 * time.Millisecond

	if err := loza.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	for i := 0; i < 50; i++ {
		ctx := loza.StartEvent(context.Background(), loza.Params{Event: "pipeline.test"})
		loza.Finish(ctx, "success")
		if err := loza.Emit(ctx); err != nil {
			t.Fatalf("emit: %v", err)
		}
	}

	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if got := store.Len(); got < 50 {
		t.Fatalf("expected >=50 events, got %d", got)
	}
}

func TestAsyncPipelineNoLossConcurrent(t *testing.T) {
	sink, store := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	cfg.Async.Enabled = true
	cfg.Async.QueueSize = 256
	cfg.Async.Workers = 4
	cfg.Async.FlushInterval = 10 * time.Millisecond
	cfg.Async.Backpressure = loza.Block

	if err := loza.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	const goroutines = 16
	const perGoroutine = 300
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				ctx := loza.StartEvent(context.Background(), loza.Params{Event: "pipeline.concurrent"})
				loza.Enrich(ctx, loza.Int("worker", gid), loza.Int("seq", i))
				loza.Finish(ctx, "success")
				if err := loza.Emit(ctx); err != nil {
					t.Errorf("emit: %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	want := goroutines * perGoroutine
	if got := store.Len(); got != want {
		t.Fatalf("expected %d events, got %d", want, got)
	}
}

func TestConfigureDrainsPreviousLogger(t *testing.T) {
	sinkA, storeA := loza.MemorySink()
	cfgA := loza.Test().WithSink(sinkA)
	cfgA.Async.Enabled = true
	cfgA.Async.QueueSize = 256
	cfgA.Async.Workers = 2
	cfgA.Async.FlushInterval = time.Second
	cfgA.Async.Backpressure = loza.Block
	if err := loza.Configure(cfgA); err != nil {
		t.Fatalf("configure A: %v", err)
	}

	const total = 400
	for i := 0; i < total; i++ {
		ctx := loza.StartEvent(context.Background(), loza.Params{Event: "pipeline.reconfigure"})
		loza.Finish(ctx, "success")
		if err := loza.Emit(ctx); err != nil {
			t.Fatalf("emit A: %v", err)
		}
	}

	sinkB, _ := loza.MemorySink()
	cfgB := loza.Test().WithSink(sinkB)
	cfgB.Async.Enabled = true
	cfgB.Async.QueueSize = 128
	cfgB.Async.Workers = 2
	cfgB.Async.FlushInterval = 10 * time.Millisecond
	cfgB.Async.Backpressure = loza.Block
	if err := loza.Configure(cfgB); err != nil {
		t.Fatalf("configure B: %v", err)
	}

	if got := storeA.Len(); got != total {
		t.Fatalf("expected %d events from previous logger after reconfigure, got %d", total, got)
	}
}
