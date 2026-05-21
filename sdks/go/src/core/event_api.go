package core

import "context"

// NewEvent creates a manual event instance without storing it in context.
func NewEvent(params Params) *Event {
	l := Default()
	l.mu.RLock()
	cfg := l.cfg
	l.mu.RUnlock()
	ev := buildEvent(params, &cfg)
	ev.SetLogger(l)
	return ev
}

// Emit emits this event using its logger (or the default logger).
func (e *Event) Emit() error {
	e.mu.Lock()
	l := e.logger
	e.mu.Unlock()
	if l == nil {
		l = Default()
	}
	return l.EmitEvent(e)
}

// Flush flushes sinks for this event's logger.
func (e *Event) Flush(ctx context.Context) error {
	e.mu.Lock()
	l := e.logger
	e.mu.Unlock()
	if l == nil {
		l = Default()
	}
	return l.Flush(ctx)
}
