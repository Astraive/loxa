# Sink Reference

The collector supports eight sink implementations. Each sink is configured independently and receives events through the fanout stage.

## Configuration

Sinks are configured under the `sinks` section of the collector config file. Each sink has a `name`, `type`, and type-specific configuration fields.

```yaml
sinks:
  - name: primary-db
    type: duckdb
    # ... type-specific fields
  - name: analytics
    type: clickhouse
    # ... type-specific fields
```

## Sink Overview

| Sink | Type | Batching | Durability | Use Case |
|---|---|---|---|---|
| DuckDB | Columnar embedded DB | Micro-batch | File-based | Local query, analytics |
| Kafka | Distributed log | Per-message | Replicated | Event streaming, fanout |
| ClickHouse | Columnar OLAP | Micro-batch | Replicated | Analytics, aggregations |
| Postgres | Relational DB | Per-message | WAL | Metadata, relational queries |
| Loki | Log aggregation | Micro-batch | Replicated | Log search, dashboards |
| OTLP | OpenTelemetry export | Batch | Protocol-level | Vendor-agnostic export |
| S3 | Object storage | Micro-batch | Replicated | Cold storage, archival |
| GCS | Object storage | Micro-batch | Replicated | Cold storage, archival |

---

## DuckDB

Embedded columnar database. The default sink for local development and single-node deployments. Shared with Cortex for incident intelligence.

**Config Fields**

| Field | Type | Default | Description |
|---|---|---|---|
| path | string | `loxa.db` | Database file path |
| table | string | `events` | Target table name |
| column_types | map | (auto) | Column type overrides |

**Example**

```yaml
sinks:
  - name: local-db
    type: duckdb
    path: /data/loxa.db
    table: events
```

**Behavior Notes**

- Events are inserted in micro-batches for throughput.
- The database file is locked per-process. Do not run multiple collector instances against the same file.
- Cortex opens the same database file for read queries.
- Retention is managed by the `storage.duckdb.retention_days` config.

---

## Kafka

Distributed event log for high-throughput streaming and multi-consumer fanout.

**Config Fields**

| Field | Type | Default | Description |
|---|---|---|---|
| brokers | string[] | (required) | Kafka broker addresses |
| topic | string | (required) | Target topic |
| acks | string | `all` | Acknowledgment mode: `all`, `1`, `0` |
| enable_idempotence | bool | `true` | Idempotent producer |
| compression | string | `snappy` | Compression: `none`, `gzip`, `snappy`, `lz4`, `zstd` |
| max_message_bytes | int | `1048576` | Max message size in bytes |
| flush_frequency | duration | `100ms` | Producer flush interval |

**Example**

```yaml
sinks:
  - name: event-stream
    type: kafka
    brokers:
      - "127.0.0.1:9092"
    topic: loxa-events
    acks: all
    enable_idempotence: true
    compression: snappy
```

**Behavior Notes**

- Each event is sent as a single Kafka message. The event ID is used as the message key.
- The producer is created once and reused across flushes.
- Connection failures trigger the retry/DLQ path.

---

## ClickHouse

Columnar OLAP database for analytics workloads.

**Config Fields**

| Field | Type | Default | Description |
|---|---|---|---|
| dsn | string | (required) | ClickHouse connection DSN |
| table | string | `events` | Target table name |
| batch_size | int | `1000` | Insert batch size |
| flush_interval | duration | `5s` | Flush interval |

**Example**

```yaml
sinks:
  - name: analytics
    type: clickhouse
    dsn: "clickhouse://localhost:9000/loxa"
    table: events
    batch_size: 1000
    flush_interval: 5s
```

**Behavior Notes**

- Events are batched and inserted using `INSERT INTO ... FORMAT JSONEachRow`.
- The schema SQL is in `internal/sinks/clickhouse/schema.sql`.
- Retries are handled at the batch level.

---

## Postgres

Relational database for metadata storage and relational queries.

**Config Fields**

| Field | Type | Default | Description |
|---|---|---|---|
| dsn | string | (required) | PostgreSQL connection DSN |
| table | string | `events` | Target table name |
| batch_size | int | `100` | Insert batch size |
| flush_interval | duration | `5s` | Flush interval |

