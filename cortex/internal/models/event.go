package models

import (
	"errors"
	"time"
)

var (
	ErrEmptyID         = errors.New("event ID cannot be empty")
	ErrInvalidTimestamp = errors.New("timestamp cannot be in the future")
	ErrInvalidKind     = errors.New("invalid event kind")
	ErrEmptyService    = errors.New("service cannot be empty")
	ErrInvalidProvenance = errors.New("invalid provenance")
)

type EventKind string

const (
	EventKindDeploy          EventKind = "deploy"
	EventKindLog            EventKind = "log"
	EventKindMetric         EventKind = "metric"
	EventKindTrace           EventKind = "trace"
	EventKindTopology        EventKind = "topology"
	EventKindIncidentSignal EventKind = "incident_signal"
	EventKindRemediation     EventKind = "remediation"
	EventKindLozaEvent     EventKind = "loza_event"
	EventKindOTELLog       EventKind = "otel_log"
	EventKindOTELSpan       EventKind = "otel_span"
	EventKindCollectorEvent EventKind = "collector_event"
	EventKindHTTP           EventKind = "http"
	EventKindJob            EventKind = "job"
	EventKindQueue          EventKind = "queue"
	EventKindCLI            EventKind = "cli"
	EventKindCron           EventKind = "cron"
	EventKindAgent          EventKind = "agent"
	EventKindAI             EventKind = "ai"
)

var validEventKinds = map[EventKind]bool{
	EventKindDeploy:          true,
	EventKindLog:            true,
	EventKindMetric:         true,
	EventKindTrace:          true,
	EventKindTopology:       true,
	EventKindIncidentSignal: true,
	EventKindRemediation:    true,
	EventKindLozaEvent:      true,
	EventKindOTELLog:        true,
	EventKindOTELSpan:       true,
	EventKindCollectorEvent: true,
	EventKindHTTP:           true,
	EventKindJob:            true,
	EventKindQueue:          true,
	EventKindCLI:            true,
	EventKindCron:           true,
	EventKindAgent:          true,
	EventKindAI:             true,
}

// EventCheckpoint represents a named breadcrumb at a point in time
type EventCheckpoint struct {
	Name      string                 `json:"name"`
	AtMs      int64                  `json:"at_ms"`
	Attrs     map[string]interface{} `json:"attrs,omitempty"`
}

// EventProcess represents an ordered numbered step inside an event
type EventProcess struct {
	Step        int                    `json:"step"`
	Name        string                 `json:"name"`
	StatusCode  int                    `json:"status_code,omitempty"`
	StartedAtMs int64                  `json:"started_at_ms"`
	EndedAtMs   int64                  `json:"ended_at_ms,omitempty"`
	DurationMs  int64                  `json:"duration_ms,omitempty"`
	Attrs       map[string]interface{} `json:"attrs,omitempty"`
	Outcome     string                 `json:"outcome,omitempty"`
	Error       *EventError            `json:"error,omitempty"`
}

// EventGroup represents a named phase/block with duration
type EventGroup struct {
	Name        string                 `json:"name"`
	StatusCode  int                    `json:"status_code,omitempty"`
	StartedAtMs int64                  `json:"started_at_ms"`
	EndedAtMs   int64                  `json:"ended_at_ms,omitempty"`
	DurationMs  int64                  `json:"duration_ms,omitempty"`
	Attrs       map[string]interface{} `json:"attrs,omitempty"`
	Outcome     string                 `json:"outcome,omitempty"`
	Error       *EventError            `json:"error,omitempty"`
}

// EventTimer represents a named latency measurement
type EventTimer struct {
	Name       string                 `json:"name"`
	DurationMs int64                  `json:"duration_ms"`
	StatusCode int                    `json:"status_code,omitempty"`
	Attrs      map[string]interface{} `json:"attrs,omitempty"`
}

// EventLink represents a cross-event relationship
type EventLink struct {
	Type   string                 `json:"type"`
	Target string                 `json:"target"`
	Attrs  map[string]interface{} `json:"attrs,omitempty"`
}

// EventError represents error context
type EventError struct {
	Type      string `json:"type,omitempty"`
	Message   string `json:"message,omitempty"`
	Code      string `json:"code,omitempty"`
	Retriable bool   `json:"retriable,omitempty"`
	Cause     string `json:"cause,omitempty"`
	Stack     string `json:"stack,omitempty"`
}

// HttpContext represents HTTP request/response context
type HttpContext struct {
	Method     string `json:"method,omitempty"`
	Path       string `json:"path,omitempty"`
	Route      string `json:"route,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	ClientIP   string `json:"client_ip,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
	URL        string `json:"url,omitempty"`
	Host       string `json:"host,omitempty"`
}

