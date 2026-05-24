package conformance

import (
	"context"
	"testing"

	"github.com/astraive/loxa/sdks/go"
)

func TestSinkQueueFlushAndShutdownHelpers(t *testing.T) {
	sink, store := loxa.MemorySink()
	if err := loxa.Configure(loxa.Test().WithSink(sink).WithAsync(false)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := loxa.Default().Event(context.Background(), "sink.event", loxa.String("family", "sink")); err != nil {
		t.Fatalf("event: %v", err)
	}
	if err := loxa.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if store.Len() == 0 {
		t.Fatalf("expected sink event to be flushed")
	}
	if err := loxa.Drain(context.Background(), sink); err != nil {
		t.Fatalf("drain: %v", err)
	}
	loxa.Pause(sink)
	loxa.Resume(sink)
	if got := loxa.QueueSize(sink); got != 0 {
		t.Fatalf("expected empty queue, got %d", got)
	}
	if err := loxa.Health(context.Background(), sink); err != nil {
		t.Fatalf("health: %v", err)
	}
	if loxa.StdoutSink() == nil {
		t.Fatalf("expected stdout sink")
	}
	if loxa.NoopSink() == nil {
		t.Fatalf("expected noop sink")
	}
	if loxa.MultiSink(sink, loxa.NoopSink()) == nil {
		t.Fatalf("expected multi sink")
	}
	if err := loxa.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
