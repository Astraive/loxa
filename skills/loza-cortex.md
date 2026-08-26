---
name: loza-cortex
description: Help release users install, configure, secure, and operate LOZA Cortex for incident intelligence: Collector ingestion/bridging, reconstruction, causal context, similar incidents, service and incident graphs, GraphQL/WebSocket access, remediation records, feedback, storage, authentication, rate limits, and troubleshooting. Do not turn usage into repository or CI development work.
compatibility: Requires a released LOZA Cortex image or binary, a reachable compatible Collector, and the configured Cortex storage backend. Verify all optional surfaces against the installed release.
---

# LOZA Cortex release-user guide

Cortex is LOZA's control plane. It consumes Collector events, stores incident memory, correlates events, reconstructs causal context, builds service/incident graphs, matches similar incidents, and records remediation outcomes. It is not a replacement for the Collector's durable event ingestion/query path.

## Installation and topology

Use a pinned matching release from <https://github.com/Astraive/loza/releases> or the pinned image at <https://hub.docker.com/r/astraive/loza-cortex>. Keep Cortex and Collector versions compatible. The checked-in default HTTP listener is `0.0.0.0:9312`; bind to a private interface or protect it with network policy when appropriate.

Recommended topology:

```text
SDKs -> Collector (:9308) -> durable event store
                         \-> optional Cortex bridge/pull
Cortex (:9312) -> incident storage + graph/matcher/reconstructor
Operators -> authenticated Cortex API/CLI
```

Persist the Cortex database and Collector cursor path. A Cortex restart without its cursor/storage can lose continuity or rebuild incomplete context. Verify Collector health independently; Cortex liveness does not prove Collector connectivity.

## Configuration map

Start with the installed defaults and override secrets through the release's environment/secret mechanism. The checked-in defaults contain:

| Section | Key fields | Purpose |
|---|---|---|
| `server` | `host`, `port`, read/write/shutdown timeouts, `max_body_bytes` | HTTP listener and bounds |
| `storage` | `backend`, DuckDB path, PostgreSQL host/port/database/user/password, `ssl_mode`, pool sizes/lifetime, similarity threshold/weights | Incident memory and similarity persistence |
| `matcher` | mode, parallelism, cache size/TTL, thresholds, top-k, feature/symptom/temporal/topology weights | Similar-incident ranking |
| `authentication` | `enabled`, runtime `api_keys` | API authentication |
| `rate_limit` | enabled, per-key RPM, per-IP RPM | Request throttling |
| `tls` | enabled, cert/key, minimum version, mutual TLS/client CA | Transport security |
| `pii_redaction` | enabled, mode | Incident-data redaction |
| `logging` | level, format | Structured/text diagnostics |
| `collector` | mode, URL/DSN, collector, environment, service, source-of-truth, API key, TLS CA, timeout, max response, poll/tail settings, query/raw/timestamp columns, cursor path | Collector integration |
| `correlation` | enabled, analysis interval, co-occurrence/deployment windows, minimum count | Background correlation |
| `reconstructor` | fast/deep max depth/events/time window, confidence weights, graph depth, explanation limits, signal scores | Incident context generation |
| `memory` | decay period/rate, archive threshold, merge tolerance, vector alpha | Incident memory lifecycle |
| `learner` | learning rate and feature weight bounds | Feedback-driven ranking updates |
| `salience` | learning rate/default score | Event/signature salience |
| `eventbus` | subscriber buffer size | Internal event delivery |

The checked-in default storage backend is PostgreSQL with TLS mode `require`; a DuckDB path exists in configuration for deployments that select it. Do not use a blank database password or disable certificate verification across a trust boundary. Set `tls.min_version` to at least 1.2 and use mutual TLS only with a managed client CA when required.

The Collector integration supports pull/tail settings, collector name, environment, API key, timeout, bounded response size, polling interval, batch/tail sizes, reconnect backoff, query table/columns, and a persistent cursor. Scope the API key to the intended collector/environment and use a dedicated Cortex service identity.

## Authentication and exposure

When `authentication.enabled` is true, protected routes use the configured API key as:

```text
Authorization: Bearer <cortex-api-key>
```

The implemented HTTP server leaves `/healthz`, `/readyz`, `/version`, and `/metrics` public. Ingest and feedback require the writer role; reconstruction, graph, GraphQL, and WebSocket require the reader role. Rate limiting can apply per API key and per source IP. Configure trusted proxies correctly before trusting forwarded client IPs.

Do not log bearer tokens, database passwords, Collector API keys, raw event bodies, or incident attributes containing secrets. Put credentials in a secret manager, not YAML, shell history, source control, or image layers.

## Start and verify

```bash
# Run the released binary/container with its release config.
curl http://127.0.0.1:9312/healthz
curl http://127.0.0.1:9312/readyz
curl http://127.0.0.1:9312/version
curl http://127.0.0.1:9312/metrics
```

`/healthz` is liveness. `/readyz` returns JSON status/checks and can be unavailable when graph/storage initialization is incomplete. `/version` returns JSON. `/metrics` is Prometheus exposition. Confirm the Collector's `/health`, `/readyz`, `/version`, and a bounded authenticated read before diagnosing Cortex intelligence.

## HTTP API reference

All paths below are implemented by the checked-in Cortex HTTP router. Protected routes require the reader/writer role described above. Body size is bounded by `server.max_body_bytes` (10 MiB by default in the checked-in config).

### Ingest

```text
POST /events                 writer
POST /events/batch           writer
POST /events/jsonl           writer
```