// Event is the full Loza business event with complete lifecycle support
type Event struct {
	// Core identifiers
	ID          string    `json:"id" db:"id"`
	EventID     string    `json:"event_id,omitempty" db:"event_id"`
	Timestamp   time.Time `json:"timestamp" db:"timestamp"`
	Service     string    `json:"service" db:"service"`
	Environment string    `json:"environment,omitempty" db:"environment"`
	Release     string    `json:"release,omitempty" db:"release"`

	// Event fields
	SchemaVersion string     `json:"schema_version,omitempty" db:"schema_version"`
	EventVersion  string     `json:"event_version,omitempty" db:"event_version"`
	Version       string     `json:"version,omitempty" db:"version"`
	Event         string     `json:"event,omitempty" db:"event"`
	Kind          EventKind  `json:"kind" db:"kind"`
	Level         string     `json:"level,omitempty" db:"level"`
	Outcome       string     `json:"outcome,omitempty" db:"outcome"`
	DurationMs    float64    `json:"duration_ms,omitempty" db:"duration_ms"`

	// Trace correlation
	TraceID    string `json:"trace_id,omitempty" db:"trace_id"`
	SpanID     string `json:"span_id,omitempty" db:"span_id"`
	TraceFlags string `json:"trace_flags,omitempty" db:"trace_flags"`
	RequestID  string `json:"request_id,omitempty" db:"request_id"`

	// Rich contexts
	HTTP   *HttpContext           `json:"http,omitempty" db:"-"`
	User   map[string]interface{} `json:"user,omitempty" db:"-"`
	Tenant map[string]interface{} `json:"tenant,omitempty" db:"-"`

	// Structured attributes (typed key-value pairs)
	Attrs map[string]interface{} `json:"attrs,omitempty" db:"-"`
	Error *EventError            `json:"error,omitempty" db:"-"`

	// Lifecycle primitives
	Checkpoints []*EventCheckpoint `json:"checkpoints,omitempty" db:"-"`
	Processes   []*EventProcess    `json:"processes,omitempty" db:"-"`
	Groups      []*EventGroup      `json:"groups,omitempty" db:"-"`
	Timers      []*EventTimer      `json:"timers,omitempty" db:"-"`
	Links       []*EventLink       `json:"links,omitempty" db:"-"`

	// SDK metadata
	SDKName    string `json:"sdk_name,omitempty" db:"sdk_name"`
	SDKVersion string `json:"sdk_version,omitempty" db:"sdk_version"`
	SDKLanguage string `json:"sdk_language,omitempty" db:"sdk_language"`

	// Raw payload (original event JSON)
	Raw        map[string]interface{} `json:"raw,omitempty" db:"raw"`
	Provenance string                 `json:"provenance" db:"provenance"`
	CreatedAt  time.Time              `json:"created_at" db:"created_at"`

	// Cortex-specific fields
	IncidentID string `json:"incident_id,omitempty" db:"incident_id"`
}

func (e *Event) Validate() error {
	if e.ID == "" {
		return ErrEmptyID
	}
	if e.Timestamp.IsZero() {
		return errors.New("timestamp is required")
	}
	if e.Timestamp.After(time.Now().Add(time.Minute)) {
		return ErrInvalidTimestamp
	}
	if !validEventKinds[e.Kind] {
		return ErrInvalidKind
	}
	if e.Service == "" {
		return ErrEmptyService
	}
	if e.Provenance != "" && !validProvenances[e.Provenance] {
		return ErrInvalidProvenance
	}
	return nil
}

var validProvenances = map[string]bool{
	"loza":      true,
	"otlp":     true,
	"collector": true,
	"jsonl":    true,
	"grpc":     true,
}

func IsValidProvenance(p string) bool {
	return validProvenances[p]
}

func IsValidKind(k EventKind) bool {
	return validEventKinds[k]
}

// LifecycleSummary provides a queryable summary of event lifecycle primitives
type LifecycleSummary struct {
	EventID        string  `json:"event_id"`
	Event          string  `json:"event"`
	Service        string  `json:"service"`
	Outcome        string  `json:"outcome"`
	DurationMs     float64 `json:"duration_ms"`
	CheckpointCount int    `json:"checkpoint_count"`
	ProcessCount    int    `json:"process_count"`
	GroupCount      int    `json:"group_count"`
	TimerCount      int    `json:"timer_count"`
	LinkCount       int    `json:"link_count"`
	TraceID         string `json:"trace_id"`
}
