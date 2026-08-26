---
name: loza-collector
description: Help release users install, configure, secure, and operate the published LOZA Collector: authentication, scoped routes, event ingestion, LQL/query, tailing, durability, DuckDB/Kafka/event buses, sinks, schema governance, DLQ, quarantine, retention, keys, and troubleshooting. Stay on released runtime behavior; do not provide CI or source-development workflows.
compatibility: Requires a published LOZA Collector release, matching Docker image, or released binary and its configuration documentation. Repository paths below describe the checked-in release contract only when the installed release matches it.
---

# LOZA Collector release-user guide

The Collector is LOZA's data plane. It accepts canonical events, applies limits/identity/privacy/schema/dedupe policy, delivers to primary and secondary sinks, persists durable state, serves query and tail operations, and exposes replay/delete/admin controls.

## Installation and process model

Use the matching release artifact:

- Container: pin `astraive/loza:<release>` from <https://hub.docker.com/r/astraive/loza>.
- Host: download the Collector binary and checksum from <https://github.com/Astraive/loza/releases>.
- Development-only local source: `loza collector run` can invoke a configured repository binary; this is not a production installation method.

The default HTTP listener is `:9308`. Publish only the required port. Mount persistent storage for DuckDB, spool, queue, DLQ, quarantine, and export paths. Pass configuration with `LOZA_CONFIG`/`LOZA_CONFIG_FILE` or the release-documented equivalent; never bake secrets into an image layer.

## Secure minimum configuration

Authentication must be enabled for any network-accessible deployment. When enabled, the runtime requires a resolved `auth.server_secret` and at least one configured key/token/legacy credential. Keep these values in a secret manager or environment:

```bash
export COLLECTOR_AUTH_SERVER_SECRET="$(openssl rand -hex 32)"
export COLLECTOR_INGEST_KEY_SECRET="$(openssl rand -hex 24)"
export COLLECTOR_READ_KEY_SECRET="$(openssl rand -hex 24)"
export LOZA_STORAGE_ENCRYPTION_KEY="$(openssl rand -hex 32)"
```

`auth.server_secret` protects key-record hashing; it is not the DuckDB/raw-event encryption key. Use separate credentials for:

- event writes (`events:write`),
- reads/tail/query (`events:read`),
- deletion (`events:delete`),
- schema publication/admin (`schema:write` or `project:admin`), and
- service-to-service bridge access.

A key/token can be constrained by kind/access mode, collector, environments, services, origins, source IPs, payload bytes, request rate, and event rate. Browser/public credentials must have origin restrictions and must not be reused as server credentials. Avoid legacy single-key mode unless the release explicitly requires it.

## Complete configuration map

Start with the release defaults (`loza config print`) and change only required values. The checked-in Collector accepts these functional sections:

| Section | Important fields | Purpose |
|---|---|---|
| `collector`/HTTP | `addr`, `read_header_timeout`, `shutdown_timeout`, `max_body_bytes`, `max_events_per_request`, `ingest_path`, `health_path`, `ready_path`, `metrics_path` | Listener, request bounds, lifecycle, route aliases |
| `server.http` | read/write/idle timeouts, max header/body settings | HTTP server limits |
| `server.grpc` | `enabled`, address, TLS settings | Optional gRPC surface; enable only when documented |
| `server.graphql` | `enabled`, address/path settings | Optional GraphQL surface; protect it |
| `auth` | `enabled`, `server_secret`, `cache_ttl`, `negative_cache_ttl`, `default_collector`, `collectors`, `keys`, `tokens`, `allow_local_dev_keys`, `api_key_header` | Key/token authentication and scope |
| `storage` | `primary`, `encryption_key_env` | Primary backend and raw storage encryption |
| `duckdb` | `path`, `driver`, `table`, `raw_column`, `store_raw`, `checkpoint_on_stop`, `max_open_conns`, `max_idle_conns`, `batch_size`, `flush_interval`, `writer_loop`, `writer_queue_size`, `use_appender`, `write_timeout`, retry settings, checkpoint/export settings, schema/column types | Local durable event storage and export |
| `lql` | `enabled`, binary/protocol/compiler/language, startup/compile timeouts, max concurrency | LQL compiler integration |
| `rate_limit` | `enabled`, `rps`, `burst` | Global request limiting |
| `logging` | `level`, `format` | Structured/text logging |
| `metrics` | `prometheus` | Metrics exposition |
| `reliability` | `mode`, `spool_dir`, `spool_file`, `max_spool_bytes`, `fsync`, `delivery_queue_size`, queue settings | `direct`, `spool`, `queue`, or release-supported `hybrid` delivery |
| `limits` | inflight requests/events, queue bytes, event bytes, attr count/depth, string length | Resource and payload bounds |
| `identity` | `mode`, auth precedence, payload allowance, bound service/version/environment/region/tenant/workspace/org | Tenant/service identity resolution |
| `privacy` | `mode`, collector/emergency redaction, allow/block lists, secret scan, right-to-delete | Redaction and privacy policy |
| `components` | receivers/processors/exporters/extensions | Registered implementations |
| `retry` | enabled, attempts, initial/max backoff, jitter | Sink delivery retry |
| `dead_letter` | enabled, path | Undeliverable event storage |
| `fanout` | named outputs, type, role, backend options, delivery policy, fallback and DLQ policy | Primary/secondary/fallback sink routing |
| `dedupe` | enabled, key, window, backend, Redis settings | Duplicate suppression |
| `schema_governance` | mode, schema/event version, registry file/entries, quarantine path | Schema validation and quarantine |
| `retention` | enabled, days, max size | Age/size-based deletion |
| `kafka`/worker | brokers/topic/acks/idempotence/retries; consumer group/poll timeout | Legacy Kafka queue path and worker |
| `eventbus` | type/topic/DLQ/consumer group; memory, Redis, NATS, Kafka settings | Queue backend selection |
| `cortex_bridge` | enabled/mode/endpoint/insecure/timeout/batch/flush/queue/header/API key | Optional Collector-to-Cortex forwarding |
| `cortex_schema` | enabled | Cortex tables/schema in local storage |

