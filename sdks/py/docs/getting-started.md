# Getting Started

A 5-minute quickstart for the LOXA Python SDK. By the end you will have a working application that creates, enriches, finishes, and emits a wide-event.

## Default Client

Use `loxa.<method>()` for quick starts and single-client applications.

## Custom Client / Alias

Use `create_loxa(...)` for an independent client and `loxa.alias("name")` for a same-config child that emits `loxa.alias`.

## Cross-Language Parity

Python maps to the v0.0.2 parity family as `loxa`, `create_loxa`, optional `Loxa`, and user-defined variables such as `logger.info(...)`.

## Install

From the repository root:

```bash
cd sdks/py
pip install -e .
```

Or install from a package index:

```bash
pip install loxa
```

## Full Working Example

```python
import loxa

# 1. Configure the default logger (stdout sink for dev).
loxa.configure(
    loxa.dev("checkout-service").with_sink(loxa.StdoutSink())
)

# 2. Start an event.
ctx = loxa.start_event(loxa.Params(
    event="checkout.request",
    kind="http",
))

# 3. Enrich with typed attributes.
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

# 6. Flush buffered events and shut down.
loxa.flush()
loxa.shutdown()
```

Every call in this example goes through the module-level `loxa.*` facade, which delegates to the global default `Logger` instance. No logger variable is needed.

## What Each Step Does

| Step | Call | Purpose |
|------|------|---------|
| Configure | `loxa.configure(cfg)` | Sets the global default logger with service name, sink, sampler, and redactor. |
| Start event | `loxa.start_event(Params(...))` | Creates a new `EventContext` with a UUIDv7 event ID and initial metadata. |
| Enrich | `loxa.enrich(ctx, *attrs)` | Adds typed attributes to the event. Attributes are merged into the event map. |
| Finish | `loxa.finish(ctx, outcome, *attrs)` | Marks the event as finished, records the outcome, and calculates duration. |
| Emit | `loxa.emit(ctx)` | Serializes the event and sends it to the configured sink. Returns the event ID. |
| Flush | `loxa.flush()` | Flushes any buffered events in async sinks. |
| Shutdown | `loxa.shutdown()` | Shuts down the logger, flushing remaining events. |

## Connecting to a Collector

In production, send events to the LOXA collector with authentication:

```python
import os
import loxa

loxa.configure(
    loxa.production("checkout-service")
    .with_api_key(os.environ["LOXA_API_KEY"])
    .with_collector_endpoint("https://collector.loxa.dev")
    .with_sampler(loxa.SampleErrors())
)
```

The SDK automatically sets `Authorization: Bearer <key>` headers.

### Authentication

| Config Field | Env Var | Description |
|---|---|---|
| `api_key` | `LOXA_API_KEY` | Ingest API key (`lx_sec_live_k_xxx_yyyy`) |

```python
import loxa

# Production
loxa.configure(loxa.production("my-service").with_api_key("lx_sec_live_k_xxx_yyyy"))

# Local dev
loxa.configure(loxa.dev("my-service").with_api_key("lx_local_dev_mytoken"))
```

See [Security](../../docs/security.md) for key types and RBAC roles.

## Custom Instances

Use `loxa.create_loxa(service=...)` when you need an independent logger with its own config. Custom instances do not share state with the default logger.

```python
import loxa

# Create an isolated logger for a specific service.
logger = loxa.create_loxa(service="checkout-api")

ctx = logger.start_event(loxa.Params(event="checkout.charge", kind="http"))
logger.enrich(ctx, loxa.OrderID("ord-99"), loxa.Amount(49.99))

try:
    result = process_payment()
    logger.finish(ctx, "success", loxa.String("provider", "stripe"))
except Exception as exc:
    logger.finish_error(ctx, exc)

logger.emit(ctx)
logger.shutdown()
```

You can also pass config options directly:

```python
logger = loxa.create_loxa(
    service="worker",
    collector_endpoint="https://collector.loxa.dev",
    api_key=os.environ["LOXA_API_KEY"],
)
```

## Aliases

Use `loxa.alias("name")` to create a logger that inherits the default logger's config and emits `loxa.alias` metadata. Aliases share the same service, sink, and sampler as the default.

```python
import loxa

# Configure the default logger once.
loxa.configure(
    loxa.production("api")
    .with_collector_endpoint("https://collector.loxa.dev")
    .with_api_key(os.environ["LOXA_API_KEY"])
)

# Create an alias with the same config and loxa.alias metadata.
audit = loxa.alias("audit")

# Use the alias exactly like any other logger.
ctx = audit.start_event(loxa.Params(event="audit.login", kind="http"))
audit.enrich(ctx, loxa.UserID("u-abc"), loxa.String("action", "login"))
audit.finish(ctx, "success")
audit.emit(ctx)
```

Aliases are useful when a single process emits events for multiple logical streams (for example, an API server that also writes audit trails).

## Event Lifecycle

```
start_event -> enrich -> finish -> emit
     |            |        |       |
  creates     adds      marks   serializes
  context    attrs     done     + sends
```

## Immediate Log Helpers

For simple log lines that do not need the full start/enrich/finish/emit cycle, use the shorthand helpers. Each creates an event, finishes it, and emits it in one call.

```python
import loxa

loxa.configure(loxa.dev("my-service"))

loxa.info("Server started", port=8080)
loxa.warn("Cache miss", key="user:123")
loxa.error("Request failed", status=500)
```

## Cross-Language Parity

All four LOXA SDKs follow the same API pattern. The table below shows equivalent calls.

| Operation | Python | JavaScript | Go | Rust |
|---|---|---|---|---|
| Import | `import loxa` | `import { loxa } from "loxa-js"` | `import "github.com/astraive/loxa/sdks/go"` | `use loxa_rs::prelude::*` |
| Configure default | `loxa.configure(...)` | `loxa.configure(...)` | `loxa.Configure(...)` | `loxa::configure(...)` |
| Custom instance | `loxa.create_loxa(service="x")` | `createLoxa({ service: "x" })` | `loxa.New(loxa.WithService("x"))` | `loxa::create_loxa(config)` |
| Alias | `loxa.alias("x")` | `loxa.alias("x")` | `loxa.Default().Alias("x")` | `loxa::alias("x")` |
| Start event | `loxa.start_event(Params(...))` | `loxa.startEvent({...})` | `loxa.Default().StartEvent(...)` | `loxa::start_event(...)` |
| Enrich | `loxa.enrich(ctx, ...)` | `loxa.enrich(ctx, ...)` | `loxa.Default().Enrich(ctx, ...)` | `loxa::enrich(&ctx, ...)` |
| Finish | `loxa.finish(ctx, "ok")` | `loxa.finish(ctx, "ok")` | `loxa.Default().Finish(ctx, "ok")` | `loxa::finish(&ctx, "ok")` |
| Emit | `loxa.emit(ctx)` | `await loxa.emit(ctx)` | `loxa.Default().Emit(ctx)` | `loxa::emit(&ctx)` |
| Info (shorthand) | `loxa.info("msg")` | `await loxa.info("msg")` | `loxa.Default().Info("msg")` | `loxa::info("msg")` |

## Next Steps

- [Public API](public-api.md) -- Full API reference.
- [Middleware](middleware.md) -- ASGI/WSGI framework integration.
- [Security](security.md) -- Redaction and field controls.
- [Testing](testing.md) -- Running tests and conformance.
- [Integrations](integrations.md) -- Logging framework bridges.
