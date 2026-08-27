package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/astraive/loza/collector/internal/auth"
	collectorconfig "github.com/astraive/loza/collector/internal/config"
)

func TestMain(m *testing.M) {
	for key, value := range map[string]string{
		"COLLECTOR_AUTH_SERVER_SECRET": "test-auth-server-secret",
		"COLLECTOR_INGEST_KEY_SECRET":  "test-ingest-key-secret",
		"COLLECTOR_ADMIN_KEY_SECRET":   "test-admin-key-secret",
		"LOZA_STORAGE_ENCRYPTION_KEY":  "test-storage-encryption-key",
	} {
		if err := os.Setenv(key, value); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}

func validFileConfig() fileConfig {
	cfg := defaultFileConfig()
	cfg.Storage.EncryptionKey = "test-storage-encryption-key"
	return cfg
}

func TestValidateNamedDatabaseConnections(t *testing.T) {
	cfg := validFileConfig()
	cfg.Database.Connections = []collectorconfig.DatabaseConnectionConfig{
		{Name: "local", Type: "duckdb", Enabled: true, Path: filepath.Join(t.TempDir(), "events.db")},
	}
	cfg.Storage.Connection = "local"
	if err := validateFileConfig(cfg); err != nil {
		t.Fatalf("valid named DuckDB connection rejected: %v", err)
	}
}

func TestValidateNamedPostgresConnectionRequiresSecrets(t *testing.T) {
	cfg := validFileConfig()
	cfg.Database.Connections = []collectorconfig.DatabaseConnectionConfig{
		{Name: "pg", Type: "postgres", Enabled: true, Host: "localhost", Database: "loza", UsernameEnv: "MISSING_USER", PasswordEnv: "MISSING_PASSWORD"},
	}
	cfg.Storage.Primary = "postgres"
	cfg.Storage.Connection = "pg"
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "username_env") {
		t.Fatalf("expected missing postgres secret error, got %v", err)
	}
}

func TestLoadCollectorConfigFromArgsPrecedence(t *testing.T) {
	t.Setenv("COLLECTOR_ADDR", ":9001")
	t.Setenv("DUCKDB_BATCH_SIZE", "20")
	t.Setenv("COLLECTOR_API_KEY", "lz_sec_live_klegacy_envsecret")

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
	if cfg.apiKey != "lz_sec_live_klegacy_envsecret" || !cfg.authEnabled {
		t.Fatalf("expected env API key to enable auth")
	}
}

