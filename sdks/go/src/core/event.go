package core

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type EventState string

const (
	EventStateCreated          EventState = "created"
	EventStateActive           EventState = "active"
	EventStateFinished         EventState = "finished"
	EventStateEmitting         EventState = "emitting"
	EventStateEmitted          EventState = "emitted"
	EventStateInvalid          EventState = "invalid"
	EventStateDropped          EventState = "dropped"
	EventStateEmitFailed       EventState = "emit_failed"
	EventStateSpooled          EventState = "spooled"
	EventStateDLQWritten       EventState = "dlq_written"
	EventStateFailedValidation EventState = "failed_validation"
	EventStateDeliveryFailed   EventState = "delivery_failed"
)

// Event is the canonical wide event built for one request, job, or service hop.
// Canonical fields are typed struct members — encoded without reflection.
// Custom business context lives in Attrs.
// All methods are safe for concurrent use.
type Event struct {
	mu sync.Mutex

	// ── Correlation IDs ──────────────────────────────────────────────────────
	Timestamp     time.Time
	SchemaVersion string
	EventVersion  string
	EventID       string
	RequestID     string
	TraceID       string
	SpanID        string
	ParentID      string
	IncidentID    string

	// ── Classification ───────────────────────────────────────────────────────
	Level   Level
	Event   string
	Kind    string
	Message string
	Outcome string

	// ── Service metadata ─────────────────────────────────────────────────────
	Service      string
	Version      string
	Environment  string
	DeploymentID string
	Region       string
	Host         string
	Runtime      string

	// ── Request metadata ─────────────────────────────────────────────────────
	Method     string
	Path       string
	Route      string
	StatusCode int
	DurationMS int64

	// ── Timing ───────────────────────────────────────────────────────────────
	StartedAt  time.Time
	FinishedAt time.Time

	// ── Custom context ───────────────────────────────────────────────────────
	Attrs       []Attr
	Checkpoints []EventCheckpoint
	Processes   []EventProcess
	Groups      []EventGroup
	Timers      []EventTimer
	Error       *ErrorInfo

	// ── Internal timing state ────────────────────────────────────────────────
	processStep int

	// ── Internal state ───────────────────────────────────────────────────────
	emitted atomic.Bool
	state   EventState
	logger  *Logger
}

// MuLock acquires the event mutex. Use sparingly; prefer the accessor methods.
func (e *Event) MuLock() { e.mu.Lock() }

// MuUnlock releases the event mutex.
func (e *Event) MuUnlock() { e.mu.Unlock() }

// SetLogger binds the Logger that owns this event (used by logger.go).
func (e *Event) SetLogger(l *Logger) {
	e.mu.Lock()
	e.logger = l
	e.mu.Unlock()
}

// SetError sets the ErrorInfo field (used by immediate logger).
func (e *Event) SetError(info *ErrorInfo) {
	e.mu.Lock()
	e.Error = info
	e.mu.Unlock()
}

// SetOutcome sets the Outcome field.
func (e *Event) SetOutcome(outcome string) {
	e.mu.Lock()
	e.Outcome = outcome
	e.mu.Unlock()
}

// State returns the current event state for observability.
// Requirements: 1.10
func (e *Event) State() EventState {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.state == "" {
		return EventStateCreated
	}
	return e.state
}

// MarkEmitted is kept for compatibility with older tests and helper code.
// New emit paths should use beginEmit/markEmitted so validation failures do not
// burn the emitted state.
func (e *Event) MarkEmitted() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.state == EventStateEmitted || e.emitted.Load() {
		return false
	}
	e.setStateLocked(EventStateEmitted)
	return true
}

func (e *Event) setStateLocked(state EventState) {
	e.state = state
	e.emitted.Store(state == EventStateEmitted)
}

func (e *Event) ensureMutableLocked() error {
	switch e.state {
	case "", EventStateCreated, EventStateActive, EventStateFinished:
		return nil
	case EventStateEmitted:
		return &EventClosedError{EventID: e.EventID, State: e.state}
	default:
		return &EventClosedError{EventID: e.EventID, State: e.state}
	}
}

// ensureTraceContext generates trace/span IDs if not already set.
// Called after sampling, so sampled-out events skip the PRNG cost.
func (e *Event) ensureTraceContext() {
	if e.TraceID != "" && e.SpanID != "" {
		return // Already set by caller
	}
	tc := GenerateTraceContext()
	if e.TraceID == "" {
		e.TraceID = tc.TraceID
	}
	if e.SpanID == "" {
		e.SpanID = tc.SpanID
	}
}

