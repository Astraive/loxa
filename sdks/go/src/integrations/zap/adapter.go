package zap

import (
	"context"

	"github.com/astraive/loxa-go"
	"go.uber.org/zap/zapcore"
)

// AdapterCore wraps a zapcore.Core and mirrors entries into loxa.
type AdapterCore struct {
	zapcore.Core
}

// Core wraps a base zap core with loxa mirroring.
func Core(base zapcore.Core) *AdapterCore {
	return &AdapterCore{Core: base}
}

// Deprecated: use Core.
func NewCore(base zapcore.Core) *AdapterCore { return Core(base) }

// Write forwards to the wrapped core and emits to loxa.
func (c *AdapterCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	ctx := context.Background()
	switch ent.Level {
	case zapcore.DebugLevel:
		loxa.DebugContext(ctx, ent.Message, "zap.event")
	case zapcore.InfoLevel:
		loxa.InfoContext(ctx, ent.Message, "zap.event")
	case zapcore.WarnLevel:
		loxa.WarnContext(ctx, ent.Message, "zap.event")
	default:
		loxa.ErrorContext(ctx, ent.Message, nil, "zap.event")
	}
	return c.Core.Write(ent, fields)
}