func TestValidateFileConfigRejectsConcurrentLQLStdioRequests(t *testing.T) {
	cfg := validFileConfig()
	cfg.LQL.MaxConcurrentRequests = 2
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "must be 1") {
		t.Fatalf("expected sequential LQL stdio validation error, got %v", err)
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

func TestLoadCollectorConfigFromArgsExplicitEmptyFlagOverridesEnv(t *testing.T) {
	t.Setenv("COLLECTOR_API_KEY", "lz_sec_live_klegacy_envsecret")
	cfg, err := loadCollectorConfigFromArgs([]string{"--api-key="})
	if err != nil {
		t.Fatalf("explicit empty legacy key should leave configured auth keys usable: %v", err)
	}
	if cfg.apiKey != "" {
		t.Fatalf("expected explicit empty API key to override env, got %q", cfg.apiKey)
	}
}

func TestLoadCollectorConfigFromArgsRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.yaml")
	raw := `
collector:
  addr: ":9191"
  extra_field: true
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := loadCollectorConfigFromArgs([]string{"-c", path}); err == nil {
		t.Fatalf("expected active loader to reject an unknown field")
	}
}

func TestLoadCollectorConfigFromArgsSecureDefaultsRequireSecrets(t *testing.T) {
	t.Setenv("COLLECTOR_AUTH_SERVER_SECRET", "")
	t.Setenv("COLLECTOR_INGEST_KEY_SECRET", "")
	t.Setenv("COLLECTOR_ADMIN_KEY_SECRET", "")
	t.Setenv("LOZA_STORAGE_ENCRYPTION_KEY", "")
	if _, err := loadCollectorConfigFromArgs(nil); err == nil {
		t.Fatal("expected secure defaults to reject missing credentials and encryption key")
	}
}

func TestLoadCollectorConfigFromArgsResolvesConfiguredKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.yaml")
	raw := `
auth:
  enabled: true
  server_secret: ${COLLECTOR_AUTH_SERVER_SECRET}
  cache_ttl: 1m
  negative_cache_ttl: 10s
  keys:
    - name: public-ingest
      key_id: kpublic
      secret_env: COLLECTOR_INGEST_KEY_SECRET
      kind: pub
      roles: [collector_ingest_public]
      allowed_origins: [https://console.example.test]
    - name: administrator
      key_id: kadmin
      secret_env: COLLECTOR_ADMIN_KEY_SECRET
      kind: sec
      roles: [project_admin]
storage:
  encryption_key_env: LOZA_STORAGE_ENCRYPTION_KEY
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := loadCollectorConfigFromArgs([]string{"-c", path})
	if err != nil {
		t.Fatalf("load secure config: %v", err)
	}
	if cfg.authServerSecret != "test-auth-server-secret" || len(cfg.authKeys) != 2 {
		t.Fatalf("resolved auth configuration was not carried to runtime: %+v", cfg)
	}
	if cfg.authKeys[0].secret != "test-ingest-key-secret" || cfg.authKeys[1].kind != auth.KeyKindSecret {
		t.Fatalf("configured key secrets or kind were not resolved")
	}
	if cfg.authDefaultCollector != "default" {
		t.Fatalf("legacy root routes must retain the implicit default collector, got %q", cfg.authDefaultCollector)
	}
}

func TestLoadCollectorConfigFromArgsResolvesScopedConfiguredKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.yaml")
	raw := `
auth:
  enabled: true
  server_secret: ${COLLECTOR_AUTH_SERVER_SECRET}
  cache_ttl: 1m
  negative_cache_ttl: 10s
  collectors:
    - slug: checkout
  keys:
    - name: checkout-writer
      key_id: kcheckoutwriter
      secret_env: COLLECTOR_INGEST_KEY_SECRET
      kind: sec
      collector: checkout
      permissions: [events:write]
      allowed_envs: [prod]
storage:
  encryption_key_env: LOZA_STORAGE_ENCRYPTION_KEY
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := loadCollectorConfigFromArgs([]string{"-c", path})
	if err != nil {
		t.Fatalf("load scoped key configuration: %v", err)
	}
	if len(cfg.authKeys) != 1 || cfg.authKeys[0].collector != "checkout" {
		t.Fatalf("scoped key was not carried to runtime: %+v", cfg.authKeys)
	}
	if got := cfg.authKeys[0].permissions; len(got) != 1 || got[0] != auth.PermEventsWrite {
		t.Fatalf("scoped key permissions = %v, want events:write", got)
	}
	if len(cfg.authCollectors) != 1 || cfg.authCollectors[0] != "checkout" {
		t.Fatalf("configured collector was not carried to runtime: %v", cfg.authCollectors)
	}
}

func TestResolveCollectorAuthGrantsBuildsPrivateAndPublicCredentials(t *testing.T) {
	publicID := "lz_pub_0123456789abcdefghijklmnopqrstuv"
	t.Setenv("COLLECTOR_PRIVATE_GRANT_PASSWORD", "private-grant-password")
	t.Setenv("COLLECTOR_PUBLIC_ACCESS_ID", publicID)
	cfg := validFileConfig()
	cfg.Auth.Collectors = []collectorconfig.AuthCollectorConfig{{Slug: "browser-events"}}
	cfg.Auth.DefaultCollector = "browser-events"
	cfg.Auth.Grants = []collectorconfig.AuthGrantConfig{
		{
			Name:        "service",
			Collector:   "browser-events",
			Username:    "service-writer",
			PasswordEnv: "COLLECTOR_PRIVATE_GRANT_PASSWORD",
			Permissions: []string{"events:write"},
			AllowedEnvs: []string{"prod"},
		},
		{
			Name:           "browser",
			Collector:      "browser-events",
			PublicIDEnv:    "COLLECTOR_PUBLIC_ACCESS_ID",
			Permissions:    []string{"events:write"},
			AllowedEnvs:    []string{"prod"},
			AllowedOrigins: []string{"https://console.example.test"},
		},
	}

	collectors, grants, err := resolveCollectorAuthGrants(cfg)
	if err != nil {
		t.Fatalf("resolve grants: %v", err)
	}
	if len(collectors) != 1 || collectors[0] != "browser-events" || len(grants) != 2 {
		t.Fatalf("unexpected resolved grants: collectors=%v grants=%+v", collectors, grants)
	}
	if grants[0].kind != auth.KeyKindSecret || grants[0].secret != "private-grant-password" || grants[0].keyID != "service-writer" {
		t.Fatalf("private grant was not resolved: %+v", grants[0])
	}
	if grants[1].kind != auth.KeyKindPublic || grants[1].secret != "" || grants[1].keyID != publicID {
		t.Fatalf("public grant was not constructed as a passwordless capability: %+v", grants[1])
	}
}

func TestResolveAuthConfigBuildsCollectorScopedKey(t *testing.T) {
	cfg := validFileConfig()
	cfg.Auth.Collectors = []collectorconfig.AuthCollectorConfig{{Slug: "orders"}}
	cfg.Auth.Keys = []collectorconfig.AuthKeyConfig{{
		Name:        "orders-operator",
		KeyID:       "korders",
		SecretEnv:   "COLLECTOR_INGEST_KEY_SECRET",
		Kind:        "sec",
		Collector:   "orders",
		Permissions: []string{"events:read", "events:write", "events:delete"},
		AllowedEnvs: []string{"production"},
	}}

	_, keys, err := resolveAuthConfig(cfg)
	if err != nil {
		t.Fatalf("resolve scoped key: %v", err)
	}
	if len(keys) != 1 || keys[0].collector != "orders" {
		t.Fatalf("scoped key collector was not resolved: %+v", keys)
	}
	if got := keys[0].permissions; len(got) != 3 || got[0] != auth.PermEventsRead || got[1] != auth.PermEventsWrite || got[2] != auth.PermEventsDelete {
		t.Fatalf("scoped key permissions = %v, want read/write/delete", got)
	}

	cfg.Auth.Keys[0].Permissions = nil
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "auth.keys[0].permissions") {
		t.Fatalf("expected scoped key permissions rejection, got %v", err)
	}
	cfg.Auth.Keys[0].Permissions = []string{"events:write"}
	cfg.Auth.Keys[0].Collector = "unknown"
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "configured collector") {
		t.Fatalf("expected unknown scoped key collector rejection, got %v", err)
	}
}

func TestValidateFileConfigRejectsInvalidCollectorGrantConfiguration(t *testing.T) {
	publicID := "lz_pub_0123456789abcdefghijklmnopqrstuv"
	t.Setenv("COLLECTOR_PUBLIC_ACCESS_ID", publicID)
	cfg := validFileConfig()
	cfg.Auth.Collectors = []collectorconfig.AuthCollectorConfig{{Slug: "web"}, {Slug: "web"}}
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("expected duplicate collector rejection, got %v", err)
	}

	cfg = validFileConfig()
	cfg.Auth.Collectors = []collectorconfig.AuthCollectorConfig{{Slug: "Not_valid"}}
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "valid collector slug") {
		t.Fatalf("expected invalid collector slug rejection, got %v", err)
	}

	cfg = validFileConfig()
	cfg.Auth.Collectors = []collectorconfig.AuthCollectorConfig{{Slug: "web"}}
	cfg.Auth.Grants = []collectorconfig.AuthGrantConfig{{
		Collector:      "unknown",
		PublicIDEnv:    "COLLECTOR_PUBLIC_ACCESS_ID",
		Permissions:    []string{"events:write"},
		AllowedEnvs:    []string{"prod"},
		AllowedOrigins: []string{"https://console.example.test"},
	}}
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "configured collector") {
		t.Fatalf("expected unknown collector rejection, got %v", err)
	}

	cfg.Auth.Grants[0].Collector = "web"
	t.Setenv("COLLECTOR_PUBLIC_ACCESS_ID", "lz_pub_invalid")
	if err := validateFileConfig(cfg); err == nil || strings.Contains(err.Error(), "lz_pub_invalid") {
		t.Fatalf("expected redacted malformed public ID rejection, got %v", err)
	}
}

func TestValidateFileConfigRejectsInvalidConfiguredAuth(t *testing.T) {
	cfg := validFileConfig()
	cfg.Auth.Enabled = true
	cfg.Auth.ServerSecret = "${COLLECTOR_AUTH_SERVER_SECRET}"
	cfg.Auth.CacheTTL = time.Minute
	cfg.Auth.NegativeCacheTTL = time.Second
	cfg.Storage.EncryptionKey = "test-storage-encryption-key"
	cfg.Auth.Keys = []collectorconfig.AuthKeyConfig{
		{KeyID: "duplicate", SecretEnv: "COLLECTOR_INGEST_KEY_SECRET", Kind: "sec", Roles: []string{"collector_ingest_server"}},

		{KeyID: "duplicate", SecretEnv: "COLLECTOR_ADMIN_KEY_SECRET", Kind: "sec", Roles: []string{"not-a-role"}},
	}
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "must be unique") {
		t.Fatalf("expected duplicate configured key rejection, got %v", err)
	}
	cfg.Auth.Keys = []collectorconfig.AuthKeyConfig{
		{KeyID: "public", SecretEnv: "COLLECTOR_INGEST_KEY_SECRET", Kind: "pub", Roles: []string{"collector_ingest_public"}},
	}
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "allowed_origins") {
		t.Fatalf("expected public origin restriction validation, got %v", err)
	}
	cfg.Auth.Keys[0].AllowedOrigins = []string{"https://console.example.test"}
	cfg.Auth.Keys[0].Roles = []string{"not-a-role"}
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "not recognized") {
		t.Fatalf("expected configured role validation, got %v", err)
	}
	cfg.Auth.Keys[0].KeyID = "invalid_key_id"
	cfg.Auth.Keys[0].Roles = []string{"collector_ingest_public"}
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "must not contain underscores") {
		t.Fatalf("expected token-safe key ID validation error, got %v", err)
	}
}

func TestResolveAuthTokensBuildsRBACCredentials(t *testing.T) {
	t.Setenv("COLLECTOR_ADMIN_TOKEN", "lxt_private_admin_token")
	cfg := validFileConfig()
	cfg.Auth.Tokens = []collectorconfig.AuthTokenConfig{{
		Name:        "admin-token",
		TokenEnv:    "COLLECTOR_ADMIN_TOKEN",
		Mode:        "private",
		Roles:       []string{"client"},
		Collector:   "logs",
		Permissions: []string{"logs:write"},
		AllowedEnvs: []string{"prod"},
	}}

	tokens, err := resolveAuthTokens(cfg)
	if err != nil {
		t.Fatalf("resolve token: %v", err)
	}
	if len(tokens) != 1 || tokens[0].mode != auth.ModePrivate || len(tokens[0].roles) != 1 || tokens[0].roles[0] != auth.RoleClient {
		t.Fatalf("token RBAC configuration was not resolved: %+v", tokens)
	}
	if len(tokens[0].permissions) != 1 || tokens[0].permissions[0] != auth.PermLogsWrite {
		t.Fatalf("token log scope was not resolved: %+v", tokens[0])
	}

	cfg.Auth.Tokens[0].Roles = []string{"admin"}
	cfg.Auth.Tokens[0].Mode = "public"
	if _, err := resolveAuthTokens(cfg); err == nil || !strings.Contains(err.Error(), "public credentials") {
		t.Fatalf("expected privileged public token rejection, got %v", err)
	}
}

func TestValidateFileConfigRejectsInvalidValues(t *testing.T) {
	cfg := validFileConfig()
	cfg.DuckDB.Table = "events;drop"
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected invalid table name error")
	}

	cfg = validFileConfig()
	cfg.Collector.MaxBodyBytes = 0
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected max body validation error")
	}

	cfg = validFileConfig()
	cfg.Auth.Enabled = true
	cfg.Auth.Value = ""
	cfg.Auth.ValueEnv = ""
	cfg.Auth.Keys = nil
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected auth validation error")
	}

	cfg = validFileConfig()
	cfg.Reliability.Mode = "spool"
	cfg.Reliability.MaxSpoolBytes = 0
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected spool max bytes validation error")
	}

	cfg = validFileConfig()
	cfg.Retry.Enabled = true
	cfg.Retry.MaxAttempts = 0
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected retry validation error")
	}

	cfg = validFileConfig()
	cfg.DeadLetter.Enabled = true
	cfg.DeadLetter.Path = ""
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected dlq path validation error")
	}

	cfg = validFileConfig()
	cfg.DuckDB.WriterQueueSize = -1
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected writer queue size validation error")
	}

	cfg = validFileConfig()
	cfg.DuckDB.Export.Enabled = true
	cfg.DuckDB.Export.Format = "json"
	if err := validateFileConfig(cfg); err == nil {
		t.Fatalf("expected export format validation error")
	}

	cfg = validFileConfig()
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
	cfg := validFileConfig()
	cfg.Fanout.Delivery.Policy = "invalid"
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "fanout.delivery.policy") {
		t.Fatalf("expected fanout policy validation error, got: %v", err)
	}
}

func TestValidateFileConfigFanoutFallbackRequiresOutput(t *testing.T) {
	cfg := validFileConfig()
	cfg.Fanout.Delivery.Fallback.Enabled = true
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "requires exactly one enabled fallback output") {
		t.Fatalf("expected fallback output validation error, got: %v", err)
	}
}

func TestValidateFileConfigFanoutOutputValidation(t *testing.T) {
	cfg := validFileConfig()
	cfg.Storage.EncryptionKey = "test-storage-encryption-key"
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
	cfg := validFileConfig()
	cfg.Storage.EncryptionKey = "test-storage-encryption-key"
	cfg.Fanout.Outputs = []collectorconfig.FanoutOutputConfig{
		{
			Name:    "pg-copy",
			Role:    "secondary",
			Type:    "postgres",
			Enabled: true,
			Postgres: collectorconfig.FanoutPostgresConfig{
				DSN:       "postgres://user:pass@localhost:5432/loza?sslmode=disable",
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
	cfg := validFileConfig()
	cfg.Reliability.Mode = "queue"
	cfg.Kafka.Brokers = nil
	cfg.Kafka.Topic = ""
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "kafka.brokers") {
		t.Fatalf("expected kafka broker validation error, got: %v", err)
	}
}

func TestValidateFileConfigRejectsInvalidKafkaAcks(t *testing.T) {
	cfg := validFileConfig()
	cfg.Kafka.Acks = "leader"
	if err := validateFileConfig(cfg); err == nil || !strings.Contains(err.Error(), "kafka.acks") {
		t.Fatalf("expected kafka.acks validation error, got: %v", err)
	}
}

func TestValidateFileConfigRedisDedupeRequiresRedisAddr(t *testing.T) {
	cfg := validFileConfig()
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
	t.Setenv("COLLECTOR_KAFKA_TOPIC", "loza-events")
	t.Setenv("LOZA_WORKER_CONSUMER_GROUP", "loza-worker-test")
	t.Setenv("LOZA_WORKER_POLL_TIMEOUT", "3s")

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
	if cfg.kafkaTopic != "loza-events" {
		t.Fatalf("unexpected kafka topic: %q", cfg.kafkaTopic)
	}
	if cfg.workerConsumerGroup != "loza-worker-test" {
		t.Fatalf("unexpected worker group: %q", cfg.workerConsumerGroup)
	}
	if cfg.workerPollTimeout != 3*time.Second {
		t.Fatalf("unexpected worker poll timeout: %s", cfg.workerPollTimeout)
	}
}
