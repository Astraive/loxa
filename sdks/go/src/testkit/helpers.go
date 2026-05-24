package testkit

import (
	"github.com/astraive/loxa/sdks/go"
	"github.com/astraive/loxa/sdks/go/src/core"
	"testing"
)

// Capture runs fn with a temporary memory sink and returns captured events.
func Capture(fn func()) ([]*loxa.Event, error) {
	return core.Capture(fn)
}

// AssertEvent asserts that event key equals expected value.
func AssertEvent(t testing.TB, ev *loxa.Event, key string, expected any) {
	core.AssertEvent(t, ev, key, expected)
}

// TestKit creates a Logger backed by a MemorySink for testing.
func TestKit() (*loxa.Logger, *loxa.MemorySinkStore, error) {
	return loxa.TestKit()
}

// SanitizeEvent clones the event and applies the global config's redactor
// and security settings. The original event is never mutated.
func SanitizeEvent(ev *loxa.Event) *loxa.Event {
	return loxa.SanitizeEvent(ev)
}

// ResetForTest clears all global mutable state.
func ResetForTest() {
	loxa.ResetForTest()
}
