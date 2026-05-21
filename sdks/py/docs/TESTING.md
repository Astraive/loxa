# Testing

How to run tests, understand test structure, and verify conformance for the LOXA Python SDK.

## Running Tests

### All Tests

```bash
cd sdks/py
python -m pytest
```

### With Verbose Output

```bash
python -m pytest -v
```

### Specific Test File

```bash
python -m pytest tests/test_logger.py -v
```

### Specific Test

```bash
python -m pytest tests/test_logger.py::test_emit_cycle -v
```

## Test Structure

```
sdks/py/tests/
  test_logger.py        -- Core logger lifecycle tests
  test_config.py        -- Configuration and presets
  test_event.py         -- Event context, attrs, state machine
  test_sinks.py         -- Sink implementations (Memory, Stdout, HTTP)
  test_sampler.py       -- Sampler decision tests
  test_redactor.py      -- Redaction and field security
  test_schema.py        -- Schema encoding (default, flat, nested, ECS, OTel)
  test_middleware.py     -- ASGI/WSGI middleware tests
  test_integrations.py  -- Logging framework bridges
  test_timing.py        -- Process, Timer, Group, Stopwatch
  test_metrics.py       -- MetricsCollector and Prometheus rendering
  test_cortex.py        -- CortexClient tests
  test_conformance.py   -- Cross-SDK conformance checks
```

## Test Helpers

### MemorySink

Capture emitted events in memory for assertions:

```python
import loxa

sink = loxa.MemorySink()
logger = loxa.new(loxa.test("my-test").with_sink(sink))

ctx = logger.start_event(loxa.Params(event="test.event"))
loxa.finish(ctx, "success")
loxa.emit(ctx)

events = sink.events()
assert len(events) == 1
assert events[0]["event"] == "test.event"
```

### Test Config Preset

The `loxa.test()` preset configures:
- Service name from argument
- MemorySink
- SampleAll (no sampling drops)
- NoopRedactor (no redaction unless explicitly set)

```python
cfg = loxa.test("my-test")
```

## Conformance

The conformance test suite verifies that the Python SDK matches the cross-language parity manifest:

```bash
python -m pytest tests/test_conformance.py -v
```

Conformance checks include:
- All lifecycle functions are exported.
- All attribute constructors are exported.
- All sinks, samplers, and redactors are exported.
- Event state machine transitions are correct.
- Spec version constants match.

## Writing Tests

Use `MemorySink` and `loxa.test()` for isolated unit tests:

```python
import loxa


def test_enrich_sets_attributes():
    sink = loxa.MemorySink()
    logger = loxa.new(loxa.test("t").with_sink(sink))

    ctx = logger.start_event(loxa.Params(event="test.enrich"))
    loxa.enrich(ctx, loxa.String("key", "value"))
    loxa.finish(ctx, "success")
    loxa.emit(ctx)

    event = sink.events()[0]
    assert event["attrs"]["key"] == "value"
```

## CI Integration

The test suite runs in CI with:

```bash
cd sdks/py
python -m pytest --tb=short -q
```

Exit code 0 means all tests pass. Non-zero indicates failures.
