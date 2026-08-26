# LOZA troubleshooting

Use an evidence-first loop: identify the component, confirm the installed version, reproduce with the smallest safe request, then change one setting at a time. Do not expose tokens or raw event bodies while diagnosing.

## Triage order

1. Record Collector, Cortex, CLI, and SDK versions from the deployed artifacts.
2. Check Collector `/health`, `/readyz`, and `/version`.
3. Check Cortex `/healthz`, `/readyz`, and `/version` when Cortex is involved.
4. Run `loza doctor` and inspect configured endpoint, environment, collector scope, and credential kind/permissions.
5. Test one authenticated event write with a disposable scoped credential.
6. Run one bounded query using a separate read credential.
7. Inspect sink health, retries, queue/spool growth, disk/free space, schema validation, and deduplication.
8. Reproduce with one event and one sink before changing batching, retry, or durability settings.
9. Restart after configuration changes unless the installed release explicitly documents hot reload.

## Common failure classes

### Health succeeds but ingest is unauthorized

Health is public liveness. Confirm `Authorization: Bearer` or the release's documented DSN/Basic flow, the exact collector path, `X-Loza-Env`, credential kind, permission, collector scope, and allowed environment. Do not fix this by granting `project:admin` or using an unscoped key.

### Events are accepted but queries are empty

Check that the write targeted the intended collector/environment, the query credential can read that scope, the event passed schema validation, and the sink has flushed. Bound the query by event/service/time and verify the actual event ID. Acceptance is not proof of immediate sink visibility when ingestion is batched.

### Ready is false

Read the readiness reason if exposed, then inspect sink connectivity, storage permissions, encryption-key resolution, disk space, spool/queue bounds, and retry exhaustion. Do not remove readiness checks to make deployment green.

### Duplicate events appear

Preserve stable event IDs and configure the intended deduplication window/policy. Check whether middleware and manual instrumentation both emit the same operation, or whether a retry changed the event ID. Fix the lifecycle boundary rather than filtering duplicates in queries.

### Query fails or is slow

Reduce the time range and result limit, add a selective service/event predicate, and run against the correct engine. Use `loza query --help` for parameter syntax. Never concatenate untrusted input into raw SQL. Verify whether a function is supported by DuckDB, ClickHouse, and the deployed LQL version.

### Shutdown loses events

Stop new work, give the SDK a bounded flush window, then shut down sinks and the HTTP process in the release-documented order. Inspect queue/spool and sink metrics. Do not claim graceful delivery if the process was force-killed or the flush context expired.

## Safe evidence to collect

Collect versions, endpoint paths without credentials, HTTP status codes, readiness reasons, sanitized error messages, sink names/health, queue/spool sizes, event IDs, and timestamps. Redact `Authorization`, DSN userinfo, API keys, encryption-key names when sensitive, raw attributes, and customer identifiers before sharing logs or tickets.
