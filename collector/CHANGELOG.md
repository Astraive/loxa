# Changelog

All notable changes to this project are documented in this file.

## [0.1.0] - 2026-05-11

### Horizon 1 — Foundation Gate
- Updated README "Known Limitations" to reflect that spool, retry, DLQ, and dedupe are available in the current release.
- Moved remaining hardcoded collector values to config fields:
  - `duckdb.column_types` for schema column type mapping.
  - `reliability.spool_file` for WAL file name.
  - `reliability.delivery_queue_size` for spool delivery queue.
- Reference config `build/loxa-collector.yaml` updated with new fields.
- Strict gate: `go test ./... -race` green across all 9 modules; `go vet` clean.

### Added

- Collector owns production delivery:
  - Kafka, ClickHouse, Postgres, DuckDB, OTLP, S3, GCS, and Loki sink implementations are collector-side code.
  - application SDKs emit to the collector through lightweight transports.
- Collector-local event and sink contracts under `internal/event`.
- Dependency boundary removed: collector no longer imports or requires any LOXA SDK module.
- Collector fanout and worker paths cover heavy production sinks.

### Changed

- Collector module dependency graph no longer includes any LOXA SDK module.
- Removed dependency on the old SDK sinks module.
- Integration DuckDB stress test uses temp DB path to avoid cross-run file lock collisions.
- README/docs repositioned LOXA as canonical wide-event layer.

### Breaking

- Collector sink implementations now use `github.com/astraive/loxa-collector/internal/event` instead of SDK-owned interfaces.
