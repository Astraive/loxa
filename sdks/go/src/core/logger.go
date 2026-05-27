package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/astraive/loxa/sdks/go/src/internal/pool"
	speccontract "github.com/astraive/loxa/spec/generated/go/contract"
)

// Ensure is used (pipeline only).
var _ = NewPipeline

// Logger is an instance of the LOXA-Go logging pipeline.
type Logger struct {
	cfg      Config
	mu       sync.RWMutex
	pipeline *Pipeline
}

// New creates a Logger from cfg, validating and applying defaults.
func New(cfg Config) (*Logger, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	applyConfigDefaults(&cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	l := &Logger{cfg: cfg}
	if cfg.Async.Enabled {
		l.pipeline = NewPipeline(l.makePipelineCfg())
	}
	return l, nil
}

// Child creates a nested logger with config overrides applied.
func (l *Logger) Child(options ...ConfigOption) (*Logger, error) {
	l.mu.RLock()
	cfg := l.cfg
	l.mu.RUnlock()
	cfg = ApplyConfig(cfg, options...)
	return New(cfg)
}

// Alias creates an immutable child logger that preserves config and emits loxa.alias.
func (l *Logger) Alias(name string) (*Logger, error) {
	return l.Child(WithAlias(name))
}

// WithSchema creates a nested logger with a different output schema.
func (l *Logger) WithSchema(schema Schema) (*Logger, error) {
	return l.Child(WithSchema(schema))
}

// applyConfigDefaults fills zero-value Config fields with sensible defaults.
func applyConfigDefaults(cfg *Config) {
	if cfg.IDGen == nil {
		cfg.IDGen = globalIDGen
	}
	if cfg.Clock == nil {
		cfg.Clock = realClock{}
	}
	if cfg.Encoder == nil {
		enc := JSONEncoder()
		enc.ExpandDotKeys = cfg.FieldNaming.ExpandDotKeys
		cfg.Encoder = enc
	} else if !cfg.FieldNaming.ExpandDotKeys {
		if enc, ok := cfg.Encoder.(*JSONEventEncoder); ok {
			enc.ExpandDotKeys = false
		}
	}
	if cfg.Sink != nil && len(cfg.Sinks) == 0 {
		cfg.Sinks = []Sink{cfg.Sink}
	} else if cfg.Sink != nil {
		cfg.Sinks = appendSinkIfMissing(cfg.Sinks, cfg.Sink)
	}
	if cfg.Sinks == nil && cfg.Sink == nil {
		cfg.Sinks = []Sink{StdoutSink()}
	}
	if cfg.Sampler == nil {
		cfg.Sampler = SampleAll()
	}
	if cfg.ErrorExtractor == nil {
		if cfg.IncludeSource {
			cfg.ErrorExtractor = DefaultErrorExtractor
		} else {
			cfg.ErrorExtractor = DefaultErrorExtractorNoStack
		}
	}
	if cfg.Redactor == nil && cfg.Security.RedactByDefault && !cfg.Security.AllowPII {
		cfg.Redactor = DefaultRedactor()
	}
	if cfg.Checkpoints.MaxCheckpoints == 0 {
		cfg.Checkpoints.MaxCheckpoints = 32
	}
	if cfg.Strict && !cfg.codeSetValidateEncoded {
		cfg.ValidateEncoded = true
	}
}

func appendSinkIfMissing(sinks []Sink, sink Sink) []Sink {
	for _, existing := range sinks {
		if sameSinkInstance(existing, sink) {
			return sinks
		}
	}
	return append(sinks, sink)
}

func sameSinkInstance(a, b Sink) bool {
	if a == nil || b == nil {
		return a == b
	}
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)
	if va.Type() != vb.Type() {
		return false
	}
	if va.Type().Comparable() && a == b {
		return true
	}
	if va.Kind() == reflect.Pointer && vb.Kind() == reflect.Pointer {
		return va.Pointer() == vb.Pointer()
	}
	return false
}

