# LOZA Configuration Reference

This guide documents all configuration options for LOZA components.

## File Format

Configuration is stored in YAML format. By default, LOZA looks for:
- `loza.yaml` (current directory)
- `./loza.yaml`
- Parent directories up to 5 levels
- Executable directory

Environment variable overrides: Set `LOZA_COLLECTOR_DEFAULTS` to specify an alternative defaults file.

## Collector Configuration

### `collector`

Server listening and request handling.

```yaml
collector:
  addr: ":9308"                    # Listen address (host:port)
  read_header_timeout: 5s          # Header read timeout
  shutdown_timeout: 10s            # Graceful shutdown timeout
  max_body_bytes: 10485760         # Max POST body (10MB)
  max_events_per_request: 5000     # Max events per ingest request
  
  server:
    http:
      enabled: true
      addr: ":9308"
      read_header_timeout: 5s
      max_body_bytes: 10485760
      max_header_bytes: 1048576    # Max header size (1MB)
      idle_timeout: 90s
    
    grpc:
      enabled: false
      port: ":9309"
      max_connections: 1000
      max_concurrent_streams: 100
      max_recv_msg_size: 4194304   # 4MB
      max_send_msg_size: 4194304
      keepalive:
        max_connection_age: 5m
        max_connection_age_grace: 30s
        time: 2m
        timeout: 20s
      tls:
        enabled: false
        cert_file: /path/to/cert.pem
        key_file: /path/to/key.pem
    
    graphql:
      enabled: false
      port: ":9310"
      playground: true
      depth_limit: 10
      batch_limit: 10
```

### `auth`

API-key authentication uses `Authorization: Bearer` and is enabled in every
tracked Collector configuration. `server_secret` is used only for HMAC
key-record hashing and must not be reused as the storage encryption key.

```yaml
auth:
  enabled: true
  server_secret: "${COLLECTOR_AUTH_SERVER_SECRET}"
  cache_ttl: 5m
  negative_cache_ttl: 30s
  collectors:
    - slug: checkout
  keys:
    - name: checkout-writer
      key_id: kcheckoutwriter
      secret_env: CHECKOUT_WRITER_KEY_SECRET
      kind: sec
      mode: private
      roles: [collector_ingest_server]
      collector: checkout
      permissions: [events:write]
      allowed_envs: [prod]
    - name: checkout-reader
      key_id: kcheckoutreader
      secret_env: CHECKOUT_READER_KEY_SECRET
      kind: sec
      mode: private
      roles: [project_readonly]
      collector: checkout
      permissions: [events:read]
      allowed_envs: [prod]
```

`secret_env` is the secret segment of
`lx_{kind}_live_{key_id}_{secret}`, not the full token. Key IDs are unique;
`kind` must be `sec` or `pub`; `mode` is `private` or `public`; public
credentials require `allowed_origins` and may use only the `client` role.
Scoped keys require a configured `collector`, non-empty `permissions`, and
non-empty `allowed_envs`. Scoped permissions are exact:
`events:read`, `events:write`, `events:delete`, `logs:read`, `logs:write`,
`logs:edit`, `logs:delete`, and `project:admin` are authorized independently
on `/collectors/{collector}/...`. Unscoped keys remain limited to explicitly
configured legacy root routes.

Opaque Bearer tokens use the same RBAC and scope model:

```yaml
auth:
  tokens:
    - name: browser-log-writer
      token_env: BROWSER_LOG_TOKEN
      mode: public
      roles: [client]
      collector: checkout
      permissions: [logs:write]
      allowed_envs: [prod]
      allowed_origins: [https://app.example.com]
```

`token_env` must contain an opaque value, not an `lx_` API key. The raw token
is only accepted in `Authorization: Bearer`; it is not used as a stored ID.