// beginEmit validates and transitions the event to emitting state.
// Requirements: 1.5, 1.9
func (e *Event) beginEmit() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch e.state {
	case EventStateEmitted:
		return &DuplicateEmitError{EventID: e.EventID}
	case EventStateEmitting, EventStateDeliveryFailed, EventStateFailedValidation:
		return &EventClosedError{EventID: e.EventID, State: e.state}
	case "", EventStateCreated, EventStateActive, EventStateFinished:
		e.setStateLocked(EventStateEmitting)
		return nil
	default:
		return &EventClosedError{EventID: e.EventID, State: e.state}
	}
}

// markValidationFailed transitions the event to validation_failed state.
// Requirements: 1.7
func (e *Event) markValidationFailed() {
	e.mu.Lock()
	e.setStateLocked(EventStateFailedValidation)
	e.mu.Unlock()
}

// markDeliveryFailed transitions the event to delivery_failed state.
// Requirements: 1.8
func (e *Event) markDeliveryFailed() {
	e.mu.Lock()
	e.setStateLocked(EventStateDeliveryFailed)
	e.mu.Unlock()
}

// markEmitted transitions the event to emitted state.
// Requirements: 1.6
func (e *Event) markEmitted() {
	e.mu.Lock()
	e.setStateLocked(EventStateEmitted)
	e.mu.Unlock()
}

// AddAttrs appends attrs to the event under the mutex.
func (e *Event) AddAttrs(attrs []Attr) error {
	if len(attrs) == 0 {
		return nil
	}
	e.mu.Lock()
	if err := e.ensureMutableLocked(); err != nil {
		e.mu.Unlock()
		return err
	}
	// Transition to active state if in created state
	if e.state == "" || e.state == EventStateCreated {
		e.setStateLocked(EventStateActive)
	}
	e.Attrs = append(e.Attrs, attrs...)
	e.mu.Unlock()
	return nil
}

// AddCheckpoint appends an EventCheckpoint under the mutex.
func (e *Event) AddCheckpoint(cp EventCheckpoint) error {
	e.mu.Lock()
	if err := e.ensureMutableLocked(); err != nil {
		e.mu.Unlock()
		return err
	}
	e.Checkpoints = append(e.Checkpoints, cp)
	e.mu.Unlock()
	return nil
}

// finish applies outcome, computes duration, and processes extra attrs.
// Requirements: 1.4, 2.5
func (e *Event) finish(now time.Time, outcome string, attrs []Attr) error {
	e.mu.Lock()
	if err := e.ensureMutableLocked(); err != nil {
		e.mu.Unlock()
		return err
	}
	if !e.FinishedAt.IsZero() {
		e.mu.Unlock()
		return &EventAlreadyFinishedError{EventID: e.EventID}
	}
	e.FinishedAt = now
	e.DurationMS = now.Sub(e.StartedAt).Milliseconds()
	if outcome != "" {
		e.Outcome = outcome
	}
	for _, a := range attrs {
		e.applyCanonical(a)
	}
	e.setStateLocked(EventStateFinished)
	e.mu.Unlock()

	var custom []Attr
	for _, a := range attrs {
		if !isCanonicalKey(a.Key) {
			custom = append(custom, a)
		}
	}
	return e.AddAttrs(custom)
}

// finishWithError marks the event as errored, extracts error info, and processes extra attrs.
// Requirements: 1.4, 2.6
func (e *Event) finishWithError(now time.Time, err error, extractor ErrorExtractor, attrs []Attr) error {
	if extractor == nil {
		extractor = DefaultErrorExtractor
	}
	info := extractor(err)

	e.mu.Lock()
	if err := e.ensureMutableLocked(); err != nil {
		e.mu.Unlock()
		return err
	}
	if !e.FinishedAt.IsZero() {
		e.mu.Unlock()
		return &EventAlreadyFinishedError{EventID: e.EventID}
	}
	e.FinishedAt = now
	e.DurationMS = now.Sub(e.StartedAt).Milliseconds()
	e.Outcome = "error"
	e.Level = LevelError
	e.Error = info
	for _, a := range attrs {
		e.applyCanonical(a)
	}
	e.setStateLocked(EventStateFinished)
	e.mu.Unlock()

	var custom []Attr
	for _, a := range attrs {
		if !isCanonicalKey(a.Key) {
			custom = append(custom, a)
		}
	}
	return e.AddAttrs(custom)
}

