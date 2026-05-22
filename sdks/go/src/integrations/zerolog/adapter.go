package zerolog

import (
	"context"

	"github.com/astraive/loxa/sdks/go"
	"github.com/rs/zerolog"
)

// AdapterHook routes zerolog events into loxa immediate logs.
type AdapterHook struct{}

// Hook creates a zerolog hook adapter.
func Hook() AdapterHook { return AdapterHook{} }

// Run implements zerolog.Hook.
func (AdapterHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	ctx := context.Background()
	switch level {
	case zerolog.DebugLevel, zerolog.TraceLevel:
		loxa.DebugContext(ctx, msg, "zerolog.event")
	case zerolog.InfoLevel:
		loxa.InfoContext(ctx, msg, "zerolog.event")
	case zerolog.WarnLevel:
		loxa.WarnContext(ctx, msg, "zerolog.event")
	default:
		loxa.ErrorContext(ctx, msg, nil, "zerolog.event")
	}
	_ = e
}