```bash
curl -X POST http://localhost:9308/collectors/checkout/events \
  -H "Authorization: Bearer lx_sec_live_kcheckoutwriter_${CHECKOUT_WRITER_KEY_SECRET}" \
  -H "X-Loza-Env: prod" \
  -H 'Content-Type: application/json' -d '[{"event":"example"}]'
curl http://localhost:9308/health
```

See [Authentication](authentication.md) and [Authorization](authorization.md) for full details.

### `routes`

HTTP endpoint paths. The default ingest endpoint is `/events`; `/health` is public. Other data and administration routes require the corresponding permission.

```yaml
routes:
  ingest: /events
  health: /health
  ready: /readyz
  metrics: /metrics
```

### `rate_limit`

Rate limiting (optional).

```yaml
rate_limit:
  enabled: true
  rps: 1000                            # Requests per second
  burst: 1000                          # Burst allowance
```

### `storage`

DuckDB is the supported primary storage backend. Raw event persistence is encrypted by Collector construction, so this environment reference must resolve to a non-empty key before startup.

```yaml
storage:
  primary: duckdb
  encryption_key_env: LOZA_STORAGE_ENCRYPTION_KEY
```

### `duckdb`

DuckDB database configuration (when `storage.primary: duckdb`).

```yaml
duckdb:
  path: loza.db                        # Database file path
  driver: duckdb
  table: events                        # Table name
  raw_column: raw                      # Column for raw event JSON
  store_raw: true                      # Store full event JSON
  checkpoint_on_shutdown: true         # Create checkpoint on stop
  checkpoint_interval: 0s              # Periodic checkpoint (0=disabled)
  max_open_conns: 1                    # Connection pool size
  max_idle_conns: 1
  batch_size: 0                        # Events per batch (0=no batching)
  flush_interval: 0s                   # Flush buffered events interval
  writer_loop: false                   # Use writer goroutine (high throughput)
  writer_queue_size: 0                 # Queue size for writer
  use_appender: false                  # Use DuckDB Appender API when available (higher throughput)
  write_timeout: 10s                   # Bound background write operations when no caller deadline set
  retry_attempts: 0                    # Number of retries for transient DB errors (0=disabled)
  retry_backoff: 50ms                  # Backoff between retry attempts (jitter added at runtime to avoid thundering-herd)
  
  export:
    enabled: false
    format: parquet                    # parquet | csv | json
    interval: 1h                       # Export interval
    path: exports                      # Export directory
  
  schema:                              # Column mappings (path projections)
    event_id: event_id
    event_type: event
    service: service
    status_code: http.status
    duration_ms: duration_ms
    timestamp: timestamp
  
  column_types:                        # Override DuckDB column types
    status_code: INTEGER
    duration_ms: DOUBLE
    timestamp: TIMESTAMP
```

### `retention`

Data retention policies (optional).

```yaml
retention:
  enabled: true
  days: 30                             # Keep events for N days
  max_size: 10737418240                # Keep table up to N bytes (10GB)
```

Policies run daily. Both can be configured:
- If age > days: DELETE
- If size > max_size: DELETE oldest until size <= threshold

### `kafka`

Kafka configuration (when `storage.primary: kafka`).

```yaml
kafka:
  brokers:
    - 127.0.0.1:9092                   # Broker addresses
  topic: loza-events
  acks: all                            # 0 | 1 | all (delivery guarantee)
  request_timeout: 10s
  enable_idempotence: true             # Enable exactly-once semantics
  max_retries: 3
  retry_backoff: 100ms
```

### `worker`

Worker process configuration (consuming from Kafka).

```yaml
worker:
  consumer_group: loza-worker
  poll_timeout: 2s
```

### `logging`

Logger configuration.

```yaml
logging:
  level: info                          # debug | info | warn | error
  format: json                         # json | text
```

### `metrics`

Metrics export.

```yaml
metrics:
  prometheus: true                     # Enable Prometheus /metrics endpoint
```

### `reliability`

Delivery reliability modes.

