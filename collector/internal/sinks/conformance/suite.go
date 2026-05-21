package conformance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	collectorevent "github.com/astraive/loxa-collector/internal/event"
)

type Factory func(t testing.TB) collectorevent.Sink

type Options struct {
	Name               string
	Factory            Factory
	LargePayloadBytes  int
	Timeout            time.Duration
	RequiresIdempotent bool
}

func Run(t *testing.T, opts Options) {
	t.Helper()
	if opts.Factory == nil {
		t.Fatal("conformance: Factory is required")
	}
	if opts.Name == "" {
		opts.Name = "sink"
	}
	if opts.Timeout <= 0 {
		opts.Timeout = time.Second
	}
	if opts.LargePayloadBytes <= 0 {
		opts.LargePayloadBytes = 256 * 1024
	}

	t.Run("open_close", func(t *testing.T) {
		sink := opts.Factory(t)
		closeSink(t, sink, opts.Timeout)
	})
	t.Run("write_one_event", func(t *testing.T) {
		sink := opts.Factory(t)
		defer closeSink(t, sink, opts.Timeout)
		writeEvent(t, sink, opts.Timeout, `{"event_id":"evt_conf_one","event":"conformance.one"}`)
	})
	t.Run("write_batch_serial", func(t *testing.T) {
		sink := opts.Factory(t)
		defer closeSink(t, sink, opts.Timeout)
		for i := 0; i < 3; i++ {
			writeEvent(t, sink, opts.Timeout, fmt.Sprintf(`{"event_id":"evt_conf_batch_%d","event":"conformance.batch"}`, i))
		}
	})
	t.Run("flush", func(t *testing.T) {
		sink := opts.Factory(t)
		defer closeSink(t, sink, opts.Timeout)
		ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()
		if err := sink.Flush(ctx); err != nil {
			t.Fatalf("flush: %v", err)
		}
	})
	t.Run("large_payload", func(t *testing.T) {
		sink := opts.Factory(t)
		defer closeSink(t, sink, opts.Timeout)
		payload := `{"event_id":"evt_conf_large","event":"conformance.large","attrs":{"blob":"` +
			strings.Repeat("x", opts.LargePayloadBytes) + `"}}`
		writeEvent(t, sink, opts.Timeout, payload)
	})
	t.Run("context_cancellation", func(t *testing.T) {
		sink := opts.Factory(t)
		defer closeSink(t, sink, opts.Timeout)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := sink.WriteEvent(ctx, []byte(`{"event_id":"evt_conf_cancel","event":"conformance.cancel"}`), nil)
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("expected nil or context.Canceled, got %v", err)
		}
	})
}

func writeEvent(t testing.TB, sink collectorevent.Sink, timeout time.Duration, payload string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := sink.WriteEvent(ctx, []byte(payload), nil); err != nil {
		t.Fatalf("write event: %v", err)
	}
}

func closeSink(t testing.TB, sink collectorevent.Sink, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := sink.Flush(ctx); err != nil {
		t.Fatalf("flush before close: %v", err)
	}
	if err := sink.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
}
