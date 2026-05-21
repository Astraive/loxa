package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerConfigDefaults(t *testing.T) {
	cfg := Default()

	assert.True(t, cfg.Collector.Server.HTTP.Enabled)
	assert.Equal(t, ":9090", cfg.Collector.Server.HTTP.Addr)
	assert.Equal(t, 5*time.Second, cfg.Collector.Server.HTTP.ReadHeaderTimeout)
	assert.Equal(t, int64(10*1024*1024), cfg.Collector.Server.HTTP.MaxBodyBytes)

	assert.False(t, cfg.Collector.Server.GRPC.Enabled)
	assert.Equal(t, ":9091", cfg.Collector.Server.GRPC.Port)
	assert.Equal(t, 1000, cfg.Collector.Server.GRPC.MaxConnections)
	assert.Equal(t, 100, cfg.Collector.Server.GRPC.MaxConcurrentStreams)

	assert.False(t, cfg.Collector.Server.GraphQL.Enabled)
	assert.Equal(t, ":9092", cfg.Collector.Server.GraphQL.Port)
	assert.True(t, cfg.Collector.Server.GraphQL.Playground)
}

func TestServerConfigYAML(t *testing.T) {
	yaml := `
collector:
  addr: ":9090"
  server:
    http:
      enabled: true
      addr: ":9090"
      read_header_timeout: 10s
      max_body_bytes: 20971520
    grpc:
      enabled: true
      port: ":9091"
      max_connections: 500
      max_concurrent_streams: 50
      keepalive:
        max_connection_age: 10m
        time: 5m
    graphql:
      enabled: true
      port: ":9092"
      playground: true
      depth_limit: 15
`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "loxa.yaml")
	err := os.WriteFile(configFile, []byte(yaml), 0644)
	require.NoError(t, err)

	cfg := Default()
	err = LoadFile(&cfg, configFile)
	require.NoError(t, err)

	assert.True(t, cfg.Collector.Server.HTTP.Enabled)
	assert.Equal(t, ":9090", cfg.Collector.Server.HTTP.Addr)
	assert.Equal(t, 10*time.Second, cfg.Collector.Server.HTTP.ReadHeaderTimeout)
	assert.Equal(t, int64(20971520), cfg.Collector.Server.HTTP.MaxBodyBytes)

	assert.True(t, cfg.Collector.Server.GRPC.Enabled)
	assert.Equal(t, ":9091", cfg.Collector.Server.GRPC.Port)
	assert.Equal(t, 500, cfg.Collector.Server.GRPC.MaxConnections)
	assert.Equal(t, 50, cfg.Collector.Server.GRPC.MaxConcurrentStreams)
	assert.Equal(t, 10*time.Minute, cfg.Collector.Server.GRPC.Keepalive.MaxConnectionAge)
	assert.Equal(t, 5*time.Minute, cfg.Collector.Server.GRPC.Keepalive.Time)

	assert.True(t, cfg.Collector.Server.GraphQL.Enabled)
	assert.Equal(t, ":9092", cfg.Collector.Server.GraphQL.Port)
	assert.True(t, cfg.Collector.Server.GraphQL.Playground)
	assert.Equal(t, 15, cfg.Collector.Server.GraphQL.DepthLimit)
}

func TestServerConfigTLS(t *testing.T) {
	yaml := `
collector:
  server:
    grpc:
      enabled: true
      port: ":9091"
      tls:
        enabled: true
        cert_file: "/path/to/cert.pem"
        key_file: "/path/to/key.pem"
`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "loxa.yaml")
	err := os.WriteFile(configFile, []byte(yaml), 0644)
	require.NoError(t, err)

	cfg := Default()
	err = LoadFile(&cfg, configFile)
	require.NoError(t, err)

	assert.True(t, cfg.Collector.Server.GRPC.TLS.Enabled)
	assert.Equal(t, "/path/to/cert.pem", cfg.Collector.Server.GRPC.TLS.CertFile)
	assert.Equal(t, "/path/to/key.pem", cfg.Collector.Server.GRPC.TLS.KeyFile)
}

func TestDefaultLoadsFromDefaultsFile(t *testing.T) {
	tmpDir := t.TempDir()
	defaultsPath := filepath.Join(tmpDir, "loxa-collector.defaults.yaml")
	yaml := `
collector:
  addr: ":9191"
  read_header_timeout: 7s
auth:
  enabled: false
  header: X-API-Key
  value_env: COLLECTOR_API_KEY
rate_limit:
  enabled: true
  rps: 10
  burst: 10
routes:
  ingest: /ingest
  health: /health
  ready: /ready
  metrics: /metrics
storage:
  primary: duckdb
duckdb:
  path: loxa.db
  driver: duckdb
  table: events
  raw_column: raw
  store_raw: true
  checkpoint_on_shutdown: true
  checkpoint_interval: 0s
  max_open_conns: 1
  max_idle_conns: 1
  batch_size: 0
  flush_interval: 0s
  writer_loop: false
  writer_queue_size: 0
  export:
    enabled: false
    format: parquet
    interval: 1h
    path: exports
logging:
  level: info
  format: json
metrics:
  prometheus: true
reliability:
  mode: direct
  spool_dir: loxa-spool
  spool_file: spool.ndjson
  max_spool_bytes: 1024
  fsync: true
  delivery_queue_size: 16
limits:
  max_inflight_requests: 1
  max_inflight_events: 2
  max_queue_bytes: 3
  max_event_bytes: 4
  max_attr_count: 5
  max_attr_depth: 6
  max_string_length: 7
identity:
  mode: payload
  auth_identity_wins: true
  allow_payload_identity: true
privacy:
  mode: warn
  collector_redaction: true
  emergency_redaction: false
components:
  receivers: [loxa_http]
  processors: [validate]
  exporters: [duckdb]
  extensions: [health]
retry:
  enabled: true
  max_attempts: 1
  initial_backoff: 1s
  max_backoff: 2s
  jitter: false
dead_letter:
  enabled: false
  path: loxa-dlq.ndjson
fanout:
  outputs: []
  delivery:
    policy: require_primary
dedupe:
  enabled: false
  key: event_id
  window: 1h
  backend: memory
schema_governance:
  mode: off
  schema_version: v1
  event_version: v1
  quarantine_path: loxa-quarantine.ndjson
`
	require.NoError(t, os.WriteFile(defaultsPath, []byte(yaml), 0o644))
	t.Setenv("LOXA_COLLECTOR_DEFAULTS", defaultsPath)

	cfg := Default()
	assert.Equal(t, ":9191", cfg.Collector.Addr)
	assert.Equal(t, 7*time.Second, cfg.Collector.ReadHeaderTimeout)
	assert.Equal(t, int64(4), cfg.Limits.MaxEventBytes)
}
