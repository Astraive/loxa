# LOXA SDK Conformance Contract (v0.0.2)

## Purpose

This document defines the canonical behavior all SDKs (Go, Python, Rust, JavaScript) MUST implement for the v0.0.2 product-parity contract and beyond.

Stable-v1 scope is intentionally **collector-first**:

- SDKs emit canonical events
- SDKs deliver to the collector over the documented ingest contract
- collector-owned sinks and storage remain out of scope for SDK parity

---

## 0. Cross-Language Client Parity

Every SDK MUST expose the same mental model:

- Default/global client: `loxa.<method>()` or package/module equivalents.
- Cross-language factory: `createLoxa` / `create_loxa` / `CreateLoxa`.
- Idiomatic constructor: optional language-native constructor (`Loxa`, `New`, `Loxa::new`).
- Same-config alias: `alias(name)` returns an immutable child client.

Alias semantics are fixed in v0.0.2:

- `alias(name)` MUST preserve parent config, including `service`, endpoint, auth, sampling, redaction, and sink configuration.
- `alias(name)` MUST NOT mutate the parent/default client.
- Events emitted by an alias MUST include custom metadata key `loxa.alias` with the alias name.

## 1. Event Lifecycle

### 1.1 StartEvent → Append/Enrich → Primitives → Finish → Emit Flow
- **StartEvent(ctx, Params)** creates a new event context
  - MUST set canonical fields: service, event, kind, level (default: info)
  - MUST initialize state machine to STARTED
  - MUST generate event_id (UUIDv7 or equivalent)
  - MUST capture timestamp at creation time (immutable)
  - MUST capture duration_start for later duration_ms calculation

- **Append/Enrich(ctx, attrs...)** adds business attributes to `attrs` or canonical nested objects.
- **Checkpoint(ctx, name, attrs...)** records a breadcrumb with offset time and no duration.
- **Process(ctx, name, attrs...)** records an ordered numbered step with duration.
- **Group(ctx, name, attrs...)** records a named phase/block with duration.
- **Timer(ctx, name, attrs...)** records an independent latency measurement.
- **Stopwatch()** records local elapsed time before optional attachment to an event.

- **Finish(ctx, outcome, attrs...)** transitions event to FINISHED
  - MUST set outcome field (minimum stable outcomes: success, error, partial, abandoned, retried)
  - MUST calculate duration_ms = now - duration_start
  - MUST accept optional attrs to merge into event
  - MUST NOT allow state transitions after Finish (idempotent)
  - MUST validate outcome is canonical (reject unknown outcomes)

- **Emit(ctx)** serializes and delivers event
  - MUST serialize to JSON per event.schema.json
  - MUST validate against strict schema if strictMode enabled
  - MUST call StatsHandler hooks: OnEmit, OnDrop, OnDeliveryFailed
  - MUST return delivery status to caller
  - MUST support idempotent emit (same event_id, same outcome, retry-safe)

### 1.2 State Machine
```
INIT → STARTED → FINISHED → EMITTED
         ↓         ↓
       INVALID   DROPPED
                 EMIT_FAILED
                 SPOOLED
                 DLQ_WRITTEN
```
- INIT: Event created but not started
- STARTED: Between StartEvent and Finish
- FINISHED: Finish called, ready for Emit
- EMITTED: Successfully sent to sink
- DROPPED: Dropped by SDK or collector (sampler, validation, capacity)
- EMIT_FAILED: Sink/transport delivery failed
- SPOOLED: Event was durably spooled
- DLQ_WRITTEN: Event was written to DLQ
- INVALID: Failed state transition (e.g., Emit before Finish)

---

## 2. Canonical Fields

### 2.1 Reserved Top-Level Fields (READ-ONLY in attrs)
These MUST NOT be overridden by custom attributes:
- schema_version (always "v1")
- event_version (always "v1")
- timestamp (ISO 8601, immutable at creation)
- event_id (UUIDv7 or equivalent)
- service (from Config or Params)
- event (event name, from Params)
- kind (http, async, batch, cron, etc.)
- level (info, warn, error, debug, trace)
- outcome (success, error, partial, abandoned, retried)
- duration_ms (calculated after Finish)
- request_id (trace correlation)
- trace_id (distributed trace ID, W3C compatible)
- span_id (span identifier, W3C compatible)
- http (nested object for HTTP events)
- user (nested object for user identity)
- tenant (nested object for multi-tenancy)
- attrs (business-specific attributes)
- checkpoints (breadcrumb timeline)
- processes (ordered numbered steps)
- groups (named phases)
- timers (latency measurements)
- links (cross-event/trace/job relationships)
- sdk (SDK metadata)
- collector (collector metadata)
- sampling (sampling decision metadata)
- redaction (redaction policy metadata)

