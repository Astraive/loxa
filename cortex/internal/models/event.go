package models

import (
	"errors"
	"time"
)

var (
	ErrEmptyID        = errors.New("event ID cannot be empty")
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
	EventKindLoxaEvent     EventKind = "loxa_event"
	EventKindOTELLog       EventKind = "otel_log"
	EventKindOTELSpan       EventKind = "otel_span"
	EventKindCollectorEvent EventKind = "collector_event"
)

var validEventKinds = map[EventKind]bool{
	EventKindDeploy:          true,
	EventKindLog:            true,
	EventKindMetric:         true,
	EventKindTrace:          true,
	EventKindTopology:       true,
	EventKindIncidentSignal: true,
	EventKindRemediation:    true,
	EventKindLoxaEvent:      true,
	EventKindOTELLog:        true,
	EventKindOTELSpan:       true,
	EventKindCollectorEvent: true,
}

type Event struct {
	ID          string                 `json:"id" db:"id"`
	Timestamp   time.Time              `json:"timestamp" db:"timestamp"`
	Kind        EventKind              `json:"kind" db:"kind"`
	Service     string                 `json:"service" db:"service"`
	TraceID     string                 `json:"trace_id,omitempty" db:"trace_id"`
	IncidentID  string                 `json:"incident_id,omitempty" db:"incident_id"`
	Raw         map[string]interface{} `json:"raw" db:"raw"`
	Provenance  string                 `json:"provenance" db:"provenance"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
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
	"loxa":      true,
	"otlp":     true,
	"collector": true,
	"jsonl":    true,
}

func IsValidProvenance(p string) bool {
	return validProvenances[p]
}

func IsValidKind(k EventKind) bool {
	return validEventKinds[k]
}