// makePipelineCfg builds an PipelineConfig from the logger config.
func (l *Logger) makePipelineCfg() PipelineConfig {
	sinks := make([]SinkWriter, len(l.cfg.Sinks))
	for i, s := range l.cfg.Sinks {
		sinks[i] = &sinkAdapter{s}
	}
	var fallback SinkWriter
	if l.cfg.FallbackSink != nil {
		fallback = &sinkAdapter{l.cfg.FallbackSink}
	}
	var onDrop func(string)
	var onError func(error)
	if l.cfg.StatsHandler != nil {
		stats := l.cfg.StatsHandler
		onDrop = stats.OnDrop
		onError = stats.OnError
	}
	return PipelineConfig{
		QueueSize:     l.cfg.Async.QueueSize,
		Workers:       l.cfg.Async.Workers,
		FlushInterval: l.cfg.Async.FlushInterval,
		MaxBatchBytes: l.cfg.Async.MaxBatchBytes,
		Backpressure:  BackpressurePolicy(l.cfg.Async.Backpressure),
		Sinks:         sinks,
		Fallback:      fallback,
		OnDrop:        onDrop,
		OnError:       onError,
	}
}

// sinkAdapter wraps a loxa.Sink to satisfy SinkWriter (avoids import cycle).
type sinkAdapter struct{ Sink }

func (a *sinkAdapter) WriteEvent(ctx context.Context, encoded []byte, ev *Event) error {
	return a.Sink.WriteEvent(ctx, encoded, ev)
}

type batchSink interface {
	WriteBatch(ctx context.Context, encoded [][]byte, events []*Event) error
}

func (a *sinkAdapter) WriteBatch(ctx context.Context, items []PipelineItem) error {
	bs, ok := a.Sink.(batchSink)
	if !ok {
		for _, item := range items {
			if err := a.Sink.WriteEvent(ctx, item.Encoded, item.Event); err != nil {
				return err
			}
		}
		return nil
	}
	encoded := make([][]byte, 0, len(items))
	events := make([]*Event, 0, len(items))
	for _, item := range items {
		encoded = append(encoded, item.Encoded)
		events = append(events, item.Event)
	}
	return bs.WriteBatch(ctx, encoded, events)
}

// PanicRecoveryEnabled reports whether runtime wrappers should recover panics.
func (l *Logger) PanicRecoveryEnabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.cfg.PanicRecovery
}

// ── Lifecycle methods on Logger ───────────────────────────────────────────────

// StartEvent begins a canonical wide event, stores it in ctx, and returns the new ctx.
// Requirements: 39.1, 39.2, 39.3, 39.4, 39.5, 39.6, 39.7, 39.8
func (l *Logger) StartEvent(ctx context.Context, params Params) context.Context {
	l.mu.RLock()
	cfg := l.cfg
	l.mu.RUnlock()

	// Extract trace context from context if not provided in params.
	// Skip OTel extraction when OTel bridge is not configured to avoid
	// the ~50-100ns context.Value lookup on every StartEvent.
	// Requirements: 39.2, 39.8
	if cfg.OTelBridge && (params.TraceID == "" || params.SpanID == "") {
		otelTraceID, otelSpanID := TraceFromOTel(ctx)
		if params.TraceID == "" && otelTraceID != "" {
			params.TraceID = otelTraceID
		}
		if params.SpanID == "" && otelSpanID != "" {
			// When extracting from context, the current span becomes the parent
			// and we generate a new span ID for this event
			params.ParentID = otelSpanID
		}
	}

	ev := buildEvent(params, &cfg)
	ev.SetLogger(l)
	notifyEventCreated(cfg.StatsHandler)
	return storeEvent(ctx, ev)
}

// Event emits a simple success event with optional attrs.
func (l *Logger) Event(ctx context.Context, name string, attrs ...Attr) error {
	evCtx := l.StartEvent(ctx, Params{Event: name})
	if len(attrs) > 0 {
		if err := l.Enrich(evCtx, attrs...); err != nil {
			return err
		}
	}
	if err := l.Finish(evCtx, "success"); err != nil {
		return err
	}
	return l.Emit(evCtx)
}

// Enrich appends attrs to the canonical event in ctx.
func (l *Logger) Enrich(ctx context.Context, attrs ...Attr) error {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil
	}
	return ev.AddAttrs(attrs)
}

// Append appends attrs to the canonical event in ctx.
func (l *Logger) Append(ctx context.Context, attrs ...Attr) error {
	return l.Enrich(ctx, attrs...)
}

// Add appends a value to an array field on the active event.
// Requirements: 2.4
func (l *Logger) Add(ctx context.Context, key string, value interface{}) error {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil
	}
	return ev.Add(key, value)
}

// EnrichGroup appends attrs as a named group to the event in ctx.
func (l *Logger) EnrichGroup(ctx context.Context, key string, attrs ...Attr) error {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil
	}
	return ev.AddAttrs([]Attr{Group(key, attrs...)})
}

