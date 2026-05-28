package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSimpleYAML(t *testing.T) {
	content := `
# LOXA SDK configuration
collector_url: http://localhost:9308
service_name: my-service
service_version: 1.2.3
environment: production
tenant_id: tenant-abc
batch_size: 200
flush_interval: 10s
max_buffer_size: 5000
max_retries: 5
max_backoff: 60s
timeout: 15s
connection_timeout: 8s
enable_compression: true
`
	var fc FileConfig
	if err := parseSimpleYAML(content, &fc); err != nil {
		t.Fatalf("parseSimpleYAML() error = %v", err)
	}

	if fc.CollectorURL != "http://localhost:9308" {
		t.Errorf("CollectorURL = %q, want %q", fc.CollectorURL, "http://localhost:9308")
	}
	if fc.ServiceName != "my-service" {
		t.Errorf("ServiceName = %q, want %q", fc.ServiceName, "my-service")
	}
	if fc.ServiceVersion != "1.2.3" {
		t.Errorf("ServiceVersion = %q, want %q", fc.ServiceVersion, "1.2.3")
	}
	if fc.Environment != "production" {
		t.Errorf("Environment = %q, want %q", fc.Environment, "production")
	}
	if fc.TenantID != "tenant-abc" {
		t.Errorf("TenantID = %q, want %q", fc.TenantID, "tenant-abc")
	}
	if fc.BatchSize != 200 {
		t.Errorf("BatchSize = %d, want %d", fc.BatchSize, 200)
	}
	if fc.FlushInterval != "10s" {
		t.Errorf("FlushInterval = %q, want %q", fc.FlushInterval, "10s")
	}
	if fc.MaxBufferSize != 5000 {
		t.Errorf("MaxBufferSize = %d, want %d", fc.MaxBufferSize, 5000)
	}
	if fc.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want %d", fc.MaxRetries, 5)
	}
	if fc.MaxBackoff != "60s" {
		t.Errorf("MaxBackoff = %q, want %q", fc.MaxBackoff, "60s")
	}
	if fc.Timeout != "15s" {
		t.Errorf("Timeout = %q, want %q", fc.Timeout, "15s")
	}
	if fc.ConnectionTimeout != "8s" {
		t.Errorf("ConnectionTimeout = %q, want %q", fc.ConnectionTimeout, "8s")
	}
	if fc.EnableCompression == nil || !*fc.EnableCompression {
		t.Errorf("EnableCompression = %v, want true", fc.EnableCompression)
	}
}

func TestParseSimpleYAML_CompressionFalse(t *testing.T) {
	content := "enable_compression: false\n"
	var fc FileConfig
	if err := parseSimpleYAML(content, &fc); err != nil {
		t.Fatalf("parseSimpleYAML() error = %v", err)
	}
	if fc.EnableCompression == nil || *fc.EnableCompression {
		t.Errorf("EnableCompression = %v, want false", fc.EnableCompression)
	}
}

func TestParseSimpleYAML_QuotedValues(t *testing.T) {
	content := `
collector_url: "http://quoted:9308"
service_name: 'single-quoted'
`
	var fc FileConfig
	if err := parseSimpleYAML(content, &fc); err != nil {
		t.Fatalf("parseSimpleYAML() error = %v", err)
	}
	if fc.CollectorURL != "http://quoted:9308" {
		t.Errorf("CollectorURL = %q, want %q", fc.CollectorURL, "http://quoted:9308")
	}
	if fc.ServiceName != "single-quoted" {
		t.Errorf("ServiceName = %q, want %q", fc.ServiceName, "single-quoted")
	}
}

