# Changelog for LOXA Spec

All notable changes to the contract repository are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-01-15

### Added

- **v1 Schema Structure**: Created versioned schema directory structure
  - `v1/event.schema.json` - Core event schema with required fields
  - `v1/README.md` - Schema documentation and usage guide
  - `v1/compatibility.md` - Compatibility matrix and versioning rules
  - `v1/examples/` - Example events directory

- **Core Event Schema (v1.0.0)**: Defined canonical event schema with required fields
  - `event_id` (string, UUID): Unique event identifier
  - `event_type` (string): Event type in dot notation
  - `timestamp` (string, ISO 8601): Event timestamp
  - `service` (object): Service identity with name and version
  - `deployment` (object): Deployment context with environment

- **Optional Fields**: Added standard optional fields
  - `tenant` (object): Multi-tenant identifier
  - `trace` (object): W3C distributed tracing context
  - `user` (object): User context information
  - `host` (object): Host or container information
  - `data` (object): Event-specific payload

- **Enriched Fields**: Defined collector-enriched fields
  - `received_at`: Collector receipt timestamp
  - `collector_version`: Collector version
  - `collector_hostname`: Collector hostname
  - `ingestion_latency_ms`: Ingestion latency

- **Example Events**: Created reference examples
  - `v1/examples/user_login.json` - User authentication event
  - `v1/examples/api_request.json` - API request event

- **PII Classification**: Added PII annotations using `x-pii` extension
  - `internal`: Internal use only
  - `confidential`: Requires redaction
  - `restricted`: Highly sensitive

- **Schema Versioning**: Established semantic versioning policy
  - Major version: Breaking changes (field removal, type changes)
  - Minor version: Backward-compatible additions (new optional fields)
  - Patch version: Documentation and example updates

### Changed

- Reorganized schema files into versioned directories (v1/)
- Updated compatibility documentation with version coexistence rules
- Enhanced schema with detailed descriptions and examples

## [0.1.0] - 2026-05-11

### Horizon 1 — Foundation Gate

### Added

- Initial protobuf definitions for loxa-collector API:
  - `proto/loxa/v1/collector.proto` - CollectorService with Health/Ready endpoints
  - `proto/loxa/v1/event.proto` - Event message definition
  - `proto/loxa/v1/ingest.proto` - LogIngest service with Push/PushStream
- OpenAPI specification: `openapi/collector.openapi.yaml`
- JSON Schema definitions: `schema/` directory
- Reference configuration: `build/loxa-collector.yaml`

### Changed

- Standardized `go_package` option across all proto files to `github.com/astraive/loxa-go/proto/loxa/v1;loxav1`
- Event message expanded with structured fields (event_id, event_name, service, timestamp, level, attrs, error_message)
- Integration DuckDB stress test uses temp DB path to avoid cross-run file lock collisions.
- README/docs repositioned LOXA as canonical wide-event layer.

### Breaking

- Removed `loxa.Capture` and `loxa.AssertEvent`; use `github.com/astraive/loxa-go/testkit`.
- Removed root middleware wrapper; use `github.com/astraive/loxa-go/middleware/nethttp`.
