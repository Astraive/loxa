package otlp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	collectorevent "github.com/astraive/loxa-collector/internal/event"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config controls OTLP sink behavior.
type Config struct {
	Endpoint string
	Insecure bool
	// Protocol controls transport: "grpc" (default) or "http".
	Protocol string
	// Timeout controls export timeout. When unset, a sane default is used.
	Timeout time.Duration
	// GRPCReconnectionPeriod controls minimum delay between gRPC reconnect attempts.
	GRPCReconnectionPeriod time.Duration
	// Retry controls OTLP exporter retry/backoff behavior.
	Retry RetryConfig
	// Options are gRPC exporter options.
	Options []otlploggrpc.Option
	// HTTPOptions are HTTP exporter options.
	HTTPOptions []otlploghttp.Option
	Logger      string
	// ResourceAttributes are attached to the OTLP Resource.
	ResourceAttributes map[string]string
}

// RetryConfig controls OTLP exporter retry behavior.
type RetryConfig struct {
	Enabled         *bool
	InitialInterval time.Duration
	MaxInterval     time.Duration
	MaxElapsedTime  time.Duration
}

type resolvedConfig struct {
	Endpoint               string
	Insecure               bool
	Protocol               string
	Timeout                time.Duration
	GRPCReconnectionPeriod time.Duration
	Retry                  resolvedRetryConfig
	Options                []otlploggrpc.Option
	HTTPOptions            []otlploghttp.Option
	Logger                 string
	ResourceAttributes     map[string]string
}

type resolvedRetryConfig struct {
	Enabled         bool
	InitialInterval time.Duration
	MaxInterval     time.Duration
	MaxElapsedTime  time.Duration
}

type sink struct {
	provider *log.LoggerProvider
	logger   otellog.Logger
}

const (
	defaultProtocol             = "grpc"
	defaultLogger               = "loxa-collector"
	defaultTimeout              = 10 * time.Second
	defaultRetryInitialInterval = 5 * time.Second
	defaultRetryMaxInterval     = 30 * time.Second
	defaultRetryMaxElapsedTime  = time.Minute
)

// New creates an OTLP log sink backed by the OpenTelemetry Logs SDK.
func New(ctx context.Context, cfg Config) (collectorevent.Sink, error) {
	resolved, err := resolveConfig(cfg)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	exporter, err := newExporter(ctx, resolved)
	if err != nil {
		return nil, fmt.Errorf("otlp: create exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(mapToAttributes(resolved.ResourceAttributes)...),
		resource.WithAttributes(semconv.ServiceName(resolved.Logger)),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, fmt.Errorf("otlp: create resource: %w", err)
	}

	provider := log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(exporter)),
		log.WithResource(res),
	)

	return &sink{
		provider: provider,
		logger:   provider.Logger(resolved.Logger),
	}, nil
}

func (s *sink) Name() string { return "otlp" }

func (s *sink) WriteEvent(ctx context.Context, encoded []byte, ev *collectorevent.Event) error {
	if ctx == nil {
		ctx = context.Background()
	}
	payloadBytes := bytes.TrimSpace(encoded)
	if len(payloadBytes) == 0 {
		return errors.New("otlp: empty encoded event")
	}
	payload := string(payloadBytes)

	var record otellog.Record
	record.SetBody(otellog.StringValue(payload))
	record.SetSeverity(mapSeverity(ev))
	if ev != nil {
		if !ev.Timestamp.IsZero() {
			record.SetTimestamp(ev.Timestamp)
		}
		record.SetEventName(ev.Event)
		record.AddAttributes(
			otellog.String("event_id", ev.EventID),
			otellog.String("request_id", ev.RequestID),
			otellog.String("service", ev.Service),
			otellog.String("outcome", ev.Outcome),
		)
	}
	s.logger.Emit(ctx, record)
	return nil
}

func (s *sink) Flush(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.provider.ForceFlush(ctx)
}

func (s *sink) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.provider.Shutdown(ctx)
}

func mapSeverity(ev *collectorevent.Event) otellog.Severity {
	if ev == nil {
		return otellog.SeverityInfo
	}
	switch ev.Level {
	case collectorevent.LevelDebug:
		return otellog.SeverityDebug
	case collectorevent.LevelInfo:
		return otellog.SeverityInfo
	case collectorevent.LevelWarn:
		return otellog.SeverityWarn
	case collectorevent.LevelError:
		return otellog.SeverityError
	case collectorevent.LevelFatal:
		return otellog.SeverityFatal
	default:
		return otellog.SeverityInfo
	}
}

