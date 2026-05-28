# Changelog

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
