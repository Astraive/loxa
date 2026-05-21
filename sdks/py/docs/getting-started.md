# Getting Started

A 5-minute quickstart for the LOXA Python SDK. By the end you will have a working application that creates, enriches, finishes, and emits a wide-event.

## Install

From the repository root:

```bash
cd sdks/py
pip install -e .
```

## Full Working Example

```python
import loxa

# 1. Configure a logger with a stdout sink (dev mode).
cfg = loxa.dev("checkout-service").with_sink(loxa.StdoutSink())
logger = loxa.configure(cfg)

# 2. Start an event.
ctx = loxa.start_event(loxa.Params(
    event="checkout.request",
    kind="http",
))

# 3. Enrich with attributes.
loxa.enrich(ctx,
    loxa.UserID("u-abc123"),
    loxa.TenantID("tenant-acme"),
    loxa.String("cart.id", "cart-42"),
    loxa.Int("item_count", 3),
)

# 4. Finish the event with an outcome.
loxa.finish(ctx, "success",
    loxa.Int("status_code", 200),
)

# 5. Emit the event to the configured sink.
event_id = loxa.emit(ctx)
print(f"Emitted event: {event_id}")

# 6. Flush any buffered events and shut down.
loxa.flush()
loxa.shutdown()
```

## What Each Step Does

| Step | Function | Purpose |
|------|----------|---------|
| Configure | `loxa.configure(cfg)` | Sets the global default logger with service name, sink, sampler, and redactor. |
| start_event | `loxa.start_event(params)` | Creates a new `EventContext` with a UUIDv7 event ID and initial metadata. |
| enrich | `loxa.enrich(ctx, *attrs)` | Adds typed attributes to the event. Attributes are merged into the event map. |
| finish | `loxa.finish(ctx, outcome, *attrs)` | Marks the event as finished, records the outcome, and calculates duration. |
| emit | `loxa.emit(ctx)` | Serializes the event and sends it to the configured sink. Returns the event ID. |
| flush | `loxa.flush()` | Flushes any buffered events in async sinks. |
| shutdown | `loxa.shutdown()` | Shuts down the logger, flushing remaining events. |

## Connecting to a Collector

In production, send events to the LOXA collector via HTTPBatchSink:

```python
cfg = (
    loxa.production("checkout-service")
    .with_sink(loxa.HTTPBatchSink("http://collector:9090/v1/events"))
    .with_sampler(loxa.SampleErrors())
)
logger = loxa.configure(cfg)
```

## Using Logger Directly

Instead of the module-level facade, you can use a Logger instance directly:

```python
logger = loxa.new(loxa.production("my-service").with_sink(loxa.StdoutSink()))

ctx = logger.start_event(loxa.Params(event="my.event"))
loxa.enrich(ctx, loxa.String("key", "value"))
loxa.finish(ctx, "success")
loxa.emit(ctx)
```

## Event Lifecycle

```
start_event -> enrich -> finish -> emit
     |            |        |       |
  creates     adds      marks   serializes
  context    attrs     done     + sends
```

## Next Steps

- [Public API](public-api.md) -- Full API reference.
- [Middleware](middleware.md) -- ASGI/WSGI framework integration.
- [Security](security.md) -- Redaction and field controls.
- [Testing](testing.md) -- Running tests and conformance.
- [Integrations](integrations.md) -- Logging framework bridges.