// Enrich appends attrs to this event.
// Requirements: 1.3, 2.2
func (e *Event) Enrich(attrs ...Attr) error {
	return e.AddAttrs(attrs)
}

// Append appends attrs to this event.
func (e *Event) Append(attrs ...Attr) error {
	return e.AddAttrs(attrs)
}

// Add appends a value to an array field on an active event.
// If the field doesn't exist, it creates a new array with the value.
// If the field exists but is not an array, it returns an error.
// Requirements: 2.4
func (e *Event) Add(key string, value interface{}) error {
	if key == "" {
		return nil
	}
	
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if err := e.ensureMutableLocked(); err != nil {
		return err
	}
	
	// Transition to active state if in created state
	if e.state == "" || e.state == EventStateCreated {
		e.setStateLocked(EventStateActive)
	}
	
	// Find existing attribute with this key
	for i := range e.Attrs {
		if e.Attrs[i].Key == key {
			// Check if it's an array
			switch arr := e.Attrs[i].Value.(type) {
			case []interface{}:
				e.Attrs[i].Value = append(arr, value)
				return nil
			case []string:
				if strVal, ok := value.(string); ok {
					e.Attrs[i].Value = append(arr, strVal)
					return nil
				}
				// Convert to []interface{} for mixed types
				newArr := make([]interface{}, len(arr)+1)
				for j, v := range arr {
					newArr[j] = v
				}
				newArr[len(arr)] = value
				e.Attrs[i].Value = newArr
				return nil
			case []int:
				if intVal, ok := value.(int); ok {
					e.Attrs[i].Value = append(arr, intVal)
					return nil
				}
				// Convert to []interface{} for mixed types
				newArr := make([]interface{}, len(arr)+1)
				for j, v := range arr {
					newArr[j] = v
				}
				newArr[len(arr)] = value
				e.Attrs[i].Value = newArr
				return nil
			case []int64:
				if intVal, ok := value.(int64); ok {
					e.Attrs[i].Value = append(arr, intVal)
					return nil
				}
				// Convert to []interface{} for mixed types
				newArr := make([]interface{}, len(arr)+1)
				for j, v := range arr {
					newArr[j] = v
				}
				newArr[len(arr)] = value
				e.Attrs[i].Value = newArr
				return nil
			default:
				// Field exists but is not an array - convert to array
				e.Attrs[i].Value = []interface{}{arr, value}
				return nil
			}
		}
	}
	
	// Key doesn't exist - create new array field
	e.Attrs = append(e.Attrs, Attr{
		Key:   key,
		Kind:  KindAny,
		Value: []interface{}{value},
	})
	return nil
}

