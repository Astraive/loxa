package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/astraive/loxa/sdks/go"
)

func TestAsyncPipelineDrain(t *testing.T) {
	sink, store := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink)
	cfg.Async.Enabled = true
	cfg.Async.QueueSize = 128
	cfg.Async.Workers = 2
	cfg.Async.FlushInterval = 10 * time.Millisecond

	if err := loxa.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	for i := 0; i < 50; i++ {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "pipeline.test"})
		loxa.Finish(ctx, "success")
		if err := loxa.Emit(ctx); err != nil {
			t.Fatalf("emit: %v", err)
		}
	}

	if err := loxa.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if got := store.Len(); got < 50 {
		t.Fatalf("expected >=50 events, got %d", got)
	}
}

func TestAsyncPipelineNoLossConcurrent(t *testing.T) {
	sink, store := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink)
	cfg.Async.Enabled = true
	cfg.Async.QueueSize = 256
	cfg.Async.Workers = 4
	cfg.Async.FlushInterval = 10 * time.Millisecond
	cfg.Async.Backpressure = loxa.Block

	if err := loxa.Configure(cfg); err != nil {
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
				ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "pipeline.concurrent"})
				loxa.Enrich(ctx, loxa.Int("worker", gid), loxa.Int("seq", i))
				loxa.Finish(ctx, "success")
				if err := loxa.Emit(ctx); err != nil {
					t.Errorf("emit: %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	if err := loxa.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	want := goroutines * perGoroutine
	if got := store.Len(); got != want {
		t.Fatalf("expected %d events, got %d", want, got)
	}
}

func TestConfigureDrainsPreviousLogger(t *testing.T) {
	sinkA, storeA := loxa.MemorySink()
	cfgA := loxa.Test().WithSink(sinkA)
	cfgA.Async.Enabled = true
	cfgA.Async.QueueSize = 256
	cfgA.Async.Workers = 2
	cfgA.Async.FlushInterval = time.Second
	cfgA.Async.Backpressure = loxa.Block
	if err := loxa.Configure(cfgA); err != nil {
		t.Fatalf("configure A: %v", err)
	}

	const total = 400
	for i := 0; i < total; i++ {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "pipeline.reconfigure"})
		loxa.Finish(ctx, "success")
		if err := loxa.Emit(ctx); err != nil {
			t.Fatalf("emit A: %v", err)
		}
	}

	sinkB, _ := loxa.MemorySink()
	cfgB := loxa.Test().WithSink(sinkB)
	cfgB.Async.Enabled = true
	cfgB.Async.QueueSize = 128
	cfgB.Async.Workers = 2
	cfgB.Async.FlushInterval = 10 * time.Millisecond
	cfgB.Async.Backpressure = loxa.Block
	if err := loxa.Configure(cfgB); err != nil {
		t.Fatalf("configure B: %v", err)
	}

	if got := storeA.Len(); got != total {
		t.Fatalf("expected %d events from previous logger after reconfigure, got %d", total, got)
	}
}
