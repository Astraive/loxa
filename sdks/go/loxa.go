// Package loxa provides canonical wide-event logging for Go services.
package loxa

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/astraive/loxa/sdks/go/src/core"
	"github.com/astraive/loxa/sdks/go/src/cortex"
)

// ── Types ───────────────────────────────────────────────────────────────────

type (
	// Attr is a typed key-value pair.
	Attr = core.Attr
	// Event is the canonical wide event.
	Event = core.Event
	// Params carries metadata to start an event.
	Params = core.Params
	// Logger is an instance of the logging pipeline.
	Logger = core.Logger
	// Config is the SDK configuration.
	Config = core.Config
	// FileConfig is the YAML-serializable SDK configuration.
	FileConfig = core.FileConfig
	// Level is the log level.
	Level = core.Level
	// Sink is the interface for event destinations.
	Sink = core.Sink
	// Sampler decides whether an event should be emitted.
	Sampler = core.Sampler
	// Encoder serializes events for sink delivery.
	Encoder = core.Encoder
	// Redactor masks sensitive data.
	Redactor = core.Redactor
	// SecurityConfig controls event-size and sensitive-data limits.
	SecurityConfig = core.SecurityConfig
	// BackpressurePolicy determines what happens when the async queue is full.
	BackpressurePolicy = core.BackpressurePolicy
	// DuplicateFieldPolicy controls custom attr collisions with canonical fields.
	DuplicateFieldPolicy = core.DuplicateFieldPolicy
	// DuplicateFieldError is returned for ErrorOnDuplicate policy violations.
	DuplicateFieldError = core.DuplicateFieldError
	// DuplicateEmitError is returned when Emit is called after emitted.
	DuplicateEmitError = core.DuplicateEmitError
	// EventClosedError is returned when mutating or finishing a closed event.
	EventClosedError = core.EventClosedError
	// EventAlreadyFinishedError is returned when finishing twice.
	EventAlreadyFinishedError = core.EventAlreadyFinishedError
	// EventState is the canonical event lifecycle state.
	EventState = core.EventState
	// ConfigValidationError is returned when config validation fails.
	ConfigValidationError = core.ConfigValidationError
	// EventFunc wraps operation code in lifecycle helpers.
	EventFunc = core.EventFunc
	// Schema controls final output shape for emitted events.
	Schema = core.Schema
	// EventView is a read-only event view for schemas.
	EventView = core.EventView
	// SchemaFunc maps EventView to output object.
	SchemaFunc = core.SchemaFunc
	// ContextEnricher derives attrs from context during Emit(ctx).
	ContextEnricher = core.ContextEnricher
	// StatsHandler receives logger telemetry callbacks.
	StatsHandler = core.StatsHandler
	// DeliveryFailureHandler receives explicit delivery-failure callbacks.
	DeliveryFailureHandler = core.DeliveryFailureHandler
	// MetricsCollector exposes Prometheus metrics for SDK and transport observability.
	MetricsCollector = core.MetricsCollector
	// MetricsSnapshot is the stable cross-SDK metrics snapshot placeholder.
	MetricsSnapshot = map[string]any
	// PrometheusStatsHandler wraps a MetricsCollector for StatsHandler integration.
	PrometheusStatsHandler = core.PrometheusStatsHandler
	// ConfigOption mutates and returns Config.
	ConfigOption = core.ConfigOption

	// ── Sink config types ────────────────────────────────────────────────────

	// HTTPBatchSinkConfig configures the HTTP batch sink.
	HTTPBatchSinkConfig = core.HTTPBatchSinkConfig
	// RotatingFileConfig configures the rotating file sink.
	RotatingFileConfig = core.RotatingFileConfig
	// MemorySinkStore holds captured events for testing.
	MemorySinkStore = core.MemorySinkStore
	// CollectorSinkConfig configures the collector sink.
	CollectorSinkConfig = core.CollectorSinkConfig
	// JSONEventEncoder is the default JSON encoder.
	JSONEventEncoder = core.JSONEventEncoder

	// ── Cortex types ──────────────────────────────────────────────────────────

	// CortexClient is an HTTP client for the Cortex incident intelligence API.
	CortexClient = cortex.Client
	// IncidentContext is the result of incident reconstruction.
	IncidentContext = cortex.IncidentContext
	// GraphView represents a service or incident dependency graph.
	GraphView = cortex.GraphView
	// Remediation records a remediation action taken for an incident.
	Remediation = cortex.Remediation
	// RemediationFeedback records the outcome of a remediation action.
	RemediationFeedback = cortex.RemediationFeedback

	// ── Timing types ─────────────────────────────────────────────────────────

	// ProcessHandle tracks a running process step.
	ProcessHandle = core.ProcessHandle
	// TimerHandle tracks a running timer.
	TimerHandle = core.TimerHandle
	// GroupHandle tracks a running group phase.
	GroupHandle = core.GroupHandle
	// StopwatchHandle is a standalone elapsed-time measurer.
	StopwatchHandle = core.StopwatchHandle
)

