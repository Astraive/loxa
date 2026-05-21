# Python SDK Docs

These docs cover the Python package that exists today.

Current implementation areas:

- event lifecycle and canonical encoding
- collector delivery through `HTTPBatchSink`
- middleware for `asgi`, `django`, `fastapi`, `flask`, and `starlette`
- integrations for `logging`, `loguru`, `structlog`, and `otel`

Collector-owned concerns are documented in the collector repo, not here:

- Kafka, OTLP, and other heavy fanout sinks
- durability modes
- storage/query ownership
