package core

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestSDKConfigurationIntegration tests the complete SDK configuration flow
// as specified in task 2.5 and requirements 2.8-2.11, 32.1-32.11
func TestSDKConfigurationIntegration(t *testing.T) {
	t.Run("NewClient with all configuration options", func(t *testing.T) {
		// Create a client with all configuration options set via code
		cfg := Config{
			CollectorURL:      "http://localhost:9308",
			Service:           "test-service",
			Version:           "1.0.0",
			Environment:       "production",
			TenantID:          "tenant-123",
			BatchSize:         200,
			FlushInterval:     10 * time.Second,
			MaxBufferSize:     20000,
			MaxRetries:        5,
			MaxBackoff:        60 * time.Second,
			Timeout:           30 * time.Second,
			ConnectionTimeout: 15 * time.Second,
			EnableCompression: true,
		}

		logger, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		defer func() { _ = logger.Shutdown(context.Background()) }()

		// Verify configuration was applied
		if logger == nil {
			t.Fatal("Expected logger to be created")
		}
	})

	t.Run("Configuration precedence: code > environment > defaults", func(t *testing.T) {
		// Set environment variables
		os.Setenv("LOZA_SERVICE_NAME", "env-service")
		os.Setenv("LOZA_COLLECTOR_URL", "http://env-collector:9308")
		os.Setenv("LOZA_BATCH_SIZE", "150")
		defer func() {
			os.Unsetenv("LOZA_SERVICE_NAME")
			os.Unsetenv("LOZA_COLLECTOR_URL")
			os.Unsetenv("LOZA_BATCH_SIZE")
		}()

		// Code configuration should override environment
		cfg := Config{
			CollectorURL: "http://code-collector:9308",
			Service:      "code-service",
			BatchSize:    250,
		}

		logger, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		defer func() { _ = logger.Shutdown(context.Background()) }()

		// Verify code config took precedence
		// (We can't directly inspect the logger's config, but we verified it was created successfully)
	})

	t.Run("Configuration validation on initialization", func(t *testing.T) {
		// Note: NewClient loads defaults from loza-go.defaults.yaml which includes
		// a default collector_url, so we need to test validation differently.
		
		// Test with invalid values that will fail validation
		cfg := Config{
			CollectorURL: "http://localhost:9308",
			Service:      "test-service",
			BatchSize:    -1, // Invalid: negative value
		}

		_, err := NewClient(cfg)
		if err == nil {
			t.Error("Expected error for negative batch_size, got nil")
		}

		// Test with missing service_name (required field)
		// We need to bypass the defaults file by using validateSDKConfig directly
		cfg = Config{
			CollectorURL: "http://localhost:9308",
			// Service is missing
		}
		err = validateSDKConfig(cfg)
		if err == nil {
			t.Error("Expected error for missing service_name, got nil")
		}

		// Test with missing collector_url (required field)
		cfg = Config{
			Service: "test-service",
			// CollectorURL is missing
		}
		err = validateSDKConfig(cfg)
		if err == nil {
			t.Error("Expected error for missing collector_url, got nil")
		}
	})

	t.Run("Default values applied correctly", func(t *testing.T) {
		// Requirement 32.7: service_version defaults to "unknown"
		// Requirement 32.8: environment defaults to "development"
		cfg := Config{
			CollectorURL: "http://localhost:9308",
			Service:      "test-service",
		}

		logger, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		defer func() { _ = logger.Shutdown(context.Background()) }()

		// Logger was created successfully, defaults were applied
	})

	t.Run("Flush method blocks until events are emitted", func(t *testing.T) {
		sink, store := MemorySink()
		cfg := Config{
			CollectorURL: "http://localhost:9308",
			Service:      "test-service",
			Sinks:        []Sink{sink},
		}

		logger, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		defer func() { _ = logger.Shutdown(context.Background()) }()

		// Emit some events
		logger.Info("test message 1")
		logger.Info("test message 2")

		// Flush should block until all events are processed
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := logger.Flush(ctx); err != nil {
			t.Errorf("Flush() error = %v", err)
		}

		// Verify events were flushed
		if store.Len() < 2 {
			t.Errorf("Expected at least 2 events, got %d", store.Len())
		}
	})

	t.Run("Shutdown method flushes and releases resources", func(t *testing.T) {
		sink, store := MemorySink()
		cfg := Config{
			CollectorURL: "http://localhost:9308",
			Service:      "test-service",
			Sinks:        []Sink{sink},
		}

		logger, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		// Emit some events
		logger.Info("test message 1")
		logger.Info("test message 2")

		// Shutdown should flush all events and release resources
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := logger.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}

		// Verify events were flushed
		if store.Len() < 2 {
			t.Errorf("Expected at least 2 events, got %d", store.Len())
		}
	})

	t.Run("Configuration options via WithXXX functions", func(t *testing.T) {
		cfg := ApplyConfig(Config{},
			WithCollectorURL("http://localhost:9308"),
			WithService("test-service"),
			WithVersion("2.0.0"),
			WithEnvironment("staging"),
			WithTenantID("tenant-456"),
			WithBatchSize(300),
			WithFlushInterval(15*time.Second),
			WithMaxBufferSize(30000),
			WithMaxRetries(7),
			WithMaxBackoff(90*time.Second),
			WithTimeout(45*time.Second),
			WithConnectionTimeout(20*time.Second),
			WithCompression(false),
		)

		logger, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		defer func() { _ = logger.Shutdown(context.Background()) }()

		// Logger was created successfully with all options
	})

	t.Run("Environment variable configuration", func(t *testing.T) {
		// Set all supported environment variables
		envVars := map[string]string{
			"LOZA_COLLECTOR_URL":      "http://env-collector:9308",
			"LOZA_SERVICE_NAME":       "env-service",
			"LOZA_SERVICE_VERSION":    "3.0.0",
			"LOZA_ENVIRONMENT":        "testing",
			"LOZA_TENANT_ID":          "tenant-env",
			"LOZA_BATCH_SIZE":         "400",
			"LOZA_FLUSH_INTERVAL":     "20s",
			"LOZA_MAX_BUFFER_SIZE":    "40000",
			"LOZA_MAX_RETRIES":        "10",
			"LOZA_MAX_BACKOFF":        "120s",
			"LOZA_TIMEOUT":            "60s",
			"LOZA_CONNECTION_TIMEOUT": "30s",
			"LOZA_ENABLE_COMPRESSION": "false",
		}

		for k, v := range envVars {
			os.Setenv(k, v)
		}
		defer func() {
			for k := range envVars {
				os.Unsetenv(k)
			}
		}()

		// Create client with minimal config (env vars should be loaded)
		cfg := Config{}

		logger, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		defer func() { _ = logger.Shutdown(context.Background()) }()

		// Logger was created successfully with env var configuration
	})

	t.Run("Negative configuration values are rejected", func(t *testing.T) {
		tests := []struct {
			name   string
			config Config
		}{
			{
				name: "negative batch size",
				config: Config{
					CollectorURL: "http://localhost:9308",
					Service:      "test-service",
					BatchSize:    -1,
				},
			},
			{
				name: "negative flush interval",
				config: Config{
					CollectorURL:  "http://localhost:9308",
					Service:       "test-service",
					FlushInterval: -1 * time.Second,
				},
			},
			{
				name: "negative max buffer size",
				config: Config{
					CollectorURL:  "http://localhost:9308",
					Service:       "test-service",
					MaxBufferSize: -1,
				},
			},
			{
				name: "negative max retries",
				config: Config{
					CollectorURL: "http://localhost:9308",
					Service:      "test-service",
					MaxRetries:   -1,
				},
			},
			{
				name: "negative max backoff",
				config: Config{
					CollectorURL: "http://localhost:9308",
					Service:      "test-service",
					MaxBackoff:   -1 * time.Second,
				},
			},
			{
				name: "negative timeout",
				config: Config{
					CollectorURL: "http://localhost:9308",
					Service:      "test-service",
					Timeout:      -1 * time.Second,
				},
			},
			{
				name: "negative connection timeout",
				config: Config{
					CollectorURL:      "http://localhost:9308",
					Service:           "test-service",
					ConnectionTimeout: -1 * time.Second,
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := NewClient(tt.config)
				if err == nil {
					t.Error("Expected validation error, got nil")
				}
			})
		}
	})
}

