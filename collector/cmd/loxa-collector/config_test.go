package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	collectorconfig "github.com/astraive/loxa/collector/internal/config"
)

func TestLoadCollectorConfigFromArgsPrecedence(t *testing.T) {
	t.Setenv("COLLECTOR_ADDR", ":9001")
	t.Setenv("DUCKDB_BATCH_SIZE", "20")
	t.Setenv("COLLECTOR_API_KEY", "env-secret")

	path := filepath.Join(t.TempDir(), "collector.yaml")
	raw := `
collector:
  addr: ":9000"
duckdb:
  path: "from-file.db"
  batch_size: 10
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadCollectorConfigFromArgs([]string{
		"-c", path,
		"--addr", ":9002",
		"--duckdb-path", "from-flag.db",
		"--batch-size", "30",
	})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.addr != ":9002" {
		t.Fatalf("addr precedence mismatch: %q", cfg.addr)
	}
	if cfg.duckDBPath != "from-flag.db" {
		t.Fatalf("duckdb path precedence mismatch: %q", cfg.duckDBPath)
	}
	if cfg.duckDBBatchSize != 30 {
		t.Fatalf("batch size precedence mismatch: %d", cfg.duckDBBatchSize)
	}
	if cfg.apiKey != "env-secret" || !cfg.authEnabled {
		t.Fatalf("expected env API key to enable auth")
	}
}

func TestLoadCollectorConfigFromArgsFailFastInvalidEnv(t *testing.T) {
	t.Setenv("COLLECTOR_MAX_EVENTS", "abc")
	if _, err := loadCollectorConfigFromArgs(nil); err == nil || !strings.Contains(err.Error(), "COLLECTOR_MAX_EVENTS") {
		t.Fatalf("expected invalid env parse error, got: %v", err)
	}
}

func TestLoadCollectorConfigFromArgsFailFastInvalidFlag(t *testing.T) {
	if _, err := loadCollectorConfigFromArgs([]string{"--max-body-bytes", "0"}); err == nil || !strings.Contains(err.Error(), "collector.max_body_bytes must be > 0") {
		t.Fatalf("expected max body validation error, got: %v", err)
	}
}

func TestLoadCollectorConfigFromArgsExplicitFlagOverridesEnv(t *testing.T) {
	t.Setenv("COLLECTOR_API_KEY", "env-secret")
	if _, err := loadCollectorConfigFromArgs([]string{"--api-key="}); err == nil || !strings.Contains(err.Error(), "auth enabled but no API key resolved") {
		t.Fatalf("expected explicit empty flag to override env and fail validation, got: %v", err)
	}
}

func TestMergeConfigFileRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.yaml")
	raw := `
collector:
  addr: ":9191"
  extra_field: true
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg := defaultFileConfig()
	if err := mergeConfigFile(&cfg, path); err == nil {
		t.Fatalf("expected unknown field error")
	}
}

func TestValidateFileConfigRejectsInvalidValues(t *testing.T) {
	cfg := defaultFileConfig()
	cfg.DuckDB.Table = "events;drop"
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected invalid table name error")
	}

	cfg = defaultFileConfig()
	cfg.Collector.MaxBodyBytes = 0
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected max body validation error")
	}

	cfg = defaultFileConfig()
	cfg.Auth.Enabled = true
	cfg.Auth.Value = ""
	cfg.Auth.ValueEnv = ""
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected auth validation error")
	}

	cfg = defaultFileConfig()
	cfg.Reliability.Mode = "spool"
	cfg.Reliability.MaxSpoolBytes = 0
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected spool max bytes validation error")
	}

	cfg = defaultFileConfig()
	cfg.Retry.Enabled = true
	cfg.Retry.MaxAttempts = 0
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected retry validation error")
	}

	cfg = defaultFileConfig()
	cfg.DeadLetter.Enabled = true
	cfg.DeadLetter.Path = ""
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected dlq path validation error")
	}

	cfg = defaultFileConfig()
	cfg.DuckDB.WriterQueueSize = -1
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected writer queue size validation error")
	}

	cfg = defaultFileConfig()
	cfg.DuckDB.Export.Enabled = true
	cfg.DuckDB.Export.Format = "json"
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected export format validation error")
	}

	cfg = defaultFileConfig()
	cfg.Components.Processors = append(cfg.Components.Processors, "whoops")
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "unknown component") {
		t.Fatalf("expected unknown component validation error, got: %v", err)
	}
}

