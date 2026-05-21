package zap

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestNewCoreCompatibility(t *testing.T) {
	base := zapcore.NewNopCore()
	c := NewCore(base)
	if c == nil {
		t.Fatalf("expected adapter core")
	}
}