var (
	// ErrInvalidConfig indicates config validation failure.
	ErrInvalidConfig = core.ErrInvalidConfig
	// ErrConfigFileNotFound is returned when no config file is found.
	ErrConfigFileNotFound = core.ErrConfigFileNotFound
)

// ── Constants ────────────────────────────────────────────────────────────────

const (
	LOXA_SPEC_VERSION       = core.LOXA_SPEC_VERSION
	LOXA_INGEST_API_VERSION = core.LOXA_INGEST_API_VERSION
	LOXA_EVENT_VERSION      = core.LOXA_EVENT_VERSION

	LevelDebug = core.LevelDebug
	LevelInfo  = core.LevelInfo
	LevelWarn  = core.LevelWarn
	LevelError = core.LevelError
	LevelFatal = core.LevelFatal

	// Backpressure policies
	Block        = core.Block
	DropNewest   = core.DropNewest
	DropOldest   = core.DropOldest
	DropDebug    = core.DropDebug
	DropSampled  = core.DropSampled
	SyncFallback = core.SyncFallback

	// Duplicate field policies
	CanonicalWins      = core.CanonicalWins
	AttrWins           = core.AttrWins // Deprecated: Use UserWins.
	AttrsWin           = core.AttrsWin // Deprecated: Use UserWins.
	UserWins           = core.UserWins
	FirstWins          = core.FirstWins
	LastWins           = core.LastWins
	KeepBothUnderAttrs = core.KeepBothUnderAttrs // Deprecated: Use KeepBoth.
	DropDuplicateAttr  = core.DropDuplicateAttr  // Deprecated: Use CanonicalWins.
	ErrorOnDuplicate   = core.ErrorOnDuplicate
	KeepBoth           = core.KeepBoth

	EventStateCreated          = core.EventStateCreated
	EventStateActive           = core.EventStateActive
	EventStateFinished         = core.EventStateFinished
	EventStateEmitting         = core.EventStateEmitting
	EventStateEmitted          = core.EventStateEmitted
	EventStateFailedValidation = core.EventStateFailedValidation
	EventStateDeliveryFailed   = core.EventStateDeliveryFailed
)

// ── Global Lifecycle API ──────────────────────────────────────────────────────

// Default returns the global default logger instance.
func Default() *Logger {
	return core.Default()
}

// Configure replaces the global default logger with a new one built from cfg.
func Configure(cfg Config) error {
	return core.Configure(cfg)
}

// SetDefault replaces the global default logger instance.
func SetDefault(l *Logger) {
	core.SetDefault(l)
}

// PanicRecoveryEnabled reports whether wrapper helpers recover panics.
func PanicRecoveryEnabled() bool {
	return core.PanicRecoveryEnabled()
}

// CreateLoxa creates a new Logger. Cross-language parity factory.
func CreateLoxa(cfg Config) (*Logger, error) {
	return core.New(cfg)
}

// New creates a new Logger. Idiomatic Go alias for CreateLoxa.
func New(cfg Config) (*Logger, error) {
	return core.New(cfg)
}

// TryNew creates a new Logger and returns validation errors instead of panicking.
func TryNew(cfg Config) (*Logger, error) {
	return core.New(cfg)
}

// NewClient creates a new Logger applying the full configuration precedence:
// code initialization > environment variables > configuration file > defaults.
// This is the recommended way to create a production SDK client.
// Requirements: 32.1, 32.4, 32.5, 32.6, 32.7, 32.8, 32.9
func NewClient(cfg Config) (*Logger, error) {
	return core.NewClient(cfg)
}

// LoadFromFile loads configuration from a loxa.yaml file.
// If path is empty, it searches for loxa.yaml in the current directory
// and then in ~/.loxa/loxa.yaml.
// Requirements: 32.3
func LoadFromFile(path string) (FileConfig, error) {
	return core.LoadFromFile(path)
}

// StartEvent begins a canonical wide event.
func StartEvent(ctx context.Context, params Params) context.Context {
	return Default().StartEvent(ctx, params)
}

