package core

import (
	"time"
)

// EventProcess represents a named step in a multi-step process.
// Processes are recorded in the "process" array in the emitted JSON.
type EventProcess struct {
	Step        int
	Name        string
	StatusCode  int
	StartedAtMS int64
	EndedAtMS   int64
	DurationMS  int64
	Attrs       []Attr
}

// EventGroup represents a parent phase containing processes.
// Groups are recorded in the "groups" array in the emitted JSON.
type EventGroup struct {
	Name        string
	StatusCode  int
	StartedAtMS int64
	EndedAtMS   int64
	DurationMS  int64
	Attrs       []Attr
}

// EventTimer represents a named duration measurement.
// Timers are recorded in the "timers" array in the emitted JSON.
type EventTimer struct {
	Name       string
	DurationMS int64
	StatusCode int
	Attrs      []Attr
}

// ProcessHandle is returned by Event.StartProcess and tracks a running process step.
type ProcessHandle struct {
	event     *Event
	name      string
	step      int
	startedAt time.Time
}

// Finish completes the process with the given attrs.
func (h *ProcessHandle) Finish(attrs ...Attr) error {
	return h.finishInternal(0, attrs)
}

// FinishError completes the process with an error status code and error info.
func (h *ProcessHandle) FinishError(err error, statusCode int, attrs ...Attr) error {
	if err != nil {
		attrs = append(attrs, Attr{Key: "error_message", Kind: KindString, Value: err.Error()})
	}
	return h.finishInternal(statusCode, attrs)
}

func (h *ProcessHandle) finishInternal(statusCode int, attrs []Attr) error {
	now := time.Now()
	startedMS := h.startedAt.Sub(h.event.StartedAt).Milliseconds()
	endedMS := now.Sub(h.event.StartedAt).Milliseconds()

	// If statusCode is 0, try to extract from attrs
	if statusCode == 0 {
		statusCode = extractStatusCode(attrs)
		attrs = removeStatusCode(attrs)
	}

	entry := EventProcess{
		Step:        h.step,
		Name:        h.name,
		StatusCode:  statusCode,
		StartedAtMS: startedMS,
		EndedAtMS:   endedMS,
		DurationMS:  endedMS - startedMS,
		Attrs:       attrs,
	}

	h.event.mu.Lock()
	h.event.Processes = append(h.event.Processes, entry)
	h.event.mu.Unlock()
	return nil
}

// Duration returns the elapsed duration since the process started.
func (h *ProcessHandle) Duration() time.Duration {
	return time.Since(h.startedAt)
}

// TimerHandle is returned by Event.StartTimer and tracks a running timer.
type TimerHandle struct {
	event     *Event
	name      string
	startedAt time.Time
}

// Stop completes the timer with the given attrs.
func (h *TimerHandle) Stop(attrs ...Attr) error {
	now := time.Now()
	durationMS := now.Sub(h.startedAt).Milliseconds()

	statusCode := extractStatusCode(attrs)
	attrs = removeStatusCode(attrs)

	entry := EventTimer{
		Name:       h.name,
		DurationMS: durationMS,
		StatusCode: statusCode,
		Attrs:      attrs,
	}

	h.event.mu.Lock()
	h.event.Timers = append(h.event.Timers, entry)
	h.event.mu.Unlock()
	return nil
}

// Duration returns the elapsed duration since the timer started.
func (h *TimerHandle) Duration() time.Duration {
	return time.Since(h.startedAt)
}

// GroupHandle is returned by Event.StartGroup and tracks a running group phase.
type GroupHandle struct {
	event     *Event
	name      string
	startedAt time.Time
}

// Finish completes the group with the given attrs.
func (h *GroupHandle) Finish(attrs ...Attr) error {
	return h.finishInternal(0, attrs)
}

// FinishError completes the group with an error status code.
func (h *GroupHandle) FinishError(statusCode int, attrs ...Attr) error {
	return h.finishInternal(statusCode, attrs)
}

func (h *GroupHandle) finishInternal(statusCode int, attrs []Attr) error {
	now := time.Now()
	startedMS := h.startedAt.Sub(h.event.StartedAt).Milliseconds()
	endedMS := now.Sub(h.event.StartedAt).Milliseconds()

	// If statusCode is 0, try to extract from attrs
	if statusCode == 0 {
		statusCode = extractStatusCode(attrs)
		attrs = removeStatusCode(attrs)
	}

	entry := EventGroup{
		Name:        h.name,
		StatusCode:  statusCode,
		StartedAtMS: startedMS,
		EndedAtMS:   endedMS,
		DurationMS:  endedMS - startedMS,
		Attrs:       attrs,
	}

	h.event.mu.Lock()
	h.event.Groups = append(h.event.Groups, entry)
	h.event.mu.Unlock()
	return nil
}

// Duration returns the elapsed duration since the group started.
func (h *GroupHandle) Duration() time.Duration {
	return time.Since(h.startedAt)
}

// StopwatchHandle is a standalone timer that measures elapsed time without an event reference.
type StopwatchHandle struct {
	startedAt time.Time
}

// Stopwatch creates a new standalone stopwatch.
func Stopwatch() *StopwatchHandle {
	return &StopwatchHandle{startedAt: time.Now()}
}

// Elapsed returns the duration since the stopwatch was created.
func (h *StopwatchHandle) Elapsed() time.Duration {
	return time.Since(h.startedAt)
}

// extractStatusCode finds and returns the status_code value from attrs, or 0 if not found.
func extractStatusCode(attrs []Attr) int {
	for _, a := range attrs {
		if a.Key == "status_code" {
			switch v := a.Value.(type) {
			case int:
				return v
			case int64:
				return int(v)
			case float64:
				return int(v)
			}
		}
	}
	return 0
}

// removeStatusCode returns a new attrs slice without the status_code entry.
func removeStatusCode(attrs []Attr) []Attr {
	result := make([]Attr, 0, len(attrs))
	for _, a := range attrs {
		if a.Key != "status_code" {
			result = append(result, a)
		}
	}
	return result
}
