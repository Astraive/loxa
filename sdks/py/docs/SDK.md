# SDK Overview

The LOZA Python SDK is a lightweight bridge connector to the LOZA collector. It provides the wide-event lifecycle (create, enrich, finish, emit) without owning heavy production sinks.

## Architecture

```
Application Code
      |
  loza SDK (Python)
      |
  HTTPBatchSink
      |
  loza-collector (Go)
      |
  ClickHouse / Postgres / DuckDB / Kafka / OTLP / S3 / GCS / Loki
```

The SDK owns:
- Event lifecycle (start, enrich, finish, emit)
- JSON encoding
- Sampling decisions
- Redaction
- HTTP transport to collector

The collector owns:
- Heavy sink integrations
- Retries and DLQ
- WAL/spool
- Fan-out

## Core Classes

| Class | Purpose |
|-------|---------|
| `Logger` | Main entry point. Manages config, sinks, samplers, redactors. |
| `Config` | Logger configuration. Use `dev()`, `production()`, `test()` presets. |
| `EventContext` | Per-event state. Created by `start_event`, mutated by `enrich`/`finish`. |
| `Params` | Event creation parameters (event name, kind, initial metadata). |
| `HTTPBatchSink` | Batches and sends events via HTTP to the collector. |

## Recommended Production Shape

1. Build a `Logger` with `Config.production(service_name)`.
2. Attach `HTTPBatchSink("http://collector/events")`.
3. Create one event per operation via `start_event`.
4. Let the collector handle storage, retries, fan-out, and heavy sink integration.

```python
import loza

cfg = (
    loza.production("checkout-service")
    .with_sink(loza.HTTPBatchSink("http://collector:9308/events"))
    .with_sampler(loza.SampleErrors())
)
loza.configure(cfg)
```

## Module-Level Facade

The SDK provides a module-level facade that delegates to the global default logger:

```python
import loza

ctx = loza.start_event(loza.Params(event="my.event"))
loza.enrich(ctx, loza.String("key", "value"))
loza.finish(ctx, "success")
loza.emit(ctx)
```

All functions are available in both lowercase (Pythonic) and Uppercase (Go-style alias) forms:

```python
loza.start_event(params)  # Pythonic
loza.StartEvent(params)   # Go-style alias
```

## Dependencies

The SDK has minimal dependencies:
- `requests` -- for HTTPBatchSink
- Standard library only for core functionality

## Configuration Sources

Configuration can be loaded from multiple sources (layered):

1. Code defaults
2. Config file (YAML)
3. Environment variables
4. Programmatic overrides

```python
from loza.core.config import load_layered_config

cfg = load_layered_config()  # merges all sources
```
