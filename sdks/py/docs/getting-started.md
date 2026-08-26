# Getting Started

A 5-minute quickstart for the LOZA Python SDK. By the end you will have a working application that creates, enriches, finishes, and emits a wide-event.

## Default Client

Use `loza.<method>()` for quick starts and single-client applications.

## Custom Client / Alias

Use `create_loza(...)` for an independent client and `loza.alias("name")` for a same-config child that emits `loza.alias`.

## Cross-Language Parity

Python maps to the v0.0.2 parity family as `loza`, `create_loza`, optional `Loza`, and user-defined variables such as `logger.info(...)`.

## Install

From the repository root:

```bash
cd sdks/py
pip install -e .
```

Or install from a package index:

```bash
pip install loza
```

## Full Working Example

```python
import loza

# 1. Configure the default logger (stdout sink for dev).
loza.configure(
    loza.dev("checkout-service").with_sink(loza.StdoutSink())
)

# 2. Start an event.
ctx = loza.start_event(loza.Params(
    event="checkout.request",
    kind="http",
))

# 3. Enrich with typed attributes.
loza.enrich(ctx,
    loza.UserID("u-abc123"),
    loza.TenantID("tenant-acme"),
    loza.String("cart.id", "cart-42"),
    loza.Int("item_count", 3),
)

# 4. Finish the event with an outcome.
loza.finish(ctx, "success",
    loza.Int("status_code", 200),
)

# 5. Emit the event to the configured sink.
event_id = loza.emit(ctx)
print(f"Emitted event: {event_id}")

# 6. Flush buffered events and shut down.
loza.flush()
loza.shutdown()
```

Every call in this example goes through the module-level `loza.*` facade, which delegates to the global default `Logger` instance. No logger variable is needed.

## What Each Step Does

| Step | Call | Purpose |
|------|------|---------|
| Configure | `loza.configure(cfg)` | Sets the global default logger with service name, sink, sampler, and redactor. |
| Start event | `loza.start_event(Params(...))` | Creates a new `EventContext` with a UUIDv7 event ID and initial metadata. |
| Enrich | `loza.enrich(ctx, *attrs)` | Adds typed attributes to the event. Attributes are merged into the event map. |
| Finish | `loza.finish(ctx, outcome, *attrs)` | Marks the event as finished, records the outcome, and calculates duration. |
| Emit | `loza.emit(ctx)` | Serializes the event and sends it to the configured sink. Returns the event ID. |
| Flush | `loza.flush()` | Flushes any buffered events in async sinks. |
| Shutdown | `loza.shutdown()` | Shuts down the logger, flushing remaining events. |

## Connecting to a Collector

In production, send events to the LOZA collector with authentication:

```python
import os
import loza

loza.configure(
    loza.production("checkout-service")
    .with_api_key(os.environ["LOZA_API_KEY"])
    .with_collector_endpoint("https://collector.loza.dev")
    .with_sampler(loza.SampleErrors())
)
```

The SDK automatically sets `Authorization: Bearer <key>` headers.

### Authentication

| Config Field | Env Var | Description |
|---|---|---|
| `api_key` | `LOZA_API_KEY` | Ingest API key (`lz_sec_live_k_xxx_yyyy`) |

```python
import loza

# Production
loza.configure(loza.production("my-service").with_api_key("lz_sec_live_k_xxx_yyyy"))

# Local dev
loza.configure(loza.dev("my-service").with_api_key("lz_local_dev_mytoken"))
```

See [Security](../../docs/security.md) for key types and RBAC roles.

## Custom Instances

Use `loza.create_loza(service=...)` when you need an independent logger with its own config. Custom instances do not share state with the default logger.

```python
import loza

# Create an isolated logger for a specific service.
logger = loza.create_loza(service="checkout-api")

ctx = logger.start_event(loza.Params(event="checkout.charge", kind="http"))
logger.enrich(ctx, loza.OrderID("ord-99"), loza.Amount(49.99))

try:
    result = process_payment()
    logger.finish(ctx, "success", loza.String("provider", "stripe"))
except Exception as exc:
    logger.finish_error(ctx, exc)

logger.emit(ctx)
logger.shutdown()
```

You can also pass config options directly:

```python
logger = loza.create_loza(
    service="worker",
    collector_endpoint="https://collector.loza.dev",
    api_key=os.environ["LOZA_API_KEY"],
)
```

## Aliases

Use `loza.alias("name")` to create a logger that inherits the default logger's config and emits `loza.alias` metadata. Aliases share the same service, sink, and sampler as the default.

```python
import loza

# Configure the default logger once.
loza.configure(
    loza.production("api")
    .with_collector_endpoint("https://collector.loza.dev")
    .with_api_key(os.environ["LOZA_API_KEY"])
)

# Create an alias with the same config and loza.alias metadata.
audit = loza.alias("audit")

# Use the alias exactly like any other logger.
ctx = audit.start_event(loza.Params(event="audit.login", kind="http"))
audit.enrich(ctx, loza.UserID("u-abc"), loza.String("action", "login"))
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
import loza

loza.configure(loza.dev("my-service"))

loza.info("Server started", port=8080)
loza.warn("Cache miss", key="user:123")
loza.error("Request failed", status=500)
```

## Cross-Language Parity

All four LOZA SDKs follow the same API pattern. The table below shows equivalent calls.

| Operation | Python | JavaScript | Go | Rust |
|---|---|---|---|---|
| Import | `import loza` | `import { loza } from "@astraive/loza"` | `import "github.com/astraive/loza/sdks/go"` | `use loza_rs::prelude::*` |
| Configure default | `loza.configure(...)` | `loza.configure(...)` | `loza.Configure(...)` | `loza::configure(...)` |
| Custom instance | `loza.create_loza(service="x")` | `createLoza({ service: "x" })` | `loza.New(loza.WithService("x"))` | `loza::create_loza(config)` |
| Alias | `loza.alias("x")` | `loza.alias("x")` | `loza.Default().Alias("x")` | `loza::alias("x")` |
| Start event | `loza.start_event(Params(...))` | `loza.startEvent({...})` | `loza.Default().StartEvent(...)` | `loza::start_event(...)` |
| Enrich | `loza.enrich(ctx, ...)` | `loza.enrich(ctx, ...)` | `loza.Default().Enrich(ctx, ...)` | `loza::enrich(&ctx, ...)` |
| Finish | `loza.finish(ctx, "ok")` | `loza.finish(ctx, "ok")` | `loza.Default().Finish(ctx, "ok")` | `loza::finish(&ctx, "ok")` |
| Emit | `loza.emit(ctx)` | `await loza.emit(ctx)` | `loza.Default().Emit(ctx)` | `loza::emit(&ctx)` |
| Info (shorthand) | `loza.info("msg")` | `await loza.info("msg")` | `loza.Default().Info("msg")` | `loza::info("msg")` |

## Next Steps

- [Public API](public-api.md) -- Full API reference.
- [Middleware](middleware.md) -- ASGI/WSGI framework integration.
- [Security](security.md) -- Redaction and field controls.
- [Testing](testing.md) -- Running tests and conformance.
- [Integrations](integrations.md) -- Logging framework bridges.
