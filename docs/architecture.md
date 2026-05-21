# LOXA Architecture Guide

## System Overview

LOXA is a five-tier event observability system designed for high-volume, reliable event capture and analysis across distributed systems.

```
┌─────────────────────────────────────────────────────────────────┐
│  Application Layer (SDKs)                                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                       │
│  │ Go SDK   │  │ Py SDK   │  │ RS SDK   │ (emit events)         │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘                       │
└───────┼──────────────┼──────────────┼────────────────────────────┘
        │              │              │
        └──────────────┼──────────────┘
                       ↓
┌─────────────────────────────────────────────────────────────────┐
│  HTTP Ingest (POST /ingest)                                     │
│  ┌─────────────────────────────────────────────────────────────┐
│  │ Request Parsing & Validation                                │
│  │ - JSON array / wrapped envelope / NDJSON                    │
│  │ - Content-Encoding (gzip) decompression                    │
│  │ - Schema validation (strict/warn/allow)                    │
│  └─────────────────────────────────────────────────────────────┘
└────────────────────┬────────────────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────────────────────────┐
│  Processing Pipeline                                             │
│  ┌──────────────┬──────────────┬──────────────┐                │
│  │ Schema       │ Identity     │ Privacy      │                │
│  │ Validation   │ Resolution   │ Redaction    │                │
│  ├──────────────┼──────────────┼──────────────┤                │
│  │ Deduplication│ Cardinality  │ Size         │                │
│  │              │ Limiting     │ Limiting     │                │
│  └──────────────┴──────────────┴──────────────┘                │
└────────────────────┬────────────────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────────────────────────┐
│  Delivery Strategy Selection                                     │
│  ┌──────────────┬──────────────┬──────────────┐                │
│  │ Direct Mode  │ Queue Mode   │ Spool Mode   │                │
│  │ (DuckDB)     │ (Kafka)      │ (Local FS)   │                │
│  └──────┬───────┴──────┬───────┴──────┬───────┘                │
└─────────┼──────────────┼──────────────┼──────────────────────────┘
          ↓              ↓              ↓
┌─────────────────┐ ┌──────────────┐ ┌──────────────────┐
│ Primary Sink    │ │ Queue (Kafka)│ │ Spool Buffer     │
│ - DuckDB        │ │              │ │ (local durability)
│ - Kafka         │ │ (at-least-   │ │                  │
│                 │ │  once)       │ │ → Worker         │
└────────┬────────┘ └──────┬───────┘ └──────┬───────────┘
         ↓                 ↓                 ↓
   ┌─────────────────────────────────┐
   │ Fanout Delivery (if configured) │
   │ ┌──────┬──────┬──────┬──────┐   │
   │ │Loki  │Click │Postgr│S3/GCS│  │
   │ └──────┴──────┴──────┴──────┘   │
   │ (with delivery policy)           │
   │ - require_primary                │
   │ - require_all                    │
   │ - best_effort                    │
   └─────────────────────────────────┘
         ↓
┌─────────────────────────────────────────────────────────────────┐
│  Control Plane                                                   │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐     │
│  │ /status     │ /sinks      │ /query      │ /dlq        │     │
│  │ Health/     │ Sink list   │ SQL query   │ Dead Letter │     │
│  │ readiness   │             │ interface   │ Queue mgmt  │     │
│  └─────────────┴─────────────┴─────────────┴─────────────┘     │
└─────────────────────────────────────────────────────────────────┘
```

## Component Details

### 1. Application SDKs (sdks/go, sdks/py, sdks/rs, sdks/js)

**Responsibilities:**
- Provide simple, intuitive API for creating and emitting events
- Handle local batching and buffering
- Implement automatic retry with exponential backoff
- Support custom schemas and validation
- Integrate with popular frameworks (HTTP, gRPC, messaging)

**Key Features:**
- **Lifecycle Management**: Create → Active → Closed states
- **Config Hierarchy**: Environment → File → Code precedence
- **Transport Options**: Direct HTTP, batch HTTP, queue-based
- **Middleware**: Auto-instrumentation for HTTP/gRPC handlers
- **Sink Abstraction**: Pluggable sink implementations

### 2. HTTP Ingest Endpoint

**Request Processing:**
```
POST /ingest
Content-Type: application/json
Content-Encoding: gzip (optional)

Body Format Options:
1. JSON Array: [{"event": "a"}, {"event": "b"}]
2. Wrapped Envelope: {"events": [{"event": "a"}]}
3. NDJSON: {"event": "a"}\n{"event": "b"}
```