`/events` accepts one `models.Event`; `/events/batch` accepts `{ "events": [...] }`; `/events/jsonl` accepts newline-delimited JSON. Successful ingestion returns `202` with `{"status":"accepted"}`. Invalid event/body returns `400`; processing failures return `500`. Events require an ID, timestamp, service, and valid kind; timestamps too far in the future and invalid provenance are rejected by the model validator.

A minimal event:

```json
{
  "id": "evt-001",
  "timestamp": "2026-08-26T12:00:00Z",
  "service": "checkout",
  "event": "checkout.request",
  "kind": "http",
  "outcome": "error",
  "trace_id": "trace-001",
  "incident_id": "inc-001",
  "attrs": {"status_code": 500}
}
```

The model also supports `event_id`, environment/release/schema versions, level/duration, span/request/trace flags, HTTP/user/tenant contexts, structured attributes, error data, lifecycle checkpoints/processes/groups/timers/links, SDK metadata, raw payload, provenance, and incident ID. Use the fields accepted by the installed release's model/schema.

### Reconstruction

```text
POST /reconstruct
POST /incidents/{incident_id}/reconstruct
```

The body form is `{ "incident_id": "inc-001", "mode": "fast" }`; the path form takes `?mode=fast|deep`. Any mode other than the exact `deep` value follows the fast path in the checked-in handler, so pass `fast` or `deep` explicitly. A successful response is an `IncidentContext` containing incident ID/timestamp, causal chain, symptoms, similar incidents, suggested actions/remediations, confidence, and explanation fields where populated. Missing incidents return `404`.

Fast/deep limits and confidence weights are configured under `reconstructor`. Treat confidence as evidence ranking, not authorization to execute a remediation.

### Graphs

```text
GET /graph/service/{service}?depth=3       reader
GET /graph/incident/{incident_id}?depth=3   reader
```

Depth defaults to 3 and is validated from 1 through 100. Invalid depth returns `400`; missing graph data returns `404`. Responses contain graph nodes and edges; service graphs describe neighborhood topology, incident graphs describe event relationships.

### Remediation and feedback

```text
POST /feedback/remediation   writer
POST /feedback/incident      writer
```

The API accepts the installed model fields. The checked-in models use:

- `Remediation`: `remediation_id`, `incident_id`, optional `signature_id`, `action`, timestamp, optional operator, and attributes.
- `RemediationFeedback`: feedback/remediation/incident IDs, `outcome_code` (HTTP-style 100–599 and required by validation), derived `outcome_category`, `time_to_resolve_seconds`, timestamp, and optional notes.

The checked-in model is not the same as a generic `{outcome:"resolved", confidence:0.9}` example. Confirm payload fields and validation from the installed release before posting feedback. A successful write returns `{"status":"recorded"}`. Record only truthful outcomes after the operator confirms the incident/action relationship; false feedback changes future ranking.

### GraphQL and WebSocket

```text
POST /graphql   reader
GET  /ws        reader
```

GraphQL supports release-provided incident/event/signature/graph/remediation queries. WebSocket clients subscribe to the live event stream using the protocol and message shape documented by the installed release. Do not expose either surface without authentication and connection/resource limits.

## CLI operations

The CLI uses `LOZA_CORTEX_URL` or the configured Cortex URL, defaulting to `http://localhost:9312` in the checked-in command implementation:

```text
loza cortex run
loza cortex ingest --file events.ndjson
loza cortex ingest --url https://trusted.example/events.json
loza cortex reconstruct --incident INC_ID [--mode fast|deep]
loza cortex similar --incident INC_ID [--limit N]
loza cortex remediation --incident INC_ID --action ACTION [--operator NAME]
loza cortex feedback --remediation REM_ID --outcome OUTCOME [--time-to-resolve SECONDS]
loza cortex graph --service SERVICE [--depth N]
loza cortex graph --incident INC_ID [--depth N]
loza incident INC_ID [--mode fast|deep] [--depth N] [--format json|text]
loza graph service|incident NAME_OR_ID [--depth N] [--format json|ascii]
```

`loza cortex run` and repository-path behavior are local/source operations; use a released process in a customer deployment. `loza cortex ingest --url` fetches the supplied URL; use only trusted HTTPS sources and validate the response. `loza cortex replay` exists in the checked-in CLI but returns an explicit not-implemented error; use `cortex ingest` or the HTTP API to replay supported event data instead.

## Operational interpretation

Cortex's causal chain and suggested actions are derived context. Before taking action, validate:

- incident and service identity,
- event time window and release/deployment changes,
- causal evidence and confidence,
- blast radius and current system state,
- action identifier and approval policy.

Keep Collector redaction enabled before forwarding sensitive events. Keep incident feedback free of secrets and unnecessary PII. Set retention/decay/archive behavior deliberately; do not assume memory decay is equivalent to deletion compliance.

## Troubleshooting order

1. Confirm Cortex/Collector/image/config versions and compatibility.
2. Check Cortex `/healthz`, `/readyz`, `/version`, storage connectivity, and writable cursor path.
3. Check Collector health/readiness, URL/DSN, API key, collector scope, environment, and TLS CA.
4. Confirm the Cortex service can receive or pull one small valid event.
5. Reconstruct one known incident in `fast` mode, then inspect graph and similarity limits.
6. Check rate-limit counters, body-size limits, database pool exhaustion, cursor advancement, and metrics.
7. Inspect structured logs without tokens or raw event bodies.
8. Restart after config changes unless the installed release explicitly documents live reload; repeat liveness, readiness, ingestion, reconstruction, and graph checks.
