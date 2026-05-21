package logging

import (
	"testing"

	"github.com/rs/zerolog"
)

func TestParseLevel(t *testing.T) {
	if got := parseLevel("debug"); got != zerolog.DebugLevel {
		t.Fatalf("expected debug level, got %v", got)
	}
	if got := parseLevel("unknown"); got != zerolog.InfoLevel {
		t.Fatalf("expected info fallback, got %v", got)
	}
}

func TestNewReturnsLogger(t *testing.T) {
	if got := New("info", "json"); got == nil {
		t.Fatal("expected logger")
	}
}