// Merge merges attrs into a named group on the active event.
func (l *Logger) Merge(ctx context.Context, group string, attrs ...Attr) error {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil
	}
	return ev.Merge(group, attrs...)
}

// Set upserts attrs on the active event.
func (l *Logger) Set(ctx context.Context, attrs ...Attr) error {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil
	}
	return ev.Set(attrs...)
}

// Delete removes attrs by key from the active event.
func (l *Logger) Delete(ctx context.Context, keys ...string) error {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil
	}
	return ev.Delete(keys...)
}

// Get reads a value by key (dot-path supported) from the active event.
func (l *Logger) Get(ctx context.Context, key string) (any, bool) {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil, false
	}
	return ev.Get(key)
}

// GetGroup reads a group object by key from the active event.
func (l *Logger) GetGroup(ctx context.Context, key string) (map[string]any, bool) {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil, false
	}
	return ev.GetGroup(key)
}

// Finish records the outcome and computes duration.
func (l *Logger) Finish(ctx context.Context, outcome string, attrs ...Attr) error {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil
	}
	l.mu.RLock()
	clock := l.cfg.Clock
	stats := l.cfg.StatsHandler
	l.mu.RUnlock()
	if err := ev.finish(clock.Now(), outcome, attrs); err != nil {
		return err
	}
	notifyEventFinished(stats)
	return nil
}

// FinishError records an error outcome, extracts error metadata, and computes duration.
func (l *Logger) FinishError(ctx context.Context, err error, attrs ...Attr) error {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil
	}
	l.mu.RLock()
	clock := l.cfg.Clock
	extractor := l.cfg.ErrorExtractor
	stats := l.cfg.StatsHandler
	l.mu.RUnlock()
	if ferr := ev.finishWithError(clock.Now(), err, extractor, attrs); ferr != nil {
		return ferr
	}
	notifyEventFinished(stats)
	return nil
}

// Checkpoint records a named breadcrumb inside the event.
func (l *Logger) Checkpoint(ctx context.Context, name string, attrs ...Attr) error {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil
	}
	l.mu.RLock()
	cfg := l.cfg
	l.mu.RUnlock()

	if !cfg.Checkpoints.Enabled {
		return nil
	}

	ev.MuLock()
	count := len(ev.Checkpoints)
	if err := ev.ensureMutableLocked(); err != nil {
		ev.MuUnlock()
		return err
	}
	ev.MuUnlock()

	if count >= cfg.Checkpoints.MaxCheckpoints {
		return nil
	}

	atMS := cfg.Clock.Now().Sub(ev.StartedAt).Milliseconds()
	if err := ev.AddCheckpoint(EventCheckpoint{Name: name, AtMS: atMS, Attrs: attrs}); err != nil {
		return err
	}
	if !cfg.Checkpoints.EmitImmediately {
		return nil
	}

	snapshot := ev.Clone()
	snapshot.Timestamp = cfg.Clock.Now()
	snapshot.EventID = cfg.IDGen.NewID()
	snapshot.Kind = "checkpoint"
	snapshot.Event = "checkpoint." + name
	snapshot.Message = name
	snapshot.DurationMS = atMS
	snapshot.FinishedAt = cfg.Clock.Now()
	snapshot.Attrs = cloneAttrs(attrs)
	snapshot.Checkpoints = nil
	snapshot.logger = nil
	if err := l.EmitEventWithContext(ctx, snapshot); err != nil {
		fmt.Fprintf(os.Stderr, "[loxa] checkpoint emit error: %v\n", err)
	}
	return nil
}

// Process starts a named process step and returns a handle to finish it.
func (l *Logger) Process(ctx context.Context, name string, attrs ...Attr) (*ProcessHandle, error) {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil, nil
	}
	return ev.StartProcess(name, attrs...)
}

// StartProcess is an alias for Process.
func (l *Logger) StartProcess(ctx context.Context, name string, attrs ...Attr) (*ProcessHandle, error) {
	return l.Process(ctx, name, attrs...)
}

// FinishProcess completes a process handle.
func (l *Logger) FinishProcess(h *ProcessHandle, attrs ...Attr) error {
	if h == nil {
		return nil
	}
	return h.Finish(attrs...)
}

// FinishProcessError completes a process handle with error metadata.
func (l *Logger) FinishProcessError(h *ProcessHandle, err error, statusCode int, attrs ...Attr) error {
	if h == nil {
		return nil
	}
	return h.FinishError(err, statusCode, attrs...)
}

