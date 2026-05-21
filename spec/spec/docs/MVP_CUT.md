# Minimum Lovable MVP

Milestone A: Contract Lock

- `loxa-spec` schema
- ingest envelope
- response schema
- golden fixtures
- Go/Python/Rust constants
- conformance runner

Milestone B: Collector Core

- HTTP `/v1/events`
- validation `enforce`, `warn`, `quarantine`
- DuckDB primary
- local durable spool
- DLQ
- Prometheus metrics
- `/query`
- `/status`
- `/health`
- `/ready`

Milestone C: SDK Parity

- Go/Python/Rust lifecycle
- collector HTTP batch sink
- idempotent emit
- flush/shutdown
- redaction
- context propagation
- testkit

Milestone D: CLI UX

- `init`
- `dev`
- `doctor`
- `emit sample`
- `schema validate`
- `tail`
- `query`
- `dlq list/replay`

ClickHouse, Kafka, Loki, OTLP, S3, GCS, and Postgres become fully required after
this MVP slice is green.