### 2.2 Duplicate Field Policy
- SDKs MAY warn (log) if custom attrs collide with canonical names
- SDKs MUST NOT silently override canonical fields with custom attrs
- Strict mode MUST reject events with duplicate fields (default: warn)
- Canonical names take precedence in all SDK implementations

### 2.3 HTTP Event Canonical Fields
When kind="http", http object MUST contain:
- method: HTTP method (GET, POST, PUT, DELETE, etc.)
- path: Request path
- route: Route pattern (e.g., /users/:id)
- status_code: HTTP response status
- user_agent (optional): User-Agent header
- host (optional): Host header

---

## 3. Delivery Semantics

### 3.1 Sink Delivery Strategy
- **Primary Sink**: First configured sink
- **Fallback Sink**: Used if primary fails
- **Batch Sink**: HTTPBatchSink collects events before sending

- **OnEmit Success**: Sink.WriteEvent returned nil error
  - MUST call StatsHandler.OnEmit(event)
  - MUST increment events_emitted_total{status="success"}
  - MUST return err=nil to caller

- **OnEmit Failure**: Sink.WriteEvent returned error
  - IF Fallback configured: try fallback sink
    - IF fallback succeeds: call OnEmit success
    - IF fallback fails: call OnDeliveryFailed, increment events_emitted_total{status="failure"}
  - IF No Fallback: call OnDeliveryFailed, increment events_emitted_total{status="failure"}
  - MUST NOT drop event silently (caller must decide)

- **OnDeliveryFailed**: Explicit tracking of failed delivery
  - MUST call StatsHandler.OnDeliveryFailed(event, err)
  - MUST increment retry_total if SDK retries
  - MUST NOT retry beyond maxRetries configured

### 3.2 At-Most-Once Semantics
- SDK delivers each event AT MOST once to collector
- Collector MAY deduplicate by event_id
- Collector MUST track events in DLQ on sink failure
- Client libraries MUST NOT retry beyond SDK retry policy

---

## 4. Duplicate Field Collision Detection

### 4.1 Detection Algorithm
```
1. Parse event as map[string]interface{}
2. Check if custom attrs contain keys from canonical reserved list
3. If strict_mode == "enforce":
     Return error "duplicate field: X"
4. If strict_mode == "warn":
     Log warning, allow event but mark as potentially problematic
5. If strict_mode == "off":
     Allow silently (not recommended)
```

### 4.2 Test Cases
- ✅ Custom attr "service" when service already canonical → WARN/ERROR
- ✅ Custom attr "trace_id" override attempt → WARN/ERROR
- ✅ Custom attr "nested.field" (non-conflicting) → ALLOW
- ✅ Custom attr "custom_service_name" (non-conflicting) → ALLOW

---

## 5. Sampling Behavior

### 5.1 Sampling Policies
- **SampleAll()**: Sample rate 100% (default for development)
- **SampleNone()**: Sample rate 0% (skip all events)
- **SampleRate(0.1)**: Sample 10% of events

### 5.2 Sampling Lifecycle
1. StartEvent: Check if event should be sampled
2. If NOT sampled: Set sampled=false in attrs
3. Emit: Only serialize and deliver if sampled=true (or collector will sample)
4. StatsHandler.OnDrop("sampled") if sampled out

### 5.3 W3C Trace Context Integration
- If trace_id present: respect sampled bit from W3C Trace Context header
- If no trace_id: use SDK sampler
- Propagate sampled decision to downstream services

---

## 6. Panic Recovery

### 6.1 SDK Panic Safety
- StartEvent MUST NOT panic (if config invalid, return configured error sink)
- Finish MUST NOT panic (validate outcome, log unknown outcomes)
- Emit MUST NOT panic (catch JSON encoding errors, fallback to error sink)
- Configuration parsing MUST NOT panic (return error to caller)

### 6.2 Error Handling
- All errors MUST be returned to caller via (error) return value
- StatsHandler.OnError(err) MUST be called for async errors
- FallbackSink MUST be safe for error cases

---

## 7. Configuration Precedence

### 7.1 Priority Order (highest to lowest)
1. Environment variables (LOXA_*)
2. SDK Config struct fields
3. Default configuration file (~/.loxa/config.yaml)
4. Built-in defaults

### 7.2 Canonical Config Fields
All SDKs MUST support:
- service (required)
- endpoint (required for collector sink)
- apiKey (required for production)
- strictMode (warn | enforce | off)
- sampleRate (0.0 to 1.0)
- maxRetries (default: 3)
- timeoutMs (default: 5000)

---

## 8. Metrics and Stats

### 8.1 Required Metrics (Prometheus format)
- events_created_total
- events_finished_total
- events_emitted_total{status="success|failure"}
- events_dropped_total{reason="sampled|validation|capacity"}
- emit_duration_seconds (histogram)
- buffer_size (gauge)
- buffer_capacity (gauge)
- retry_total{attempt="1|2|3"}
- backpressure_total (429/503 responses)

