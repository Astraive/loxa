# Changelog

## [0.2.6] - 2026-05-30

### Security
- Enable auth and rate limiting by default in K8s manifests
- Require secret-backed credentials for database connections
- SSL mode defaults to "require" for PostgreSQL
- FFI null pointer checks in cortex-match crate
- Tighten Helm NetworkPolicy defaults

### Fixed
- Bump version to 0.2.6

## [0.2.5] - 2026-05-28

### Fixed
- Security fixes for DuckDB connection pool safety, key cache invalidation
- Version loaded from YAML metadata instead of hardcoded constants
- Bump version to 0.2.5

## [0.2.4] - 2026-05-28

### Fixed
- Version bumped to 0.2.4

## [0.0.2] - 2026-05-20

### Added
- Initial release
- Core event lifecycle: StartEvent -> Enrich -> Finish -> Emit
- Logger with async/sync delivery
- HTTPBatchSink with gzip compression and retry
- StdoutSink, StderrSink, MemorySink, NoopSink
- 14-key safety-net redactor
- Full sampler suite (random, errors, status codes, routes, etc.)
- DefaultSchema and FlatSchema
- Spec contract from spec
- Express middleware
- Node.js http middleware
- AsyncLocalStorage context propagation
- UUIDv7 event IDs
