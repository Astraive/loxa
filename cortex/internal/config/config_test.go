package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Server.Port != 8080 {
		t.Fatalf("expected server port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Storage.Backend != "duckdb" {
		t.Fatalf("expected duckdb backend, got %s", cfg.Storage.Backend)
	}
	if cfg.Logging.Level != "info" || cfg.Logging.Format != "json" {
		t.Fatalf("unexpected logging defaults: %+v", cfg.Logging)
	}
	if cfg.Matcher.Mode != "go" {
		t.Fatalf("expected go matcher by default, got %s", cfg.Matcher.Mode)
	}
	if cfg.Matcher.CacheSize != 1024 || cfg.Matcher.CacheTTL <= 0 {
		t.Fatalf("unexpected matcher defaults: %+v", cfg.Matcher)
	}
	if cfg.Collector.TailTransport != "http" {
		t.Fatalf("unexpected collector tail transport default: %+v", cfg.Collector)
	}
	if !cfg.Collector.TailEnabled || cfg.Collector.TailBufferSize != 2048 || cfg.Collector.TailBatchSize != 256 {
		t.Fatalf("unexpected collector tail defaults: %+v", cfg.Collector)
	}
	if cfg.Collector.TailFlushInterval != 500*time.Millisecond || cfg.Collector.TailReconnectBackoff != 2*time.Second {
		t.Fatalf("unexpected collector tail timing defaults: %+v", cfg.Collector)
	}
}

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := []byte(`
server:
  host: 127.0.0.1
  port: 8080
storage:
  backend: duckdb
  duckdb:
    path: ./cortex.db
logging:
  level: info
  format: json
collector:
  url: http://localhost:8081
  source_of_truth: true
  query_table: events
  raw_column: raw
  timestamp_column: timestamp
`)
	if err := os.WriteFile(path, yaml, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CORTEX_SERVER_HOST", "0.0.0.0")
	t.Setenv("CORTEX_SERVER_PORT", "9091")
	t.Setenv("CORTEX_DUCKDB_PATH", "./test.db")
	t.Setenv("CORTEX_LOG_LEVEL", "debug")
	t.Setenv("CORTEX_MATCHER_MODE", "rust")
	t.Setenv("CORTEX_MATCHER_PARALLELISM", "8")
	t.Setenv("CORTEX_MATCHER_CACHE_SIZE", "2048")
	t.Setenv("CORTEX_MATCHER_CACHE_TTL", "45s")
	t.Setenv("CORTEX_COLLECTOR_TAIL_ENABLED", "true")
	t.Setenv("CORTEX_COLLECTOR_TAIL_TRANSPORT", "websocket")
	t.Setenv("CORTEX_COLLECTOR_TAIL_BUFFER_SIZE", "4096")
	t.Setenv("CORTEX_COLLECTOR_TAIL_BATCH_SIZE", "128")
	t.Setenv("CORTEX_COLLECTOR_TAIL_FLUSH_INTERVAL", "750ms")
	t.Setenv("CORTEX_COLLECTOR_TAIL_RECONNECT_BACKOFF", "5s")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" || cfg.Server.Port != 9091 {
		t.Fatalf("env overrides not applied: %+v", cfg.Server)
	}
	if cfg.Storage.DuckDB.Path != "./test.db" {
		t.Fatalf("expected duckdb path override, got %s", cfg.Storage.DuckDB.Path)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("expected log level override, got %s", cfg.Logging.Level)
	}
	if cfg.Matcher.Mode != "rust" {
		t.Fatalf("expected matcher mode override, got %s", cfg.Matcher.Mode)
	}
	if cfg.Matcher.Parallelism != 8 || cfg.Matcher.CacheSize != 2048 || cfg.Matcher.CacheTTL != 45*time.Second {
		t.Fatalf("expected matcher overrides, got %+v", cfg.Matcher)
	}
	if cfg.Collector.TailTransport != "websocket" {
		t.Fatalf("expected collector tail transport override, got %+v", cfg.Collector)
	}
	if !cfg.Collector.TailEnabled || cfg.Collector.TailBufferSize != 4096 || cfg.Collector.TailBatchSize != 128 {
		t.Fatalf("expected collector tail overrides, got %+v", cfg.Collector)
	}
	if cfg.Collector.TailFlushInterval != 750*time.Millisecond || cfg.Collector.TailReconnectBackoff != 5*time.Second {
		t.Fatalf("expected collector tail timing overrides, got %+v", cfg.Collector)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid duckdb",
			cfg: Config{
				Server: ServerConfig{Port: 8080},
				Storage: StorageConfig{
					Backend: "duckdb",
					DuckDB:  DuckDBConfig{Path: "./cortex.db"},
				},
				Matcher: MatcherConfig{Mode: "go", CacheSize: 128},
				Logging: LoggingConfig{Level: "info", Format: "json"},
			},
		},
		{
			name: "invalid port",
			cfg: Config{
				Server: ServerConfig{Port: 0},
				Storage: StorageConfig{
					Backend: "duckdb",
					DuckDB:  DuckDBConfig{Path: "./cortex.db"},
				},
				Matcher: MatcherConfig{Mode: "go"},
				Logging: LoggingConfig{Level: "info", Format: "json"},
			},
			wantErr: true,
		},
		{
			name: "missing duckdb path",
			cfg: Config{
				Server:  ServerConfig{Port: 8080},
				Storage: StorageConfig{Backend: "duckdb"},
				Matcher: MatcherConfig{Mode: "go"},
				Logging: LoggingConfig{Level: "info", Format: "json"},
			},
			wantErr: true,
		},
		{
			name: "invalid matcher mode",
			cfg: Config{
				Server: ServerConfig{Port: 8080},
				Storage: StorageConfig{
					Backend: "duckdb",
					DuckDB:  DuckDBConfig{Path: "./cortex.db"},
				},
				Matcher: MatcherConfig{Mode: "wasm"},
				Logging: LoggingConfig{Level: "info", Format: "json"},
			},
			wantErr: true,
		},
		{
			name: "negative matcher cache size",
			cfg: Config{
				Server: ServerConfig{Port: 8080},
				Storage: StorageConfig{
					Backend: "duckdb",
					DuckDB:  DuckDBConfig{Path: "./cortex.db"},
				},
				Matcher: MatcherConfig{Mode: "go", CacheSize: -1},
				Logging: LoggingConfig{Level: "info", Format: "json"},
			},
			wantErr: true,
		},
		{
			name: "negative collector tail buffer",
			cfg: Config{
				Server: ServerConfig{Port: 8080},
				Storage: StorageConfig{
					Backend: "duckdb",
					DuckDB:  DuckDBConfig{Path: "./cortex.db"},
				},
				Matcher: MatcherConfig{Mode: "go"},
				Logging: LoggingConfig{Level: "info", Format: "json"},
				Collector: CollectorConfig{
					SourceOfTruth:   true,
					URL:             "http://localhost:8081",
					QueryTable:      "events",
					RawColumn:       "raw",
					TimestampColumn: "timestamp",
					TailBufferSize:  -1,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid collector tail transport",
			cfg: Config{
				Server: ServerConfig{Port: 8080},
				Storage: StorageConfig{
					Backend: "duckdb",
					DuckDB:  DuckDBConfig{Path: "./cortex.db"},
				},
				Matcher: MatcherConfig{Mode: "go"},
				Logging: LoggingConfig{Level: "info", Format: "json"},
				Collector: CollectorConfig{
					SourceOfTruth:   true,
					URL:             "http://localhost:8081",
					QueryTable:      "events",
					RawColumn:       "raw",
					TimestampColumn: "timestamp",
					TailTransport:   "grpc",
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