// StartHTTPEvent starts an event with HTTP defaults.
func StartHTTPEvent(ctx context.Context, params Params) context.Context {
	if params.Kind == "" {
		params.Kind = "http"
	}
	if params.Event == "" {
		params.Event = "http.request"
	}
	return StartEvent(ctx, params)
}

// StartHTTPEventFromRequest starts an HTTP event using request metadata and propagated headers.
// It fills Method/Path defaults from r when absent, starts the event, then enriches with
// extracted request/trace attrs.
func StartHTTPEventFromRequest(r *http.Request, params Params) context.Context {
	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
		if params.Method == "" {
			params.Method = r.Method
		}
		if params.Path == "" && r.URL != nil {
			params.Path = r.URL.Path
		}
		// Prefer explicit headers, fall back to extraction that supports traceparent/OTel
		if params.RequestID == "" {
			params.RequestID = RequestIDFromHTTP(r)
		}
		// Use ExtractHTTPHeaders to honor traceparent / Otel propagation
		headerAttrs := core.ExtractHTTPHeaders(r)
		for _, a := range headerAttrs {
			switch a.Key {
			case "request_id":
				if params.RequestID == "" {
					if v, ok := a.Value.(string); ok {
						params.RequestID = v
					}
				}
			case "trace_id":
				if params.TraceID == "" {
					if v, ok := a.Value.(string); ok {
						params.TraceID = v
					}
				}
			case "span_id":
				if params.SpanID == "" {
					if v, ok := a.Value.(string); ok {
						params.SpanID = v
					}
				}
			default:
				// attach any additional header-derived attrs to Custom
				params.Custom = append(params.Custom, a)
			}
		}
	}
	return StartHTTPEvent(ctx, params)
}

// StartJobEvent starts a background job event.
func StartJobEvent(ctx context.Context, params Params) context.Context {
	if params.Kind == "" {
		params.Kind = "job"
	}
	if params.Event == "" {
		params.Event = "job.run"
	}
	return StartEvent(ctx, params)
}

// StartQueueEvent starts a queue-processing event.
func StartQueueEvent(ctx context.Context, params Params) context.Context {
	if params.Kind == "" {
		params.Kind = "queue"
	}
	if params.Event == "" {
		params.Event = "queue.process"
	}
	return StartEvent(ctx, params)
}

// StartCLIEvent starts a CLI execution event.
func StartCLIEvent(ctx context.Context, params Params) context.Context {
	if params.Kind == "" {
		params.Kind = "cli"
	}
	if params.Event == "" {
		params.Event = "cli.run"
	}
	return StartEvent(ctx, params)
}

// StartCronEvent starts a cron execution event.
func StartCronEvent(ctx context.Context, params Params) context.Context {
	if params.Kind == "" {
		params.Kind = "cron"
	}
	if params.Event == "" {
		params.Event = "cron.run"
	}
	return StartEvent(ctx, params)
}

// StartJob is a convenience wrapper for named jobs.
func StartJob(ctx context.Context, name string) context.Context {
	return StartJobEvent(ctx, Params{Event: "job." + name, Custom: []Attr{JobName(name)}})
}

// StartQueueJob is a convenience wrapper for queue jobs.
func StartQueueJob(ctx context.Context, queue, messageID string) context.Context {
	return StartQueueEvent(ctx, Params{
		Event: "queue." + queue,
		Custom: []Attr{
			QueueName(queue),
			MessageID(messageID),
		},
	})
}

// StartCron is a convenience wrapper for cron jobs.
func StartCron(ctx context.Context, name string) context.Context {
	return StartCronEvent(ctx, Params{Event: "cron." + name, Custom: []Attr{JobName(name)}})
}

// RunEvent wraps an operation in the canonical lifecycle.
func RunEvent(ctx context.Context, params Params, fn EventFunc, finishAttrs ...Attr) error {
	return core.RunEvent(ctx, params, fn, finishAttrs...)
}

// RunHTTP wraps an operation in an HTTP canonical event lifecycle.
func RunHTTP(ctx context.Context, params Params, fn EventFunc, finishAttrs ...Attr) error {
	return core.RunHTTP(ctx, params, fn, finishAttrs...)
}

// RunJob wraps an operation in a job canonical event lifecycle.
func RunJob(ctx context.Context, params Params, fn EventFunc, finishAttrs ...Attr) error {
	return core.RunJob(ctx, params, fn, finishAttrs...)
}