func TestConfigCommandPrint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.yaml")
	raw := `
collector:
  addr: ":9191"
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	out1 := runConfigCommandCaptureOutput(t, []string{"print", "-c", path})
	if !strings.Contains(out1, `addr: :9191`) {
		t.Fatalf("unexpected config output: %s", out1)
	}
	out2 := runConfigCommandCaptureOutput(t, []string{"print", "-c", path})
	if out1 != out2 {
		t.Fatalf("expected stable config print output")
	}
}

func TestConfigCommandValidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.yaml")
	raw := `
collector:
  shutdown_timeout: 3s
duckdb:
  flush_interval: 250ms
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := configCommand([]string{"validate", "-c", path}); err != nil {
		t.Fatalf("config validate: %v", err)
	}
}

func TestConfigCommandValidateFailFast(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.yaml")
	raw := `
collector:
  max_body_bytes: 0
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	err := configCommand([]string{"validate", "-c", path})
	if err == nil || !strings.Contains(err.Error(), "invalid config: collector.max_body_bytes must be > 0") {
		t.Fatalf("expected fail-fast validation error, got: %v", err)
	}
}

func TestLoadCollectorConfigFromArgsFlushInterval(t *testing.T) {
	cfg, err := loadCollectorConfigFromArgs([]string{"--flush-interval", "500ms"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.duckDBFlushInterval != 500*time.Millisecond {
		t.Fatalf("unexpected flush interval: %s", cfg.duckDBFlushInterval)
	}
}

func TestLoadCollectorConfigWithWriterLoopAndExport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.yaml")
	raw := `
duckdb:
  writer_loop: true
  writer_queue_size: 256
  checkpoint_interval: 1m
  export:
    enabled: true
    format: parquet
    interval: 2m
    path: ./exports
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := loadCollectorConfigFromArgs([]string{"-c", path})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.duckDBWriterLoop || cfg.duckDBWriterQueueSize != 256 {
		t.Fatalf("writer loop config not mapped")
	}
	if cfg.duckDBCheckpointIntvl != time.Minute {
		t.Fatalf("checkpoint interval mismatch: %s", cfg.duckDBCheckpointIntvl)
	}
	if !cfg.duckDBExportEnabled || cfg.duckDBExportInterval != 2*time.Minute || cfg.duckDBExportPath != "./exports" {
		t.Fatalf("export config mismatch")
	}
}

func TestLoadCollectorConfigDuckDBNewOptions(t *testing.T) {
	// ensure CLI flags and env vars map to new duckdb options
	t.Setenv("DUCKDB_USE_APPENDER", "true")
	t.Setenv("DUCKDB_WRITE_TIMEOUT", "5s")
	t.Setenv("DUCKDB_RETRY_ATTEMPTS", "3")
	t.Setenv("DUCKDB_RETRY_BACKOFF", "100ms")

	cfg, err := loadCollectorConfigFromArgs([]string{"--use-appender", "--write-timeout", "10s", "--retry-attempts", "5", "--retry-backoff", "200ms"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	// CLI flags should override env
	if !cfg.duckDBUseAppender {
		t.Fatalf("expected use_appender from flag to be true")
	}
	if cfg.duckDBWriteTimeout != 10*time.Second {
		t.Fatalf("unexpected write timeout: %s", cfg.duckDBWriteTimeout)
	}
	if cfg.duckDBRetryAttempts != 5 {
		t.Fatalf("unexpected retry attempts: %d", cfg.duckDBRetryAttempts)
	}
	if cfg.duckDBRetryBackoff != 200*time.Millisecond {
		t.Fatalf("unexpected retry backoff: %s", cfg.duckDBRetryBackoff)
	}
}

func runConfigCommandCaptureOutput(t *testing.T, args []string) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	if err := configCommand(args); err != nil {
		t.Fatalf("config command: %v", err)
	}
	_ = w.Close()
	return <-done
}

func TestValidateFileConfigFanoutDeliveryPolicy(t *testing.T) {
	cfg := defaultFileConfig()
	cfg.Fanout.Delivery.Policy = "invalid"
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "fanout.delivery.policy") {
		t.Fatalf("expected fanout policy validation error, got: %v", err)
	}
}

