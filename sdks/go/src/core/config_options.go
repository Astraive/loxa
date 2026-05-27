package core

import (
	"time"

	"github.com/astraive/loxa/spec/dsn"
)

// ConfigOption mutates and returns a Config.
type ConfigOption func(Config) Config

// ApplyConfig applies options to cfg in order.
func ApplyConfig(cfg Config, options ...ConfigOption) Config {
	for _, opt := range options {
		if opt != nil {
			cfg = opt(cfg)
		}
	}
	return cfg
}

// WithService applies service name.
func WithService(service string) ConfigOption {
	return func(cfg Config) Config { return cfg.WithService(service) }
}

// WithAlias applies logical alias metadata without changing service.
func WithAlias(alias string) ConfigOption {
	return func(cfg Config) Config { return cfg.WithAlias(alias) }
}

// WithVersion applies version.
func WithVersion(version string) ConfigOption {
	return func(cfg Config) Config { return cfg.WithVersion(version) }
}

// WithEnvironment applies environment.
func WithEnvironment(environment string) ConfigOption {
	return func(cfg Config) Config { return cfg.WithEnvironment(environment) }
}

// WithSink applies sink.
func WithSink(sink Sink) ConfigOption {
	return func(cfg Config) Config { return cfg.WithSink(sink) }
}

// WithSampler applies sampler.
func WithSampler(sampler Sampler) ConfigOption {
	return func(cfg Config) Config { return cfg.WithSampler(sampler) }
}

// WithEncoder applies encoder.
func WithEncoder(encoder Encoder) ConfigOption {
	return func(cfg Config) Config { return cfg.WithEncoder(encoder) }
}

// WithRedactor applies redactor.
func WithRedactor(redactor Redactor) ConfigOption {
	return func(cfg Config) Config { return cfg.WithRedactor(redactor) }
}

// WithSchema applies event schema.
func WithSchema(schema Schema) ConfigOption {
	return func(cfg Config) Config { return cfg.WithSchema(schema) }
}

// WithEventSchema applies event schema.
func WithEventSchema(schema Schema) ConfigOption {
	return func(cfg Config) Config { return cfg.WithEventSchema(schema) }
}

// WithAsync applies async enabled state.
func WithAsync(enabled bool) ConfigOption {
	return func(cfg Config) Config { return cfg.WithAsync(enabled) }
}

// WithAsyncQueue applies async queue size.
func WithAsyncQueue(size int) ConfigOption {
	return func(cfg Config) Config { return cfg.WithAsyncQueue(size) }
}

// WithWorkers applies async worker count.
func WithWorkers(workers int) ConfigOption {
	return func(cfg Config) Config { return cfg.WithWorkers(workers) }
}

// WithAsyncFlushInterval applies async flush interval.
func WithAsyncFlushInterval(interval time.Duration) ConfigOption {
	return func(cfg Config) Config { return cfg.WithAsyncFlushInterval(interval) }
}

// WithAsyncMaxBatchBytes applies async max batch size.
func WithAsyncMaxBatchBytes(maxBytes int) ConfigOption {
	return func(cfg Config) Config { return cfg.WithAsyncMaxBatchBytes(maxBytes) }
}

// WithBackpressure applies backpressure policy.
func WithBackpressure(policy BackpressurePolicy) ConfigOption {
	return func(cfg Config) Config { return cfg.WithBackpressure(policy) }
}

// WithDuplicatePolicy applies duplicate field policy.
func WithDuplicatePolicy(policy DuplicateFieldPolicy) ConfigOption {
	return func(cfg Config) Config { return cfg.WithDuplicatePolicy(policy) }
}

// WithStrict applies strict mode.
func WithStrict(strict bool) ConfigOption {
	return func(cfg Config) Config { return cfg.WithStrict(strict) }
}

// WithValidateEncoded controls post-encode spec contract validation in strict mode.
// Default true. Set false for custom schemas that deviate from LOXA shape.
func WithValidateEncoded(validate bool) ConfigOption {
	return func(cfg Config) Config { return cfg.WithValidateEncoded(validate) }
}

// WithEnricher applies context enricher.
func WithEnricher(enricher ContextEnricher) ConfigOption {
	return func(cfg Config) Config { return cfg.WithEnricher(enricher) }
}

// WithFallbackSink applies fallback sink.
func WithFallbackSink(sink Sink) ConfigOption {
	return func(cfg Config) Config { return cfg.WithFallbackSink(sink) }
}

// WithCollectorEndpoint applies the default collector endpoint.
func WithCollectorEndpoint(endpoint string) ConfigOption {
	return func(cfg Config) Config { return cfg.WithCollectorEndpoint(endpoint) }
}

// WithStatsHandler applies stats handler callbacks.
func WithStatsHandler(handler StatsHandler) ConfigOption {
	return func(cfg Config) Config { return cfg.WithStatsHandler(handler) }
}

// WithDeploymentID applies a deployment identifier.
func WithDeploymentID(deploymentID string) ConfigOption {
	return func(cfg Config) Config { return cfg.WithDeploymentID(deploymentID) }
}

// WithIncludeHost applies host metadata inclusion.
func WithIncludeHost(includeHost bool) ConfigOption {
	return func(cfg Config) Config { return cfg.WithIncludeHost(includeHost) }
}

// WithPanicRecovery applies panic recovery behavior.
func WithPanicRecovery(panicRecovery bool) ConfigOption {
	return func(cfg Config) Config { return cfg.WithPanicRecovery(panicRecovery) }
}