Example local development configuration (disable auth only on an isolated loopback listener):

```yaml
collector:
  addr: ":9308"
auth:
  enabled: false
storage:
  primary: duckdb
duckdb:
  path: ./loza.db
logging:
  level: debug
  format: json
metrics:
  prometheus: true
```

Example production invariants:

```yaml
collector:
  addr: ":9308"
  max_body_bytes: 52428800
auth:
  enabled: true
  server_secret: "${COLLECTOR_AUTH_SERVER_SECRET}"
storage:
  primary: duckdb
  encryption_key_env: LOZA_STORAGE_ENCRYPTION_KEY
reliability:
  mode: spool
privacy:
  mode: enforce
  collector_redaction: true
limits:
  max_event_bytes: 262144
```

Use the exact key names accepted by the installed release's schema. A documentation example is not proof that a field is supported by every version.

## Start, health, and readiness

Verify process and data-plane behavior in this order:

```bash
curl http://127.0.0.1:9308/health
curl http://127.0.0.1:9308/readyz
curl http://127.0.0.1:9308/version
curl http://127.0.0.1:9308/metrics
```

`/health` and `/healthz` are liveness aliases; `/ready` and `/readyz` are readiness aliases. Public health routes are intentionally unauthenticated. Metrics are protected when the configured auth wrapper protects the route. Readiness reflects sink/disk/spool state; a healthy process can be unready when it cannot accept/deliver safely.

Then send one small authenticated event:

```bash
curl -X POST http://127.0.0.1:9308/events \
  -H "Authorization: Bearer ${LOZA_INGEST_TOKEN}" \
  -H "X-Loza-Env: production" \
  -H 'Content-Type: application/json' \
  -d '[{"event_id":"evt_01","event":"checkout.request","kind":"http","service":"checkout","outcome":"success"}]'
```

Canonical scoped deployments use `/collectors/{collector}/...`, for example `/collectors/orders/events`, and authorize the path collector plus `X-Loza-Env`. Legacy unscoped routes are bound to `auth.default_collector` only when explicitly configured. Never try to select a collector by putting an untrusted collector field in the event/query payload.

## Route and permission reference

The current HTTP mux registers the following surfaces. Canonical forms are `/collectors/{collector}` plus the route; unscoped forms exist only with an explicit default collector for data routes.

| Method/path | Permission | Use |
|---|---|---|
| `GET /health`, `/healthz`, `/ready`, `/readyz`, `/version` | public | Liveness/readiness/version |
| `POST /events`, `/ingest`, `/events/batch`, `/events/ndjson` | `events:write` | JSON/batch/NDJSON ingest |
| `POST /otlp/logs` | `logs:write` | OTLP logs where enabled |
| `GET /status`, `/sinks`, `/sinks/{name}` | `events:read` | Status and sink health |
| `POST /sinks/{name}/test` | `events:write` | Test a sink |
| `GET /schema`, `POST /schema/check`, `/schema/diff` | `schema:read` | Inspect/check schema |
| `POST /schema/publish`, `POST /schema/blueprint` | `schema:write` / `project:admin` | Publish schema/blueprint |
| `GET /schema/blueprint` | `schema:read` | List blueprints |
| `POST /query`, `POST /lql/query` | `events:read` | SQL/LQL query |
| `GET /tail`, `GET /ws/tail` | `events:read` | HTTP/WebSocket live tail |
| `POST /replay` | `events:write` | Replay event payloads |
| `DELETE /events`, `/events/{event_id}`, `/events/by-user/{user_id}`, `/events/by-tenant/{tenant_id}` | `events:delete` | Destructive deletion |
| `GET /dlq`, `GET /dlq/{id}` | `events:read` | Inspect DLQ |
| `POST /dlq/replay`, `POST /dlq/{id}/replay` | `events:write` | Replay DLQ items |
| `DELETE /dlq/{id}` | `events:delete` | Delete DLQ item |
| `POST /audit/pii`, `/policy/validate`, `/retention/apply` | admin-specific | PII/policy/retention operations |
| `POST /keys`, `/keys/{id}/rotate`, `/keys/{id}/revoke`; `DELETE /keys/{id}` | `project:admin` | Key administration |
| `GET /metrics` | `events:read` when protected | Prometheus exposition |

