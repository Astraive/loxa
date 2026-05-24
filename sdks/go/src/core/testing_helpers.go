package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
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

// TestKit creates a logger configured for tests plus its backing memory store.
// Spec-aligned alias for TestLogger.
func TestKit() (*Logger, *MemorySinkStore, error) {
	return TestLogger()
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

// ExpectEvent asserts that store contains at least one event.
func ExpectEvent(t testing.TB, store *MemorySinkStore) *Event {
	t.Helper()
	events := store.Events()
	if len(events) == 0 {
		t.Fatalf("expected at least one event, got none")
	}
	return events[0]
}

// ExpectAttr asserts ev contains an attr with the given key and value.
func ExpectAttr(t testing.TB, ev *Event, key string, expected any) {
	t.Helper()
	AssertEvent(t, ev, key, expected)
}

// SnapshotEvent returns a JSON snapshot of the event for comparison.
func SnapshotEvent(t testing.TB, ev *Event) string {
	t.Helper()
	enc := JSONEncoder()
	buf, err := enc.EncodeEvent(nil, ev)
	if err != nil {
		t.Fatalf("snapshot encode: %v", err)
	}
	return string(buf)
}

// ── MockSink ─────────────────────────────────────────────────────────────────

// MockSink is a test sink that records events and supports pausing/draining.
type MockSink struct {
	events    []*Event
	raw       [][]byte
	mu        sync.Mutex
	paused    bool
	pauseChan chan struct{}
}

func NewMockSink() *MockSink {
	return &MockSink{pauseChan: make(chan struct{})}
}

func (s *MockSink) Name() string { return "mock" }

func (s *MockSink) WriteEvent(_ context.Context, encoded []byte, ev *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paused {
		return fmt.Errorf("mock sink is paused")
	}
	cp := make([]byte, len(encoded))
	copy(cp, encoded)
	s.events = append(s.events, ev)
	s.raw = append(s.raw, cp)
	return nil
}

func (s *MockSink) Flush(_ context.Context) error { return nil }

func (s *MockSink) Close(_ context.Context) error { return nil }

func (s *MockSink) Events() []*Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Event, len(s.events))
	copy(out, s.events)
	return out
}

func (s *MockSink) Raw() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]byte, len(s.raw))
	copy(out, s.raw)
	return out
}

func (s *MockSink) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

func (s *MockSink) Clear() {
	s.mu.Lock()
	s.events = s.events[:0]
	s.raw = s.raw[:0]
	s.mu.Unlock()
}

// Pause pauses the mock sink.
func (s *MockSink) Pause() {
	s.mu.Lock()
	s.paused = true
	s.mu.Unlock()
}

// Resume resumes the mock sink.
func (s *MockSink) Resume() {
	s.mu.Lock()
	s.paused = false
	s.mu.Unlock()
}

// FakeClock implements the Clock interface with a controllable time.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func NewFakeClock(t time.Time) *FakeClock {
	return &FakeClock{now: t}
}

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	c.now = t
	c.mu.Unlock()
}

func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

// SetIDGenerator replaces the ID generator on Config for deterministic IDs.
func SetIDGenerator(cfg Config, gen IDGenerator) Config {
	cfg.IDGen = gen
	return cfg
}

// ResetForTest clears all global mutable state: global logger, clock, and ID generator.
func ResetForTest() {
	defaultLoggerMu.Lock()
	defaultLog = nil
	defaultLoggerMu.Unlock()
	globalIDGen = &uuidV7Gen{}
}
