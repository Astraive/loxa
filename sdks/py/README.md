# LOXA Python SDK

**Status**: 🟢 **STABLE** (v1.0.0) - Production-ready, full feature conformance

Full API conformance with specification is complete. See [SDK_CONFORMANCE_CONTRACT.md](../loxa-spec/docs/SDK_CONFORMANCE_CONTRACT.md) for detailed guarantees.

Stable-v1 parity and release gates are tracked through:

- [SDK_CONFORMANCE_TEST_SUITE.md](../loxa-spec/docs/SDK_CONFORMANCE_TEST_SUITE.md)
- [SDK_COMPLETION_MATRIX.md](../loxa-spec/docs/SDK_COMPLETION_MATRIX.md)

`loxa-py` is a collector-first Python SDK for wide events.

It is intentionally lightweight:

- build one canonical event per operation
- emit that event to the collector with `HTTPBatchSink`
- keep heavy delivery concerns such as Kafka, OTLP, ClickHouse, Postgres, and S3/GCS in `loxa-collector`

## Install

```bash
pip install -e .
```

## Quick Start

```python
from loxa import Config, HTTPBatchSink, Logger, Params

logger = Logger(
    Config.production("checkout").with_sink(
        HTTPBatchSink("http://127.0.0.1:9090/v1/events")
    )
)

ctx = logger.start_event(
    Params(event="checkout.request", method="POST", path="/checkout")
)
logger.enrich(ctx, "user.id", "u-1")
logger.enrich(ctx, "payment.provider", "stripe")
logger.finish(ctx, outcome="success", status_code=200)
logger.emit(ctx)
```

If your collector requires an API key, set:

```bash
export LOXA_COLLECTOR_API_KEY=your-key
export LOXA_COLLECTOR_API_KEY_HEADER=X-API-Key
```

## Shipped Modules

- middleware: `asgi`, `django`, `fastapi`, `flask`, `starlette`
- integrations: `logging`, `loguru`, `structlog`, `otel`
- sinks: `HTTPBatchSink`, `StdoutSink`, `FileSink`, `MemorySink`, `NoopSink`

## Examples

- [examples/basic/README.md](E:/astraive/loxa/loxa-py/examples/basic/README.md)
- [examples/custom_schema/README.md](E:/astraive/loxa/loxa-py/examples/custom_schema/README.md)
- [examples/fastapi/README.md](E:/astraive/loxa/loxa-py/examples/fastapi/README.md)
- [examples/flask/README.md](E:/astraive/loxa/loxa-py/examples/flask/README.md)
- [examples/httpbatch_to_collector/README.md](E:/astraive/loxa/loxa-py/examples/httpbatch_to_collector/README.md)
- [examples/logging_bridge/README.md](E:/astraive/loxa/loxa-py/examples/logging_bridge/README.md)

## Scope

Current package focus:

- canonical event lifecycle
- collector delivery over HTTP
- framework middleware for real Python web stacks
- testable local sinks and helpers

Collector-owned features stay out of this SDK:

- Kafka fanout
- OTLP export
- ClickHouse/Postgres/DuckDB storage ownership
- S3/GCS archival

## Docs

- [docs/README.md](E:/astraive/loxa/loxa-py/docs/README.md)
- [docs/SDK.md](E:/astraive/loxa/loxa-py/docs/SDK.md)
- [docs/MIDDLEWARE.md](E:/astraive/loxa/loxa-py/docs/MIDDLEWARE.md)
- [docs/INTEGRATIONS.md](E:/astraive/loxa/loxa-py/docs/INTEGRATIONS.md)

## Testing

```bash
python -m pytest -q
python ../loxa-spec/conformance/runner.py --sdk python --group all
```