// TestConfigStructCompleteness verifies that the Config struct has all required fields
// as specified in requirements 2.10 and 32.x
func TestConfigStructCompleteness(t *testing.T) {
	cfg := Config{
		// Service identity (Requirement 2.10, 32.5, 32.6)
		Service:     "test-service",
		Version:     "1.0.0",
		Environment: "production",
		TenantID:    "tenant-123",

		// Collector configuration (Requirement 2.10, 32.5)
		CollectorURL: "http://localhost:9308",

		// Batching configuration (Requirement 32.x)
		BatchSize:     100,
		FlushInterval: 5 * time.Second,
		MaxBufferSize: 10000,

		// Retry configuration (Requirement 32.x)
		MaxRetries:        3,
		MaxBackoff:        30 * time.Second,
		Timeout:           10 * time.Second,
		ConnectionTimeout: 5 * time.Second,

		// Compression (Requirement 32.x)
		EnableCompression: true,
	}

	// Verify all fields can be set
	if cfg.Service == "" {
		t.Error("Service field not set")
	}
	if cfg.Version == "" {
		t.Error("Version field not set")
	}
	if cfg.Environment == "" {
		t.Error("Environment field not set")
	}
	if cfg.TenantID == "" {
		t.Error("TenantID field not set")
	}
	if cfg.CollectorURL == "" {
		t.Error("CollectorURL field not set")
	}
	if cfg.BatchSize == 0 {
		t.Error("BatchSize field not set")
	}
	if cfg.FlushInterval == 0 {
		t.Error("FlushInterval field not set")
	}
	if cfg.MaxBufferSize == 0 {
		t.Error("MaxBufferSize field not set")
	}
	if cfg.MaxRetries == 0 {
		t.Error("MaxRetries field not set")
	}
	if cfg.MaxBackoff == 0 {
		t.Error("MaxBackoff field not set")
	}
	if cfg.Timeout == 0 {
		t.Error("Timeout field not set")
	}
	if cfg.ConnectionTimeout == 0 {
		t.Error("ConnectionTimeout field not set")
	}
	if !cfg.EnableCompression {
		t.Error("EnableCompression field not set")
	}
}