**Example**

```yaml
sinks:
  - name: metadata
    type: postgres
    dsn: "postgres://user:pass@localhost:5432/loxa?sslmode=disable"
    table: events
```

**Behavior Notes**

- Events are inserted using prepared statements.
- The schema SQL is in `internal/sinks/postgres/schema.sql`.
- Connection pooling is managed by `pgxpool`.

---

## Loki

Log aggregation system for log search and dashboard integration.

**Config Fields**

| Field | Type | Default | Description |
|---|---|---|---|
| url | string | (required) | Loki push URL |
| tenant_id | string | `` | Loki tenant ID (X-Scope-OrgID) |
| labels | map | `{}` | Static labels to attach |
| batch_size | int | `100` | Push batch size |
| flush_interval | duration | `5s` | Push interval |

**Example**

```yaml
sinks:
  - name: logs
    type: loki
    url: "http://localhost:3100/loki/api/core/push"
    tenant_id: loxa
    labels:
      source: loxa-collector
```

**Behavior Notes**

- Events are converted to Loki log entries with labels extracted from event fields.
- Pushes use the `/loki/api/core/push` endpoint.
- Retries are handled at the batch level.

---

## OTLP

OpenTelemetry Protocol export for vendor-agnostic event forwarding.

**Config Fields**

| Field | Type | Default | Description |
|---|---|---|---|
| endpoint | string | (required) | OTLP gRPC or HTTP endpoint |
| protocol | string | `grpc` | Transport: `grpc` or `http` |
| headers | map | `{}` | Additional headers |
| insecure | bool | `false` | Disable TLS |
| batch_size | int | `512` | Export batch size |
| flush_interval | duration | `5s` | Export interval |

**Example**

```yaml
sinks:
  - name: otlp-export
    type: otlp
    endpoint: "otel-collector:4317"
    protocol: grpc
    insecure: true
```

**Behavior Notes**

- Events are converted to OTLP log records using `internal/otlpconv`.
- Supports both gRPC and HTTP transports.
- Compatible with any OTLP-compliant backend (Jaeger, Grafana Tempo, etc.).

---

## S3

AWS S3 object storage for cold storage and archival.

**Config Fields**

| Field | Type | Default | Description |
|---|---|---|---|
| bucket | string | (required) | S3 bucket name |
| prefix | string | `` | Object key prefix |
| region | string | `us-east-1` | AWS region |
| endpoint | string | `` | Custom endpoint (for MinIO, etc.) |
| batch_size | int | `1000` | Objects per flush |
| flush_interval | duration | `60s` | Flush interval |
| format | string | `ndjson` | Output format: `ndjson`, `parquet` |

**Example**

```yaml
sinks:
  - name: archive
    type: s3
    bucket: loxa-events
    prefix: "2026/05/"
    region: us-east-1
    format: ndjson
    flush_interval: 60s
```

**Behavior Notes**

- Events are batched into NDJSON files and uploaded as S3 objects.
- Object keys follow the pattern: `{prefix}{date}/{batch_id}.jsonl`
- Uses the AWS SDK v2 for Go. Credentials are resolved from the standard AWS chain.

---

## GCS

Google Cloud Storage for cold storage and archival.

**Config Fields**

| Field | Type | Default | Description |
|---|---|---|---|
| bucket | string | (required) | GCS bucket name |
| prefix | string | `` | Object key prefix |
| batch_size | int | `1000` | Objects per flush |
| flush_interval | duration | `60s` | Flush interval |
| format | string | `ndjson` | Output format: `ndjson`, `parquet` |

**Example**

```yaml
sinks:
  - name: gcs-archive
    type: gcs
    bucket: loxa-events-gcs
    prefix: "events/"
    format: ndjson
```

**Behavior Notes**

- Events are batched into NDJSON files and uploaded as GCS objects.
- Uses the Google Cloud Go client library. Credentials are resolved from the standard GCP chain (service account, workload identity, etc.).
- Object naming follows the same pattern as S3.