// RunQueue wraps an operation in a queue canonical event lifecycle.
func RunQueue(ctx context.Context, params Params, fn EventFunc, finishAttrs ...Attr) error {
	return core.RunQueue(ctx, params, fn, finishAttrs...)
}

// RunCLI wraps an operation in a CLI canonical event lifecycle.
func RunCLI(ctx context.Context, params Params, fn EventFunc, finishAttrs ...Attr) error {
	return core.RunCLI(ctx, params, fn, finishAttrs...)
}

// RunCron wraps an operation in a cron canonical event lifecycle.
func RunCron(ctx context.Context, params Params, fn EventFunc, finishAttrs ...Attr) error {
	return core.RunCron(ctx, params, fn, finishAttrs...)
}

// Enrich appends attrs to the event in ctx.
func Enrich(ctx context.Context, attrs ...Attr) error {
	return Default().Enrich(ctx, attrs...)
}

// Append appends attrs to the event in ctx.
func Append(ctx context.Context, attrs ...Attr) error {
	return Default().Append(ctx, attrs...)
}

// Add appends a value to an array field on the event in ctx.
func Add(ctx context.Context, key string, value interface{}) error {
	return Default().Add(ctx, key, value)
}

// Set upserts attrs on the event in ctx.
func Set(ctx context.Context, attrs ...Attr) error {
	return Default().Set(ctx, attrs...)
}

// Merge merges attrs into a named group on the event in ctx.
func Merge(ctx context.Context, group string, attrs ...Attr) error {
	return Default().Merge(ctx, group, attrs...)
}

// Delete removes attrs by key from the event in ctx.
func Delete(ctx context.Context, keys ...string) error {
	return Default().Delete(ctx, keys...)
}

// Get fetches a value by key (dot-path supported) from event in ctx.
func Get(ctx context.Context, key string) (any, bool) {
	return Default().Get(ctx, key)
}

// GetGroup fetches a group object from event in ctx.
func GetGroup(ctx context.Context, name string) (map[string]any, bool) {
	return Default().GetGroup(ctx, name)
}

// EnrichGroup appends attrs as a named group.
func EnrichGroup(ctx context.Context, key string, attrs ...Attr) error {
	return Default().EnrichGroup(ctx, key, attrs...)
}

// Finish records outcome and duration.
func Finish(ctx context.Context, outcome string, attrs ...Attr) error {
	return Default().Finish(ctx, outcome, attrs...)
}

// FinishError records error outcome and metadata.
func FinishError(ctx context.Context, err error, attrs ...Attr) error {
	return Default().FinishError(ctx, err, attrs...)
}

// Emit delivers the event.
func Emit(ctx context.Context) error {
	return Default().Emit(ctx)
}

// EmitEvent delivers an event directly.
func EmitEvent(ev *Event) error {
	return Default().EmitEvent(ev)
}

// NewEvent creates a manual event instance.
func NewEvent(params Params) *Event {
	return core.NewEvent(params)
}

// Checkpoint records a named breadcrumb.
func Checkpoint(ctx context.Context, name string, attrs ...Attr) error {
	return Default().Checkpoint(ctx, name, attrs...)
}

// Process starts a named process step and returns a handle to finish it.
func Process(ctx context.Context, name string, attrs ...Attr) (*ProcessHandle, error) {
	return Default().Process(ctx, name, attrs...)
}

// StartTimer starts a named timer and returns a handle to stop it.
func StartTimer(ctx context.Context, name string, attrs ...Attr) (*TimerHandle, error) {
	return Default().StartTimer(ctx, name, attrs...)
}

// StartGroup starts a named group phase and returns a handle to finish it.
func StartGroup(ctx context.Context, name string, attrs ...Attr) (*GroupHandle, error) {
	return Default().StartGroup(ctx, name, attrs...)
}

// Stopwatch creates a standalone stopwatch for manual timing.
func Stopwatch() *StopwatchHandle {
	return core.Stopwatch()
}

// Flush drains the queue.
func Flush(ctx context.Context) error {
	return Default().Flush(ctx)
}

// Shutdown drains and closes all sinks.
func Shutdown(ctx context.Context) error {
	return Default().Shutdown(ctx)
}

// ShutdownTimeout drains and closes sinks with timeout-bound context.
func ShutdownTimeout(timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("loxa: shutdown timeout must be > 0 (got %s)", timeout)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return Shutdown(ctx)
}

