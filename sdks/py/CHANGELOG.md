# Changelog

## [0.0.1] - 2026-05-20

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
