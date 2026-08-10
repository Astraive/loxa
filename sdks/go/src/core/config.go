package core

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/astraive/loza/spec/dsn"
)

// ── Field naming ──────────────────────────────────────────────────────────────

// FieldNamingConfig controls how custom attr keys are treated during encoding.
type FieldNamingConfig struct {
	// ExpandDotKeys splits keys like "user.id" into nested JSON objects.
	// Default: true.
	ExpandDotKeys bool
}

// ── Duplicate field policy ────────────────────────────────────────────────────

// DuplicateFieldPolicy controls what happens when a custom Attr has the same
// key as a canonical field (e.g. loza.String("service", "x") when service is
// already set from Params).
type DuplicateFieldPolicy int

const (
	// CanonicalWins keeps the canonical value and silently drops the attr (default).
	CanonicalWins DuplicateFieldPolicy = iota
	// AttrWins overwrites the canonical field with the attr value.
	//
	// Deprecated: Use UserWins.
	AttrWins
	// KeepBothUnderAttrs keeps the canonical value and moves the conflicting
	// attr under an "attrs" key.
	//
	// Deprecated: Use KeepBoth.
	KeepBothUnderAttrs
	// DropDuplicateAttr silently drops the attr (same as CanonicalWins).
	//
	// Deprecated: Use CanonicalWins.
	DropDuplicateAttr
	// ErrorOnDuplicate returns an error when a custom attr conflicts with a canonical field.
	ErrorOnDuplicate

	// KeepBoth is alias for KeepBothUnderAttrs.
	KeepBoth = KeepBothUnderAttrs
	// UserWins lets user attrs overwrite canonical fields when possible.
	UserWins = AttrWins
	// AttrsWin lets attrs overwrite canonical fields when possible.
	//
	// Deprecated: Use UserWins.
	AttrsWin = AttrWins
	// FirstWins keeps the first canonical value for duplicate canonical attrs.
	FirstWins = CanonicalWins
	// LastWins lets the latest duplicate attr overwrite canonical fields when possible.
	LastWins = AttrWins
)

// ── Checkpoint config ─────────────────────────────────────────────────────────

// CheckpointConfig controls checkpoint behaviour.
type CheckpointConfig struct {
	// Enabled allows checkpoints to be recorded. Default: true.
	Enabled bool
	// EmitImmediately emits each checkpoint as a standalone log line in
	// addition to including it in the final event. Default: false.
	EmitImmediately bool
	// MaxCheckpoints caps how many checkpoints are stored per event. Default: 32.
	MaxCheckpoints int
}

// ── Async pipeline config ─────────────────────────────────────────────────────

// BackpressurePolicy determines what happens when the async queue is full.
type BackpressurePolicy int

const (
	// Block waits until space is available (default — protects against data loss).
	Block BackpressurePolicy = iota
	// DropNewest drops the incoming event when the queue is full.
	DropNewest
	// DropOldest discards the oldest queued event to make room.
	DropOldest
	// DropDebug drops debug-level events first under pressure.
	DropDebug
	// DropSampled drops sampled (non-error) events first.
	DropSampled
	// SyncFallback writes synchronously to sinks if the queue is full.
	SyncFallback
)

// AsyncConfig configures the background async emit pipeline.
type AsyncConfig struct {
	// Enabled turns on async mode. Default: true in Production(), false in Dev().
	Enabled bool
	// QueueSize is the channel depth. Default: 8192.
	QueueSize int
	// Workers is the number of goroutines draining the queue. Default: 4.
	Workers int
	// FlushInterval is how often buffered sinks are flushed. Default: 1s.
	FlushInterval time.Duration
	// MaxBatchBytes caps the byte size of each sink batch. Default: 4MB.
	MaxBatchBytes int
	// Backpressure controls what happens when the queue is full. Default: Block.
	Backpressure BackpressurePolicy
}

// ContextEnricher appends attrs derived from request/job context during Emit.
type ContextEnricher func(ctx context.Context) []Attr

// StatsHandler receives logger pipeline telemetry callbacks.
type StatsHandler interface {
	OnEmit(ev *Event)
	OnDrop(reason string)
	OnError(err error)
}

// DeliveryFailureHandler is an optional extension for StatsHandler
// implementations that want explicit delivery-failure callbacks.
type DeliveryFailureHandler interface {
	OnDeliveryFailed(ev *Event, err error)
}

// ── Top-level config ──────────────────────────────────────────────────────────