// StartTimer starts a named timer and returns a handle to stop it.
func (l *Logger) StartTimer(ctx context.Context, name string, attrs ...Attr) (*TimerHandle, error) {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil, nil
	}
	return ev.StartTimer(name, attrs...)
}

// Timer is an alias for StartTimer.
func (l *Logger) Timer(ctx context.Context, name string, attrs ...Attr) (*TimerHandle, error) {
	return l.StartTimer(ctx, name, attrs...)
}

// StopTimer completes a timer handle.
func (l *Logger) StopTimer(h *TimerHandle, attrs ...Attr) error {
	if h == nil {
		return nil
	}
	return h.Stop(attrs...)
}

// StartGroup starts a named group phase and returns a handle to finish it.
func (l *Logger) StartGroup(ctx context.Context, name string, attrs ...Attr) (*GroupHandle, error) {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil, nil
	}
	return ev.StartGroup(name, attrs...)
}

// FinishGroup completes a group handle.
func (l *Logger) FinishGroup(h *GroupHandle, attrs ...Attr) error {
	if h == nil {
		return nil
	}
	return h.Finish(attrs...)
}

// FinishGroupError completes a group handle with error metadata.
func (l *Logger) FinishGroupError(h *GroupHandle, err error, attrs ...Attr) error {
	if h == nil {
		return nil
	}
	return FinishGroupError(h, err, attrs...)
}

// Emit encodes and delivers the canonical event in ctx to all sinks.
// Idempotent — safe to call via defer and also explicitly.
func (l *Logger) Emit(ctx context.Context) error {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil
	}
	l.mu.RLock()
	enricher := l.cfg.Enricher
	l.mu.RUnlock()
	if enricher != nil {
		if err := ev.AddAttrs(enricher(ctx)); err != nil {
			return err
		}
	}
	return l.EmitEventWithContext(ctx, ev)
}

// EmitEvent encodes and delivers ev directly.
func (l *Logger) EmitEvent(ev *Event) error {
	return l.EmitEventWithContext(context.Background(), ev)
}