// MustShutdown drains and closes sinks, panicking on error.
func MustShutdown(timeout time.Duration) {
	if err := ShutdownTimeout(timeout); err != nil {
		panic(err)
	}
}

// Alias creates a new Logger with the same config as Default() but a different service name.
func Alias(service string) (*Logger, error) {
	return Default().Alias(service)
}

// ── Immediate logging API ─────────────────────────────────────────────────────

func Debug(msg string, attrs ...Attr) { Default().Debug(msg, attrs...) }
func Info(msg string, attrs ...Attr)  { Default().Info(msg, attrs...) }
func Warn(msg string, attrs ...Attr)  { Default().Warn(msg, attrs...) }
func Error(msg string, attrs ...Attr) { Default().Error(msg, attrs...) }
func Fatal(msg string, attrs ...Attr) { Default().Fatal(msg, attrs...) }

// DebugContext emits an immediate debug log line with explicit context and event name.
func DebugContext(ctx context.Context, msg, event string, attrs ...Attr) {
	Default().DebugContext(ctx, msg, event, attrs...)
}

// InfoContext emits an immediate info log line with explicit context and event name.
func InfoContext(ctx context.Context, msg, event string, attrs ...Attr) {
	Default().InfoContext(ctx, msg, event, attrs...)
}

// WarnContext emits an immediate warn log line with explicit context and event name.
func WarnContext(ctx context.Context, msg, event string, attrs ...Attr) {
	Default().WarnContext(ctx, msg, event, attrs...)
}

// ErrorContext emits an immediate error log line with explicit context and event name.
func ErrorContext(ctx context.Context, msg string, err error, event string, attrs ...Attr) {
	Default().ErrorContext(ctx, msg, err, event, attrs...)
}

// FatalContext emits an immediate fatal log line with explicit context and exits the process.
func FatalContext(ctx context.Context, msg string, err error, event string, attrs ...Attr) {
	Default().FatalContext(ctx, msg, err, event, attrs...)
}

// FromContext retrieves the active Event from ctx.
func FromContext(ctx context.Context) (*Event, bool) {
	return core.FromContext(ctx)
}

// HasEvent returns true if ctx contains a LOXA event.
func HasEvent(ctx context.Context) bool {
	return core.HasEvent(ctx)
}

// EventID returns the ID of the active event in ctx.
func EventID(ctx context.Context) string {
	return core.EventID(ctx)
}

// RequestIDFromContext returns the request ID of the active event.
func RequestIDFromContext(ctx context.Context) string {
	return core.RequestIDFromContext(ctx)
}

// TraceIDFromContext returns the trace ID of the active event.
func TraceIDFromContext(ctx context.Context) string {
	return core.TraceIDFromContext(ctx)
}

// SpanIDFromContext returns the span ID of the active event.
func SpanIDFromContext(ctx context.Context) string {
	return core.SpanIDFromContext(ctx)
}

// RequestIDFromHTTP resolves request id from header or active event context.
func RequestIDFromHTTP(r *http.Request) string {
	return core.RequestIDFromHTTP(r)
}

// TraceFromOTel returns trace/span ids from OTel span context.
func TraceFromOTel(ctx context.Context) (traceID string, spanID string) {
	return core.TraceFromOTel(ctx)
}

// InjectHTTPHeaders injects LOXA + trace headers into an outbound request.
func InjectHTTPHeaders(req *http.Request) {
	core.InjectHTTPHeaders(req)
}

// InjectHTTPHeaderCarrier injects LOXA + trace headers into a header map.
func InjectHTTPHeaderCarrier(ctx context.Context, header http.Header) http.Header {
	return core.InjectHTTPHeaderCarrier(ctx, header)
}

// ExtractHTTPHeaders extracts common LOXA tracing/request attrs from inbound headers.
func ExtractHTTPHeaders(r *http.Request) []Attr {
	return core.ExtractHTTPHeaders(r)
}

// ExtractHTTPHeaderAttrs extracts common LOXA tracing/request attrs from headers.
func ExtractHTTPHeaderAttrs(header http.Header) []Attr {
	return core.ExtractHTTPHeaderAttrs(header)
}

// ── Constructors (Re-exported from core) ──────────────────────────────────────