// Config is the top-level LOZA-Go configuration.
type Config struct {
	// ── Service identity ──────────────────────────────────────────────────────
	Service      string
	Alias        string
	Version      string
	Environment  string
	DeploymentID string
	Region       string
	TenantID     string // Multi-tenant identifier

	// ── Authentication ───────────────────────────────────────────────────────
	APIKey   string // Ingest API key (e.g., "lx_sec_live_k_xxx_yyyy")
	Insecure bool   // Allow plain HTTP (local dev only). Default: false.

	// ── Collector configuration ───────────────────────────────────────────────
	CollectorURL string // URL of the LOZA collector (required)

	// ── Batching configuration ────────────────────────────────────────────────
	BatchSize     int           // Number of events per batch (default: 100)
	FlushInterval time.Duration // Time between automatic flushes (default: 5s)
	MaxBufferSize int           // Maximum events in buffer before dropping (default: 10000)

	// ── Retry configuration ───────────────────────────────────────────────────
	MaxRetries        int           // Maximum retry attempts (default: 3)
	MaxBackoff        time.Duration // Maximum backoff duration (default: 30s)
	Timeout           time.Duration // Request timeout (default: 10s)
	ConnectionTimeout time.Duration // Connection timeout (default: 5s)

	// ── Compression ───────────────────────────────────────────────────────────
	EnableCompression bool // Enable gzip compression for HTTP requests (default: true)

	// ── Log level ─────────────────────────────────────────────────────────────
	// Events below Level are dropped before encoding. Default: LevelInfo.
	Level Level

	// ── Pipeline components ───────────────────────────────────────────────────
	Sampler           Sampler
	Encoder           Encoder
	Schema            Schema
	Sink              Sink
	Sinks             []Sink
	Redactor          Redactor
	ErrorExtractor    ErrorExtractor
	Enricher          ContextEnricher
	FallbackSink      Sink
	StatsHandler      StatsHandler
	CollectorEndpoint string // Deprecated: use CollectorURL instead

	// ── Optional metadata ─────────────────────────────────────────────────────
	IncludeHost    bool // include os.Hostname() in every event
	IncludeRuntime bool // include Go runtime version
	IncludeSource  bool // include caller file:line and default error stacks (expensive)

	// ── Subsystem configs ─────────────────────────────────────────────────────
	Async                AsyncConfig
	FieldNaming          FieldNamingConfig
	DuplicateFieldPolicy DuplicateFieldPolicy
	Checkpoints          CheckpointConfig
	PanicRecovery        bool
	// OTelBridge enables OpenTelemetry trace context extraction from context.
	// When false (default), TraceFromOTel is skipped to avoid the ~50-100ns context.Value lookup.
	OTelBridge           bool
	Security             SecurityConfig
	// Strict enables stronger runtime validation for event shape and attrs.
	Strict bool
	// ValidateEncoded controls post-encode spec contract validation in strict mode.
	// Default: true when Strict is true. Set false for custom schemas
	// (FlatSchema, ECSchema, DatadogSchema) that deviate from LOZA shape.
	ValidateEncoded bool

	// ── ID generation ─────────────────────────────────────────────────────────
	IDGen IDGenerator

	// ── Clock ─────────────────────────────────────────────────────────────────
	Clock Clock

	codeSetCompression     bool
	codeSetAsync           bool
	codeSetBackpressure    bool
	codeSetStrict          bool
	codeSetValidateEncoded bool
	codeSetRedactByDefault bool
	codeSetAllowPII        bool
	codeSetDropOversized   bool
}

// WithService returns a copy of cfg with Service set.
func (c Config) WithService(service string) Config {
	c.Service = service
	return c
}

// WithAlias returns a copy of cfg with the logical loza.alias metadata set.
func (c Config) WithAlias(alias string) Config {
	c.Alias = alias
	return c
}

// WithVersion returns a copy of cfg with Version set.
func (c Config) WithVersion(version string) Config {
	c.Version = version
	return c
}

// WithEnvironment returns a copy of cfg with Environment set.
func (c Config) WithEnvironment(environment string) Config {
	c.Environment = environment
	return c
}

// WithSink appends a sink.
func (c Config) WithSink(sink Sink) Config {
	c.Sinks = append(c.Sinks, sink)
	return c
}

// WithSampler sets the sampler.
func (c Config) WithSampler(s Sampler) Config {
	c.Sampler = s
	return c
}

// WithEncoder sets the encoder.
func (c Config) WithEncoder(e Encoder) Config {
	c.Encoder = e
	return c
}

// WithSchema sets the output event schema.
func (c Config) WithSchema(schema Schema) Config {
	c.Schema = schema
	return c
}

// WithEventSchema sets the output event schema.
func (c Config) WithEventSchema(schema Schema) Config {
	return c.WithSchema(schema)
}

// WithRedactor sets the redactor.
func (c Config) WithRedactor(r Redactor) Config {
	c.Redactor = r
	return c
}

// WithAsync enables or disables async mode.
func (c Config) WithAsync(enabled bool) Config {
	c.Async.Enabled = enabled
	c.codeSetAsync = true
	return c
}