// Set upserts attrs by key and applies canonical fields when keys match.
// Requirements: 1.3, 2.3
func (e *Event) Set(attrs ...Attr) error {
	if len(attrs) == 0 {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.ensureMutableLocked(); err != nil {
		return err
	}
	
	// Transition to active state if in created state
	if e.state == "" || e.state == EventStateCreated {
		e.setStateLocked(EventStateActive)
	}

	for _, a := range attrs {
		if e.applyCanonical(a) {
			continue
		}
		replaced := false
		for i := range e.Attrs {
			if e.Attrs[i].Key == a.Key {
				e.Attrs[i] = a
				replaced = true
				break
			}
		}
		if !replaced {
			e.Attrs = append(e.Attrs, a)
		}
	}
	return nil
}

// Merge merges attrs into a named group.
func (e *Event) Merge(group string, attrs ...Attr) error {
	if group == "" || len(attrs) == 0 {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.ensureMutableLocked(); err != nil {
		return err
	}

	for i := range e.Attrs {
		if e.Attrs[i].Key != group || e.Attrs[i].Kind != KindGroup {
			continue
		}
		children, _ := e.Attrs[i].Value.([]Attr)
		for _, a := range attrs {
			replaced := false
			for j := range children {
				if children[j].Key == a.Key {
					children[j] = a
					replaced = true
					break
				}
			}
			if !replaced {
				children = append(children, a)
			}
		}
		e.Attrs[i].Value = children
		return nil
	}

	cp := make([]Attr, len(attrs))
	copy(cp, attrs)
	e.Attrs = append(e.Attrs, Group(group, cp...))
	return nil
}

// Delete removes attrs by key. Dot keys can target group children (e.g. "user.id").
func (e *Event) Delete(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.ensureMutableLocked(); err != nil {
		return err
	}

	for _, key := range keys {
		if key == "" {
			continue
		}
		parts := splitPath(key)
		if len(parts) == 1 {
			e.Attrs = removeAttrByKey(e.Attrs, key)
			continue
		}
		e.Attrs = removeNestedAttr(e.Attrs, parts)
	}
	return nil
}

// Get returns a value by key. Dot paths are supported.
func (e *Event) Get(key string) (any, bool) {
	if key == "" {
		return nil, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	if v, ok := lookupAttrValue(e.Attrs, key); ok {
		return v, true
	}
	switch key {
	case "event_id":
		return e.EventID, e.EventID != ""
	case "schema_version":
		return e.SchemaVersion, e.SchemaVersion != ""
	case "event_version":
		return e.EventVersion, e.EventVersion != ""
	case "request_id":
		return e.RequestID, e.RequestID != ""
	case "trace_id":
		return e.TraceID, e.TraceID != ""
	case "span_id":
		return e.SpanID, e.SpanID != ""
	case "incident_id":
		return e.IncidentID, e.IncidentID != ""
	case "service":
		return e.Service, e.Service != ""
	case "event":
		return e.Event, e.Event != ""
	case "kind":
		return e.Kind, e.Kind != ""
	case "message":
		return e.Message, e.Message != ""
	case "outcome":
		return e.Outcome, e.Outcome != ""
	case "status_code":
		return e.StatusCode, e.StatusCode != 0
	case "duration_ms":
		return e.DurationMS, e.DurationMS != 0
	case "method":
		return e.Method, e.Method != ""
	case "path":
		return e.Path, e.Path != ""
	case "route":
		return e.Route, e.Route != ""
	default:
		return nil, false
	}
}

// GetGroup returns a group as map by group key.
func (e *Event) GetGroup(name string) (map[string]any, bool) {
	if name == "" {
		return nil, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	raw, ok := lookupAttrValue(e.Attrs, name)
	if !ok {
		return nil, false
	}
	children, ok := raw.([]Attr)
	if !ok {
		return nil, false
	}
	return attrsToMap(children, true), true
}

// Checkpoint records a checkpoint on this event.
func (e *Event) Checkpoint(name string, attrs ...Attr) error {
	e.mu.Lock()
	if err := e.ensureMutableLocked(); err != nil {
		e.mu.Unlock()
		return err
	}
	start := e.StartedAt
	e.mu.Unlock()
	at := time.Since(start).Milliseconds()
	return e.AddCheckpoint(EventCheckpoint{Name: name, AtMS: at, Attrs: attrs})
}

// StartProcess begins a named process step and returns a handle to finish it later.
// The step number is auto-incremented per event (1-indexed).
func (e *Event) StartProcess(name string, attrs ...Attr) (*ProcessHandle, error) {
	e.mu.Lock()
	if err := e.ensureMutableLocked(); err != nil {
		e.mu.Unlock()
		return nil, err
	}
	e.processStep++
	step := e.processStep
	e.setStateLocked(EventStateActive)
	e.mu.Unlock()

	return &ProcessHandle{
		event:     e,
		name:      name,
		step:      step,
		startedAt: time.Now(),
	}, nil
}

// StartTimer begins a named timer and returns a handle to stop it later.
func (e *Event) StartTimer(name string, attrs ...Attr) (*TimerHandle, error) {
	e.mu.Lock()
	if err := e.ensureMutableLocked(); err != nil {
		e.mu.Unlock()
		return nil, err
	}
	e.setStateLocked(EventStateActive)
	e.mu.Unlock()

	return &TimerHandle{
		event:     e,
		name:      name,
		startedAt: time.Now(),
	}, nil
}

// StartGroup begins a named group phase and returns a handle to finish it later.
func (e *Event) StartGroup(name string, attrs ...Attr) (*GroupHandle, error) {
	e.mu.Lock()
	if err := e.ensureMutableLocked(); err != nil {
		e.mu.Unlock()
		return nil, err
	}
	e.setStateLocked(EventStateActive)
	e.mu.Unlock()

	return &GroupHandle{
		event:     e,
		name:      name,
		startedAt: time.Now(),
	}, nil
}

// Finish records a successful/completed outcome on this event.
func (e *Event) Finish(outcome string, attrs ...Attr) error {
	now := time.Now()
	e.mu.Lock()
	if e.logger != nil {
		e.logger.mu.RLock()
		now = e.logger.cfg.Clock.Now()
		e.logger.mu.RUnlock()
	}
	e.mu.Unlock()
	return e.finish(now, outcome, attrs)
}

// FinishError marks this event as failed.
func (e *Event) FinishError(err error, attrs ...Attr) error {
	now := time.Now()
	extractor := DefaultErrorExtractor
	e.mu.Lock()
	if e.logger != nil {
		e.logger.mu.RLock()
		now = e.logger.cfg.Clock.Now()
		extractor = e.logger.cfg.ErrorExtractor
		e.logger.mu.RUnlock()
	}
	e.mu.Unlock()
	return e.finishWithError(now, err, extractor, attrs)
}

// ID returns event id.
func (e *Event) ID() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.EventID
}

// Request returns request id.
func (e *Event) Request() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.RequestID
}

// Trace returns trace id.
func (e *Event) Trace() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.TraceID
}

// StartTime returns start time.
func (e *Event) StartTime() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.StartedAt
}

