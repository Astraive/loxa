package core

// EventCheckpoint is a named breadcrumb recorded inside a canonical event.
// Checkpoints are lightweight — they store a name, elapsed milliseconds,
// and optional attrs. They appear in the final emitted JSON under "checkpoints".
type EventCheckpoint struct {
	// Name is a dot-separated event name, e.g. "payment.started".
	Name string
	// AtMS is milliseconds elapsed from Event.StartedAt to when this was recorded.
	AtMS int64
	// Attrs holds optional key-value context for this checkpoint.
	Attrs []Attr
}