// WithAsyncQueue sets async queue size and enables async mode.
func (c Config) WithAsyncQueue(size int) Config {
	c.Async.Enabled = true
	c.Async.QueueSize = size
	c.codeSetAsync = true
	return c
}

// WithWorkers sets async worker count and enables async mode.
func (c Config) WithWorkers(workers int) Config {
	c.Async.Enabled = true
	c.Async.Workers = workers
	c.codeSetAsync = true
	return c
}

// WithAsyncFlushInterval sets async flush interval and enables async mode.
func (c Config) WithAsyncFlushInterval(interval time.Duration) Config {
	c.Async.Enabled = true
	c.Async.FlushInterval = interval
	c.codeSetAsync = true
	return c
}

// WithAsyncMaxBatchBytes sets async max batch size and enables async mode.
func (c Config) WithAsyncMaxBatchBytes(maxBytes int) Config {
	c.Async.Enabled = true
	c.Async.MaxBatchBytes = maxBytes
	c.codeSetAsync = true
	return c
}

// WithBackpressure sets async backpressure policy.
func (c Config) WithBackpressure(policy BackpressurePolicy) Config {
	c.Async.Enabled = true
	c.Async.Backpressure = policy
	c.codeSetAsync = true
	c.codeSetBackpressure = true
	return c
}

// WithDuplicatePolicy sets duplicate field conflict policy.
func (c Config) WithDuplicatePolicy(policy DuplicateFieldPolicy) Config {
	c.DuplicateFieldPolicy = policy
	return c
}

// WithStrict enables or disables strict mode validation.
func (c Config) WithStrict(strict bool) Config {
	c.Strict = strict
	c.codeSetStrict = true
	return c
}

// WithValidateEncoded returns a copy of cfg with ValidateEncoded set.
func (c Config) WithValidateEncoded(validate bool) Config {
	c.ValidateEncoded = validate
	c.codeSetValidateEncoded = true
	return c
}

// WithEnricher sets a context enricher hook that runs during Emit(ctx).
func (c Config) WithEnricher(fn ContextEnricher) Config {
	c.Enricher = fn
	return c
}

// WithFallbackSink configures a sink used when primary sink writes fail.
func (c Config) WithFallbackSink(sink Sink) Config {
	c.FallbackSink = sink
	return c
}

func (c Config) WithCollectorEndpoint(endpoint string) Config {
	c.CollectorEndpoint = strings.TrimSpace(endpoint)
	return c
}

// WithAPIKey sets the ingest API key for collector authentication.
func (c Config) WithAPIKey(apiKey string) Config {
	c.APIKey = strings.TrimSpace(apiKey)
	return c
}

// WithInsecure allows plain HTTP connections (for local dev only).
func (c Config) WithInsecure(insecure bool) Config {
	c.Insecure = insecure
	return c
}

// WithStatsHandler sets callbacks for emit/drop/error telemetry.
func (c Config) WithStatsHandler(handler StatsHandler) Config {
	c.StatsHandler = handler
	return c
}

// WithDeploymentID sets the deployment identifier attached to emitted events.
func (c Config) WithDeploymentID(deploymentID string) Config {
	c.DeploymentID = strings.TrimSpace(deploymentID)
	return c
}

// WithIncludeHost controls whether the host name is attached to emitted events.
func (c Config) WithIncludeHost(includeHost bool) Config {
	c.IncludeHost = includeHost
	return c
}

// WithPanicRecovery controls whether lifecycle helpers recover panics.
func (c Config) WithPanicRecovery(panicRecovery bool) Config {
	c.PanicRecovery = panicRecovery
	return c
}

// WithOTelBridge enables OpenTelemetry trace context extraction from context.
// When enabled, StartEvent extracts trace_id and span_id from OTel span context.
// When disabled (default), the OTel context.Value lookup is skipped for performance.
func (c Config) WithOTelBridge(enabled bool) Config {
	c.OTelBridge = enabled
	return c
}

// WithRedactByDefault enables or disables redaction of sensitive fields by default.
func (c Config) WithRedactByDefault(redact bool) Config {
	c.Security.RedactByDefault = redact
	c.codeSetRedactByDefault = true
	return c
}

// WithAllowPII enables or disables PII exposure when RedactByDefault is true.
func (c Config) WithAllowPII(allow bool) Config {
	c.Security.AllowPII = allow
	c.codeSetAllowPII = true
	return c
}

// WithCompression enables or disables gzip compression for collector requests.
func (c Config) WithCompression(enabled bool) Config {
	c.EnableCompression = enabled
	c.codeSetCompression = true
	return c
}

// WithMaxFieldBytes sets the maximum byte size for individual field values.
func (c Config) WithMaxFieldBytes(max int) Config {
	c.Security.MaxFieldBytes = max
	return c
}