var (
	String   = core.String
	Int      = core.Int
	Int64    = core.Int64
	Uint64   = core.Uint64
	Float64  = core.Float64
	Bool     = core.Bool
	Time     = core.Time
	Duration = core.Duration
	Any      = core.Any
	Null     = core.Null
	Err      = core.Err
	Stringer = core.Stringer
	Group    = core.Group

	// Canonical fields
	RequestID    = core.RequestID
	TraceID      = core.TraceID
	SpanID       = core.SpanID
	Service      = core.Service
	Version      = core.Version
	DeploymentID = core.DeploymentID
	Region       = core.Region
	Method       = core.Method
	Path         = core.Path
	Route        = core.Route
	StatusCode   = core.StatusCode
	DurationMS   = core.DurationMS
	Outcome      = core.Outcome

	// Domain fields
	UserID         = core.UserID
	TenantID       = core.TenantID
	WorkspaceID    = core.WorkspaceID
	OrganizationID = core.OrganizationID
	SessionID      = core.SessionID
	OrderID        = core.OrderID
	CartID         = core.CartID
	ProductID      = core.ProductID
	CustomerID     = core.CustomerID
	Plan           = core.Plan
	Currency       = core.Currency
	Amount         = core.Amount
	Country        = core.Country
	Device         = core.Device
	Platform       = core.Platform
	AppVersion     = core.AppVersion
	JobName        = core.JobName
	QueueName      = core.QueueName
	MessageID      = core.MessageID
	Attempt        = core.Attempt
	ErrorType      = core.ErrorType
	ErrorCode      = core.ErrorCode
	ErrorMessage   = core.ErrorMessage
	ErrorStack     = core.ErrorStack
	Retryable      = core.Retryable

	// Sensitive
	MarkSensitive   = core.MarkSensitive
	SensitiveString = core.SensitiveString
	HashString      = core.HashString

	// Domain logic
	FeatureFlag     = core.FeatureFlag
	FeatureFlagBool = core.FeatureFlagBool
	Experiment      = core.Experiment
)

// ── Core Sinks ───────────────────────────────────────────────────────────────

func StdoutSink() Sink                                      { return core.StdoutSink() }
func StderrSink() Sink                                      { return core.StderrSink() }
func FileSink(path string) (Sink, error)                    { return core.FileSink(path) }
func RotatingFileSink(cfg RotatingFileConfig) (Sink, error) { return core.RotatingFileSink(cfg) }
func MemorySink() (Sink, *MemorySinkStore)                  { return core.MemorySink() }

// TestLogger creates a Logger backed by a MemorySink for testing.
func TestLogger() (*Logger, *MemorySinkStore, error) { return core.TestLogger() }

// Capture runs fn and returns all events emitted during execution.
func Capture(fn func()) ([]*Event, error) { return core.Capture(fn) }

// AssertEvent checks that ev has the expected value at the given key.
func AssertEvent(t testing.TB, ev *Event, key string, expected any) {
	core.AssertEvent(t, ev, key, expected)
}

// AssertRedacted checks that ev has "[REDACTED]" at the given key.
func AssertRedacted(t testing.TB, ev *Event, key string) {
	core.AssertRedacted(t, ev, key)
}

// AssertHasCheckpoint checks that ev contains a checkpoint with the given name.
func AssertHasCheckpoint(t testing.TB, ev *Event, name string) {
	core.AssertHasCheckpoint(t, ev, name)
}
func NoopSink() Sink                                      { return core.NoopSink() }
func CollectorSink(cfg CollectorSinkConfig) (Sink, error) { return core.CollectorSink(cfg) }

// HTTPBatchSink creates a sink that batches events as NDJSON and flushes
// to the endpoint when BatchSize is reached or FlushInterval elapses.
// This is the default sink when CollectorURL is configured.
func HTTPBatchSink(cfg HTTPBatchSinkConfig) (Sink, error) { return core.HTTPBatchSink(cfg) }

// LegacyHTTPBatchSink is a convenience wrapper around CollectorSink.
// Deprecated: Use HTTPBatchSink with HTTPBatchSinkConfig for real batching.
func LegacyHTTPBatchSink(endpoint string) (Sink, error) { return core.LegacyHTTPBatchSink(endpoint) }

// ── Presets ───────────────────────────────────────────────────────────────────

func Production() Config { return core.Production() }
func Dev() Config        { return core.Dev() }
func Test() Config       { return core.Test() }

// ParseLevel parses a level string.
func ParseLevel(s string) Level { return core.ParseLevel(s) }

// LoadFromEnv loads configuration from environment variables.
func LoadFromEnv(base Config) Config { return core.LoadFromEnv(base) }

// ── Config options ─────────────────────────────────────────────────────────────

// ApplyConfig applies options to cfg in order.
func ApplyConfig(cfg Config, options ...ConfigOption) Config {
	return core.ApplyConfig(cfg, options...)
}

