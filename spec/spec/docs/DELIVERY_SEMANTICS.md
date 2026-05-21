# Delivery Semantics

LOXA does not claim impossible network-level exactly-once delivery.

The canonical wording is:

- SDKs emit at-most-once per `EventContext` locally.
- The collector provides idempotent ingestion using `event_id` and optional `idempotency_key`.
- End-to-end behavior is effectively-once when collector dedupe is enabled and backed by a distributed-safe dedupe store.

Collector duplicate behavior:

- A retry with the same `event_id` MUST NOT create another collector row.
- The collector MUST return a deterministic response for a duplicate `event_id`.
- The duplicate response MUST include `duplicate: true` on the per-event ACK.
- Duplicate accepted events count toward `accepted` and `duplicates`; legacy `deduped` mirrors `duplicates`.

Required acceptance criteria:

- SDK retry with the same `event_id` does not create a duplicate collector row.
- Collector returns the same accepted duplicate status for the same duplicate `event_id`.
- Duplicate accepted response includes `duplicate=true`.