// EmitEventWithContext encodes and delivers ev with the given context.
func (l *Logger) EmitEventWithContext(ctx context.Context, ev *Event) error {
	l.mu.RLock()
	cfg := l.cfg
	pipeline := l.pipeline
	l.mu.RUnlock()
	stats := cfg.StatsHandler
	startedAt := time.Now()
	defer func() { notifyEmitDuration(stats, time.Since(startedAt)) }()

	notifyDrop := func(reason string) {
		if stats != nil {
			stats.OnDrop(reason)
		}
	}
	notifyEmit := func() {
		if stats != nil {
			stats.OnEmit(ev)
		}
	}
	notifyDeliveryFailed := func(err error) {
		if err != nil {
			if handler, ok := stats.(DeliveryFailureHandler); ok {
				handler.OnDeliveryFailed(ev, err)
			}
		}
	}
	notifyError := func(err error) {
		if err != nil && stats != nil {
			stats.OnError(err)
		}
	}

	state := ev.State()
	switch state {
	case EventStateEmitted:
		return nil
	case EventStateFailedValidation, EventStateDeliveryFailed, EventStateEmitting:
		return &EventClosedError{EventID: ev.EventID, State: state}
	}

	// Sampling
	if cfg.Sampler != nil && !cfg.Sampler.ShouldSample(ev) {
		notifyDrop("sampled_out")
		if err := ev.beginEmit(); err != nil {
			return err
		}
		ev.markEmitted()
		return nil
	}
	if cfg.Strict {
		if err := validateStrictEvent(ev, cfg); err != nil {
			notifyError(err)
			ev.markValidationFailed()
			return err
		}
	}
	if err := applyDuplicateFieldPolicy(ev, cfg.DuplicateFieldPolicy); err != nil {
		notifyError(err)
		ev.markValidationFailed()
		return err
	}
	if err := ev.beginEmit(); err != nil {
		notifyDrop("already_closed")
		return err
	}

	// Generate trace/span IDs after sampling so sampled-out events skip PRNG cost.
	ev.ensureTraceContext()

	deliverEv := ev
	if cfg.Redactor != nil || cfg.Security.MaxAttrCount > 0 || cfg.Security.MaxFieldBytes > 0 || (cfg.Security.RedactByDefault && !cfg.Security.AllowPII) {
		deliverEv = ev.Clone()
		if deliverEv != nil {
			deliverEv.logger = nil
			if cfg.Redactor != nil {
				deliverEv.Attrs = applyRedactor(deliverEv.Attrs, cfg.Redactor)
			}
			if cfg.Security.MaxAttrCount > 0 || cfg.Security.MaxFieldBytes > 0 || (cfg.Security.RedactByDefault && !cfg.Security.AllowPII) {
				deliverEv.Attrs = applySecurity(deliverEv.Attrs, cfg.Security)
			}
		}
	}

	// Encoding
	buf := pool.Get()
	var err error
	if cfg.Schema != nil {
		var out []byte
		out, err = cfg.Schema.Encode(newEventView(deliverEv))
		if err == nil {
			buf = append(buf[:0], out...)
			if len(buf) == 0 || buf[len(buf)-1] != '\n' {
				buf = append(buf, '\n')
			}
		}
	} else {
		buf, err = cfg.Encoder.EncodeEvent(buf, deliverEv)
	}
	if err != nil {
		pool.Put(buf)
		notifyError(err)
		ev.markValidationFailed()
		return fmt.Errorf("loxa: encode: %w", err)
	}
	if cfg.Security.MaxEventBytes > 0 && len(buf) > cfg.Security.MaxEventBytes && cfg.Security.DropOversizedEvents {
		pool.Put(buf)
		notifyDrop("oversized_event")
		ev.markDeliveryFailed()
		return nil
	}
	if cfg.Strict && cfg.Security.MaxEventBytes > 0 && len(buf) > cfg.Security.MaxEventBytes {
		pool.Put(buf)
		ev.markValidationFailed()
		return fmt.Errorf("loxa: strict mode: event exceeds max_event_bytes (%d > %d)", len(buf), cfg.Security.MaxEventBytes)
	}
	if cfg.Strict && cfg.ValidateEncoded {
		if err := speccontract.ValidateEventBytes(bytes.TrimSpace(buf), true); err != nil {
			pool.Put(buf)
			ev.markValidationFailed()
			return fmt.Errorf("loxa: strict mode: generated spec validation failed: %w", err)
		}
	}

	// Delivery
	if cfg.Async.Enabled && pipeline != nil {
		item := PipelineItem{
			Encoded: buf,
			Event:   deliverEv,
			Level:   int(ev.Level),
			IsError: ev.Error != nil || ev.Level >= LevelError,
		}
		var enqueued bool
		enqueued, err = pipeline.Enqueue(item)
		if err != nil {
			notifyError(err)
			ev.markDeliveryFailed()
			notifyDeliveryFailed(err)
		} else if enqueued {
			notifyEmit()
			ev.markEmitted()
		} else {
			ev.markDeliveryFailed()
			notifyDeliveryFailed(errors.New("async enqueue rejected"))
		}
		} else {
			failed := false
			delivered := len(cfg.Sinks) == 0
			for _, s := range cfg.Sinks {
				if werr := s.WriteEvent(ctx, buf, deliverEv); werr != nil {
					notifyError(werr)
					failed = true
					err = werr
				} else {
					delivered = true
				}
			}
		if failed && cfg.FallbackSink != nil {
			if ferr := cfg.FallbackSink.WriteEvent(ctx, buf, deliverEv); ferr != nil {
				notifyError(ferr)
				err = ferr
			} else {
				delivered = true
			}
		}
		if delivered {
			notifyEmit()
			ev.markEmitted()
		} else {
			ev.markDeliveryFailed()
			notifyDeliveryFailed(err)
		}
	}

	// Safe to return to pool: sync sinks process synchronously, and async pipeline copies bytes.
	pool.Put(buf)
	return err
}

type eventCreatedObserver interface {
	OnEventCreated()
}

type eventFinishedObserver interface {
	OnEventFinished()
}

type emitDurationObserver interface {
	ObserveEmitDuration(time.Duration)
}

func notifyEventCreated(handler StatsHandler) {
	if observer, ok := handler.(eventCreatedObserver); ok {
		observer.OnEventCreated()
	}
}

func notifyEventFinished(handler StatsHandler) {
	if observer, ok := handler.(eventFinishedObserver); ok {
		observer.OnEventFinished()
	}
}

func notifyEmitDuration(handler StatsHandler, duration time.Duration) {
	if observer, ok := handler.(emitDurationObserver); ok {
		observer.ObserveEmitDuration(duration)
	}
}

func validateStrictEvent(ev *Event, cfg Config) error {
	ev.MuLock()
	defer ev.MuUnlock()

	if ev.Event == "" {
		return errors.New("loxa: strict mode: missing event name")
	}
	if ev.Service == "" {
		return errors.New("loxa: strict mode: missing service")
	}
	return validateStrictAttrs(ev.Attrs)
}

