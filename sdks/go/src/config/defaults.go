package config

import (
	"context"

	"github.com/astraive/loza/sdks/go/src/core"
)

// Production returns a Config suitable for production use.
func Production() Config { return core.Production() }

// Dev returns a Config suitable for development.
func Dev() Config { return core.Dev() }

// Test returns a Config suitable for testing.
func Test() Config { return core.Test() }

// WithService sets the service name.
func WithService(name string) ConfigOption { return core.WithService(name) }

// WithVersion sets the service version.
func WithVersion(version string) ConfigOption { return core.WithVersion(version) }

// WithEnvironment sets the deployment environment.
func WithEnvironment(env string) ConfigOption { return core.WithEnvironment(env) }

// WithSink sets the event sink.
func WithSink(sink core.Sink) ConfigOption { return core.WithSink(sink) }

// WithSampler sets the event sampler.
func WithSampler(sampler core.Sampler) ConfigOption { return core.WithSampler(sampler) }

// WithSchema sets the output schema.
func WithSchema(schema core.Schema) ConfigOption { return core.WithSchema(schema) }

// WithRedactor sets the redactor.
func WithRedactor(r core.Redactor) ConfigOption { return core.WithRedactor(r) }

// WithAsync enables async event processing.
func WithAsync(enabled bool) ConfigOption { return core.WithAsync(enabled) }

// WithCollectorURL sets the collector endpoint URL.
func WithCollectorURL(url string) ConfigOption { return core.WithCollectorURL(url) }

// WithDSN parses a loza:// connection URI and applies the resolved values.
func WithDSN(raw string) ConfigOption { return core.WithDSN(raw) }

// WithBatchSize sets the batch size for async processing.
func WithBatchSize(size int) ConfigOption { return core.WithBatchSize(size) }

// WithStrict enables strict mode validation.
func WithStrict(enabled bool) ConfigOption { return core.WithStrict(enabled) }

// WithEnricher sets a context enricher function.
func WithEnricher(fn func(ctx context.Context) []core.Attr) ConfigOption {
	return core.WithEnricher(fn)
}