### 8.2 StatsHandler Interface
```go
type StatsHandler interface {
  OnEmit(event *Event)           // Successful delivery
  OnDrop(reason string)           // Event dropped
  OnError(err error)              // Async error
  OnDeliveryFailed(event, error)  // Explicit delivery failure
}
```

---

## 9. Testing Requirements

### 9.1 Conformance Test Suite
Every SDK MUST pass:
1. **State Machine Tests**: Verify valid transitions, reject invalid ones
2. **Canonical Field Tests**: Ensure reserved fields not overridable
3. **Duplicate Detection Tests**: With all strictModes (off/warn/enforce)
4. **Sampling Tests**: All policies, W3C context propagation
5. **Delivery Tests**: Primary, fallback, failure scenarios
6. **Metrics Tests**: All metrics exported correctly
7. **Panic Safety Tests**: No panics in error cases
8. **Config Precedence Tests**: All 4 levels override correctly
9. **Integration Tests**: End-to-end with collector

### 9.2 Test Fixtures
All SDKs test against golden fixtures in:
- `spec/examples/golden/valid/`
  - http_success.json
  - error_event.json
  - duplicate_fields.json
  - minimal_event.json
  - trace_context_event.json
  - (more TBD)

---

## 10. Version and Compatibility

### 10.1 Schema Versioning
- schema_version: Always "v1" in v1.x releases
- event_version: Always "v1" in v1.x releases
- FUTURE: May introduce v2 with backward compatibility path

### 10.2 SDK Maturity Status (v0.0.1)
- **Go**: Stable - Full conformance, production-ready
- **Python**: Stable - Full conformance, production-ready
- **Rust**: Stable - Full conformance, production-ready
- **JavaScript**: Stable - Full conformance, production-ready

### 10.3 Breaking Changes
- Changes to canonical field list require major version bump
- Changes to duplicate policy require major version bump
- New optional fields: Minor version bump (backward compatible)

---

## 11. Collector Integration

### 11.1 HTTP Ingest Endpoint
```
POST /v1/events
Content-Type: application/json
Authorization: Bearer <API_KEY>
```
- MUST support JSON batch (array of events)
- MUST support NDJSON (newline-delimited JSON)
- MUST validate against collector schema
- MUST track accept/drop/partial/retryable responses

### 11.2 Ingest Response Contract
```json
{
  "accepted": 100,
  "dropped": 0,
  "partial": 0,
  "retryable": 0,
  "errors": []
}
```
- SDK MUST handle all response codes correctly
- 2xx: All events accepted
- 4xx: Validation errors, don't retry
- 5xx: Server errors, retry with backoff
- 429: Backpressure, respect Retry-After header

### 11.3 Collector-Owned Exclusions

The following capabilities are intentionally collector-owned and MUST NOT be treated as stable cross-language SDK parity requirements:

- Kafka
- DuckDB
- ClickHouse
- Postgres
- Loki
- OTLP fanout
- S3
- GCS

---

## 12. Backward/Forward Compatibility

### 12.1 Unknown Fields
- SDKs MUST accept unknown top-level fields (ignore them)
- Collectors MUST accept unknown custom attrs
- This enables gradual feature rollouts

### 12.2 Optional Fields
- All fields except (service, event, timestamp, outcome) are optional
- SDKs MUST fill sensible defaults
- Collectors MUST handle missing optional fields

---

## 12. Default API Surface

### 12.1 `loxa` Namespace
- All SDKs MUST export a `loxa` object/namespace as the primary API surface
- `loxa.info("msg")` etc. MUST delegate to the global default logger
- `loxa.New(cfg)` / `loxa.new(cfg)` / `loxa::New(cfg)` creates a custom instance

### 12.2 `CreateLoxa()` Factory
- JS, Python, Rust MUST export `CreateLoxa()` / `create_loxa()` as an alias for `New()`/`new()`
- Go does not need this because `loxa.New(cfg)` is already idiomatic
- Both PascalCase and camelCase/snake_case variants MUST be exported

### 12.3 `Alias()` Method
- All SDKs MUST support `Alias("name")` on Logger instances
- MUST create a new Logger with identical config but different `service` field
- MUST NOT mutate the original logger
- MUST be available as a module-level function delegating to `Default().Alias()`

---

## Conformance Checklist

For a release to be marked "stable" (not alpha), SDKs must pass:

- [ ] All state machine tests
- [ ] All canonical field tests
- [ ] All duplicate detection tests (all 3 modes)
- [ ] All sampling tests
- [ ] All delivery scenario tests
- [ ] All metrics tests
- [ ] All panic safety tests
- [ ] All config precedence tests
- [ ] All integration tests with collector
- [ ] All golden fixture tests (valid + invalid)
- [ ] Performance benchmarks meet targets
- [ ] Documentation complete
- [ ] Error messages are helpful and actionable

---

**Last Updated**: May 15, 2026
**Version**: 0.0.1
**Status**: Stable v1