func resolveConfig(cfg Config) (resolvedConfig, error) {
	protocol := strings.ToLower(strings.TrimSpace(cfg.Protocol))
	if protocol == "" {
		protocol = defaultProtocol
	}
	if protocol != "grpc" && protocol != "http" {
		return resolvedConfig{}, fmt.Errorf("otlp: unsupported protocol %q", cfg.Protocol)
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	if err := validateEndpoint(endpoint); err != nil {
		return resolvedConfig{}, err
	}

	timeout := cfg.Timeout
	if timeout < 0 {
		return resolvedConfig{}, fmt.Errorf("otlp: timeout must be >= 0")
	}
	if timeout == 0 {
		timeout = defaultTimeout
	}

	reconnect := cfg.GRPCReconnectionPeriod
	if reconnect < 0 {
		return resolvedConfig{}, fmt.Errorf("otlp: grpc reconnection period must be >= 0")
	}

	retry, err := resolveRetryConfig(cfg.Retry)
	if err != nil {
		return resolvedConfig{}, err
	}

	logger := strings.TrimSpace(cfg.Logger)
	if logger == "" {
		logger = defaultLogger
	}

	return resolvedConfig{
		Endpoint:               endpoint,
		Insecure:               cfg.Insecure,
		Protocol:               protocol,
		Timeout:                timeout,
		GRPCReconnectionPeriod: reconnect,
		Retry:                  retry,
		Options:                append([]otlploggrpc.Option(nil), cfg.Options...),
		HTTPOptions:            append([]otlploghttp.Option(nil), cfg.HTTPOptions...),
		Logger:                 logger,
		ResourceAttributes:     copyStringMap(cfg.ResourceAttributes),
	}, nil
}

func resolveRetryConfig(cfg RetryConfig) (resolvedRetryConfig, error) {
	enabled := true
	if cfg.Enabled != nil {
		enabled = *cfg.Enabled
	}

	initial := cfg.InitialInterval
	if initial < 0 {
		return resolvedRetryConfig{}, fmt.Errorf("otlp: retry initial interval must be >= 0")
	}
	if initial == 0 {
		initial = defaultRetryInitialInterval
	}

	maxInterval := cfg.MaxInterval
	if maxInterval < 0 {
		return resolvedRetryConfig{}, fmt.Errorf("otlp: retry max interval must be >= 0")
	}
	if maxInterval == 0 {
		maxInterval = defaultRetryMaxInterval
	}
	if maxInterval < initial {
		return resolvedRetryConfig{}, fmt.Errorf("otlp: retry max interval must be >= initial interval")
	}

	maxElapsed := cfg.MaxElapsedTime
	if maxElapsed < 0 {
		return resolvedRetryConfig{}, fmt.Errorf("otlp: retry max elapsed time must be >= 0")
	}
	if maxElapsed == 0 {
		maxElapsed = defaultRetryMaxElapsedTime
	}
	if maxElapsed < initial {
		return resolvedRetryConfig{}, fmt.Errorf("otlp: retry max elapsed time must be >= initial interval")
	}

	return resolvedRetryConfig{
		Enabled:         enabled,
		InitialInterval: initial,
		MaxInterval:     maxInterval,
		MaxElapsedTime:  maxElapsed,
	}, nil
}

func validateEndpoint(endpoint string) error {
	if endpoint == "" {
		return nil
	}
	if strings.Contains(endpoint, "://") {
		u, err := url.Parse(endpoint)
		if err != nil {
			return fmt.Errorf("otlp: invalid endpoint URL: %w", err)
		}
		if strings.TrimSpace(u.Scheme) == "" || strings.TrimSpace(u.Host) == "" {
			return fmt.Errorf("otlp: endpoint URL must include scheme and host")
		}
		if u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("otlp: endpoint URL must not include query or fragment")
		}
		return nil
	}
	if strings.ContainsAny(endpoint, "?#") {
		return fmt.Errorf("otlp: endpoint must not include query or fragment")
	}
	if strings.Contains(endpoint, "/") {
		return fmt.Errorf("otlp: endpoint without scheme must be host[:port]")
	}
	return nil
}

func newExporter(ctx context.Context, cfg resolvedConfig) (log.Exporter, error) {
	switch cfg.Protocol {
	case "grpc":
		opts := append([]otlploggrpc.Option(nil), cfg.Options...)
		if cfg.Endpoint != "" {
			if strings.Contains(cfg.Endpoint, "://") {
				opts = append(opts, otlploggrpc.WithEndpointURL(cfg.Endpoint))
			} else {
				opts = append(opts, otlploggrpc.WithEndpoint(cfg.Endpoint))
			}
		}
		if cfg.Insecure {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		opts = append(opts,
			otlploggrpc.WithTimeout(cfg.Timeout),
			otlploggrpc.WithRetry(otlploggrpc.RetryConfig{
				Enabled:         cfg.Retry.Enabled,
				InitialInterval: cfg.Retry.InitialInterval,
				MaxInterval:     cfg.Retry.MaxInterval,
				MaxElapsedTime:  cfg.Retry.MaxElapsedTime,
			}),
		)
		if cfg.GRPCReconnectionPeriod > 0 {
			opts = append(opts, otlploggrpc.WithReconnectionPeriod(cfg.GRPCReconnectionPeriod))
		}
		return otlploggrpc.New(ctx, opts...)
	case "http":
		opts := append([]otlploghttp.Option(nil), cfg.HTTPOptions...)
		if cfg.Endpoint != "" {
			if strings.Contains(cfg.Endpoint, "://") {
				opts = append(opts, otlploghttp.WithEndpointURL(cfg.Endpoint))
			} else {
				opts = append(opts, otlploghttp.WithEndpoint(cfg.Endpoint))
			}
		}
		if cfg.Insecure {
			opts = append(opts, otlploghttp.WithInsecure())
		}
		opts = append(opts,
			otlploghttp.WithTimeout(cfg.Timeout),
			otlploghttp.WithRetry(otlploghttp.RetryConfig{
				Enabled:         cfg.Retry.Enabled,
				InitialInterval: cfg.Retry.InitialInterval,
				MaxInterval:     cfg.Retry.MaxInterval,
				MaxElapsedTime:  cfg.Retry.MaxElapsedTime,
			}),
		)
		return otlploghttp.New(ctx, opts...)
	default:
		return nil, fmt.Errorf("otlp: unsupported protocol %q", cfg.Protocol)
	}
}

func mapToAttributes(m map[string]string) []attribute.KeyValue {
	if len(m) == 0 {
		return nil
	}
	attrs := make([]attribute.KeyValue, 0, len(m))
	for k, v := range m {
		attrs = append(attrs, attribute.String(k, v))
	}
	return attrs
}

func copyStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