// WithMaxEventBytes sets the maximum byte size for entire events.
func (c Config) WithMaxEventBytes(max int) Config {
	c.Security.MaxEventBytes = max
	return c
}

// WithMaxAttrCount sets the maximum number of attributes per event.
func (c Config) WithMaxAttrCount(max int) Config {
	c.Security.MaxAttrCount = max
	return c
}

// WithDropOversizedEvents enables or disables dropping of events that exceed max_event_bytes.
func (c Config) WithDropOversizedEvents(drop bool) Config {
	c.Security.DropOversizedEvents = drop
	c.codeSetDropOversized = true
	return c
}

// Validate validates cfg and returns explicit field-level errors.
// In strict mode, additional config checks are enforced.
func (c Config) Validate() error {
	// Validate service name (required in strict mode)
	if c.Strict && strings.TrimSpace(c.Service) == "" {
		return &ConfigValidationError{
			Field:   "Service",
			Problem: "service name is required in strict mode",
		}
	}

	// Validate batch size
	if c.BatchSize < 0 {
		return &ConfigValidationError{
			Field:   "BatchSize",
			Problem: fmt.Sprintf("must be >= 0 (got %d)", c.BatchSize),
		}
	}

	// Validate flush interval
	if c.FlushInterval < 0 {
		return &ConfigValidationError{
			Field:   "FlushInterval",
			Problem: fmt.Sprintf("must be >= 0 (got %s)", c.FlushInterval),
		}
	}

	// Validate max buffer size
	if c.MaxBufferSize < 0 {
		return &ConfigValidationError{
			Field:   "MaxBufferSize",
			Problem: fmt.Sprintf("must be >= 0 (got %d)", c.MaxBufferSize),
		}
	}

	// Validate max retries
	if c.MaxRetries < 0 {
		return &ConfigValidationError{
			Field:   "MaxRetries",
			Problem: fmt.Sprintf("must be >= 0 (got %d)", c.MaxRetries),
		}
	}

	// Validate max backoff
	if c.MaxBackoff < 0 {
		return &ConfigValidationError{
			Field:   "MaxBackoff",
			Problem: fmt.Sprintf("must be >= 0 (got %s)", c.MaxBackoff),
		}
	}

	// Validate timeout
	if c.Timeout < 0 {
		return &ConfigValidationError{
			Field:   "Timeout",
			Problem: fmt.Sprintf("must be >= 0 (got %s)", c.Timeout),
		}
	}

	// Validate connection timeout
	if c.ConnectionTimeout < 0 {
		return &ConfigValidationError{
			Field:   "ConnectionTimeout",
			Problem: fmt.Sprintf("must be >= 0 (got %s)", c.ConnectionTimeout),
		}
	}

	if c.Async.Enabled {
		if c.Async.QueueSize < 0 {
			return &ConfigValidationError{
				Field:   "Async.QueueSize",
				Problem: fmt.Sprintf("must be >= 0 (got %d)", c.Async.QueueSize),
			}
		}
		if c.Async.Workers < 0 {
			return &ConfigValidationError{
				Field:   "Async.Workers",
				Problem: fmt.Sprintf("must be >= 0 (got %d)", c.Async.Workers),
			}
		}
		if c.Async.FlushInterval < 0 {
			return &ConfigValidationError{
				Field:   "Async.FlushInterval",
				Problem: fmt.Sprintf("must be >= 0 (got %s)", c.Async.FlushInterval),
			}
		}
		if c.Async.MaxBatchBytes < 0 {
			return &ConfigValidationError{
				Field:   "Async.MaxBatchBytes",
				Problem: fmt.Sprintf("must be >= 0 (got %d)", c.Async.MaxBatchBytes),
			}
		}
	}
	if c.Security.MaxFieldBytes < 0 {
		return &ConfigValidationError{
			Field:   "Security.MaxFieldBytes",
			Problem: fmt.Sprintf("must be >= 0 (got %d)", c.Security.MaxFieldBytes),
		}
	}
	if c.Security.MaxEventBytes < 0 {
		return &ConfigValidationError{
			Field:   "Security.MaxEventBytes",
			Problem: fmt.Sprintf("must be >= 0 (got %d)", c.Security.MaxEventBytes),
		}
	}
	if c.Security.MaxAttrCount < 0 {
		return &ConfigValidationError{
			Field:   "Security.MaxAttrCount",
			Problem: fmt.Sprintf("must be >= 0 (got %d)", c.Security.MaxAttrCount),
		}
	}
	if !c.Strict {
		return nil
	}
	if c.Async.Enabled {
		if c.Async.QueueSize <= 0 {
			return &ConfigValidationError{
				Field:   "Async.QueueSize",
				Problem: fmt.Sprintf("must be > 0 in strict mode (got %d)", c.Async.QueueSize),
			}
		}
		if c.Async.Workers <= 0 {
			return &ConfigValidationError{
				Field:   "Async.Workers",
				Problem: fmt.Sprintf("must be > 0 in strict mode (got %d)", c.Async.Workers),
			}
		}
		if c.Async.FlushInterval <= 0 {
			return &ConfigValidationError{
				Field:   "Async.FlushInterval",
				Problem: fmt.Sprintf("must be > 0 in strict mode (got %s)", c.Async.FlushInterval),
			}
		}
		if c.Async.MaxBatchBytes <= 0 {
			return &ConfigValidationError{
				Field:   "Async.MaxBatchBytes",
				Problem: fmt.Sprintf("must be > 0 in strict mode (got %d)", c.Async.MaxBatchBytes),
			}
		}
	}
	return nil
}

