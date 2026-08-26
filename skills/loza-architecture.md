# LOZA architecture for users

LOZA separates the application/client plane, data plane, and control plane.

```text
Application → SDK → Collector → durable storage / sinks
                                  ↘ Cortex bridge/control plane
CLI and operators ───────────────→ Collector or Cortex APIs
```

## SDK/client plane

SDKs create canonical wide events around logical operations. They should remain lightweight: add identity, timing, outcomes, correlation, and typed business attributes; avoid owning durable storage, fanout, or incident reconstruction.

## Collector/data plane

The Collector is the trust and durability boundary. It authenticates and scopes requests, validates the canonical envelope/schema, applies privacy/redaction policy, deduplicates event IDs, persists or buffers events, fans out to sinks, exposes bounded reads/tails, and manages replay/deletion workflows.

## Cortex/control plane

Cortex consumes Collector events and maintains incident context. Use it for incident reconstruction, causal chains, service/incident graphs, similar-incident search, remediation records, and feedback. Do not use Cortex as a replacement for Collector ingest/storage unless the release explicitly documents that topology.

## Sink choices

- DuckDB: local/embedded durable query storage.
- Kafka or queue-backed delivery: decoupled buffering and worker-based fanout.
- ClickHouse, Postgres, Loki, OTLP, object storage, or other sinks: use only when enabled and documented by the installed release.

The selected reliability mode changes delivery semantics. Direct mode favors request latency but has little buffering; queue/spool modes improve durability but introduce eventual delivery and readiness/backlog considerations.

## Boundary rule

When explaining a product workflow, identify which component owns each operation. Instrumentation belongs in an SDK; authentication, validation, redaction, persistence, query, tail, and delivery belong in the Collector; incident intelligence and feedback belong in Cortex. This prevents duplicate events, bypassed policy, and unsupported direct integrations.