```yaml
reliability:
  mode: direct                         # direct | queue | spool
  
  # Spool mode settings
  spool_dir: loza-spool
  spool_file: spool.ndjson
  max_spool_bytes: 10737418240         # 10GB max spool size
  fsync: true                          # Sync to disk on each write
  delivery_queue_size: 4096            # In-flight events queue
  
  # Queue mode settings (with Kafka)
  queue_dir: loza-queue
  queue_batch_size: 100                # Events per worker batch
  queue_batch_timeout: 5s
  queue_flush_interval: 1s
  queue_circuit_threshold: 10          # Failures before circuit opens
  queue_circuit_timeout: 30s
```

### `limits`

Event processing limits.

```yaml
limits:
  max_inflight_requests: 1024          # Concurrent ingest requests
  max_inflight_events: 100000          # Total events in flight
  max_queue_bytes: 268435456           # 256MB queue size limit
  max_event_bytes: 262144              # 256KB per event
  max_attr_count: 512                  # Attributes per event
  max_attr_depth: 16                   # Nesting depth
  max_string_length: 16384             # Max string value
```

### `identity`

Tenant/service identity resolution.

```yaml
identity:
  mode: payload                        # payload | auth | service | bound
  auth_identity_wins: true             # Prefer auth header identity
  allow_payload_identity: true         # Allow identity from event
  
  # Bound identity (for single-tenant deployments)
  service_name: my-service
  service_version: 0.2.0
  deployment_environment: production
  deployment_region: us-west-2
  tenant_id: tenant-123
  workspace_id: workspace-456
  organization_id: org-789
```

### `privacy`

Privacy and redaction settings.

```yaml
privacy:
  mode: warn                           # enforce | warn | allow
  collector_redaction: true            # Apply redaction on ingest
  emergency_redaction: false           # Redact entire event on error
  
  blocklist:                           # Patterns to redact
    - password
    - passwd
    - secret
    - token
    - api_key
    - apikey
    - authorization
    - cookie
    - set-cookie
    - private_key
    - access_token
    - refresh_token
    - session
  
  allowlist: []                        # Patterns to NOT redact
  secret_scan: true                    # Scan for common secrets
  right_to_delete_enabled: true        # Support data deletion requests
```

### `components`

Registered component implementations.

```yaml
components:
  receivers:
    - loza_http
    - loza_ndjson
  
  processors:
    - validate
    - identity
    - redact
    - dedupe
    - memory_limiter
  
  exporters:
    - duckdb
    - kafka
  
  extensions:
    - health
    - ready
    - metrics
```

### `retry`

Automatic retry configuration.

```yaml
retry:
  enabled: true
  max_attempts: 3
  initial_backoff: 100ms
  max_backoff: 30s
  jitter: true                         # Randomize backoff
```

### `dead_letter`

Dead letter queue for undeliverable events.

```yaml
dead_letter:
  enabled: true
  path: loza-dlq                       # DLQ directory
```

### `fanout`

Multi-sink fanout routing (optional).

```yaml
fanout:
  outputs:
    - name: loki-output
      enabled: true
      type: loki
      role: secondary                  # primary | secondary | fallback
      
      loki:
        url: http://loki:3100
        tenant_id: tenant-123
        labels:
          service: my-app
          env: production
        batch_size: 100
        flush_interval: 5s
        timeout: 10s
    
    - name: clickhouse-output
      type: clickhouse
      clickhouse:
        addrs:
          - http://clickhouse:8123
        database: loza
        username: default
        password: default
        table: events
        batch_size: 100
        flush_interval: 5s
    
    - name: postgres-output
      type: postgres
      postgres:
        addr: postgres:5432
        database: loza
        username: postgres
        password: postgres
        table: events
    
    - name: s3-output
      type: s3
      s3:
        bucket: my-bucket
        region: us-west-2
        prefix: loza-events/
        batch_size: 100
  
  delivery:
    policy: require_primary            # require_primary | require_all | best_effort
    
    fallback:
      enabled: false
      on_primary_failure: false
      on_secondary_failure: false
      on_policy_failure: false
    
    dlq:
      on_primary_failure: true
      on_secondary_failure: false
      on_fallback_failure: true
      on_policy_failure: true
```

