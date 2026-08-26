# Changelog

## [0.3.4] - 2026-08-26

### Changed

- Updated the Python SDK release version and credential examples to the
  `lz_` prefix.

## [0.2.6] - 2026-05-30

### Security
- Fix path traversal in DiskOfflineBuffer, FileSink, RotatingFileSink
- Fix SSRF in collector endpoint validation
- Fix Cortex client endpoint SSRF validation

### Fixed
- Fix config with_* methods to be non-mutating
- Fix flush() to respect timeout parameter
- Fix FileSink and DiskOfflineBuffer file handle leaks
- Fix Starlette middleware to use async HTTP
- Fix DeliveryStats thread safety
- Add response body caps in HTTP clients
- Bump version to 0.2.6

## [0.2.5] - 2026-05-28

### Fixed
- Version bumped to 0.2.5
- SDK parity manifest updated to 0.2.5

## [0.2.4] - 2026-05-28

### Fixed
- Version bumped to 0.2.4

## [0.0.2] - 2026-05-20

### Added
- Initial stable release
- Core event lifecycle: StartEvent, Enrich, Finish, Emit
- Logger with async/sync delivery
- HTTPBatchSink with gzip compression and retry
- StdoutSink, FileSink, MemorySink, NoopSink
- 14-key safety-net redactor
- Full sampler suite
- DefaultSchema and FlatSchema
- Spec contract from spec/
- Middleware: asgi, django, fastapi, flask, starlette
- Integrations: logging, loguru, structlog, otel
