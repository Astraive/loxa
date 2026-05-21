package testkit

import (
	"github.com/astraive/loxa-go"
	"github.com/astraive/loxa-go/src/core"
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
