package conformance

import (
	"context"
	"testing"

	"github.com/astraive/loza/sdks/go"
)

func TestSinkQueueFlushAndShutdownHelpers(t *testing.T) {
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink).WithAsync(false)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := loza.Default().Event(context.Background(), "sink.event", loza.String("family", "sink")); err != nil {
		t.Fatalf("event: %v", err)
	}
	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if store.Len() == 0 {
		t.Fatalf("expected sink event to be flushed")
	}
	if err := loza.Drain(context.Background(), sink); err != nil {
		t.Fatalf("drain: %v", err)
	}
	loza.Pause(sink)
	loza.Resume(sink)
	if got := loza.QueueSize(sink); got != 0 {
		t.Fatalf("expected empty queue, got %d", got)
	}
	if err := loza.Health(context.Background(), sink); err != nil {
		t.Fatalf("health: %v", err)
	}
	if loza.StdoutSink() == nil {
		t.Fatalf("expected stdout sink")
	}
	if loza.NoopSink() == nil {
		t.Fatalf("expected noop sink")
	}
	if loza.MultiSink(sink, loza.NoopSink()) == nil {
		t.Fatalf("expected multi sink")
	}
	if err := loza.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
