package otlp

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/log"
)

func TestNewRejectsUnsupportedProtocol(t *testing.T) {
	_, err := New(context.Background(), Config{
		Protocol: "udp",
	})
	if err == nil {
		t.Fatalf("expected unsupported protocol error")
	}
	if !strings.Contains(err.Error(), "unsupported protocol") {
		t.Fatalf("expected unsupported protocol error, got: %v", err)
	}
}

func TestResolveConfigRejectsEndpointQuery(t *testing.T) {
	_, err := resolveConfig(Config{
		Endpoint: "https://otel.example:4317/v1/logs?bad=1",
	})
	if err == nil {
		t.Fatalf("expected endpoint validation error")
	}
	if !strings.Contains(err.Error(), "must not include query") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveConfigRejectsInvalidRetryWindow(t *testing.T) {
	_, err := resolveConfig(Config{
		Retry: RetryConfig{
			InitialInterval: 2 * time.Second,
			MaxInterval:     time.Second,
		},
	})
	if err == nil {
		t.Fatalf("expected retry validation error")
	}
	if !strings.Contains(err.Error(), "max interval") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveConfigAppliesDefaults(t *testing.T) {
	cfg, err := resolveConfig(Config{})
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	if cfg.Protocol != "grpc" {
		t.Fatalf("expected default protocol grpc, got %q", cfg.Protocol)
	}
	if cfg.Logger != "loxa-collector" {
		t.Fatalf("expected default logger loxa-collector, got %q", cfg.Logger)
	}
	if cfg.Timeout != 10*time.Second {
		t.Fatalf("expected default timeout 10s, got %s", cfg.Timeout)
	}
	if !cfg.Retry.Enabled || cfg.Retry.InitialInterval <= 0 || cfg.Retry.MaxInterval < cfg.Retry.InitialInterval {
		t.Fatalf("unexpected default retry config: %+v", cfg.Retry)
	}
}

func TestWriteEventRejectsEmptyPayload(t *testing.T) {
	provider := log.NewLoggerProvider()
	s := &sink{
		provider: provider,
		logger:   provider.Logger("test"),
	}
	err := s.WriteEvent(context.Background(), []byte("   \n"), nil)
	if err == nil {
		t.Fatalf("expected empty payload error")
	}
	if !strings.Contains(err.Error(), "empty encoded event") {
		t.Fatalf("unexpected error: %v", err)
	}
}
