package event

import (
	"context"
	"strings"
	"time"
)

// Sink is the collector-owned delivery contract for encoded wide events.
// The collector intentionally owns this interface so it never depends on an SDK.
type Sink interface {
	Name() string
	WriteEvent(ctx context.Context, encoded []byte, ev *Event) error
	Flush(ctx context.Context) error
	Close(ctx context.Context) error
}

// Level represents collector-side event severity.
type Level uint8

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelFatal:
		return "fatal"
	default:
		return "info"
	}
}

func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// ErrorInfo is the normalized collector-side error payload.
type ErrorInfo struct {
	Type      string `json:"type,omitempty"`
	Message   string `json:"message,omitempty"`
	Code      string `json:"code,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
	Stack     string `json:"stack,omitempty"`
}

// Event is the collector's optional typed view for sinks that benefit from
// canonical fields. Most collector paths operate on encoded bytes only.
type Event struct {
	Timestamp     time.Time
	SchemaVersion string
	EventVersion  string
	EventID       string
	RequestID     string
	TraceID       string
	SpanID        string
	ParentID      string
	Level         Level
	Event         string
	Kind          string
	Message       string
	Outcome       string
	Service       string
	Version       string
	Environment   string
	DeploymentID  string
	Region        string
	Host          string
	Runtime       string
	Method        string
	Path          string
	Route         string
	StatusCode    int
	DurationMS    int64
	StartedAt     time.Time
	FinishedAt    time.Time
	Error         *ErrorInfo
}