If the installed release advertises extra gRPC/GraphQL endpoints, enable and secure them explicitly; the canonical HTTP mux above is the source for the checked-in Collector release.

## Ingest, query, and tail behavior

- Ingest accepts canonical event envelopes; content type and envelope shape are release-validated.
- `Content-Encoding: gzip` is supported in the checked-in path. Do not assume zstd or compressed responses.
- `loza query -q '...'` uses LQL by default; `--raw-sql` sends explicit SQL only where the selected engine supports it. Use `--param key=value` for typed values, not string concatenation.
- Bound queries by time, service, event kind, and `limit`; query results are scoped by the authorized collector.
- `tail` accepts filters such as `since`, `after_event_id`, `service`, `kind`, `level`, `trace_id`, `incident_id`, and `limit` where supported. WebSocket clients must authenticate during the upgrade using the release-documented mechanism.

Example:

```bash
loza query -q 'from events where service = "checkout" and level = "error" limit 100' --format table
loza tail --service checkout --level error
```

## Durability and delivery

Reliability modes:

- `direct`: write/deliver synchronously in the ingest path; simplest, least buffered.
- `spool`: append locally and replay from durable spool; size/fsync/queue bounds matter.
- `queue`: publish to the configured event bus/Kafka/Redis/NATS path and process asynchronously.
- `hybrid`: present in the checked-in runtime for a DuckDB primary plus Kafka queue path; use only when the installed release documents it.

Configure retries with bounded attempts/backoff/jitter. Set `dead_letter` policies for failures that cannot be delivered. A full disk/spool/queue must surface as rejection/readiness failure, not silent loss. On shutdown, the Collector marks itself unready, stops accepting traffic, flushes/closes sinks, checkpoints DuckDB when configured, and closes queue/compiler resources; choose a shutdown timeout long enough for the expected backlog.

## Fanout and storage

DuckDB is the local primary path. Keep its file on durable storage and do not copy it while writes are active unless using the documented export/checkpoint flow. Fanout outputs may include release-supported Loki, ClickHouse, Postgres, S3, Kafka, or other configured implementations. Set one explicit delivery policy (`require_primary`, `require_all`, or `best_effort` where supported), then decide whether primary/secondary/fallback/policy failures enter the DLQ.

For queue mode, configure `eventbus.type` and its backend fields (memory, Redis, NATS, or Kafka), topic, DLQ topic, consumer group, and retry/ack semantics. `enable_idempotence` is not the same as end-to-end exactly-once delivery; verify the broker and consumer contract.

## Schema, privacy, dedupe, retention, and deletion

- `schema_governance.mode`: `strict` rejects invalid events, `warn` records warnings, and `allow` bypasses enforcement where supported. Quarantine paths preserve rejected events for review.
- `privacy.mode`: `enforce`, `warn`, or `allow`; keep Collector redaction and secret scanning enabled for sensitive data.
- Dedupe needs a stable key (normally `event_id`), bounded window, and a backend that survives the required process topology. Redis settings must be secret-managed.
- Retention can delete by age and size. Confirm policy and backup implications before enabling.
- Deletion routes are irreversible from the Collector's point of view. Confirm collector, environment, tenant/user/event selector, reason/audit requirements, and backups before running them.

## CLI operator path

```text
loza config print|validate
loza status
loza sinks list|show NAME|test NAME
loza query ...
loza tail ...
loza watch ...
loza dlq list|show ID|replay ID|replay-all|delete ID
loza quarantine list|replay ID|delete ID
loza replay [--source FILE]
loza delete tenant|user|event ID [--reason TEXT]
loza audit pii [--limit N]
loza schema validate|fetch|list|diff|publish
loza keys create|revoke ID|rotate ID
loza doctor [--metrics]
```

Keep credentials out of shell history. `loza bench` is a load generator and must not target production without an explicit test plan. `loza deploy` renders/manages local deployment assets; it is not a substitute for reviewing and applying infrastructure changes.

## Troubleshooting order

1. Confirm the binary/image, config, and CLI are the same release.
2. Check `/health`, `/readyz`, `/version`, then `/status` and `/metrics` with the correct scope/key.
3. Confirm `auth.server_secret`, key/token presence, key kind, permissions, collector grant, environment, service/origin/IP restrictions, and header spelling.
4. Check DuckDB path/permissions/free space, spool/queue bytes, DLQ/quarantine paths, and sink connectivity.
5. Reduce to one small event, one collector, one sink, and one bounded query.
6. Inspect structured logs/metrics without printing `Authorization`, secrets, or raw event bodies.
7. Apply config changes with a restart unless the installed release explicitly documents the supported hot-reload subset; then repeat health, authenticated ingest, query, and tail checks.