func validateStrictAttrs(attrs []Attr) error {
	for _, a := range attrs {
		if a.Key == "" {
			return errors.New("loxa: strict mode: empty attr key")
		}
		if !strictAttrKeyPattern.MatchString(a.Key) {
			return fmt.Errorf("loxa: strict mode: invalid attr key %q", a.Key)
		}
		if IsCanonical(a.Key) {
			return fmt.Errorf("loxa: strict mode: custom attr collides with canonical key %q", a.Key)
		}
		switch a.Kind {
		case KindString, KindInt, KindInt64, KindUint64, KindFloat64, KindBool, KindTime, KindDuration, KindStringer, KindError, KindNull:
		case KindAny:
			if _, err := json.Marshal(a.Value); err != nil {
				return fmt.Errorf("loxa: strict mode: attr %q has non-serializable any value: %w", a.Key, err)
			}
		case KindGroup:
			children, ok := a.Value.([]Attr)
			if !ok {
				return fmt.Errorf("loxa: strict mode: group attr %q has invalid value type", a.Key)
			}
			if err := validateStrictAttrs(children); err != nil {
				return err
			}
		default:
			return fmt.Errorf("loxa: strict mode: unsupported attr kind %d for key %q", a.Kind, a.Key)
		}
	}
	return nil
}