func WithService(service string) ConfigOption         { return core.WithService(service) }
func WithVersion(version string) ConfigOption         { return core.WithVersion(version) }
func WithEnvironment(environment string) ConfigOption { return core.WithEnvironment(environment) }
func WithSink(sink Sink) ConfigOption                 { return core.WithSink(sink) }
func WithSampler(sampler Sampler) ConfigOption        { return core.WithSampler(sampler) }
func WithEncoder(encoder Encoder) ConfigOption        { return core.WithEncoder(encoder) }
func WithRedactor(redactor Redactor) ConfigOption     { return core.WithRedactor(redactor) }
func WithSchema(schema Schema) ConfigOption           { return core.WithSchema(schema) }
func WithEventSchema(schema Schema) ConfigOption      { return core.WithEventSchema(schema) }
func WithAsync(enabled bool) ConfigOption             { return core.WithAsync(enabled) }
func WithAsyncQueue(size int) ConfigOption            { return core.WithAsyncQueue(size) }
func WithWorkers(workers int) ConfigOption            { return core.WithWorkers(workers) }
func WithAsyncFlushInterval(interval time.Duration) ConfigOption {
	return core.WithAsyncFlushInterval(interval)
}
func WithAsyncMaxBatchBytes(maxBytes int) ConfigOption { return core.WithAsyncMaxBatchBytes(maxBytes) }
func WithBackpressure(policy BackpressurePolicy) ConfigOption {
	return core.WithBackpressure(policy)
}
func WithDuplicatePolicy(policy DuplicateFieldPolicy) ConfigOption {
	return core.WithDuplicatePolicy(policy)
}
func WithStrict(strict bool) ConfigOption                { return core.WithStrict(strict) }
func WithValidateEncoded(validate bool) ConfigOption     { return core.WithValidateEncoded(validate) }
func WithEnricher(enricher ContextEnricher) ConfigOption { return core.WithEnricher(enricher) }
func WithFallbackSink(sink Sink) ConfigOption            { return core.WithFallbackSink(sink) }
func WithCollectorEndpoint(endpoint string) ConfigOption { return core.WithCollectorEndpoint(endpoint) }
func WithStatsHandler(handler StatsHandler) ConfigOption { return core.WithStatsHandler(handler) }
func WithDeploymentID(deploymentID string) ConfigOption  { return core.WithDeploymentID(deploymentID) }
func WithIncludeHost(includeHost bool) ConfigOption      { return core.WithIncludeHost(includeHost) }
func WithPanicRecovery(panicRecovery bool) ConfigOption  { return core.WithPanicRecovery(panicRecovery) }
func WithCollectorURL(url string) ConfigOption           { return core.WithCollectorURL(url) }
func WithTenantID(tenantID string) ConfigOption          { return core.WithTenantID(tenantID) }
func WithBatchSize(size int) ConfigOption                { return core.WithBatchSize(size) }
func WithFlushInterval(interval time.Duration) ConfigOption {
	return core.WithFlushInterval(interval)
}
func WithMaxBufferSize(size int) ConfigOption { return core.WithMaxBufferSize(size) }
func WithMaxRetries(retries int) ConfigOption { return core.WithMaxRetries(retries) }
func WithMaxBackoff(backoff time.Duration) ConfigOption {
	return core.WithMaxBackoff(backoff)
}
func WithTimeout(timeout time.Duration) ConfigOption { return core.WithTimeout(timeout) }
func WithConnectionTimeout(timeout time.Duration) ConfigOption {
	return core.WithConnectionTimeout(timeout)
}
func WithCompression(enabled bool) ConfigOption { return core.WithCompression(enabled) }
func WithLevel(level Level) ConfigOption        { return core.WithLevel(level) }
func WithRegion(region string) ConfigOption     { return core.WithRegion(region) }
func NewMetricsCollector(namespace string, maxBufferSize int) *MetricsCollector {
	return core.NewMetricsCollector(namespace, maxBufferSize)
}
func NewPrometheusStatsHandler(namespace string, maxBufferSize int) *PrometheusStatsHandler {
	return core.NewPrometheusStatsHandler(namespace, maxBufferSize)
}

// RenderPrometheus returns a stable textual metrics endpoint hint for SDK parity.
func RenderPrometheus(metrics *MetricsCollector) string {
	if metrics == nil {
		return ""
	}
	return "# LOXA Go metrics are exposed through MetricsCollector.Handler()\n"
}