### `dedupe`

Event deduplication.

```yaml
dedupe:
  enabled: true
  key: event_id                        # Field to dedupe on
  window: 1h                           # Dedup window duration
  backend: memory                      # memory | redis (future)
```

### `schema_governance`

Schema validation and enforcement.

```yaml
schema_governance:
  mode: warn                           # strict | warn | allow
  schema_version: v1
  event_version: v1
  registry_file: schema-registry.yaml
  quarantine_path: loza-quarantine
  
  registry:
    - schema_version: v1
      event_version: v1
      required_fields:
        - event_id
        - event
        - timestamp
```

## SDK Configuration (Application-Side)

### Go SDK

```go
import loza "github.com/astraive/loza/sdks/go"

client, err := loza.New(loza.Config{
    Service: "my-service",
    Sink:    sink,
    APIKey:  os.Getenv("LOZA_API_KEY"),
})
if err != nil {
    panic(err)
}
defer client.Close(context.Background())
```

See [Authentication](authentication.md) for key configuration.

// Configuration precedence:
// 1. Code (NewSDK options)
// 2. Environment variables (LOZA_*)
// 3. Config file (./loza.yaml)
// 4. Defaults
```

### Python SDK

```python
from loza import SDK

sdk = SDK()

# Or with config file:
sdk = SDK(config_file="./loza.yaml")
```

### Rust SDK

```rust
use loza::SDK;

let sdk = SDK::new();

// Configuration from environment/file
```

## Environment Variables

All configuration can be overridden via environment variables:

```bash
# Collector
export LOZA_COLLECTOR_ADDR=":9309"
export LOZA_API_KEY="secret123"
export LOZA_DUCKDB_PATH="/data/loza.db"
export LOZA_RETENTION_DAYS=30
export LOZA_RETENTION_MAX_SIZE=10737418240

# Logging
export LOZA_LOGGING_LEVEL="debug"

# Reliability
export LOZA_RELIABILITY_MODE="spool"
```

## Default Configuration

The default configuration is embedded in LOZA binaries. To see defaults:

```bash
loza config print
```

To validate your configuration:

```bash
loza config validate -c loza.yaml
```

## Configuration Examples

### Development (Local)

```yaml
collector:
  addr: ":9308"

auth:
  enabled: false

duckdb:
  path: ./loza.db

logging:
  level: debug
  format: json

metrics:
  prometheus: true
```

### Production (HA)

```yaml
collector:
  addr: ":9308"
  max_body_bytes: 52428800             # 50MB

auth:
  enabled: true
  server_secret: "${COLLECTOR_SERVER_SECRET}"
  keys:
    - name: "prod"
      key_id: "k2M9aQp"
      secret_env: "PROD_KEY_SECRET"
      kind: "sec"
      roles: ["collector_ingest_server"]

duckdb:
  path: /data/loza.db                  # Persistent volume
  max_open_conns: 5
  checkpoint_interval: 5m

retention:
  enabled: true
  days: 90
  max_size: 107374182400               # 100GB

reliability:
  mode: spool
  spool_dir: /data/loza-spool
  max_spool_bytes: 107374182400

logging:
  level: warn
  format: json

metrics:
  prometheus: true
```

### Kafka Queue Mode

```yaml
storage:
  primary: kafka

kafka:
  brokers:
    - kafka-1:9092
    - kafka-2:9092
    - kafka-3:9092
  topic: loza-events
  acks: all
  enable_idempotence: true

reliability:
  mode: queue
  queue_batch_size: 100
  queue_batch_timeout: 5s
```

---

See [README](../README.md) for quick start and [architecture.md](./architecture.md) for system design.