// WithCollectorURL applies the collector URL.
func WithCollectorURL(url string) ConfigOption {
	return func(cfg Config) Config {
		cfg.CollectorURL = url
		return cfg
	}
}

// WithTenantID applies the tenant ID.
func WithTenantID(tenantID string) ConfigOption {
	return func(cfg Config) Config {
		cfg.TenantID = tenantID
		return cfg
	}
}

// WithBatchSize applies the batch size.
func WithBatchSize(size int) ConfigOption {
	return func(cfg Config) Config {
		cfg.BatchSize = size
		return cfg
	}
}

// WithFlushInterval applies the flush interval.
func WithFlushInterval(interval time.Duration) ConfigOption {
	return func(cfg Config) Config {
		cfg.FlushInterval = interval
		return cfg
	}
}

// WithMaxBufferSize applies the maximum buffer size.
func WithMaxBufferSize(size int) ConfigOption {
	return func(cfg Config) Config {
		cfg.MaxBufferSize = size
		return cfg
	}
}

// WithMaxRetries applies the maximum retry attempts.
func WithMaxRetries(retries int) ConfigOption {
	return func(cfg Config) Config {
		cfg.MaxRetries = retries
		return cfg
	}
}

// WithMaxBackoff applies the maximum backoff duration.
func WithMaxBackoff(backoff time.Duration) ConfigOption {
	return func(cfg Config) Config {
		cfg.MaxBackoff = backoff
		return cfg
	}
}

// WithTimeout applies the request timeout.
func WithTimeout(timeout time.Duration) ConfigOption {
	return func(cfg Config) Config {
		cfg.Timeout = timeout
		return cfg
	}
}

// WithConnectionTimeout applies the connection timeout.
func WithConnectionTimeout(timeout time.Duration) ConfigOption {
	return func(cfg Config) Config {
		cfg.ConnectionTimeout = timeout
		return cfg
	}
}

// WithCompression applies the compression setting.
func WithCompression(enabled bool) ConfigOption {
	return func(cfg Config) Config { return cfg.WithCompression(enabled) }
}

// WithLevel applies the minimum log level. Events below this level are dropped.
func WithLevel(level Level) ConfigOption {
	return func(cfg Config) Config {
		cfg.Level = level
		return cfg
	}
}

// WithRegion applies the deployment region.
func WithRegion(region string) ConfigOption {
	return func(cfg Config) Config {
		cfg.Region = region
		return cfg
	}
}

// WithRelease applies the release version (alias for WithVersion).
func WithRelease(release string) ConfigOption { return WithVersion(release) }

// WithNamespace sets the logical namespace for the SDK client (multi-tenant).
func WithNamespace(namespace string) ConfigOption {
	return func(cfg Config) Config {
		cfg.Environment = namespace
		return cfg
	}
}

// WithAPIKey sets the ingest API key for collector authentication.
func WithAPIKey(apiKey string) ConfigOption {
	return func(cfg Config) Config {
		cfg.APIKey = apiKey
		return cfg
	}
}

// WithOtelBridge enables or disables OpenTelemetry bridge integration.
func WithOtelBridge(enabled bool) ConfigOption {
	return func(cfg Config) Config {
		if enabled {
			cfg.Async.Enabled = true
		}
		return cfg
	}
}

// WithRetry configures the maximum retry attempts.
func WithRetry(maxRetries int) ConfigOption { return WithMaxRetries(maxRetries) }

// WithQueueSize sets the async queue size.
func WithQueueSize(size int) ConfigOption { return WithAsyncQueue(size) }

// WithLogger sets a custom logger instance as the parent.
func WithLogger(l *Logger) ConfigOption {
	return func(cfg Config) Config {
		if l != nil {
			l.mu.RLock()
			cfg = l.cfg
			l.mu.RUnlock()
		}
		return cfg
	}
}

// WithDSN parses a loxa:// connection URI and applies the resolved values
// to the config. It sets CollectorURL, Environment, Service (if present in
// the DSN), and Insecure (derived from TLS setting).
//
// Individual config options or env vars applied after WithDSN will override
// the DSN-derived values.
//
// Example:
//
//	config.NewClient(config.Production(),
//	    config.WithDSN("loxa://localhost:8080/demo?env=dev&tls=false"),
//	)
func WithDSN(raw string) ConfigOption {
	return func(cfg Config) Config {
		d, err := dsn.Parse(raw)
		if err != nil {
			// Store the parse error; it will surface during NewClient validation.
			cfg.CollectorURL = "" // signal invalid state
			return cfg
		}
		cfg.CollectorURL = d.BaseURL
		cfg.Environment = d.Env
		if d.Service != "" {
			cfg.Service = d.Service
		}
		cfg.Insecure = !d.TLS
		return cfg
	}
}

// Disabled returns a config preset that disables all output (no-op).
func Disabled() Config {
	return Config{
		Level:   LevelFatal,
		Encoder: JSONEncoder(),
		Sinks:   []Sink{NoopSink()},
		Sampler: SampleNone(),
		Async: AsyncConfig{
			Enabled: false,
		},
		FieldNaming: FieldNamingConfig{
			ExpandDotKeys: true,
		},
		DuplicateFieldPolicy: CanonicalWins,
		IDGen:                globalIDGen,
		Clock:                realClock{},
	}
}
