# CORTEX Event Model

## Canonical Cortex Event

The canonical Cortex event is the normalized form that all sources (Loxa, OTel, raw logs) are converted to before storage and processing.

### Schema Location
`schemas/json/cortex-event.schema.json`

### Required Fields

```
event_id        - UUID/unique identifier for this event
timestamp       - RFC3339 timestamp when event occurred
service         - String or object {name, version, environment}
event           - String: what happened (e.g., "user.login", "http.request.start")
kind            - Enum: event, http, job, queue, cli, cron, log, checkpoint, deploy, metric, trace, topology, incident_signal, remediation, loxa_event, otel_log, otel_span, collector_event
```

### Core Optional Fields

```
schema_version  - Version of event schema (e.g., "loxa/v1")
event_version   - Version of this specific event definition
level           - Enum: debug, info, warning, error, critical
outcome         - Enum: success, failure, partial, unknown
```

### Tracing Fields

```
trace_id        - Distributed trace ID
span_id         - Distributed span ID (for otel_span, http)
parent_span_id  - Parent span ID in trace tree
request_id      - Request ID or transaction ID
```

### Incident Fields

```
incident_id     - Links event to incident (populated by Cortex detector)
```

### Provenance

```
provenance      - Enum: loxa (from Loxa SDK), collector (forwarded by collector), 
                         otlp (OpenTelemetry Protocol), jsonl (raw import), 
                         manual (API input), replay (historical replay)
```

### HTTP-Specific Fields (when kind=http)

```
method          - HTTP method
path            - URL path
route           - Matched route pattern
status_code     - HTTP response code
```

### Raw Payload

```
raw             - Object: arbitrary additional data from source
                  - OTel span attributes
                  - Loxa event attrs
                  - Log message body
                  - Structured fields
```

### Metadata

```
created_at      - When Cortex ingested this event (different from timestamp)
```

## Event Validation Modes

### Strict Mode
- Rejects unknown top-level fields
- Rejects aliases (e.g., event_type)
- Enforces all required fields
- Enforces enum values
- Used for: Loxa collector events, API events

### Loose Mode
- Accepts unknown fields in raw{}
- Accepts and normalizes aliases
- Enforces required canonical fields after normalization
- Used for: OTel conversion, raw log ingest, manual API

## Conversion Examples

### From Loxa Event
```
Loxa Input:
{
  schema_version: "loxa/v1",
  event_id: "evt_123",
  timestamp: "2026-05-18T01:44:00Z",
  service: "api-server",
  event: "user.login",
  kind: "event",
  level: "info",
  outcome: "success",
  attrs: {user_id: "usr_456"}
}

→ Cortex Event:
{
  schema_version: "loxa/v1",
  event_id: "evt_123",
  timestamp: "2026-05-18T01:44:00Z",
  service: "api-server",
  event: "user.login",
  kind: "event",
  level: "info",
  outcome: "success",
  provenance: "loxa",
  raw: {attrs: {user_id: "usr_456"}}
}
```

### From OTel Span
```
OTel Input:
{
  trace_id: "abc123",
  span_id: "def456",
  name: "http.request",
  start_time: 1234567890,
  attributes: {http.method: "POST", http.url: "..."}
}

→ Cortex Event:
{
  event_id: "evt_<generated>",
  timestamp: "2026-05-18T01:44:00.000Z",
  service: "<from resource>",
  event: "http.request",
  kind: "http",
  level: "info",
  outcome: "success",  // inferred
  trace_id: "abc123",
  span_id: "def456",
  provenance: "otlp",
  raw: {otel_attributes: {...}}
}
```

### From Raw Log
```
Raw Log:
{
  message: "login successful",
  user_id: "usr_456",
  service: "api-server",
  timestamp: "2026-05-18T01:44:00Z"
}

→ Cortex Event:
{
  event_id: "evt_<generated>",
  timestamp: "2026-05-18T01:44:00Z",
  service: "api-server",
  event: "log.message",
  kind: "log",
  level: "info",  // inferred
  outcome: "success",  // default
  provenance: "jsonl",
  raw: {message: "login successful", user_id: "usr_456"}
}
```

## Important Invariants

1. **Idempotency**: Same event (same event_id) ingested twice produces no duplicate storage
2. **Service Canonicalization**: service field always normalized to lowercase
3. **Timestamp Ordering**: Events ordered by timestamp for timeline reconstruction
4. **Trace Linking**: All events with same trace_id linked in graph
5. **Incident Linking**: All events with same incident_id linked in graph