// ── Preset configs ────────────────────────────────────────────────────────────

// Dev returns a config suitable for local development:
// pretty-print JSON, stdout, sync, no sampling, debug level.
func Dev() Config {
	return Config{
		Level:             LevelDebug,
		Environment:       "development",
		Version:           envOr("LOZA_SERVICE_VERSION", "unknown"),
		IncludeHost:       true,
		Encoder:           PrettyJSONEncoder(),
		Sinks:             []Sink{StdoutSink()},
		BatchSize:         100,
		FlushInterval:     5 * time.Second,
		MaxBufferSize:     10000,
		MaxRetries:        3,
		MaxBackoff:        30 * time.Second,
		Timeout:           10 * time.Second,
		ConnectionTimeout: 5 * time.Second,
		EnableCompression: true,
		Async: AsyncConfig{
			Enabled: false,
		},
		FieldNaming: FieldNamingConfig{
			ExpandDotKeys: true,
		},
		Checkpoints: CheckpointConfig{
			Enabled:        true,
			MaxCheckpoints: 32,
		},
		DuplicateFieldPolicy: CanonicalWins,
		PanicRecovery:        true,
		Security: SecurityConfig{
			RedactByDefault:     false,
			AllowPII:            true,
			MaxFieldBytes:       4096,
			MaxEventBytes:       256 * 1024,
			MaxAttrCount:        512,
			DropOversizedEvents: true,
		},
		IDGen: globalIDGen,
		Clock: realClock{},
	}
}

// Production returns a config suitable for production:
// compact JSON, stdout, async, sample errors + slow requests, info level.
func Production() Config {
	return Config{
		Level:             LevelInfo,
		Environment:       envOr("LOZA_ENV", "production"),
		Version:           envOr("LOZA_SERVICE_VERSION", "unknown"),
		IncludeHost:       true,
		Encoder:           JSONEncoder(),
		Sinks:             []Sink{StdoutSink()},
		Sampler:           SampleAll(),
		BatchSize:         100,
		FlushInterval:     5 * time.Second,
		MaxBufferSize:     10000,
		MaxRetries:        3,
		MaxBackoff:        30 * time.Second,
		Timeout:           10 * time.Second,
		ConnectionTimeout: 5 * time.Second,
		EnableCompression: true,
		Async: AsyncConfig{
			Enabled:       true,
			QueueSize:     8192,
			Workers:       4,
			FlushInterval: time.Second,
			MaxBatchBytes: 4 * 1024 * 1024,
			Backpressure:  Block,
		},
		FieldNaming: FieldNamingConfig{
			ExpandDotKeys: true,
		},
		Checkpoints: CheckpointConfig{
			Enabled:        true,
			MaxCheckpoints: 32,
		},
		DuplicateFieldPolicy: CanonicalWins,
		PanicRecovery:        true,
		Security: SecurityConfig{
			RedactByDefault:     true,
			AllowPII:            false,
			MaxFieldBytes:       4096,
			MaxEventBytes:       256 * 1024,
			MaxAttrCount:        512,
			DropOversizedEvents: true,
		},
		IDGen: globalIDGen,
		Clock: realClock{},
	}
}