// ── Samplers ─────────────────────────────────────────────────────────────────

func SampleAll() Sampler                { return core.SampleAll() }
func SampleNone() Sampler               { return core.SampleNone() }
func SampleRandom(rate float64) Sampler { return core.SampleRandom(rate) }
func SampleRateLimited(rate float64, window time.Duration) Sampler {
	return core.SampleRateLimited(rate, window)
}
func SampleByHeader(header, value string) Sampler { return core.SampleByHeader(header, value) }
func SampleErrors() Sampler                       { return core.SampleErrors() }
func SampleSlowRequests(d time.Duration) Sampler  { return core.SampleSlowRequests(d) }
func SampleStatusCodes(codes ...int) Sampler      { return core.SampleStatusCodes(codes...) }
func SampleRoutes(routes ...string) Sampler       { return core.SampleRoutes(routes...) }
func SampleUsers(ids ...string) Sampler           { return core.SampleUsers(ids...) }
func SampleTenants(ids ...string) Sampler         { return core.SampleTenants(ids...) }
func SampleFeatureFlag(name string, value any) Sampler {
	return core.SampleFeatureFlag(name, value)
}
func AnySampler(samplers ...Sampler) Sampler { return core.AnySampler(samplers...) }
func AllSampler(samplers ...Sampler) Sampler { return core.AllSampler(samplers...) }
func NotSampler(sampler Sampler) Sampler     { return core.NotSampler(sampler) }

// ── Redactors ─────────────────────────────────────────────────────────────────

func ComposeRedactors(redactors ...Redactor) Redactor {
	return core.ComposeRedactors(redactors...)
}

func DefaultRedactor() Redactor                  { return core.DefaultRedactor() }
func MaskKeys(keys ...string) Redactor           { return core.MaskKeys(keys...) }
func HashKeys(keys ...string) Redactor           { return core.HashKeys(keys...) }
func RedactKeys(keys ...string) Redactor         { return core.RedactKeys(keys...) }
func DropKeys(keys ...string) Redactor           { return core.DropKeys(keys...) }
func RedactPatterns(patterns ...string) Redactor { return core.RedactPatterns(patterns...) }

// ── Schemas ────────────────────────────────────────────────────────────────────

func DefaultSchema() Schema { return core.DefaultSchema() }
func FlatSchema() Schema    { return core.FlatSchema() }
func NestedSchema() Schema  { return core.NestedSchema() }
func OTelSchema() Schema    { return core.OTelLogSchema() }
func OTelLogSchema() Schema { return core.OTelLogSchema() }
func ECSchema() Schema      { return core.ECSchema() }
func DatadogSchema() Schema { return core.DatadogSchema() }
func CustomSchema(fn func(EventView) map[string]any) Schema {
	return core.CustomSchema(fn)
}

// JSONEncoder returns the default compact JSON encoder.
func JSONEncoder() *JSONEventEncoder { return core.JSONEncoder() }

// PrettyJSONEncoder returns a pretty-print JSON encoder.
func PrettyJSONEncoder() *JSONEventEncoder { return core.PrettyJSONEncoder() }

// ── HTTP Client Instrumentation ──────────────────────────────────────────────

func WrapHTTPClient(client *http.Client) *http.Client          { return core.WrapHTTPClient(client) }
func NewRoundTripper(base http.RoundTripper) http.RoundTripper { return core.NewRoundTripper(base) }

// ── Cortex Client ───────────────────────────────────────────────────────────

// NewCortexClient creates an HTTP client for the Cortex incident intelligence API.
func NewCortexClient(endpoint string) *CortexClient { return cortex.NewClient(endpoint) }

// Cortex validation
func ValidateIncidentContext(ctx *IncidentContext) error { return cortex.ValidateIncidentContext(ctx) }
func ValidateGraphView(gv *GraphView) error              { return cortex.ValidateGraphView(gv) }
func ValidateRemediation(r *Remediation) error           { return cortex.ValidateRemediation(r) }
func ValidateRemediationFeedback(rf *RemediationFeedback) error {
	return cortex.ValidateRemediationFeedback(rf)
}

// Cortex normalization
func NormalizeIncidentContext(ctx *IncidentContext) { cortex.NormalizeIncidentContext(ctx) }
func NormalizeRemediation(r *Remediation)           { cortex.NormalizeRemediation(r) }
func NormalizeRemediationFeedback(rf *RemediationFeedback) {
	cortex.NormalizeRemediationFeedback(rf)
}
