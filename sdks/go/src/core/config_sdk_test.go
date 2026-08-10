package core

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config with all fields",
			config: Config{
				Service:           "test-service",
				Version:           "1.0.0",
				Environment:       "production",
				CollectorURL:      "http://localhost:9308",
				TenantID:          "tenant-1",
				BatchSize:         100,
				FlushInterval:     5 * time.Second,
				MaxBufferSize:     10000,
				MaxRetries:        3,
				MaxBackoff:        30 * time.Second,
				Timeout:           10 * time.Second,
				ConnectionTimeout: 5 * time.Second,
				EnableCompression: true,
			},
			wantErr: false,
		},
		{
			name: "negative batch size",
			config: Config{
				BatchSize: -1,
			},
			wantErr: true,
		},
		{
			name: "negative flush interval",
			config: Config{
				FlushInterval: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "negative max buffer size",
			config: Config{
				MaxBufferSize: -1,
			},
			wantErr: true,
		},
		{
			name: "negative max retries",
			config: Config{
				MaxRetries: -1,
			},
			wantErr: true,
		},
		{
			name: "negative max backoff",
			config: Config{
				MaxBackoff: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			config: Config{
				Timeout: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "negative connection timeout",
			config: Config{
				ConnectionTimeout: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "strict mode requires service name",
			config: Config{
				Strict: true,
			},
			wantErr: true,
		},
		{
			name: "strict mode with valid config",
			config: Config{
				Strict:  true,
				Service: "test-service",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Save original env vars
	originalEnv := map[string]string{
		"LOZA_COLLECTOR_URL":      os.Getenv("LOZA_COLLECTOR_URL"),
		"LOZA_SERVICE_NAME":       os.Getenv("LOZA_SERVICE_NAME"),
		"LOZA_SERVICE_VERSION":    os.Getenv("LOZA_SERVICE_VERSION"),
		"LOZA_ENVIRONMENT":        os.Getenv("LOZA_ENVIRONMENT"),
		"LOZA_TENANT_ID":          os.Getenv("LOZA_TENANT_ID"),
		"LOZA_FLUSH_INTERVAL":     os.Getenv("LOZA_FLUSH_INTERVAL"),
		"LOZA_MAX_BACKOFF":        os.Getenv("LOZA_MAX_BACKOFF"),
		"LOZA_TIMEOUT":            os.Getenv("LOZA_TIMEOUT"),
		"LOZA_CONNECTION_TIMEOUT": os.Getenv("LOZA_CONNECTION_TIMEOUT"),
		"LOZA_ENABLE_COMPRESSION": os.Getenv("LOZA_ENABLE_COMPRESSION"),
	}

	// Restore env vars after test
	defer func() {
		for k, v := range originalEnv {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Set test env vars
	os.Setenv("LOZA_COLLECTOR_URL", "http://test-collector:9308")
	os.Setenv("LOZA_SERVICE_NAME", "env-service")
	os.Setenv("LOZA_SERVICE_VERSION", "2.0.0")
	os.Setenv("LOZA_ENVIRONMENT", "staging")
	os.Setenv("LOZA_TENANT_ID", "tenant-env")
	os.Setenv("LOZA_FLUSH_INTERVAL", "10s")
	os.Setenv("LOZA_MAX_BACKOFF", "60s")
	os.Setenv("LOZA_TIMEOUT", "20s")
	os.Setenv("LOZA_CONNECTION_TIMEOUT", "10s")
	os.Setenv("LOZA_ENABLE_COMPRESSION", "false")

	base := Config{
		Service:           "base-service",
		Version:           "1.0.0",
		Environment:       "development",
		CollectorURL:      "http://base-collector:9308",
		TenantID:          "tenant-base",
		FlushInterval:     5 * time.Second,
		MaxBackoff:        30 * time.Second,
		Timeout:           10 * time.Second,
		ConnectionTimeout: 5 * time.Second,
		EnableCompression: true,
	}

	cfg := LoadFromEnv(base)

	// Verify env vars override base config
	if cfg.CollectorURL != "http://test-collector:9308" {
		t.Errorf("CollectorURL = %v, want %v", cfg.CollectorURL, "http://test-collector:9308")
	}
	if cfg.Service != "env-service" {
		t.Errorf("Service = %v, want %v", cfg.Service, "env-service")
	}
	if cfg.Version != "2.0.0" {
		t.Errorf("Version = %v, want %v", cfg.Version, "2.0.0")
	}
	if cfg.Environment != "staging" {
		t.Errorf("Environment = %v, want %v", cfg.Environment, "staging")
	}
	if cfg.TenantID != "tenant-env" {
		t.Errorf("TenantID = %v, want %v", cfg.TenantID, "tenant-env")
	}
	if cfg.FlushInterval != 10*time.Second {
		t.Errorf("FlushInterval = %v, want %v", cfg.FlushInterval, 10*time.Second)
	}
	if cfg.MaxBackoff != 60*time.Second {
		t.Errorf("MaxBackoff = %v, want %v", cfg.MaxBackoff, 60*time.Second)
	}
	if cfg.Timeout != 20*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 20*time.Second)
	}
	if cfg.ConnectionTimeout != 10*time.Second {
		t.Errorf("ConnectionTimeout = %v, want %v", cfg.ConnectionTimeout, 10*time.Second)
	}
	if cfg.EnableCompression != false {
		t.Errorf("EnableCompression = %v, want %v", cfg.EnableCompression, false)
	}
}

func TestConfigPrecedence(t *testing.T) {
	// Test: code > environment > defaults

	// Set env vars
	os.Setenv("LOZA_SERVICE_NAME", "env-service")
	os.Setenv("LOZA_COLLECTOR_URL", "http://env-collector:9308")
	defer func() {
		os.Unsetenv("LOZA_SERVICE_NAME")
		os.Unsetenv("LOZA_COLLECTOR_URL")
	}()

	// Start with defaults
	cfg := Dev()

	// Apply env vars
	cfg = LoadFromEnv(cfg)

	// Verify env vars override defaults
	if cfg.Service != "env-service" {
		t.Errorf("Service = %v, want %v (env should override defaults)", cfg.Service, "env-service")
	}
	if cfg.CollectorURL != "http://env-collector:9308" {
		t.Errorf("CollectorURL = %v, want %v (env should override defaults)", cfg.CollectorURL, "http://env-collector:9308")
	}

	// Apply code config (should override env)
	cfg = ApplyConfig(cfg,
		WithService("code-service"),
		WithCollectorURL("http://code-collector:9308"),
	)

	// Verify code config overrides env
	if cfg.Service != "code-service" {
		t.Errorf("Service = %v, want %v (code should override env)", cfg.Service, "code-service")
	}
	if cfg.CollectorURL != "http://code-collector:9308" {
		t.Errorf("CollectorURL = %v, want %v (code should override env)", cfg.CollectorURL, "http://code-collector:9308")
	}
}

func TestConfigOptions(t *testing.T) {
	cfg := Config{}

	cfg = ApplyConfig(cfg,
		WithCollectorURL("http://localhost:9308"),
		WithTenantID("tenant-123"),
		WithBatchSize(200),
		WithFlushInterval(10*time.Second),
		WithMaxBufferSize(20000),
		WithMaxRetries(5),
		WithMaxBackoff(60*time.Second),
		WithTimeout(30*time.Second),
		WithConnectionTimeout(15*time.Second),
		WithCompression(false),
	)

	if cfg.CollectorURL != "http://localhost:9308" {
		t.Errorf("CollectorURL = %v, want %v", cfg.CollectorURL, "http://localhost:9308")
	}
	if cfg.TenantID != "tenant-123" {
		t.Errorf("TenantID = %v, want %v", cfg.TenantID, "tenant-123")
	}
	if cfg.BatchSize != 200 {
		t.Errorf("BatchSize = %v, want %v", cfg.BatchSize, 200)
	}
	if cfg.FlushInterval != 10*time.Second {
		t.Errorf("FlushInterval = %v, want %v", cfg.FlushInterval, 10*time.Second)
	}
	if cfg.MaxBufferSize != 20000 {
		t.Errorf("MaxBufferSize = %v, want %v", cfg.MaxBufferSize, 20000)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %v, want %v", cfg.MaxRetries, 5)
	}
	if cfg.MaxBackoff != 60*time.Second {
		t.Errorf("MaxBackoff = %v, want %v", cfg.MaxBackoff, 60*time.Second)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 30*time.Second)
	}
	if cfg.ConnectionTimeout != 15*time.Second {
		t.Errorf("ConnectionTimeout = %v, want %v", cfg.ConnectionTimeout, 15*time.Second)
	}
	if cfg.EnableCompression != false {
		t.Errorf("EnableCompression = %v, want %v", cfg.EnableCompression, false)
	}
}

func TestFlushAndShutdown(t *testing.T) {
	// Create a test logger with memory sink
	sink, store := MemorySink()
	cfg := Test()
	cfg.Sinks = []Sink{sink}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Emit an event
	logger.Info("test message")

	// Flush should work
	if err := logger.Flush(context.TODO()); err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	// Verify event was flushed
	if store.Len() == 0 {
		t.Error("Expected events to be flushed, got none")
	}

	// Shutdown should work
	if err := logger.Shutdown(context.TODO()); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}
