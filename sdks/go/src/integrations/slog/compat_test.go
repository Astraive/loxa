package slog

import (
	"context"
	stdslog "log/slog"
	"testing"
)

func TestNewHandlerCompatibility(t *testing.T) {
	h := NewHandler()
	if h == nil {
		t.Fatalf("expected handler")
	}
	if !h.Enabled(context.Background(), stdslog.LevelInfo) {
		t.Fatalf("expected info level to be enabled")
	}
}