func TestParseSimpleYAML_InlineComments(t *testing.T) {
	content := "service_name: my-svc # this is a comment\n"
	var fc FileConfig
	if err := parseSimpleYAML(content, &fc); err != nil {
		t.Fatalf("parseSimpleYAML() error = %v", err)
	}
	if fc.ServiceName != "my-svc" {
		t.Errorf("ServiceName = %q, want %q", fc.ServiceName, "my-svc")
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path/loxa.yaml")
	if err == nil {
		t.Error("LoadFromFile() expected error for nonexistent file, got nil")
	}
}

func TestLoadFromFile_ValidFile(t *testing.T) {
	// Create a temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "loxa.yaml")
	content := `
collector_url: http://file-collector:9308
service_name: file-service
service_version: 2.0.0
environment: staging
batch_size: 50
flush_interval: 3s
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	fc, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if fc.CollectorURL != "http://file-collector:9308" {
		t.Errorf("CollectorURL = %q, want %q", fc.CollectorURL, "http://file-collector:9308")
	}
	if fc.ServiceName != "file-service" {
		t.Errorf("ServiceName = %q, want %q", fc.ServiceName, "file-service")
	}
	if fc.ServiceVersion != "2.0.0" {
		t.Errorf("ServiceVersion = %q, want %q", fc.ServiceVersion, "2.0.0")
	}
	if fc.Environment != "staging" {
		t.Errorf("Environment = %q, want %q", fc.Environment, "staging")
	}
	if fc.BatchSize != 50 {
		t.Errorf("BatchSize = %d, want %d", fc.BatchSize, 50)
	}
	if fc.FlushInterval != "3s" {
		t.Errorf("FlushInterval = %q, want %q", fc.FlushInterval, "3s")
	}
}

func TestLoadDefaultsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loxa-go.defaults.yaml")
	content := "collector_url: http://defaults:9308\nbatch_size: 123\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("LOXA_GO_DEFAULTS", path)

	fc, err := LoadDefaultsFile()
	if err != nil {
		t.Fatalf("LoadDefaultsFile() error = %v", err)
	}
	if fc.CollectorURL != "http://defaults:9308" {
		t.Fatalf("CollectorURL = %q, want defaults value", fc.CollectorURL)
	}
	if fc.BatchSize != 123 {
		t.Fatalf("BatchSize = %d, want 123", fc.BatchSize)
	}
}

func TestMergeFileConfig(t *testing.T) {
	base := Config{
		// Zero values — file config should fill these in
	}
	enabled := true
	fc := FileConfig{
		CollectorURL:      "http://file:9308",
		ServiceName:       "file-svc",
		ServiceVersion:    "0.2.0",
		Environment:       "production",
		TenantID:          "t1",
		BatchSize:         100,
		FlushInterval:     "5s",
		MaxBufferSize:     1000,
		MaxRetries:        3,
		MaxBackoff:        "30s",
		Timeout:           "10s",
		ConnectionTimeout: "5s",
		EnableCompression: &enabled,
	}

	merged := mergeFileConfig(base, fc)

	if merged.CollectorURL != "http://file:9308" {
		t.Errorf("CollectorURL = %q, want %q", merged.CollectorURL, "http://file:9308")
	}
	if merged.Service != "file-svc" {
		t.Errorf("Service = %q, want %q", merged.Service, "file-svc")
	}
	if merged.Version != "0.2.0" {
		t.Errorf("Version = %q, want %q", merged.Version, "0.2.0")
	}
	if merged.Environment != "production" {
		t.Errorf("Environment = %q, want %q", merged.Environment, "production")
	}
	if merged.TenantID != "t1" {
		t.Errorf("TenantID = %q, want %q", merged.TenantID, "t1")
	}
	if merged.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want %d", merged.BatchSize, 100)
	}
	if merged.FlushInterval != 5*time.Second {
		t.Errorf("FlushInterval = %v, want %v", merged.FlushInterval, 5*time.Second)
	}
	if merged.MaxBufferSize != 1000 {
		t.Errorf("MaxBufferSize = %d, want %d", merged.MaxBufferSize, 1000)
	}
	if merged.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want %d", merged.MaxRetries, 3)
	}
	if merged.MaxBackoff != 30*time.Second {
		t.Errorf("MaxBackoff = %v, want %v", merged.MaxBackoff, 30*time.Second)
	}
	if merged.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want %v", merged.Timeout, 10*time.Second)
	}
	if merged.ConnectionTimeout != 5*time.Second {
		t.Errorf("ConnectionTimeout = %v, want %v", merged.ConnectionTimeout, 5*time.Second)
	}
	if !merged.EnableCompression {
		t.Errorf("EnableCompression = false, want true")
	}
}

func TestMergeFileConfig_DoesNotOverrideExisting(t *testing.T) {
	// File config should NOT override already-set values
	base := Config{
		CollectorURL: "http://existing:9308",
		Service:      "existing-svc",
		BatchSize:    200,
	}
	enabled := true
	fc := FileConfig{
		CollectorURL:      "http://file:9308",
		ServiceName:       "file-svc",
		BatchSize:         50,
		EnableCompression: &enabled,
	}

	merged := mergeFileConfig(base, fc)

	if merged.CollectorURL != "http://existing:9308" {
		t.Errorf("CollectorURL = %q, want existing value %q", merged.CollectorURL, "http://existing:9308")
	}
	if merged.Service != "existing-svc" {
		t.Errorf("Service = %q, want existing value %q", merged.Service, "existing-svc")
	}
	if merged.BatchSize != 200 {
		t.Errorf("BatchSize = %d, want existing value %d", merged.BatchSize, 200)
	}
}

func TestNewClient_RequiresCollectorURL(t *testing.T) {
	// Clear env vars that might interfere
	os.Unsetenv("LOXA_COLLECTOR_URL")
	os.Unsetenv("LOXA_SERVICE_NAME")
	dir := t.TempDir()
	defaultsPath := filepath.Join(dir, "loxa-go.defaults.yaml")
	if err := os.WriteFile(defaultsPath, []byte("service_version: unknown\nenvironment: development\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("LOXA_GO_DEFAULTS", defaultsPath)

	_, err := NewClient(Config{
		Service: "test-service",
		// No CollectorURL
	})
	if err == nil {
		t.Error("NewClient() expected error when CollectorURL is missing, got nil")
	}
}

func TestNewClient_RequiresServiceName(t *testing.T) {
	os.Unsetenv("LOXA_SERVICE_NAME")

	_, err := NewClient(Config{
		CollectorURL: "http://localhost:9308",
		// No Service
	})
	if err == nil {
		t.Error("NewClient() expected error when Service is missing, got nil")
	}
}

func TestNewClient_ValidConfig(t *testing.T) {
	os.Unsetenv("LOXA_COLLECTOR_URL")
	os.Unsetenv("LOXA_SERVICE_NAME")

	sink, _ := MemorySink()
	client, err := NewClient(Config{
		CollectorURL: "http://localhost:9308",
		Service:      "test-service",
		Sinks:        []Sink{sink},
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}
	// Verify defaults were applied
	cfg := client.Config()
	if cfg.Version != "unknown" {
		t.Errorf("Version = %q, want %q (default)", cfg.Version, "unknown")
	}
	if cfg.Environment != "development" {
		t.Errorf("Environment = %q, want %q (default)", cfg.Environment, "development")
	}
	if cfg.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want %d (default)", cfg.BatchSize, 100)
	}
	if cfg.FlushInterval != 5*time.Second {
		t.Errorf("FlushInterval = %v, want %v (default)", cfg.FlushInterval, 5*time.Second)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want %d (default)", cfg.MaxRetries, 3)
	}
	if cfg.MaxBackoff != 30*time.Second {
		t.Errorf("MaxBackoff = %v, want %v (default)", cfg.MaxBackoff, 30*time.Second)
	}
	if cfg.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want %v (default)", cfg.Timeout, 10*time.Second)
	}
	if cfg.ConnectionTimeout != 5*time.Second {
		t.Errorf("ConnectionTimeout = %v, want %v (default)", cfg.ConnectionTimeout, 5*time.Second)
	}
}

func TestNewClient_DefaultsToCollectorSinkWhenNoExplicitSink(t *testing.T) {
	os.Unsetenv("LOXA_COLLECTOR_URL")
	os.Unsetenv("LOXA_SERVICE_NAME")

	client, err := NewClient(Config{
		CollectorURL: "http://localhost:9308",
		Service:      "test-service",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	cfg := client.Config()
	if len(cfg.Sinks) != 1 {
		t.Fatalf("expected exactly one sink, got %d", len(cfg.Sinks))
	}
	if got := cfg.Sinks[0].Name(); got != "httpbatch" {
		t.Fatalf("expected default sink 'httpbatch', got %q", got)
	}
}

func TestNewClient_PreservesExplicitSink(t *testing.T) {
	os.Unsetenv("LOXA_COLLECTOR_URL")
	os.Unsetenv("LOXA_SERVICE_NAME")

	memSink, _ := MemorySink()
	client, err := NewClient(Config{
		CollectorURL: "http://localhost:9308",
		Service:      "test-service",
		Sinks:        []Sink{memSink},
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	cfg := client.Config()
	if len(cfg.Sinks) != 1 {
		t.Fatalf("expected exactly one sink, got %d", len(cfg.Sinks))
	}
	if got := cfg.Sinks[0].Name(); got != "memory" {
		t.Fatalf("expected explicit sink 'memory' to be preserved, got %q", got)
	}
}

func TestNewClient_EnvOverridesDefaults(t *testing.T) {
	os.Setenv("LOXA_COLLECTOR_URL", "http://env-collector:9308")
	os.Setenv("LOXA_SERVICE_NAME", "env-service")
	os.Setenv("LOXA_SERVICE_VERSION", "3.0.0")
	os.Setenv("LOXA_ENVIRONMENT", "staging")
	defer func() {
		os.Unsetenv("LOXA_COLLECTOR_URL")
		os.Unsetenv("LOXA_SERVICE_NAME")
		os.Unsetenv("LOXA_SERVICE_VERSION")
		os.Unsetenv("LOXA_ENVIRONMENT")
	}()

	sink, _ := MemorySink()
	client, err := NewClient(Config{
		Sinks: []Sink{sink},
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	cfg := client.Config()
	if cfg.CollectorURL != "http://env-collector:9308" {
		t.Errorf("CollectorURL = %q, want env value", cfg.CollectorURL)
	}
	if cfg.Service != "env-service" {
		t.Errorf("Service = %q, want env value", cfg.Service)
	}
	if cfg.Version != "3.0.0" {
		t.Errorf("Version = %q, want env value", cfg.Version)
	}
	if cfg.Environment != "staging" {
		t.Errorf("Environment = %q, want env value", cfg.Environment)
	}
}

func TestNewClient_CodeOverridesEnv(t *testing.T) {
	os.Setenv("LOXA_COLLECTOR_URL", "http://env-collector:9308")
	os.Setenv("LOXA_SERVICE_NAME", "env-service")
	defer func() {
		os.Unsetenv("LOXA_COLLECTOR_URL")
		os.Unsetenv("LOXA_SERVICE_NAME")
	}()

	sink, _ := MemorySink()
	client, err := NewClient(Config{
		CollectorURL: "http://code-collector:9308",
		Service:      "code-service",
		Sinks:        []Sink{sink},
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	cfg := client.Config()
	if cfg.CollectorURL != "http://code-collector:9308" {
		t.Errorf("CollectorURL = %q, want code value %q", cfg.CollectorURL, "http://code-collector:9308")
	}
	if cfg.Service != "code-service" {
		t.Errorf("Service = %q, want code value %q", cfg.Service, "code-service")
	}
}

func TestNewClient_FileConfigLoaded(t *testing.T) {
	// Create a temp loxa.yaml and change to that directory
	dir := t.TempDir()
	content := `
collector_url: http://file-collector:9308
service_name: file-service
service_version: 4.0.0
environment: production
`
	if err := os.WriteFile(filepath.Join(dir, "loxa.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Clear env vars
	os.Unsetenv("LOXA_COLLECTOR_URL")
	os.Unsetenv("LOXA_SERVICE_NAME")

	// Load file config directly and verify it works
	fc, err := LoadFromFile(filepath.Join(dir, "loxa.yaml"))
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	if fc.CollectorURL != "http://file-collector:9308" {
		t.Errorf("CollectorURL = %q, want %q", fc.CollectorURL, "http://file-collector:9308")
	}
	if fc.ServiceName != "file-service" {
		t.Errorf("ServiceName = %q, want %q", fc.ServiceName, "file-service")
	}
}

func TestCloseMethod(t *testing.T) {
	sink, _ := MemorySink()
	cfg := Test()
	cfg.Sinks = []Sink{sink}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Close should work without error
	if err := logger.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestLoadFromEnv_IntegerParsing(t *testing.T) {
	// Verify that integer env vars are parsed correctly (not via time.ParseDuration)
	os.Setenv("LOXA_BATCH_SIZE", "500")
	os.Setenv("LOXA_MAX_BUFFER_SIZE", "20000")
	os.Setenv("LOXA_MAX_RETRIES", "10")
	defer func() {
		os.Unsetenv("LOXA_BATCH_SIZE")
		os.Unsetenv("LOXA_MAX_BUFFER_SIZE")
		os.Unsetenv("LOXA_MAX_RETRIES")
	}()

	cfg := LoadFromEnv(Config{})

	if cfg.BatchSize != 500 {
		t.Errorf("BatchSize = %d, want 500", cfg.BatchSize)
	}
	if cfg.MaxBufferSize != 20000 {
		t.Errorf("MaxBufferSize = %d, want 20000", cfg.MaxBufferSize)
	}
	if cfg.MaxRetries != 10 {
		t.Errorf("MaxRetries = %d, want 10", cfg.MaxRetries)
	}
}
