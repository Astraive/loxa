package core

import "context"

// Sink receives encoded events and delivers them to a destination.
// All methods must be safe for concurrent use.
type Sink interface {
	// Name returns a human-readable identifier for this sink.
	Name() string
	// WriteEvent delivers an already-encoded event to the sink.
	// encoded is the JSON bytes (including trailing newline).
	// ev is the original Event for sinks that need typed field access.
	WriteEvent(ctx context.Context, encoded []byte, ev *Event) error
	// Flush forces any buffered data to be written.
	Flush(ctx context.Context) error
	// Close releases resources held by the sink.
	Close(ctx context.Context) error
}
