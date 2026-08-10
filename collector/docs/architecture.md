# Architecture

## Ingest Pipeline

The collector accepts events over HTTP and processes them through a multi-stage pipeline before fanning out to sinks.

```mermaid
flowchart LR
    HTTP[HTTP Ingest] --> Parse[Parse Payload]
    Parse --> Validate[Validate]
    Validate --> Process[Process]
    Process --> Fanout[Fanout]
    Fanout --> Sinks[Sinks]

    style HTTP fill:#1a1a2e,stroke:#e94560,color:#fff
    style Parse fill:#16213e,stroke:#0f3460,color:#fff
    style Validate fill:#533483,stroke:#e94560,color:#fff
    style Process fill:#0f3460,stroke:#e94560,color:#fff
    style Fanout fill:#e94560,stroke:#fff,color:#fff
    style Sinks fill:#1a1a2e,stroke:#e94560,color:#fff
```

### HTTP Ingest

The ingest server listens on a configurable port (default: 9308) and accepts:

- Single JSON objects
- JSON arrays
- Wrapped envelopes (`{"events": [...]}`)
- NDJSON (newline-delimited JSON)
- Gzip-compressed payloads (`Content-Encoding: gzip`)
- Zstd-compressed payloads (`Content-Encoding: zstd`)

### Parse

The parser detects the payload format automatically. It reads the full body (up to `max_body_bytes`), decompresses if needed, and splits the payload into individual event byte slices.

### Validate

Validation checks that each event is a valid JSON object. If schema governance is enabled (`schema_governance.mode`), events are validated against registered schemas:

- `off` -- no schema validation
- `warn` -- log violations but accept the event
- `enforce` / `reject` -- reject events that fail validation
- `quarantine` -- write invalid events to a quarantine file instead of rejecting

### Process

The processing pipeline applies a series of transformations and checks:

```mermaid
flowchart LR
    E[Event] --> Identity[Identity Resolution]
    Identity --> Privacy[Privacy Redaction]
    Privacy --> Dedup[Deduplication]
    Dedup --> Limits[Rate Limits]
    Limits --> R[Ready for Delivery]

    style E fill:#1a1a2e,stroke:#e94560,color:#fff
    style Identity fill:#16213e,stroke:#0f3460,color:#fff
    style Privacy fill:#533483,stroke:#e94560,color:#fff
    style Dedup fill:#0f3460,stroke:#e94560,color:#fff
    style Limits fill:#e94560,stroke:#fff,color:#fff
    style R fill:#1a1a2e,stroke:#e94560,color:#fff
```

**Identity Resolution** -- tenant routing and service extraction from event fields.

**Privacy Redaction** -- applies PII redaction based on configured blocklist/allowlist patterns. Modes: `off`, `warn`, `enforce`. Optional secret scanning.

**Deduplication** -- detects duplicate events by a configurable key (default: `event_id`). Backends: in-memory or Redis. Configurable time window.

**Rate Limits** -- enforces per-tenant and global rate limits. Excess events are dropped or queued depending on the delivery policy.

## Delivery Modes

The collector supports three delivery modes configured via `reliability.mode`:

```mermaid
flowchart TD
    Event[Processed Event] --> CheckMode{Delivery Mode?}
    CheckMode -->|direct| Direct[Direct Delivery]
    CheckMode -->|spool| Spool[WAL Spool]
    CheckMode -->|queue| Queue[Queue Delivery]

    Direct --> Primary[Primary Sink]
    Primary -->|success| Done[Done]
    Primary -->|fail| Fallback[Fallback Sink]
    Fallback -->|success| Done
    Fallback -->|fail| DLQ1[DLQ]

    Spool --> WAL[Write to WAL]
    WAL --> Background[Background Flush]
    Background --> Primary2[Primary Sink]
    Primary2 -->|success| Ack[Ack and Trim WAL]
    Primary2 -->|fail| Retry1[Retry]
    Retry1 --> DLQ2[DLQ after max retries]

    Queue --> Enqueue[Enqueue to Kafka/Redis]
    Enqueue --> Worker[Worker Process]
    Worker --> Primary3[Primary Sink]
    Primary3 -->|success| Commit[Commit Offset]
    Primary3 -->|fail| Retry2[Retry]
    Retry2 --> DLQ3[DLQ after max retries]

    style Event fill:#1a1a2e,stroke:#e94560,color:#fff
    style Done fill:#e94560,stroke:#fff,color:#fff
    style Ack fill:#e94560,stroke:#fff,color:#fff
    style Commit fill:#e94560,stroke:#fff,color:#fff
    style DLQ1 fill:#533483,stroke:#e94560,color:#fff
    style DLQ2 fill:#533483,stroke:#e94560,color:#fff
    style DLQ3 fill:#533483,stroke:#e94560,color:#fff
```

| Mode | Behavior | Use Case |
|---|---|---|
| `direct` | Synchronous delivery to primary sink. Fallback to secondary on failure. | Low-latency, single-node deployments |
| `spool` | Write-ahead log (WAL). Background goroutine flushes to sinks with retry. | Single-node with durability |
| `queue` | Enqueue to Kafka/Redis. Worker processes consume and deliver. | Distributed, multi-worker deployments |

All modes support a Dead Letter Queue (DLQ) for events that exhaust retry attempts.

## Fanout

The fanout stage delivers events to all configured sinks simultaneously. Each sink runs in its own goroutine. Failures in one sink do not affect others.

```mermaid
flowchart LR
    Event[Event] --> Fanout{Fanout}
    Fanout --> DuckDB[DuckDB Sink]
    Fanout --> Kafka[Kafka Sink]
    Fanout --> ClickHouse[ClickHouse Sink]
    Fanout --> Postgres[Postgres Sink]
    Fanout --> Loki[Loki Sink]
    Fanout --> OTLP[OTLP Sink]
    Fanout --> S3[S3 Sink]
    Fanout --> GCS[GCS Sink]
    Fanout --> CortexPush[gRPC Push to Cortex]

    style Event fill:#1a1a2e,stroke:#e94560,color:#fff
    style Fanout fill:#e94560,stroke:#fff,color:#fff
```

The primary sink is the first configured sink. Secondary sinks receive events after the primary succeeds. A fallback sink can be configured to receive events when the primary fails.

## Storage

### DuckDB Schema

The DuckDB sink stores events in a columnar table. The schema is derived from the event's attribute types:

| Column | Type | Description |
|---|---|---|
| id | VARCHAR | Event ID |
| event_type | VARCHAR | Event type |
| service | VARCHAR | Source service |
| timestamp | TIMESTAMP | Event timestamp |
| trace_id | VARCHAR | Distributed trace ID |
| tenant_id | VARCHAR | Tenant identifier |
| attributes | JSON | Full attribute payload |
| ingested_at | TIMESTAMP | Collector ingestion time |

Column types are configurable via `duckdb.column_types` for custom schemas.

### Retention

DuckDB retention is managed by the `storage.duckdb.retention_days` config. Events older than the retention period are purged on a configurable schedule.

### Collector-Owned Storage

The collector owns all sink implementations. SDKs do not connect to databases directly. The collector imports `internal/event` for sink interfaces and `internal/storage` for DuckDB operations. There is no dependency on any LOZA SDK module.
