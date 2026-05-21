package core

import (
	"testing"
	"time"
)

func TestSampleRateLimited(t *testing.T) {
	s := SampleRateLimited(1, 40*time.Millisecond)
	if !s.ShouldSample(nil) {
		t.Fatalf("expected first event to be sampled")
	}
	if s.ShouldSample(nil) {
		t.Fatalf("expected second immediate event to be rate-limited")
	}
	time.Sleep(50 * time.Millisecond)
	if !s.ShouldSample(nil) {
		t.Fatalf("expected sampling budget to refill after window")
	}
}

func TestSampleByHeader(t *testing.T) {
	ev := &Event{
		Attrs: []Attr{
			String("http.header.x-debug-logging", "true"),
		},
	}

	if !SampleByHeader("X-Debug-Logging", "true").ShouldSample(ev) {
		t.Fatalf("expected matching header sampler to keep event")
	}
	if SampleByHeader("X-Debug-Logging", "false").ShouldSample(ev) {
		t.Fatalf("expected non-matching header sampler to drop event")
	}
}

