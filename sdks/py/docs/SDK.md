# SDK Overview

The LOXA Python SDK is a lightweight bridge connector to the LOXA collector. It provides the wide-event lifecycle (create, enrich, finish, emit) without owning heavy production sinks.

## Architecture

```
Application Code
      |
  loxa SDK (Python)
      |
  HTTPBatchSink
      |
  loxa-collector (Go)
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
import loxa

cfg = (
    loxa.production("checkout-service")
    .with_sink(loxa.HTTPBatchSink("http://collector:9090/events"))
    .with_sampler(loxa.SampleErrors())
)
loxa.configure(cfg)
```

## Module-Level Facade

The SDK provides a module-level facade that delegates to the global default logger:

```python
import loxa

ctx = loxa.start_event(loxa.Params(event="my.event"))
loxa.enrich(ctx, loxa.String("key", "value"))
loxa.finish(ctx, "success")
loxa.emit(ctx)
```

All functions are available in both lowercase (Pythonic) and Uppercase (Go-style alias) forms:

```python
loxa.start_event(params)  # Pythonic
loxa.StartEvent(params)   # Go-style alias
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
from loxa.core.config import load_layered_config

cfg = load_layered_config()  # merges all sources
```
