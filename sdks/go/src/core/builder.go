package core

import "time"

// BuildInput contains normalized values used to construct a canonical event map.
type BuildInput struct {
	Timestamp   time.Time
	EventID     string
	RequestID   string
	TraceID     string
	SpanID      string
	ParentID    string
	Level       string
	Event       string
	Message     string
	Outcome     string
	Service     string
	Version     string
	Environment string
	Method      string
	Path        string
	Route       string
	StatusCode  int
	DurationMS  int64
}

// NewEventMap builds a canonical map representation used by optional internal paths.
func NewEventMap(in BuildInput) map[string]any {
	m := map[string]any{
		"timestamp":  in.Timestamp.UTC().Format(time.RFC3339Nano),
		"event_id":   in.EventID,
		"request_id": in.RequestID,
		"level":      in.Level,
		"event":      in.Event,
	}

	putIfNonEmpty(m, "trace_id", in.TraceID)
	putIfNonEmpty(m, "span_id", in.SpanID)
	putIfNonEmpty(m, "parent_id", in.ParentID)
	putIfNonEmpty(m, "message", in.Message)
	putIfNonEmpty(m, "outcome", in.Outcome)
	putIfNonEmpty(m, "service", in.Service)
	putIfNonEmpty(m, "version", in.Version)
	putIfNonEmpty(m, "environment", in.Environment)
	putIfNonEmpty(m, "method", in.Method)
	putIfNonEmpty(m, "path", in.Path)
	putIfNonEmpty(m, "route", in.Route)
	if in.StatusCode != 0 {
		m["status_code"] = in.StatusCode
	}
	if in.DurationMS != 0 {
		m["duration_ms"] = in.DurationMS
	}
	return m
}

func putIfNonEmpty(m map[string]any, key, value string) {
	if value != "" {
		m[key] = value
	}
}
