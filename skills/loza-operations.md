# LOZA Collector operations

## Health and readiness

Use the release-documented aliases; the current Collector contract exposes:

```text
GET /health    liveness
GET /healthz   liveness alias
GET /ready     readiness
GET /readyz    readiness alias
GET /version   version metadata
GET /metrics   Prometheus metrics when enabled/protected by auth policy
```

Liveness means the process is running. Readiness means the Collector can safely accept and deliver traffic: sink, storage, spool, queue, and disk state may make a live process unready. Health endpoints are intentionally public in the current contract; do not put credentials in their URLs.

## Verification order

```bash
curl --fail-with-body "$COLLECTOR_URL/health"
curl --fail-with-body "$COLLECTOR_URL/readyz"
curl --fail-with-body "$COLLECTOR_URL/version"
loza status
loza sinks list
```

Then perform one authenticated write and one bounded read. Use a read credential for status/query and a write credential for ingest. A successful HTTP acceptance response does not prove every secondary sink has persisted the event.

## Sink and durability symptoms

When readiness or delivery fails, check in this order:

1. Effective config and component versions.
2. DuckDB/spool/DLQ/quarantine paths, permissions, free space, and encryption-key resolution.
3. Sink health and connectivity.
4. Queue/spool growth, retry exhaustion, and rate limiting.
5. Schema validation warnings/rejections and deduplication counters.
6. Shutdown/flush behavior and whether the process was killed before draining.

A full spool, queue, or disk must reject or make readiness fail rather than silently dropping data.

## DLQ and quarantine

Use the installed CLI help for the exact verbs:

```bash
loza dlq list
loza dlq show <id>
loza dlq replay <id>
loza dlq replay-all
loza quarantine list
loza quarantine replay <id>
```

Inspect the failure and scope before replaying. Replay only after the underlying sink/schema/configuration problem is corrected. Delete DLQ or quarantine data only through an authorized administrative workflow with an audit reason where supported.

## Retention, export, and deletion

Configure retention for both age and storage bounds where supported. Keep export paths durable and access-controlled. Treat `loza export`, `loza delete tenant|user|event`, key rotation, schema publication, and DLQ deletion as privileged operations. Preview scope, select the correct collector/environment, and avoid exporting raw secrets or unnecessary PII.

## Metrics

Monitor accepted, invalid, rejected, deduplicated, parse-error, auth-error, rate-limit, sink-write-error, retry, DLQ/quarantine, spool/queue bytes, disk health, and overall readiness metrics. Verify metric names against the installed release before creating alerts; repository examples can lag the published artifact.
