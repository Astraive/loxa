package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

var captureMu sync.Mutex

// TestLogger creates a logger configured for tests plus its backing memory store.
func TestLogger() (*Logger, *MemorySinkStore, error) {
	sink, store := MemorySink()
	cfg := Test().WithSink(sink)
	l, err := New(cfg)
	if err != nil {
		return nil, nil, err
	}
	return l, store, nil
}

// Capture runs fn with a temporary memory sink and returns captured events.
func Capture(fn func()) ([]*Event, error) {
	captureMu.Lock()
	defer captureMu.Unlock()

	sink, store := MemorySink()
	prevCfg := Default().cfg

	if err := Configure(Test().WithSink(sink)); err != nil {
		return nil, err
	}
	defer func() {
		_ = Configure(prevCfg)
	}()
	fn()
	if err := Default().Flush(context.Background()); err != nil {
		return nil, err
	}
	return store.Events(), nil
}

// AssertRedacted asserts a key on event attrs has the value "[REDACTED]".
func AssertRedacted(t testing.TB, ev *Event, key string) {
	t.Helper()
	AssertEvent(t, ev, key, "[REDACTED]")
}

// AssertHasCheckpoint asserts the event contains a checkpoint with the given name.
func AssertHasCheckpoint(t testing.TB, ev *Event, name string) {
	t.Helper()
	if ev == nil {
		t.Fatalf("event is nil")
		return
	}
	ev.MuLock()
	defer ev.MuUnlock()
	for _, cp := range ev.Checkpoints {
		if cp.Name == name {
			return
		}
	}
	t.Fatalf("event checkpoint %q not found", name)
}

// AssertEvent asserts a key on event attrs equals expected.
func AssertEvent(t testing.TB, ev *Event, key string, expected any) {
	t.Helper()
	if ev == nil {
		t.Fatalf("event is nil")
		return
	}
	ev.MuLock()
	defer ev.MuUnlock()
	for _, a := range ev.Attrs {
		if a.Key == key {
			if fmt.Sprintf("%v", a.Value) != fmt.Sprintf("%v", expected) {
				t.Fatalf("event key %q: expected %v, got %v", key, expected, a.Value)
			}
			return
		}
	}
	t.Fatalf("event key %q not found", key)
}
