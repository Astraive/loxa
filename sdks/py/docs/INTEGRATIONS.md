# Integrations

Python logging framework integrations for the LOZA SDK. Each integration bridges a popular Python logging library to the LOZA event lifecycle.

## Available Integrations

| Module | Framework | Description |
|--------|-----------|-------------|
| `loza.integrations.logging` | stdlib `logging` | Bridges Python standard library logging to LOZA events. |
| `loza.integrations.loguru` | Loguru | Bridges Loguru log records to LOZA events. |
| `loza.integrations.structlog` | structlog | Bridges structlog processors to LOZA events. |
| `loza.integrations.otel` | OpenTelemetry | Bridges OpenTelemetry spans/logs to LOZA events. |

## stdlib logging

```python
import logging
from loza.integrations.logging import LozaHandler

logger = logging.getLogger("myapp")
logger.addHandler(LozaHandler(service="myapp"))
logger.setLevel(logging.INFO)

logger.info("User signed up", extra={"user.id": "u-123"})
```

The handler converts each log record into a LOZA event with the appropriate level, message, and any extra fields as attributes.

## Loguru

```python
from loguru import logger
from loza.integrations.loguru import loza_sink

logger.add(loza_sink, level="INFO", service="myapp")

logger.info("Order placed", order_id="ord-456")
```

## structlog

```python
import structlog
from loza.integrations.structlog import LozaProcessor

structlog.configure(
    processors=[
        LozaProcessor(service="myapp"),
        structlog.dev.ConsoleRenderer(),
    ]
)

log = structlog.get_logger()
log.info("payment_processed", amount=99.99, currency="USD")
```

## OpenTelemetry

```python
from loza.integrations.otel import LozaSpanExporter

exporter = LozaSpanExporter(service="myapp")
# Use with OpenTelemetry SDK tracer provider
```

## Design Notes

- Integrations are optional; the core SDK does not depend on any logging framework.
- Each integration maps framework-specific concepts (log levels, span attributes) to LOZA canonical fields.
- Go-specific integration names (`slog`, `zap`, `zerolog`) are not shipped as Python modules.
- Integrations use the global default logger unless a custom Logger is provided.
