package core

import (
	"errors"
	"fmt"
)

// ErrInvalidConfig is returned when configuration validation fails.
var ErrInvalidConfig = errors.New("loza: invalid config")

// ErrorInfo is the structured representation of an error attached to an event.
type ErrorInfo struct {
	Type      string `json:"type"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message"`
	Retriable bool   `json:"retriable,omitempty"`
	Stack     string `json:"stack,omitempty"`
	Cause     string `json:"cause,omitempty"`
}

// ── Optional error interfaces ─────────────────────────────────────────────────

// CodedError is implemented by errors that carry a machine-readable code.
type CodedError interface {
	Code() string
}

// RetriableError is implemented by errors that signal whether retry is safe.
type RetriableError interface {
	Retriable() bool
}

// StackError is implemented by errors that carry a stack trace string.
type StackError interface {
	StackTrace() string
}

// ErrorExtractor converts an error into an ErrorInfo.
// Override this on Config to customise extraction (e.g. for pkg/errors).
type ErrorExtractor func(err error) *ErrorInfo

// DuplicateFieldError is returned when duplicate canonical attrs are rejected
// by DuplicateFieldPolicy=ErrorOnDuplicate.
type DuplicateFieldError struct {
	Key string
}

// DuplicateEmitError is returned when Emit is called after an event has already
// reached the emitted terminal state.
type DuplicateEmitError struct {
	EventID string
}

// EventClosedError is returned when code attempts to mutate or finish an event
// after the lifecycle has moved past the mutable states.
type EventClosedError struct {
	EventID string
	State   EventState
}

// EventAlreadyFinishedError is returned when Finish or FinishError is called
// more than once before emit.
type EventAlreadyFinishedError struct {
	EventID string
}

// ConfigValidationError is returned when a specific config field is invalid.
type ConfigValidationError struct {
	Field   string
	Problem string
}

func (e *ConfigValidationError) Error() string {
	if e == nil {
		return ErrInvalidConfig.Error()
	}
	switch {
	case e.Field == "" && e.Problem == "":
		return ErrInvalidConfig.Error()
	case e.Field == "":
		return fmt.Sprintf("%s: %s", ErrInvalidConfig.Error(), e.Problem)
	case e.Problem == "":
		return fmt.Sprintf("%s: invalid %s", ErrInvalidConfig.Error(), e.Field)
	default:
		return fmt.Sprintf("%s: %s: %s", ErrInvalidConfig.Error(), e.Field, e.Problem)
	}
}

func (e *ConfigValidationError) Unwrap() error { return ErrInvalidConfig }

func (e *DuplicateFieldError) Error() string {
	if e == nil || e.Key == "" {
		return "loza: duplicate canonical field attr"
	}
	return fmt.Sprintf("loza: duplicate canonical field attr %q", e.Key)
}

func (e *DuplicateEmitError) Error() string {
	if e == nil || e.EventID == "" {
		return "loza: duplicate emit"
	}
	return fmt.Sprintf("loza: duplicate emit for event %q", e.EventID)
}

func (e *EventClosedError) Error() string {
	if e == nil {
		return "loza: event is closed"
	}
	if e.EventID == "" {
		return fmt.Sprintf("loza: event is closed in state %q", e.State)
	}
	return fmt.Sprintf("loza: event %q is closed in state %q", e.EventID, e.State)
}

func (e *EventAlreadyFinishedError) Error() string {
	if e == nil || e.EventID == "" {
		return "loza: event already finished"
	}
	return fmt.Sprintf("loza: event %q already finished", e.EventID)
}

// DefaultErrorExtractor is the built-in extractor used when none is configured.
func DefaultErrorExtractor(err error) *ErrorInfo {
	return defaultErrorExtractor(err, true)
}

// DefaultErrorExtractorNoStack is equivalent to DefaultErrorExtractor but omits stack traces.
func DefaultErrorExtractorNoStack(err error) *ErrorInfo {
	return defaultErrorExtractor(err, false)
}

func defaultErrorExtractor(err error, includeStack bool) *ErrorInfo {
	if err == nil {
		return nil
	}

	info := &ErrorInfo{
		Type:    fmt.Sprintf("%T", err),
		Message: err.Error(),
	}

	// Code
	var coded CodedError
	if errors.As(err, &coded) {
		info.Code = coded.Code()
	}

	// Retriable
	var retriable RetriableError
	if errors.As(err, &retriable) {
		info.Retriable = retriable.Retriable()
	}

	// Stack trace
	if includeStack {
		var stacked StackError
		if errors.As(err, &stacked) {
			info.Stack = stacked.StackTrace()
		}
	}

	// Cause (unwrapped error)
	if cause := errors.Unwrap(err); cause != nil {
		info.Cause = cause.Error()
	}

	return info
}