// Duration returns event duration.
func (e *Event) Duration() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	return time.Duration(e.DurationMS) * time.Millisecond
}

// AttrList returns a copy of event attrs.
func (e *Event) AttrList() []Attr {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Attr, len(e.Attrs))
	copy(out, e.Attrs)
	return out
}

// IsFinished reports if event finish timestamp is set.
func (e *Event) IsFinished() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return !e.FinishedAt.IsZero()
}

// IsEmitted reports if event has been emitted.
func (e *Event) IsEmitted() bool {
	return e.emitted.Load()
}

// Clone returns a deep copy of the event with emitted state reset.
func (e *Event) Clone() *Event {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	cp := &Event{
		Timestamp:     e.Timestamp,
		SchemaVersion: e.SchemaVersion,
		EventVersion:  e.EventVersion,
		EventID:       e.EventID,
		RequestID:     e.RequestID,
		TraceID:       e.TraceID,
		SpanID:        e.SpanID,
		ParentID:      e.ParentID,
		IncidentID:    e.IncidentID,
		Level:         e.Level,
		Event:         e.Event,
		Kind:          e.Kind,
		Message:       e.Message,
		Outcome:       e.Outcome,
		Service:       e.Service,
		Version:       e.Version,
		Environment:   e.Environment,
		DeploymentID:  e.DeploymentID,
		Region:        e.Region,
		Host:          e.Host,
		Runtime:       e.Runtime,
		Method:        e.Method,
		Path:          e.Path,
		Route:         e.Route,
		StatusCode:    e.StatusCode,
		DurationMS:    e.DurationMS,
		StartedAt:     e.StartedAt,
		FinishedAt:    e.FinishedAt,
		Attrs:         cloneAttrs(e.Attrs),
		Checkpoints:   cloneCheckpoints(e.Checkpoints),
		Processes:     cloneProcesses(e.Processes),
		Groups:        cloneGroups(e.Groups),
		Timers:        cloneTimers(e.Timers),
		state:         e.state,
		logger:        e.logger,
	}
	if e.Error != nil {
		errCopy := *e.Error
		cp.Error = &errCopy
	}
	return cp
}

