package testkit

import (
	"github.com/astraive/loza/sdks/go"
	"github.com/astraive/loza/sdks/go/src/core"
	"testing"
)

// Capture runs fn with a temporary memory sink and returns captured events.
func Capture(fn func()) ([]*loza.Event, error) {
	return core.Capture(fn)
}

// AssertEvent asserts that event key equals expected value.
func AssertEvent(t testing.TB, ev *loza.Event, key string, expected any) {
	core.AssertEvent(t, ev, key, expected)
}

// TestKit creates a Logger backed by a MemorySink for testing.
func TestKit() (*loza.Logger, *loza.MemorySinkStore, error) {
	return loza.TestKit()
}

// SanitizeEvent clones the event and applies the global config's redactor
// and security settings. The original event is never mutated.
func SanitizeEvent(ev *loza.Event) *loza.Event {
	return loza.SanitizeEvent(ev)
}

// ResetForTest clears all global mutable state.
func ResetForTest() {
	loza.ResetForTest()
}