// Flush drains the async queue and flushes all sinks.
func (l *Logger) Flush(ctx context.Context) error {
	l.mu.RLock()
	pipeline := l.pipeline
	sinks := l.cfg.Sinks
	l.mu.RUnlock()

	if pipeline != nil {
		if err := pipeline.Flush(ctx); err != nil {
			return err
		}
	}
	var errs []error
	for _, s := range sinks {
		if err := s.Flush(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Shutdown drains, flushes, and closes all sinks.
func (l *Logger) Shutdown(ctx context.Context) error {
	l.mu.RLock()
	pipeline := l.pipeline
	sinks := l.cfg.Sinks
	l.mu.RUnlock()

	if pipeline != nil {
		if err := pipeline.Shutdown(ctx); err != nil {
			return err
		}
	}
	var errs []error
	for _, s := range sinks {
		if err := s.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Close is an alias for Shutdown. It implements Requirement 2.9 and 32.10.
func (l *Logger) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return l.Shutdown(ctx)
}

// Config returns a copy of the logger's current configuration.
func (l *Logger) Config() Config {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.cfg
}

// ── Immediate logging ─────────────────────────────────────────────────────────

func (l *Logger) logImmediate(ctx context.Context, level Level, msg, eventName string, err error, attrs []Attr) {
	if ctx == nil {
		ctx = context.Background()
	}

	l.mu.RLock()
	cfg := l.cfg
	l.mu.RUnlock()
	if level < cfg.Level {
		return
	}

	ev := buildEvent(Params{
		Level:   level,
		Event:   eventName,
		Kind:    "log",
		Message: msg,
	}, &cfg)

	if err != nil {
		ev.SetError(cfg.ErrorExtractor(err))
		ev.SetOutcome("error")
	}
	if addErr := ev.AddAttrs(attrs); addErr != nil {
		fmt.Fprintf(os.Stderr, "[loxa] add attrs error: %v\n", addErr)
	}
	if emitErr := l.EmitEventWithContext(ctx, ev); emitErr != nil {
		fmt.Fprintf(os.Stderr, "[loxa] emit error: %v\n", emitErr)
	}
}

// DebugContext emits an immediate debug log line with explicit context and event name.
func (l *Logger) DebugContext(ctx context.Context, msg, event string, attrs ...Attr) {
	l.logImmediate(ctx, LevelDebug, msg, event, nil, attrs)
}

// InfoContext emits an immediate info log line with explicit context and event name.
func (l *Logger) InfoContext(ctx context.Context, msg, event string, attrs ...Attr) {
	l.logImmediate(ctx, LevelInfo, msg, event, nil, attrs)
}

// WarnContext emits an immediate warn log line with explicit context and event name.
func (l *Logger) WarnContext(ctx context.Context, msg, event string, attrs ...Attr) {
	l.logImmediate(ctx, LevelWarn, msg, event, nil, attrs)
}

// ErrorContext emits an immediate error log line with explicit context and event name.
func (l *Logger) ErrorContext(ctx context.Context, msg string, err error, event string, attrs ...Attr) {
	l.logImmediate(ctx, LevelError, msg, event, err, attrs)
}

// Debug emits an immediate debug log line.
func (l *Logger) Debug(msg string, attrs ...Attr) {
	l.logImmediate(context.Background(), LevelDebug, msg, "log.debug", nil, attrs)
}

// Info emits an immediate info log line.
func (l *Logger) Info(msg string, attrs ...Attr) {
	l.logImmediate(context.Background(), LevelInfo, msg, "log.info", nil, attrs)
}

// Warn emits an immediate warn log line.
func (l *Logger) Warn(msg string, attrs ...Attr) {
	l.logImmediate(context.Background(), LevelWarn, msg, "log.warn", nil, attrs)
}

// Error emits an immediate error log line.
func (l *Logger) Error(msg string, attrs ...Attr) {
	l.logImmediate(context.Background(), LevelError, msg, "log.error", nil, attrs)
}

// Fatal emits an immediate fatal log line and exits the process.
func (l *Logger) Fatal(msg string, attrs ...Attr) {
	l.logImmediate(context.Background(), LevelFatal, msg, "log.fatal", nil, attrs)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = l.Flush(ctx)
	os.Exit(1)
}

// FatalContext emits an immediate fatal log line with explicit context and exits the process.
func (l *Logger) FatalContext(ctx context.Context, msg string, err error, event string, attrs ...Attr) {
	l.logImmediate(ctx, LevelFatal, msg, event, err, attrs)
	flushCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = l.Flush(flushCtx)
	os.Exit(1)
}

// ── Lifecycle outcome helpers ──────────────────────────────────────────────────

// Drop marks the event as dropped with a reason and emits it.
func (l *Logger) Drop(ctx context.Context, reason string) error {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil
	}
	l.mu.RLock()
	clock := l.cfg.Clock
	l.mu.RUnlock()
	if err := ev.finish(clock.Now(), "dropped", []Attr{String("drop_reason", reason)}); err != nil {
		return err
	}
	return l.Emit(ctx)
}

// Cancel marks the event as cancelled with a reason and emits it.
func (l *Logger) Cancel(ctx context.Context, reason string) error {
	return l.Finish(ctx, "cancelled", String("cancel_reason", reason))
}

// Abandon marks the event as abandoned with a reason and emits it.
func (l *Logger) Abandon(ctx context.Context, reason string) error {
	return l.Finish(ctx, "abandoned", String("abandon_reason", reason))
}

// Retry marks the event for retry with attrs and emits it.
func (l *Logger) Retry(ctx context.Context, attrs ...Attr) error {
	return l.Finish(ctx, "retried", attrs...)
}

// Partial marks the event as partially completed with attrs and emits it.
func (l *Logger) Partial(ctx context.Context, attrs ...Attr) error {
	return l.Finish(ctx, "partial", attrs...)
}

// CloneEvent clones the event in ctx and returns a standalone copy.
func (l *Logger) CloneEvent(ctx context.Context) (*Event, error) {
	ev := loadEvent(ctx)
	if ev == nil {
		return nil, nil
	}
	cp := ev.Clone()
	cp.logger = l
	return cp, nil
}

// LinkEvent creates a linked child event from the current event in ctx.
func (l *Logger) LinkEvent(ctx context.Context, target string, attrs ...Attr) (context.Context, error) {
	ev := loadEvent(ctx)
	if ev == nil {
		return ctx, nil
	}
	ev.MuLock()
	params := Params{
		Event:   target,
		Kind:    ev.Kind,
		TraceID: ev.TraceID,
		SpanID:  ev.SpanID,
		Service: ev.Service,
	}
	ev.MuUnlock()
	childCtx := l.StartEvent(ctx, params)
	if len(attrs) > 0 {
		_ = l.Enrich(childCtx, attrs...)
	}
	return childCtx, nil
}

// CurrentEvent returns the active event from ctx.
func (l *Logger) CurrentEvent(ctx context.Context) (*Event, bool) {
	return FromContext(ctx)
}

// BindEvent wraps fn with the event lifecycle, similar to RunEvent but returns directly.
func BindEvent(ctx context.Context, params Params, fn EventFunc, finishAttrs ...Attr) error {
	return RunEvent(ctx, params, fn, finishAttrs...)
}

// Wrap wraps fn in a named event lifecycle returning the error.
func Wrap(name string, fn func() error) error {
	ctx := context.Background()
	return RunEvent(ctx, Params{Event: name}, func(_ context.Context) error {
		return fn()
	})
}

// ── Logging helper methods ────────────────────────────────────────────────────

// Notice emits an immediate notice log line.
func (l *Logger) Notice(msg string, attrs ...Attr) {
	l.logImmediate(context.Background(), LevelNotice, msg, "log.notice", nil, attrs)
}

// NoticeContext emits an immediate notice log line with explicit context and event name.
func (l *Logger) NoticeContext(ctx context.Context, msg, event string, attrs ...Attr) {
	l.logImmediate(ctx, LevelNotice, msg, event, nil, attrs)
}

// Track logs a track event at info level.
func (l *Logger) Track(name string, attrs ...Attr) {
	l.logImmediate(context.Background(), LevelInfo, name, name, nil, attrs)
}

// Audit logs an audit event at info level.
func (l *Logger) Audit(name string, attrs ...Attr) {
	l.logImmediate(context.Background(), LevelInfo, name, "audit."+name, nil, attrs)
}

// Security logs a security event at warn level.
func (l *Logger) Security(name string, attrs ...Attr) {
	l.logImmediate(context.Background(), LevelWarn, name, "security."+name, nil, attrs)
}

// Metric logs a metric measurement at info level.
func (l *Logger) Metric(name string, value float64, attrs ...Attr) {
	attrs = append([]Attr{Float64("value", value)}, attrs...)
	l.logImmediate(context.Background(), LevelInfo, name, "metric."+name, nil, attrs)
}

// Count logs a count metric at info level.
func (l *Logger) Count(name string, value int64, attrs ...Attr) {
	attrs = append([]Attr{Int64("count", value)}, attrs...)
	l.logImmediate(context.Background(), LevelInfo, name, "metric."+name, nil, attrs)
}

// Gauge logs a gauge metric at info level.
func (l *Logger) Gauge(name string, value float64, attrs ...Attr) {
	attrs = append([]Attr{Float64("gauge", value)}, attrs...)
	l.logImmediate(context.Background(), LevelInfo, name, "metric."+name, nil, attrs)
}

// Histogram logs a histogram observation at info level.
func (l *Logger) Histogram(name string, value float64, attrs ...Attr) {
	attrs = append([]Attr{Float64("observation", value)}, attrs...)
	l.logImmediate(context.Background(), LevelInfo, name, "metric."+name, nil, attrs)
}

// Breadcrumb logs a breadcrumb at debug level for tracing user flows.
func (l *Logger) Breadcrumb(name string, attrs ...Attr) {
	l.logImmediate(context.Background(), LevelDebug, name, "breadcrumb."+name, nil, attrs)
}

// ── Global default logger ─────────────────────────────────────────────────────

var (
	defaultLoggerMu sync.RWMutex
	defaultLog      *Logger
)

func init() {
	defaultLog = newDefaultLogger()
}

// Configure replaces the global default logger with a new one built from cfg.
// The previous default logger is drained/shutdown to avoid losing queued events.
func Configure(cfg Config) error {
	l, err := New(cfg)
	if err != nil {
		return err
	}
	defaultLoggerMu.Lock()
	old := defaultLog
	defaultLog = l
	defaultLoggerMu.Unlock()

	if old != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := old.Shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("loxa: drain previous logger: %w", err)
		}
	}
	return nil
}

// SetDefault replaces the global default logger instance.
func SetDefault(l *Logger) {
	if l == nil {
		return
	}
	defaultLoggerMu.Lock()
	defaultLog = l
	defaultLoggerMu.Unlock()
}

// PanicRecoveryEnabled reports whether the default logger recovers panics in wrappers.
func PanicRecoveryEnabled() bool {
	return Default().PanicRecoveryEnabled()
}

// Default returns the global default Logger.
func Default() *Logger {
	defaultLoggerMu.RLock()
	l := defaultLog
	defaultLoggerMu.RUnlock()
	if l != nil {
		return l
	}
	defaultLoggerMu.Lock()
	defer defaultLoggerMu.Unlock()
	if defaultLog != nil {
		return defaultLog
	}
	fallback, err := New(Dev())
	if err != nil {
		return newDefaultLogger()
	}
	defaultLog = fallback
	return defaultLog
}

func newDefaultLogger() *Logger {
	cfg := Dev()
	fallback, err := New(cfg)
	if err == nil {
		return fallback
	}
	applyConfigDefaults(&cfg)
	return &Logger{cfg: cfg}
}
