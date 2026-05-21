package core

import "context"

// contextKey is a private type to prevent key collisions with other packages.
type contextKey struct{}

// eventKey is the singleton context key used to store *Event.
var eventKey = contextKey{}

// storeEvent stores ev in ctx and returns the derived context.
func storeEvent(ctx context.Context, ev *Event) context.Context {
	return context.WithValue(ctx, eventKey, ev)
}

// loadEvent returns the event from ctx, or nil when absent.
func loadEvent(ctx context.Context) *Event {
	ev, _ := ctx.Value(eventKey).(*Event)
	return ev
}

// FromContext retrieves the canonical Event from ctx.
// Returns (nil, false) if StartEvent has not been called.
func FromContext(ctx context.Context) (*Event, bool) {
	ev := loadEvent(ctx)
	return ev, ev != nil
}

// HasEvent reports if ctx has an active canonical event.
func HasEvent(ctx context.Context) bool {
	_, ok := FromContext(ctx)
	return ok
}

// EventID returns the event id from ctx when present.
func EventID(ctx context.Context) string {
	ev := loadEvent(ctx)
	if ev == nil {
		return ""
	}
	ev.MuLock()
	defer ev.MuUnlock()
	return ev.EventID
}

// RequestIDFromContext returns the request id from ctx when present.
func RequestIDFromContext(ctx context.Context) string {
	ev := loadEvent(ctx)
	if ev == nil {
		return ""
	}
	ev.MuLock()
	defer ev.MuUnlock()
	return ev.RequestID
}

// TraceIDFromContext returns the trace id from ctx when present.
func TraceIDFromContext(ctx context.Context) string {
	ev := loadEvent(ctx)
	if ev == nil {
		return ""
	}
	ev.MuLock()
	defer ev.MuUnlock()
	return ev.TraceID
}

// SpanIDFromContext returns the span id from ctx when present.
func SpanIDFromContext(ctx context.Context) string {
	ev := loadEvent(ctx)
	if ev == nil {
		return ""
	}
	ev.MuLock()
	defer ev.MuUnlock()
	return ev.SpanID
}
