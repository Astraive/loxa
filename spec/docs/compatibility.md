# Compatibility

Compatibility is defined at the contract level:

- SDKs should emit spec-compatible JSON
- collectors should accept spec-compatible JSON
- contract changes should be documented per release
- breaking changes require explicit version increments

## Version Constants

| Contract | Version |
|---|---|
| `LOXA_SPEC_VERSION` | `v1` |
| `LOXA_INGEST_API_VERSION` | `v1` |
| `LOXA_EVENT_VERSION` | `v1` |

All shipping SDKs and collectors should emit `schema_version: "v1"` and
`event_version: "v1"` and validate against the golden fixtures under
`examples/golden`.

## SDK and Collector Ownership

SDKs are lightweight application libraries. They create, enrich, redact, sample,
encode, and send events to the collector. SDK-owned sinks are limited to stdout,
stderr, file, rotating file, memory, noop, and HTTP batch / collector delivery.

The collector owns heavy delivery sinks and operational behavior: Kafka,
ClickHouse, Postgres, DuckDB, OTLP, S3, GCS, Loki, routing, retries, batching,
spool, DLQ, dedupe, auth, health, and metrics.

## Partial Events and ACKs

Partial events use `partial`, `partial_reason`, `event_state`, and
`delivery_attempts`. SDKs may auto-finish abandoned events with
`outcome: "abandoned"` and `partial: true`.

Collector batch responses include per-event `acks`, where every ACK contains
`event_id`, `status`, `reason`, and `retryable`. Batch totals remain
`accepted`, `rejected`, `invalid`, and `deduped` so SDK offline buffers can
drop, dead-letter, or retry deterministically.

The canonical response also includes `request_id`, `status`, `duplicates`, and
`errors[]`. `deduped` is kept as a compatibility alias for `duplicates`.

## Required Contract Additions

- Event lifecycle: see `EVENT_STATE_MACHINE.md`.
- Accepted delivery semantics: see `DELIVERY_SEMANTICS.md`.
- Schema evolution policy: see `SCHEMA_EVOLUTION_POLICY.md`.
- Tenant/service identity rules: see `IDENTITY_MODEL.md`.
- Privacy/compliance: see `PRIVACY_COMPLIANCE.md`.
- Collector clustering: see `COLLECTOR_CLUSTERING.md`.
- Backpressure response contract: see `BACKPRESSURE.md`.
- Sink conformance: see `SINK_CONFORMANCE.md`.
- Limits and cardinality: see `LIMITS_AND_CARDINALITY.md`.
- MVP cut: see `MVP_CUT.md`.
