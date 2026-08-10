package zap

import (
	"context"

	"github.com/astraive/loza/sdks/go"
	"go.uber.org/zap/zapcore"
)

// AdapterCore wraps a zapcore.Core and mirrors entries into loza.
type AdapterCore struct {
	zapcore.Core
}

// Core wraps a base zap core with loza mirroring.
func Core(base zapcore.Core) *AdapterCore {
	return &AdapterCore{Core: base}
}

// Deprecated: use Core.
func NewCore(base zapcore.Core) *AdapterCore { return Core(base) }

// Write forwards to the wrapped core and emits to loza.
func (c *AdapterCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	ctx := context.Background()
	switch ent.Level {
	case zapcore.DebugLevel:
		loza.DebugContext(ctx, ent.Message, "zap.event")
	case zapcore.InfoLevel:
		loza.InfoContext(ctx, ent.Message, "zap.event")
	case zapcore.WarnLevel:
		loza.WarnContext(ctx, ent.Message, "zap.event")
	default:
		loza.ErrorContext(ctx, ent.Message, nil, "zap.event")
	}
	return c.Core.Write(ent, fields)
}