**Response Codes:**
- `202 Accepted`: Event(s) accepted for processing
- `400 Bad Request`: Malformed request or validation error
- `401 Unauthorized`: Missing/invalid API key
- `413 Payload Too Large`: Request exceeds max_body_bytes
- `429 Too Many Requests`: Rate limit exceeded
- `503 Service Unavailable`: Collector unhealthy (disk full, etc.)

### 3. Processing Pipeline

The ingest processor performs sequential transformations:

1. **Schema Validation**
   - Mode: strict (reject), warn (log), allow (pass)
   - Checks required fields against registry
   - Validates event_version and schema_version

2. **Identity Resolution**
   - Extract tenant_id from: auth header → payload → config default
   - Set service/version from: payload → config default
   - Enforce auth_identity_wins if configured

3. **Privacy Enforcement**
   - Apply blocklist/allowlist redaction rules
   - Scan for secrets (passwords, tokens, keys)
   - Emergency redaction of entire event if mode=enforce

4. **Deduplication**
   - Hash event by configured key + time window
   - Drop duplicates within window (prevent replay)

5. **Limits Enforcement**
   - Check event size, attribute count, attribute depth
   - Truncate oversized string attributes
   - Drop if exceeds max_event_bytes

### 4. Delivery Modes

#### Direct Mode (DuckDB)
- Synchronous write to DuckDB
- Lowest latency (~10ms p99)
- Best for <50k events/sec

```
Event → Validate → Write to DuckDB → Return 202
```

#### Queue Mode (Kafka)
- Asynchronous write to Kafka queue
- Higher throughput (100k+ events/sec)
- Separate worker process consumes and writes

```
Event → Validate → Write to Kafka → Return 202
                        ↓
                  Worker Process
                        ↓
                  Write to DuckDB
```

#### Spool Mode (Local Durability)
- Synchronous write to local spool file
- On-demand replay when sink recovers
- Best for distributed resilience

```
Event → Validate → Write to Spool → Async Write to DuckDB
```

### 5. Fanout Delivery

When secondary sinks are configured:

```
Primary Sink Write Success
         ↓
   Fanout Logic Applied
   ├─ require_primary: Continue (primary succeeded)
   ├─ require_all: Try secondary, fail if secondary fails
   └─ best_effort: Try secondary, ignore failures
         ↓
   Optional Fallback Chain
   ├─ If primary fails → try secondary
   ├─ If secondary fails → try fallback
   └─ If all fail → write to DLQ
```

### 6. Dead Letter Queue (DLQ)

Undeliverable events are captured with context:
- Raw event payload
- Failure reason (parsing error, sink timeout, etc.)
- Timestamp and retry count
- Original destination sink

DLQ is queryable and replayable:
```bash
loxa dlq list              # Show DLQ entries
loxa dlq get <id>          # Get DLQ entry details
loxa dlq replay <id>       # Replay single entry
loxa dlq replay --all      # Replay all DLQ entries
```

### 7. Control Plane

**Authentication:** All endpoints require API key (if auth enabled)

**Key Endpoints:**
- `GET /status` - Collector health (ready, readiness probes)
- `GET /sinks` - List configured sinks and their status
- `POST /query` - Execute SQL query against events
- `GET /tail` - Stream events in real-time (WebSocket)
- `GET /dlq` - List DLQ entries
- `POST /v1/dlq/{id}/replay` - Replay specific DLQ entry
- `GET /metrics` - Prometheus metrics (if enabled)

### 8. Data Storage

**DuckDB Table Schema:**
```sql
CREATE TABLE events (
  event_id TEXT,
  event TEXT,
  service TEXT,
  tenant_id TEXT,
  timestamp TIMESTAMP,
  created_at TIMESTAMP,
  attributes JSON,
  raw TEXT  -- Full event JSON (optional)
);
```

**Retention Policies:**
- **Age-based**: Delete events older than N days
- **Size-based**: Trim table to max size, keeping newest events
- Runs daily on configured schedule

### 9. Monitoring & Observability

**Metrics (Prometheus):**
- `loxa_collector_ingest_total` - Total events ingested
- `loxa_collector_ingest_bytes` - Total bytes ingested
- `loxa_collector_ingest_errors` - Ingest failures
- `loxa_collector_sink_write_total` - Sink writes
- `loxa_collector_sink_write_errors` - Sink write failures
- `loxa_collector_spool_bytes` - Current spool size
- `loxa_collector_ready` - Readiness indicator (1=ready, 0=not ready)

