package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Server.Port != 9312 {
		t.Fatalf("expected server port 9312, got %d", cfg.Server.Port)
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

func TestDefaultAppliesEnvOverrides(t *testing.T) {
	t.Setenv("CORTEX_SERVER_HOST", "10.0.0.1")
	t.Setenv("CORTEX_SERVER_PORT", "8888")
	t.Setenv("CORTEX_DUCKDB_PATH", "/tmp/env.db")
	t.Setenv("CORTEX_POSTGRES_SSL_MODE", "disable")
	t.Setenv("CORTEX_LOG_LEVEL", "debug")
	t.Setenv("CORTEX_MATCHER_MODE", "rust")

	cfg := Default()

	if cfg.Server.Host != "10.0.0.1" {
		t.Fatalf("expected env host override, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8888 {
		t.Fatalf("expected env port override, got %d", cfg.Server.Port)
	}
	if cfg.Storage.DuckDB.Path != "/tmp/env.db" {
		t.Fatalf("expected env duckdb path override, got %s", cfg.Storage.DuckDB.Path)
	}
	if cfg.Storage.PostgreSQL.SSLMode != "disable" {
		t.Fatalf("expected env postgres SSL mode override, got %s", cfg.Storage.PostgreSQL.SSLMode)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("expected env log level override, got %s", cfg.Logging.Level)
	}
	if cfg.Matcher.Mode != "rust" {
		t.Fatalf("expected env matcher mode override, got %s", cfg.Matcher.Mode)
	}
}

func TestDefaultLoadsUserOverrideFile(t *testing.T) {
	dir := t.TempDir()
	// Write a user override file
	userYAML := []byte(`server:
  host: "192.168.1.100"
  port: 7777
storage:
  duckdb:
    path: /data/custom.db
`)
	path := filepath.Join(dir, "loza-cortex.yaml")
	if err := os.WriteFile(path, userYAML, 0o600); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	// Temporarily change cwd so findUserConfigFile finds our file
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	cfg := Default()

	if cfg.Server.Host != "192.168.1.100" {
		t.Fatalf("expected user host override, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 7777 {
		t.Fatalf("expected user port override, got %d", cfg.Server.Port)
	}
	if cfg.Storage.DuckDB.Path != "/data/custom.db" {
		t.Fatalf("expected user duckdb path override, got %s", cfg.Storage.DuckDB.Path)
	}
}

func TestDefaultEnvOverridesUserFile(t *testing.T) {
	dir := t.TempDir()
	// User file sets port 7777
	userYAML := []byte(`server:
  port: 7777
`)
	path := filepath.Join(dir, "loza.yaml")
	if err := os.WriteFile(path, userYAML, 0o600); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	// Env should win over user file
	t.Setenv("CORTEX_SERVER_PORT", "5555")

	cfg := Default()

	if cfg.Server.Port != 5555 {
		t.Fatalf("expected env to override user file port, got %d", cfg.Server.Port)
	}
}

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := []byte(`
server:
  host: 127.0.0.1
  port: 9312
storage:
  backend: duckdb
  duckdb:
    path: ./cortex.db
logging:
  level: info
  format: json
authentication:
  enabled: false
collector:
  url: http://localhost:9308
  source_of_truth: true
  query_table: events
  raw_column: raw
  timestamp_column: timestamp
`)
	if err := os.WriteFile(path, yaml, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CORTEX_SERVER_HOST", "0.0.0.0")
	t.Setenv("CORTEX_SERVER_PORT", "9312")
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

	if cfg.Server.Host != "0.0.0.0" || cfg.Server.Port != 9312 {
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
				Server: ServerConfig{Port: 9312},
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
				Server:  ServerConfig{Port: 9312},
				Storage: StorageConfig{Backend: "duckdb"},
				Matcher: MatcherConfig{Mode: "go"},
				Logging: LoggingConfig{Level: "info", Format: "json"},
			},
			wantErr: true,
		},
		{
			name: "invalid matcher mode",
			cfg: Config{
				Server: ServerConfig{Port: 9312},
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
				Server: ServerConfig{Port: 9312},
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
				Server: ServerConfig{Port: 9312},
				Storage: StorageConfig{
					Backend: "duckdb",
					DuckDB:  DuckDBConfig{Path: "./cortex.db"},
				},
				Matcher: MatcherConfig{Mode: "go"},
				Logging: LoggingConfig{Level: "info", Format: "json"},
				Collector: CollectorConfig{
					SourceOfTruth:   true,
					URL:             "http://localhost:9308",
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
				Server: ServerConfig{Port: 9312},
				Storage: StorageConfig{
					Backend: "duckdb",
					DuckDB:  DuckDBConfig{Path: "./cortex.db"},
				},
				Matcher: MatcherConfig{Mode: "go"},
				Logging: LoggingConfig{Level: "info", Format: "json"},
				Collector: CollectorConfig{
					SourceOfTruth:   true,
					URL:             "http://localhost:9308",
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

func TestLoadDefaultPipeline(t *testing.T) {
	defaults := writeBundledDefaults(t)
	useBundledDefaults(t, defaults)

	dir := t.TempDir()
	writeConfigFile(t, filepath.Join(dir, "loza-cortex.yaml"), "server:\n  port: 7777\n")
	changeWorkingDirectory(t, dir)
	t.Setenv("CORTEX_SERVER_PORT", "8888")

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	if cfg.Server.Port != 8888 {
		t.Fatalf("environment must override cwd config, got port %d", cfg.Server.Port)
	}
	if cfg.Storage.DuckDB.Path != "./bundled.db" {
		t.Fatalf("expected bundled storage configuration, got %q", cfg.Storage.DuckDB.Path)
	}
}

func TestLoadDefaultSkipsProjectManifest(t *testing.T) {
	defaults := writeBundledDefaults(t)
	useBundledDefaults(t, defaults)

	dir := t.TempDir()
	writeConfigFile(t, filepath.Join(dir, "loza.yaml"), "name: loza\nkind: release\nmodule: github.com/astraive/loza\n")
	changeWorkingDirectory(t, dir)

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	if cfg.Server.Port != 9312 {
		t.Fatalf("release manifest must not override bundled config, got port %d", cfg.Server.Port)
	}
}

func TestLoadDefaultRejectsInvalidRuntimeOverride(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "malformed YAML", content: "server: [", want: "failed to parse runtime override"},
		{name: "invalid configuration", content: "server:\n  port: 0\n", want: "invalid configuration"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defaults := writeBundledDefaults(t)
			useBundledDefaults(t, defaults)
			dir := t.TempDir()
			writeConfigFile(t, filepath.Join(dir, "loza.yaml"), tc.content)
			changeWorkingDirectory(t, dir)

			_, err := LoadDefault()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadDefault error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestLoadOverlaysBundledDefaultsThenEnvironment(t *testing.T) {
	defaults := writeBundledDefaults(t)
	useBundledDefaults(t, defaults)

	override := filepath.Join(t.TempDir(), "runtime.yaml")
	writeConfigFile(t, override, "server:\n  port: 7777\n")
	t.Setenv("CORTEX_SERVER_PORT", "8888")

	cfg, err := Load(override)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Port != 8888 {
		t.Fatalf("environment must override named config, got port %d", cfg.Server.Port)
	}
	if cfg.Storage.DuckDB.Path != "./bundled.db" {
		t.Fatalf("named config must retain bundled fields, got %q", cfg.Storage.DuckDB.Path)
	}
}

func TestValidateEnabledAuthenticationRequiresValidKey(t *testing.T) {
	base := func() Config {
		return Config{
			Server:  ServerConfig{Port: 9312},
			Storage: StorageConfig{Backend: "duckdb", DuckDB: DuckDBConfig{Path: "./cortex.db"}},
			Matcher: MatcherConfig{Mode: "go"},
			Logging: LoggingConfig{Level: "info", Format: "json"},
			Authentication: AuthenticationConfig{
				Enabled: true,
			},
		}
	}

	tests := []struct {
		name string
		key  []APIKey
		ok   bool
	}{
		{name: "no keys"},
		{name: "empty name", key: []APIKey{{Key: "secret", Role: "reader"}}},
		{name: "empty key", key: []APIKey{{Name: "reader", Role: "reader"}}},
		{name: "invalid role", key: []APIKey{{Name: "service", Key: "secret", Role: "operator"}}},
		{name: "valid reader", key: []APIKey{{Name: "reader", Key: "secret", Role: "reader"}}, ok: true},
		{name: "valid writer", key: []APIKey{{Name: "writer", Key: "secret", Role: "writer"}}, ok: true},
		{name: "valid admin", key: []APIKey{{Name: "admin", Key: "secret", Role: "admin"}}, ok: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			cfg.Authentication.APIKeys = tc.key
			err := cfg.Validate()
			if tc.ok && err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func writeBundledDefaults(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), bundledDefaultFilename)
	writeConfigFile(t, path, `server:
  port: 9312
storage:
  backend: duckdb
  duckdb:
    path: ./bundled.db
matcher:
  mode: go
logging:
  level: info
  format: json
authentication:
  enabled: false
`)
	return path
}

func useBundledDefaults(t *testing.T, path string) {
	t.Helper()
	previous := bundledDefaultCandidates
	bundledDefaultCandidates = func() []string { return []string{path} }
	t.Cleanup(func() { bundledDefaultCandidates = previous })
}

func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config %s: %v", path, err)
	}
}

func changeWorkingDirectory(t *testing.T, dir string) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}
