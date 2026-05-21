package core

import "time"

// Clock abstracts time to allow deterministic testing.
type Clock interface {
	Now() time.Time
}

// realClock returns the actual wall clock time.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// mockClock returns a fixed time for tests.
type mockClock struct {
	t time.Time
}

func (m *mockClock) Now() time.Time { return m.t }

// NewMockClock returns a Clock that always returns t.
func NewMockClock(t time.Time) Clock { return &mockClock{t: t} }