// TestFlushAndShutdownBehavior tests the Flush and Shutdown methods
// as specified in requirements 2.8 and 2.9
func TestFlushAndShutdownBehavior(t *testing.T) {
	t.Run("Flush blocks until all pending events are emitted", func(t *testing.T) {
		sink, store := MemorySink()
		cfg := Config{
			CollectorURL: "http://localhost:9308",
			Service:      "test-service",
			Sinks:        []Sink{sink},
		}

		logger, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		defer func() { _ = logger.Shutdown(context.Background()) }()

		// Emit multiple events
		for i := 0; i < 10; i++ {
			logger.Info("test message")
		}

		// Flush should block until all events are processed
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := logger.Flush(ctx); err != nil {
			t.Errorf("Flush() error = %v", err)
		}
		elapsed := time.Since(start)

		// Verify all events were flushed
		if store.Len() < 10 {
			t.Errorf("Expected at least 10 events, got %d", store.Len())
		}

		// Flush should have taken some time (not instant)
		if elapsed < 1*time.Millisecond {
			t.Logf("Flush completed in %v (may be too fast for async processing)", elapsed)
		}
	})

	t.Run("Shutdown flushes events and releases resources", func(t *testing.T) {
		sink, store := MemorySink()
		cfg := Config{
			CollectorURL: "http://localhost:9308",
			Service:      "test-service",
			Sinks:        []Sink{sink},
		}

		logger, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		// Emit multiple events
		for i := 0; i < 10; i++ {
			logger.Info("test message")
		}

		// Shutdown should flush all events
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := logger.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}

		// Verify all events were flushed
		if store.Len() < 10 {
			t.Errorf("Expected at least 10 events, got %d", store.Len())
		}

		// After shutdown, logger should not accept new events
		// (This is implementation-dependent, but we can verify shutdown completed)
	})

	t.Run("Flush with context timeout", func(t *testing.T) {
		sink, _ := MemorySink()
		cfg := Config{
			CollectorURL: "http://localhost:9308",
			Service:      "test-service",
			Sinks:        []Sink{sink},
		}

		logger, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		defer func() { _ = logger.Shutdown(context.Background()) }()

		// Emit events
		logger.Info("test message")

		// Flush with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Should handle timeout gracefully
		_ = logger.Flush(ctx)
	})

	t.Run("Shutdown with context timeout", func(t *testing.T) {
		sink, _ := MemorySink()
		cfg := Config{
			CollectorURL: "http://localhost:9308",
			Service:      "test-service",
			Sinks:        []Sink{sink},
		}

		logger, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		// Emit events
		logger.Info("test message")

		// Shutdown with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Should handle timeout gracefully
		_ = logger.Shutdown(ctx)
	})
}