**Audit Logging:**
```json
{
  "level": "warn",
  "message": "collector_auth_failed",
  "method": "POST",
  "path": "/ingest",
  "remote_addr": "192.0.2.1:54321",
  "timestamp": "2026-05-14T08:15:42Z"
}
```

## Deployment Architectures

### Development (Single Instance)

```
┌─────────────────────┐
│ Docker Compose      │
├─────────────────────┤
│ Collector (HTTP)    │
│ DuckDB (embedded)   │
│ CLI (local)         │
└─────────────────────┘
```

### Production (HA)

```
       ┌─────────────────────────────────┐
       │ Load Balancer (HTTP + SSL)      │
       └──────┬──────────────────────────┘
              │
    ┌─────────┼─────────┐
    ↓         ↓         ↓
┌─────────┐ ┌─────────┐ ┌─────────┐
│Collector│ │Collector│ │Collector│  (Kubernetes replicas=3)
│Instance1│ │Instance2│ │Instance3│
└────┬────┘ └────┬────┘ └────┬────┘
     │           │           │
     └─────────┬─────────────┘
               ↓
       ┌──────────────────┐
       │ Shared DuckDB    │
       │ (Cloud storage)  │
       └──────────────────┘
               ↓
    ┌──────────┬──────────┐
    ↓          ↓          ↓
 [DLQ] [Query] [Tail]  (read-only replicas)
```

### Distributed (Kafka + Workers)

```
┌────────────────────────────────────────────┐
│ SDKs                                       │
└──────────┬─────────────────────────────────┘
           ↓
┌────────────────────────────────────────────┐
│ Collector Cluster (queue mode → Kafka)    │
│ ┌─────────┬─────────┬─────────┐           │
│ │ C1      │ C2      │ C3      │           │
│ └─────────┴─────────┴─────────┘           │
└────────────────────────────────────────────┘
           ↓
┌────────────────────────────────────────────┐
│ Kafka Topic: loxa-events (3 partitions)    │
└────────────────────────────────────────────┘
           ↓
┌────────────────────────────────────────────┐
│ Worker Cluster (loxa-worker)               │
│ ┌─────────┬─────────┬─────────┐           │
│ │ W1      │ W2      │ W3      │           │
│ │ Consumer│ Consumer│ Consumer│           │
│ └─────────┴─────────┴─────────┘           │
└────────────────────────────────────────────┘
           ↓
┌────────────────────────────────────────────┐
│ DuckDB (or other sinks) + Fanout           │
└────────────────────────────────────────────┘
```

## Data Flow Example

Event emission to query (happy path):

```
1. Application emits event:
   sdk.emit({event: "user.signup", user_id: "u123"})

2. SDK batches and sends HTTP POST:
   POST /ingest
   [{event: "user.signup", user_id: "u123", ...}]

3. Collector receives and parses:
   - Parse JSON array
   - Extract 2 events

4. Processor validates each event:
   - Check schema (user_id required? ✓)
   - Resolve identity (tenant_id)
   - Apply privacy redaction
   - Check size limits

5. Select delivery mode:
   - Direct mode: sync write to DuckDB
   - Queue mode: async write to Kafka
   - Spool mode: local durability

6. Collector returns 202 Accepted to SDK

7. Event available for query:
   loxa query --sql "SELECT * FROM events WHERE event='user.signup'"

8. Query hits DuckDB (or read replica)
   Results returned to CLI
```

## Error Handling

**Transient Errors (retry-able):**
- Sink timeout
- Temporary network failure
- Database locked (DuckDB)

**Permanent Errors (go to DLQ):**
- Schema validation failed (strict mode)
- Oversized event
- Unknown sink
- Corrupted event data

**Circuit Breaker Logic:**
```
After N failures → circuit opens
↓
Return 503 Service Unavailable to clients
↓
Requests go to spool (if enabled) or rejected
↓
Circuit state checks every T seconds
↓
On sink recovery → circuit closes
```

## Security & Multi-Tenancy

**API Key Authentication:**
- Configured in auth.header + auth.value_env
- Validated on every ingest and control request
- Constant-time comparison (timing attack resistant)
- Audit logged on failure

**Tenant Isolation:**
- tenant_id extracted from: payload → auth header → config
- Query filters automatically scoped (read-only)
- Separate workspace directories (spool, DLQ, etc.)

**Privacy Modes:**
- **enforce**: Redact on any pattern match, abort if error
- **warn**: Redact and log, continue processing
- **allow**: Log only, pass event through unmodified

---

See [Deployment Guides](./deployment.md) for platform-specific architectures.