// applyCanonical sets a canonical struct field from an Attr.
// Must hold e.mu. Returns true when a matching canonical field was updated.
func (e *Event) applyCanonical(a Attr) bool {
	switch a.Key {
	case "timestamp":
		if v, ok := a.Value.(time.Time); ok {
			e.Timestamp = v
			return true
		}
	case "event_id":
		if v, ok := a.Value.(string); ok {
			e.EventID = v
			return true
		}
	case "schema_version":
		if v, ok := a.Value.(string); ok {
			e.SchemaVersion = v
			return true
		}
	case "event_version":
		if v, ok := a.Value.(string); ok {
			e.EventVersion = v
			return true
		}
	case "request_id":
		if v, ok := a.Value.(string); ok {
			e.RequestID = v
			return true
		}
	case "trace_id":
		if v, ok := a.Value.(string); ok {
			e.TraceID = v
			return true
		}
	case "span_id":
		if v, ok := a.Value.(string); ok {
			e.SpanID = v
			return true
		}
	case "incident_id":
		if v, ok := a.Value.(string); ok {
			e.IncidentID = v
			return true
		}
	case "parent_id":
		if v, ok := a.Value.(string); ok {
			e.ParentID = v
			return true
		}
	case "level":
		switch v := a.Value.(type) {
		case Level:
			e.Level = v
			return true
		case string:
			e.Level = ParseLevel(v)
			return true
		case int:
			if v >= int(LevelDebug) && v <= int(LevelFatal) {
				e.Level = Level(v)
				return true
			}
		}
	case "event":
		if v, ok := a.Value.(string); ok {
			e.Event = v
			return true
		}
	case "kind":
		if v, ok := a.Value.(string); ok {
			e.Kind = v
			return true
		}
	case "message":
		if v, ok := a.Value.(string); ok {
			e.Message = v
			return true
		}
	case "outcome":
		if v, ok := a.Value.(string); ok {
			e.Outcome = v
			return true
		}
	case "status_code":
		if v, ok := canonicalInt(a.Value); ok {
			e.StatusCode = v
			return true
		}
	case "duration_ms":
		if v, ok := canonicalInt64(a.Value); ok {
			e.DurationMS = v
			return true
		}
	case "service":
		if v, ok := a.Value.(string); ok {
			e.Service = v
			return true
		}
	case "version":
		if v, ok := a.Value.(string); ok {
			e.Version = v
			return true
		}
	case "environment":
		if v, ok := a.Value.(string); ok {
			e.Environment = v
			return true
		}
	case "deployment_id":
		if v, ok := a.Value.(string); ok {
			e.DeploymentID = v
			return true
		}
	case "region":
		if v, ok := a.Value.(string); ok {
			e.Region = v
			return true
		}
	case "host":
		if v, ok := a.Value.(string); ok {
			e.Host = v
			return true
		}
	case "runtime":
		if v, ok := a.Value.(string); ok {
			e.Runtime = v
			return true
		}
	case "method":
		if v, ok := a.Value.(string); ok {
			e.Method = v
			return true
		}
	case "path":
		if v, ok := a.Value.(string); ok {
			e.Path = v
			return true
		}
	case "route":
		if v, ok := a.Value.(string); ok {
			e.Route = v
			return true
		}
	case "error":
		switch v := a.Value.(type) {
		case *ErrorInfo:
			e.Error = v
			return true
		case ErrorInfo:
			vv := v
			e.Error = &vv
			return true
		case error:
			e.Error = DefaultErrorExtractor(v)
			return true
		}
	case "checkpoints":
		if v, ok := a.Value.([]EventCheckpoint); ok {
			out := make([]EventCheckpoint, len(v))
			copy(out, v)
			e.Checkpoints = out
			return true
		}
	}
	return false
}

func canonicalInt(v any) (int, bool) {
	switch vv := v.(type) {
	case int:
		return vv, true
	case int64:
		return int(vv), true
	case uint64:
		return int(vv), true
	default:
		return 0, false
	}
}

func splitPath(path string) []string {
	out := make([]string, 0, 4)
	cur := ""
	for _, r := range path {
		if r == '.' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func removeAttrByKey(attrs []Attr, key string) []Attr {
	out := attrs[:0]
	for _, a := range attrs {
		if a.Key == key {
			continue
		}
		out = append(out, a)
	}
	return out
}

func removeNestedAttr(attrs []Attr, parts []string) []Attr {
	if len(parts) == 0 {
		return attrs
	}
	top := parts[0]
	out := attrs[:0]
	for _, a := range attrs {
		if a.Key != top {
			out = append(out, a)
			continue
		}
		if a.Kind != KindGroup {
			out = append(out, a)
			continue
		}
		children, _ := a.Value.([]Attr)
		children = removeNestedChild(children, parts[1:])
		if len(children) == 0 {
			continue
		}
		a.Value = children
		out = append(out, a)
	}
	return out
}

func removeNestedChild(attrs []Attr, parts []string) []Attr {
	if len(parts) == 0 {
		return attrs
	}
	if len(parts) == 1 {
		return removeAttrByKey(attrs, parts[0])
	}
	key := parts[0]
	out := attrs[:0]
	for _, a := range attrs {
		if a.Key != key || a.Kind != KindGroup {
			out = append(out, a)
			continue
		}
		children, _ := a.Value.([]Attr)
		children = removeNestedChild(children, parts[1:])
		if len(children) == 0 {
			continue
		}
		a.Value = children
		out = append(out, a)
	}
	return out
}

func (e *Event) String() string {
	return fmt.Sprintf("Event{id=%q,event=%q}", e.EventID, e.Event)
}

func cloneAttrs(attrs []Attr) []Attr {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]Attr, len(attrs))
	for i, a := range attrs {
		out[i] = Attr{
			Key:   a.Key,
			Kind:  a.Kind,
			Value: cloneAttrValue(a.Value),
		}
	}
	return out
}

