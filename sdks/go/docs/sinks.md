# Sinks

Root-package sinks:

- `StdoutSink`
- `StderrSink`
- `FileSink`
- `RotatingFileSink`
- `MemorySink`
- `NoopSink`

SDK-owned sink packages:

- `sinks/httpbatch`

Production delivery sinks live in `loxa-collector`, not in application SDKs:

- Kafka
- ClickHouse
- Postgres
- DuckDB
- OTLP
- S3
- GCS
- Loki
