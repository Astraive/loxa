# LOZA getting started

Use this sequence for a released Collector and one application. Keep every component on a compatible pinned release.

## Prerequisites

Check the versions required by the installed release before installing an SDK. The repository quickstart currently lists Go 1.22+, Python 3.10+, Rust 1.75+, and Bun 1.3.14; treat these as repository guidance, not a substitute for the release artifact's requirements.

## First-use sequence

1. Install or pull the exact Collector release; do not use `latest` in production.
2. Configure durable storage, authentication, redaction/privacy, size limits, and rate limits.
3. Start the Collector and check `/health`, `/readyz`, and `/version`.
4. Configure one SDK with service identity, Collector URL, and a least-privilege write credential.
5. Emit one small canonical event.
6. Verify the event with one bounded read using a separate read credential.
7. Use `loza tail` or `loza watch` for live delivery.
8. Add Cortex only when incident reconstruction, graph analysis, similarity, or remediation feedback is needed.

A healthy process is not proof of authenticated ingest, durable storage, or sink delivery. The minimum smoke test is one health/readiness check, one authenticated write, and one bounded authorized read.

## SDK installation

```bash
# Use the versions selected by the release contract.
go get github.com/astraive/loza/sdks/go
pip install loza
npm install @astraive/loza
cargo add loza
```

## Minimal event shape

```json
{
  "event_id": "evt_runtime_generated",
  "event": "order.created",
  "kind": "business",
  "service": "checkout",
  "environment": "production",
  "release": "2026.08.26",
  "timestamp": "2026-08-26T12:00:00Z",
  "outcome": "success",
  "duration_ms": 42,
  "attrs": {"order_id": "ord_01"}
}
```

Use stable event names, typed attributes, and runtime-generated IDs for smoke tests. Do not put access tokens, authorization headers, payment data, or unnecessary PII in attributes. Preserve request/trace identifiers when correlation is needed.

## Graceful shutdown

SDK lifecycle is `start → enrich → checkpoint/process/group/timer/link → finish or finish-error → emit → flush → shutdown`. Register shutdown with the application's signal/teardown path. Stop accepting new work before the final flush, use a bounded context or timeout, and do not claim delivery merely because the process exited cleanly.

## Release caveat

Repository quickstarts may show development commands or historical endpoints. Release users should use the installed binary's `loza --help`, the matching image documentation, and the endpoint contract exposed by that release. Do not substitute `go run`, `cargo run`, or source-tree paths for a published package when giving production instructions.
