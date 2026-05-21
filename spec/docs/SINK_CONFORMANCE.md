# Sink Conformance Suite

Every collector sink MUST pass the reusable sink conformance suite.

Required cases:

- open/close
- health
- write one event
- write batch
- partial failure
- timeout
- retryable error
- non-retryable error
- idempotency
- flush
- schema migration
- large payload
- context cancellation

The suite applies to DuckDB, ClickHouse, Kafka, Loki, OTLP, S3, GCS, and Postgres.