func TestValidateFileConfigFanoutFallbackRequiresOutput(t *testing.T) {
	cfg := defaultFileConfig()
	cfg.Fanout.Delivery.Fallback.Enabled = true
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "requires exactly one enabled fallback output") {
		t.Fatalf("expected fallback output validation error, got: %v", err)
	}
}

func TestValidateFileConfigFanoutOutputValidation(t *testing.T) {
	cfg := defaultFileConfig()
	cfg.Fanout.Outputs = []collectorconfig.FanoutOutputConfig{
		{
			Name:    "secondary-copy",
			Role:    "secondary",
			Type:    "duckdb",
			Enabled: true,
			DuckDB: collectorconfig.FanoutDuckDBConfig{
				Path:  "copy.db",
				Table: "events_copy",
			},
		},
		{
			Name:    "fallback-copy",
			Role:    "fallback",
			Type:    "duckdb",
			Enabled: true,
			DuckDB: collectorconfig.FanoutDuckDBConfig{
				Path: "fallback.db",
			},
		},
	}
	cfg.Fanout.Delivery.Fallback.Enabled = true
	if err := validateFileConfig(cfg); err != nil {
		t.Fatalf("expected valid fanout config, got: %v", err)
	}
}

func TestValidateFileConfigPostgresFanoutOutputValidation(t *testing.T) {
	cfg := defaultFileConfig()
	cfg.Fanout.Outputs = []collectorconfig.FanoutOutputConfig{
		{
			Name:    "pg-copy",
			Role:    "secondary",
			Type:    "postgres",
			Enabled: true,
			Postgres: collectorconfig.FanoutPostgresConfig{
				DSN:       "postgres://user:pass@localhost:5432/loxa?sslmode=disable",
				Table:     "events_copy",
				RawColumn: "raw_payload",
			},
		},
	}
	if err := validateFileConfig(cfg); err != nil {
		t.Fatalf("expected valid postgres fanout config, got: %v", err)
	}
}

func TestValidateFileConfigQueueModeRequiresKafka(t *testing.T) {
	cfg := defaultFileConfig()
	cfg.Reliability.Mode = "queue"
	cfg.Kafka.Brokers = nil
	cfg.Kafka.Topic = ""
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "kafka.brokers") {
		t.Fatalf("expected kafka broker validation error, got: %v", err)
	}
}

func TestValidateFileConfigRejectsInvalidKafkaAcks(t *testing.T) {
	cfg := defaultFileConfig()
	cfg.Kafka.Acks = "leader"
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "kafka.acks") {
		t.Fatalf("expected kafka.acks validation error, got: %v", err)
	}
}

func TestValidateFileConfigRedisDedupeRequiresRedisAddr(t *testing.T) {
	cfg := defaultFileConfig()
	cfg.Dedupe.Enabled = true
	cfg.Dedupe.Backend = "redis"
	cfg.Dedupe.RedisAddr = ""
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "dedupe.redis_addr") {
		t.Fatalf("expected dedupe.redis_addr validation error, got: %v", err)
	}
}

func TestLoadCollectorConfigQueueModeFromEnv(t *testing.T) {
	t.Setenv("COLLECTOR_RELIABILITY_MODE", "queue")
	t.Setenv("COLLECTOR_KAFKA_BROKERS", "k1:9092, k2:9092")
	t.Setenv("COLLECTOR_KAFKA_TOPIC", "loxa-events")
	t.Setenv("LOXA_WORKER_CONSUMER_GROUP", "loxa-worker-test")
	t.Setenv("LOXA_WORKER_POLL_TIMEOUT", "3s")

	cfg, err := loadCollectorConfigFromArgs(nil)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.reliabilityMode != "queue" {
		t.Fatalf("expected queue mode, got %q", cfg.reliabilityMode)
	}
	if len(cfg.kafkaBrokers) != 2 || cfg.kafkaBrokers[0] != "k1:9092" || cfg.kafkaBrokers[1] != "k2:9092" {
		t.Fatalf("unexpected kafka brokers: %#v", cfg.kafkaBrokers)
	}
	if cfg.kafkaTopic != "loxa-events" {
		t.Fatalf("unexpected kafka topic: %q", cfg.kafkaTopic)
	}
	if cfg.workerConsumerGroup != "loxa-worker-test" {
		t.Fatalf("unexpected worker group: %q", cfg.workerConsumerGroup)
	}
	if cfg.workerPollTimeout != 3*time.Second {
		t.Fatalf("unexpected worker poll timeout: %s", cfg.workerPollTimeout)
	}
}
