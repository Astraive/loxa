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

- Lightweight SDK boundary:
  - core (`github.com/astraive/loxa-go`)
  - middleware (`github.com/astraive/loxa-go/middleware`)
  - integrations (`github.com/astraive/loxa-go/integrations`)
  - collector transport (`github.com/astraive/loxa-go/sinks/httpbatch`)
  - heavy production sinks are collector-owned
- New `testkit` package for test capture/assert helpers.
- Dependency boundary conformance test for root module.
- Example set for net/http, slog bridge, custom schema, and HTTP batch to collector.

### Changed

- Root module dependency graph slimmed to core-only requirements.
- Removed SDK-owned heavy sink modules; Kafka, ClickHouse, Postgres, DuckDB, OTLP, S3, GCS, and Loki live in `loxa-collector`.
- Integration DuckDB stress test uses temp DB path to avoid cross-run file lock collisions.
- README/docs repositioned LOXA as canonical wide-event layer.

### Breaking

- Removed `loxa.Capture` and `loxa.AssertEvent`; use `github.com/astraive/loxa-go/testkit`.
- Removed root middleware wrapper; use `github.com/astraive/loxa-go/middleware/nethttp`.