func cloneAttrValue(v any) any {
	switch vv := v.(type) {
	case []Attr:
		return cloneAttrs(vv)
	case []EventCheckpoint:
		return cloneCheckpoints(vv)
	case []EventProcess:
		return cloneProcesses(vv)
	case []EventGroup:
		return cloneGroups(vv)
	case []EventTimer:
		return cloneTimers(vv)
	case *ErrorInfo:
		if vv == nil {
			return (*ErrorInfo)(nil)
		}
		cp := *vv
		return &cp
	case ErrorInfo:
		cp := vv
		return cp
	default:
		return vv
	}
}

func cloneCheckpoints(checkpoints []EventCheckpoint) []EventCheckpoint {
	if len(checkpoints) == 0 {
		return nil
	}
	out := make([]EventCheckpoint, len(checkpoints))
	for i, cp := range checkpoints {
		out[i] = EventCheckpoint{
			Name:  cp.Name,
			AtMS:  cp.AtMS,
			Attrs: cloneAttrs(cp.Attrs),
		}
	}
	return out
}

func cloneProcesses(procs []EventProcess) []EventProcess {
	if len(procs) == 0 {
		return nil
	}
	out := make([]EventProcess, len(procs))
	for i, p := range procs {
		out[i] = EventProcess{
			Step:        p.Step,
			Name:        p.Name,
			StatusCode:  p.StatusCode,
			StartedAtMS: p.StartedAtMS,
			EndedAtMS:   p.EndedAtMS,
			DurationMS:  p.DurationMS,
			Attrs:       cloneAttrs(p.Attrs),
		}
	}
	return out
}

func cloneGroups(groups []EventGroup) []EventGroup {
	if len(groups) == 0 {
		return nil
	}
	out := make([]EventGroup, len(groups))
	for i, g := range groups {
		out[i] = EventGroup{
			Name:        g.Name,
			StatusCode:  g.StatusCode,
			StartedAtMS: g.StartedAtMS,
			EndedAtMS:   g.EndedAtMS,
			DurationMS:  g.DurationMS,
			Attrs:       cloneAttrs(g.Attrs),
		}
	}
	return out
}

func cloneTimers(timers []EventTimer) []EventTimer {
	if len(timers) == 0 {
		return nil
	}
	out := make([]EventTimer, len(timers))
	for i, t := range timers {
		out[i] = EventTimer{
			Name:       t.Name,
			DurationMS: t.DurationMS,
			StatusCode: t.StatusCode,
			Attrs:      cloneAttrs(t.Attrs),
		}
	}
	return out
}

func canonicalInt64(v any) (int64, bool) {
	switch vv := v.(type) {
	case int64:
		return vv, true
	case int:
		return int64(vv), true
	case uint64:
		return int64(vv), true
	case time.Duration:
		return vv.Milliseconds(), true
	default:
		return 0, false
	}
}	// ── Canonical key lookup ──────────────────────────────────────────────────────

var canonicalKeys = map[string]struct{}{
	"timestamp": {}, "schema_version": {}, "event_version": {}, "event_id": {}, "request_id": {}, "trace_id": {},
	"span_id": {}, "parent_id": {}, "incident_id": {}, "level": {}, "event": {},
	"kind": {}, "message": {}, "outcome": {}, "duration_ms": {}, "service": {},
	"version": {}, "environment": {}, "deployment_id": {}, "region": {},
	"host": {}, "runtime": {}, "method": {}, "path": {}, "route": {},
	"status_code": {}, "error": {}, "checkpoints": {},
	"event_state": {},
}

func isCanonicalKey(key string) bool {
	_, ok := canonicalKeys[key]
	return ok
}
