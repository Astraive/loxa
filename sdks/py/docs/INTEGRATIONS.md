# Integrations

Python logging framework integrations for the LOXA SDK. Each integration bridges a popular Python logging library to the LOXA event lifecycle.

## Available Integrations

| Module | Framework | Description |
|--------|-----------|-------------|
| `loxa.integrations.logging` | stdlib `logging` | Bridges Python standard library logging to LOXA events. |
| `loxa.integrations.loguru` | Loguru | Bridges Loguru log records to LOXA events. |
| `loxa.integrations.structlog` | structlog | Bridges structlog processors to LOXA events. |
| `loxa.integrations.otel` | OpenTelemetry | Bridges OpenTelemetry spans/logs to LOXA events. |

## stdlib logging

```python
import logging
from loxa.integrations.logging import LoxaHandler

logger = logging.getLogger("myapp")
logger.addHandler(LoxaHandler(service="myapp"))
logger.setLevel(logging.INFO)

logger.info("User signed up", extra={"user.id": "u-123"})
```

The handler converts each log record into a LOXA event with the appropriate level, message, and any extra fields as attributes.

## Loguru

```python
from loguru import logger
from loxa.integrations.loguru import loxa_sink

logger.add(loxa_sink, level="INFO", service="myapp")

logger.info("Order placed", order_id="ord-456")
```

## structlog

```python
import structlog
from loxa.integrations.structlog import LoxaProcessor

structlog.configure(
    processors=[
        LoxaProcessor(service="myapp"),
        structlog.dev.ConsoleRenderer(),
    ]
)

log = structlog.get_logger()
log.info("payment_processed", amount=99.99, currency="USD")
```

## OpenTelemetry

```python
from loxa.integrations.otel import LoxaSpanExporter

exporter = LoxaSpanExporter(service="myapp")
# Use with OpenTelemetry SDK tracer provider
```

## Design Notes

- Integrations are optional; the core SDK does not depend on any logging framework.
- Each integration maps framework-specific concepts (log levels, span attributes) to LOXA canonical fields.
- Go-specific integration names (`slog`, `zap`, `zerolog`) are not shipped as Python modules.
- Integrations use the global default logger unless a custom Logger is provided.
