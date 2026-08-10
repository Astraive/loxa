package zerolog

import (
	"context"

	"github.com/astraive/loza/sdks/go"
	"github.com/rs/zerolog"
)

// AdapterHook routes zerolog events into loza immediate logs.
type AdapterHook struct{}

// Hook creates a zerolog hook adapter.
func Hook() AdapterHook { return AdapterHook{} }

// Run implements zerolog.Hook.
func (AdapterHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	ctx := context.Background()
	switch level {
	case zerolog.DebugLevel, zerolog.TraceLevel:
		loza.DebugContext(ctx, msg, "zerolog.event")
	case zerolog.InfoLevel:
		loza.InfoContext(ctx, msg, "zerolog.event")
	case zerolog.WarnLevel:
		loza.WarnContext(ctx, msg, "zerolog.event")
	default:
		loza.ErrorContext(ctx, msg, nil, "zerolog.event")
	}
	_ = e
}