// Test returns a config suitable for unit tests:
// sync, no sinks, debug level.
func Test() Config {
	return Config{
		Level:             LevelDebug,
		Environment:       "test",
		Version:           "test",
		Encoder:           JSONEncoder(),
		Sinks:             []Sink{},
		BatchSize:         100,
		FlushInterval:     5 * time.Second,
		MaxBufferSize:     10000,
		MaxRetries:        3,
		MaxBackoff:        30 * time.Second,
		Timeout:           10 * time.Second,
		ConnectionTimeout: 5 * time.Second,
		EnableCompression: true,
		Async: AsyncConfig{
			Enabled: false,
		},
		FieldNaming: FieldNamingConfig{
			ExpandDotKeys: true,
		},
		Checkpoints: CheckpointConfig{
			Enabled:        true,
			MaxCheckpoints: 32,
		},
		DuplicateFieldPolicy: CanonicalWins,
		PanicRecovery:        false,
		Security: SecurityConfig{
			RedactByDefault:     false,
			AllowPII:            true,
			MaxFieldBytes:       4096,
			MaxEventBytes:       256 * 1024,
			MaxAttrCount:        512,
			DropOversizedEvents: true,
		},
		IDGen: globalIDGen,
		Clock: realClock{},
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// LoadFromEnv loads configuration from environment variables and applies them
// to the provided config. Environment variables take precedence over the base config
// but are overridden by explicit code configuration.
//
// When LOZA_DSN is set, it is parsed first and sets CollectorURL, Environment,
// Service, and Insecure. Individual env vars (LOZA_COLLECTOR_URL, etc.) override
// DSN-derived values when both are present.
//
// Supported environment variables:
//   - LOZA_DSN: loza:// connection URI (sets CollectorURL, Environment, Service, Insecure)
//   - LOZA_COLLECTOR_URL: Collector endpoint URL (overrides DSN)
//   - LOZA_SERVICE_NAME: Service name
//   - LOZA_SERVICE_VERSION: Service version
//   - LOZA_ENVIRONMENT: Deployment environment
//   - LOZA_TENANT_ID: Tenant identifier
//   - LOZA_BATCH_SIZE: Batch size for event buffering (integer)
//   - LOZA_FLUSH_INTERVAL: Flush interval duration (e.g., "5s")
//   - LOZA_MAX_BUFFER_SIZE: Maximum buffer size (integer)
//   - LOZA_MAX_RETRIES: Maximum retry attempts (integer)
//   - LOZA_MAX_BACKOFF: Maximum backoff duration (e.g., "30s")
//   - LOZA_TIMEOUT: Request timeout (e.g., "10s")
//   - LOZA_CONNECTION_TIMEOUT: Connection timeout (e.g., "5s")
//   - LOZA_ENABLE_COMPRESSION: Enable compression ("true" or "false")
func LoadFromEnv(base Config) Config {
	cfg := base

	// Load DSN first (sets CollectorURL, Environment, Service, Insecure).
	// Individual env vars below can override DSN-derived values.
	if rawDSN := os.Getenv("LOZA_DSN"); rawDSN != "" {
		if d, err := dsn.Parse(rawDSN); err == nil {
			cfg.CollectorURL = d.BaseURL
			cfg.Environment = d.Env
			if d.Service != "" {
				cfg.Service = d.Service
			}
			cfg.Insecure = !d.TLS
		}
		// If DSN parsing fails, fall through to individual env vars.
	}

	// Load collector URL (overrides DSN-derived value if both are set)
	if url := os.Getenv("LOZA_COLLECTOR_URL"); url != "" {
		cfg.CollectorURL = url
	}

	// Load authentication
	if apiKey := os.Getenv("LOZA_API_KEY"); apiKey != "" {
		cfg.APIKey = apiKey
	}

	// Load service identity
	if service := os.Getenv("LOZA_SERVICE_NAME"); service != "" {
		cfg.Service = service
	}
	if version := os.Getenv("LOZA_SERVICE_VERSION"); version != "" {
		cfg.Version = version
	}
	if env := os.Getenv("LOZA_ENVIRONMENT"); env != "" {
		cfg.Environment = env
	}
	if tenantID := os.Getenv("LOZA_TENANT_ID"); tenantID != "" {
		cfg.TenantID = tenantID
	}

	// Load batching configuration
	if batchSize := os.Getenv("LOZA_BATCH_SIZE"); batchSize != "" {
		if size, err := strconv.Atoi(batchSize); err == nil {
			cfg.BatchSize = size
		}
	}
	if flushInterval := os.Getenv("LOZA_FLUSH_INTERVAL"); flushInterval != "" {
		if interval, err := time.ParseDuration(flushInterval); err == nil {
			cfg.FlushInterval = interval
		}
	}
	if maxBufferSize := os.Getenv("LOZA_MAX_BUFFER_SIZE"); maxBufferSize != "" {
		if size, err := strconv.Atoi(maxBufferSize); err == nil {
			cfg.MaxBufferSize = size
		}
	}

	// Load retry configuration
	if maxRetries := os.Getenv("LOZA_MAX_RETRIES"); maxRetries != "" {
		if retries, err := strconv.Atoi(maxRetries); err == nil {
			cfg.MaxRetries = retries
		}
	}
	if maxBackoff := os.Getenv("LOZA_MAX_BACKOFF"); maxBackoff != "" {
		if backoff, err := time.ParseDuration(maxBackoff); err == nil {
			cfg.MaxBackoff = backoff
		}
	}
	if timeout := os.Getenv("LOZA_TIMEOUT"); timeout != "" {
		if t, err := time.ParseDuration(timeout); err == nil {
			cfg.Timeout = t
		}
	}
	if connTimeout := os.Getenv("LOZA_CONNECTION_TIMEOUT"); connTimeout != "" {
		if t, err := time.ParseDuration(connTimeout); err == nil {
			cfg.ConnectionTimeout = t
		}
	}

	// Load compression setting
	if compression := os.Getenv("LOZA_ENABLE_COMPRESSION"); compression != "" {
		cfg.EnableCompression = compression == "true" || compression == "1"
	}

	return cfg
}


// validateSDKConfig validates the SDK configuration per Requirement 32.5 and 32.6.
// Returns an error if required fields are missing or values are invalid.
func validateSDKConfig(cfg Config) error {
	if strings.TrimSpace(cfg.CollectorURL) == "" && strings.TrimSpace(cfg.CollectorEndpoint) != "" {
		cfg.CollectorURL = strings.TrimSpace(cfg.CollectorEndpoint)
	}
	if strings.TrimSpace(cfg.CollectorURL) == "" {
		return &ConfigValidationError{
			Field:   "CollectorURL",
			Problem: "collector_url is required (set LOZA_COLLECTOR_URL or pass WithCollectorURL)",
		}
	}
	if strings.TrimSpace(cfg.Service) == "" {
		return &ConfigValidationError{
			Field:   "Service",
			Problem: "service_name is required (set LOZA_SERVICE_NAME or pass WithService)",
		}
	}
	return cfg.Validate()
}

// NewClient creates a new Logger applying the full configuration precedence:
// code initialization > environment variables > configuration file > defaults.
//
// This implements Requirement 32.1, 32.4, 32.5, 32.6, 32.7, 32.8, 32.9.
//
// The cfg parameter represents code-level configuration (highest precedence).
// Environment variables are loaded automatically. A loza.yaml file is loaded
// from the current directory if present.
func NewClient(cfg Config) (*Logger, error) {
	base := Config{
		Level:   LevelInfo,
		Encoder: JSONEncoder(),
		Sinks:   []Sink{StdoutSink()},
		Async: AsyncConfig{
			Enabled:       true,
			QueueSize:     8192,
			Workers:       4,
			FlushInterval: time.Second,
			MaxBatchBytes: 4 * 1024 * 1024,
			Backpressure:  Block,
		},
		FieldNaming: FieldNamingConfig{
			ExpandDotKeys: true,
		},
		Checkpoints: CheckpointConfig{
			Enabled:        true,
			MaxCheckpoints: 32,
		},
		DuplicateFieldPolicy: CanonicalWins,
		PanicRecovery:        true,
		Security: SecurityConfig{
			RedactByDefault:     true,
			AllowPII:            false,
			MaxFieldBytes:       4096,
			MaxEventBytes:       256 * 1024,
			MaxAttrCount:        512,
			DropOversizedEvents: true,
		},
		IDGen: globalIDGen,
		Clock: realClock{},
	}

	combinedFileCfg := FileConfig{}
	defaultsFileCfg, err := LoadDefaultsFile()
	if err != nil {
		return nil, fmt.Errorf("loza: load defaults: %w", err)
	}
	combinedFileCfg = overlayFileConfig(combinedFileCfg, defaultsFileCfg)
	if userFileCfg, err := LoadFromFile(""); err == nil {
		combinedFileCfg = overlayFileConfig(combinedFileCfg, userFileCfg)
	}

	// Step 1: Apply YAML-based defaults and user config
	merged := mergeFileConfig(base, combinedFileCfg)

	// Step 2: Apply environment variables (overrides YAML config)
	merged = LoadFromEnv(merged)

	// Step 3: Apply code-level config (highest precedence)
	merged = mergeCodeConfig(merged, cfg)

	if strings.TrimSpace(merged.CollectorURL) == "" && strings.TrimSpace(merged.CollectorEndpoint) != "" {
		merged.CollectorURL = strings.TrimSpace(merged.CollectorEndpoint)
	}

	// Step 4: Validate the final config
	if err := validateSDKConfig(merged); err != nil {
		return nil, fmt.Errorf("loza: invalid configuration: %w", err)
	}

	// If no explicit sink was configured, route events to the collector endpoint
	// using HTTPBatchSink (NDJSON batching with periodic flush).
	if shouldInstallDefaultCollectorSink(merged) {
		// Build headers from config (auth + service identity)
		headers := make(map[string]string)
		if merged.APIKey != "" {
			headers["Authorization"] = "Bearer " + merged.APIKey
		}
		if merged.Service != "" {
			headers["X-Loza-Service"] = merged.Service
		}
		if merged.Environment != "" {
			headers["X-Loza-Env"] = merged.Environment
		}

		batchSink, err := HTTPBatchSink(HTTPBatchSinkConfig{
			Endpoint:      strings.TrimRight(merged.CollectorURL, "/") + "/events",
			Headers:       headers,
			BatchSize:     merged.BatchSize,
			FlushInterval: merged.Async.FlushInterval,
			Gzip:          merged.EnableCompression,
		})
		if err != nil {
			return nil, fmt.Errorf("loza: initialize httpbatch sink: %w", err)
		}
		merged.Sinks = []Sink{batchSink}
		merged.Sink = batchSink
	}

	return New(merged)
}

func shouldInstallDefaultCollectorSink(cfg Config) bool {
	if strings.TrimSpace(cfg.CollectorURL) == "" {
		return false
	}
	if cfg.Sink != nil || len(cfg.Sinks) == 0 {
		return false
	}
	if len(cfg.Sinks) != 1 {
		return false
	}
	return cfg.Sinks[0].Name() == "stdout"
}

// mergeCodeConfig merges code-level config into base, with code taking precedence
// for any non-zero/non-nil fields.
func mergeCodeConfig(base, code Config) Config {
	if code.CollectorURL != "" {
		base.CollectorURL = code.CollectorURL
	}
	if code.CollectorEndpoint != "" {
		base.CollectorEndpoint = code.CollectorEndpoint
		base.CollectorURL = code.CollectorEndpoint
	}
	if code.Service != "" {
		base.Service = code.Service
	}
	if code.Version != "" {
		base.Version = code.Version
	}
	if code.Environment != "" {
		base.Environment = code.Environment
	}
	if code.TenantID != "" {
		base.TenantID = code.TenantID
	}
	if code.BatchSize != 0 {
		base.BatchSize = code.BatchSize
	}
	if code.FlushInterval != 0 {
		base.FlushInterval = code.FlushInterval
	}
	if code.MaxBufferSize != 0 {
		base.MaxBufferSize = code.MaxBufferSize
	}
	if code.MaxRetries != 0 {
		base.MaxRetries = code.MaxRetries
	}
	if code.MaxBackoff != 0 {
		base.MaxBackoff = code.MaxBackoff
	}
	if code.Timeout != 0 {
		base.Timeout = code.Timeout
	}
	if code.ConnectionTimeout != 0 {
		base.ConnectionTimeout = code.ConnectionTimeout
	}
	if code.codeSetCompression {
		base.EnableCompression = code.EnableCompression
	}
	if code.Sink != nil {
		base.Sink = code.Sink
	}
	if len(code.Sinks) > 0 {
		base.Sinks = code.Sinks
	}
	if code.Sampler != nil {
		base.Sampler = code.Sampler
	}
	if code.Encoder != nil {
		base.Encoder = code.Encoder
	}
	if code.Redactor != nil {
		base.Redactor = code.Redactor
	}
	if code.Schema != nil {
		base.Schema = code.Schema
	}
	if code.Enricher != nil {
		base.Enricher = code.Enricher
	}
	if code.FallbackSink != nil {
		base.FallbackSink = code.FallbackSink
	}
	if code.StatsHandler != nil {
		base.StatsHandler = code.StatsHandler
	}
	if code.Level != 0 {
		base.Level = code.Level
	}
	if code.IDGen != nil {
		base.IDGen = code.IDGen
	}
	if code.Clock != nil {
		base.Clock = code.Clock
	}
	if code.codeSetAsync {
		base.Async.Enabled = code.Async.Enabled
		if code.Async.QueueSize != 0 {
			base.Async.QueueSize = code.Async.QueueSize
		}
		if code.Async.Workers != 0 {
			base.Async.Workers = code.Async.Workers
		}
		if code.Async.FlushInterval != 0 {
			base.Async.FlushInterval = code.Async.FlushInterval
		}
		if code.Async.MaxBatchBytes != 0 {
			base.Async.MaxBatchBytes = code.Async.MaxBatchBytes
		}
		if code.codeSetBackpressure {
			base.Async.Backpressure = code.Async.Backpressure
		}
	}
	// Merge security config if any field is set
	if code.Security.MaxFieldBytes != 0 || code.Security.MaxEventBytes != 0 || code.Security.MaxAttrCount != 0 {
		base.Security = code.Security
	}
	if code.codeSetRedactByDefault {
		base.Security.RedactByDefault = code.Security.RedactByDefault
	}
	if code.codeSetAllowPII {
		base.Security.AllowPII = code.Security.AllowPII
	}
	if code.codeSetDropOversized {
		base.Security.DropOversizedEvents = code.Security.DropOversizedEvents
	}
	if code.codeSetStrict {
		base.Strict = code.Strict
	}
	if code.codeSetValidateEncoded {
		base.ValidateEncoded = code.ValidateEncoded
	}
	if code.DeploymentID != "" {
		base.DeploymentID = code.DeploymentID
	}
	if code.Region != "" {
		base.Region = code.Region
	}
	return base
}
