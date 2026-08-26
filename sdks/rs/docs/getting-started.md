# Getting Started

A 5-minute quickstart for the LOZA Rust SDK. By the end you will have a working application that creates, enriches, finishes, and emits a wide-event.

## Default Client

Use module-level `loza::<method>()` calls for quick starts and single-client applications.

## Custom Client / Alias

Use `create_loza(config)` for an independent client, `Loza::new(config)`/`Logger::new(config)` for idiomatic construction, and `loza::alias("name")` for a same-config child that emits `loza.alias`.

## Cross-Language Parity

Rust maps to the v0.0.2 parity family as module-level `loza`, `create_loza`, constructor APIs, and user-defined variables such as `logger.info(...)`.

## Add Dependency

Add to your `Cargo.toml`:

```toml
[dependencies]
loza = "1.0"
```

## Full Working Example

```rust
use loza;

fn main() -> Result<(), loza::LozaError> {
    // 1. Configure the global default logger.
    loza::configure(
        loza::Config::production("checkout-service")
            .with_collector_endpoint("https://collector.loza.dev"),
    )?;

    // 2. Start a new wide-event.
    let mut ctx = loza::start_event(
        None,
        loza::Params::new("checkout.request").with_kind("http"),
    );

    // 3. Enrich with typed attributes.
    loza::enrich(&mut ctx, [
        loza::UserID("u-abc123"),
        loza::TenantID("tenant-acme"),
        loza::String("cart.id", "cart-42"),
        loza::Int("item_count", 3),
    ]);

    // 4. Mark the event as finished.
    loza::finish(&mut ctx);

    // 5. Serialize and send to the configured sink.
    loza::emit(&mut ctx)?;

    // 6. Flush buffered events and shut down.
    loza::flush();
    loza::shutdown();
    Ok(())
}
```

## What Each Step Does

| Step | Call | Purpose |
|------|------|---------|
| Configure | `loza::configure(config)` | Installs a global default logger with service name, sink, sampler, and redactor. |
| Start event | `loza::start_event(None, params)` | Creates a new `EventContext` with a UUIDv7 event ID and records the start timestamp. Pass a parent context or `None`. |
| Enrich | `loza::enrich(&mut ctx, attrs)` | Adds one or more typed attributes to the event context. |
| Finish | `loza::finish(&mut ctx)` | Marks the event as finished with a "success" outcome and records the duration. |
| Emit | `loza::emit(&mut ctx)` | Serializes the event, applies redaction and sampling, and sends it to the sink. |
| Flush | `loza::flush()` | Forces any buffered events to be written to the sink. |
| Shutdown | `loza::shutdown()` | Flushes and closes the logger and all sinks. |

## Custom Instances

Use `loza::create_loza(config)` to create an isolated logger with its own config. This is useful when you need separate sinks, samplers, or service names for different subsystems.

```rust
use loza;

fn main() -> Result<(), loza::LozaError> {
    // Create a dedicated logger for the billing subsystem.
    let billing = loza::create_loza(
        loza::Config::production("billing-worker")
            .with_collector_endpoint("https://collector.loza.dev")
            .with_sampler(loza::SampleErrors()),
    );

    let mut ctx = billing.start_event(
        loza::Params::new("invoice.generated").with_kind("job"),
    );
    billing.enrich(&mut ctx, loza::OrderID("ORD-9876"));
    billing.finish(&mut ctx, "success")?;
    billing.emit(&ctx)?;
    billing.shutdown()?;
    Ok(())
}
```

Custom instances do not affect the global default logger. Call `loza::configure(...)` separately if you also want module-level calls like `loza::info(...)` to work.

## Aliases

Use `loza::alias("name")` to create a logger that shares the global config and emits `loza.alias` metadata. This is useful for emitting events from a logical subsystem without duplicating configuration.

```rust
use loza;

fn main() -> Result<(), loza::LozaError> {
    loza::configure(
        loza::Config::production("checkout-service")
            .with_collector_endpoint("https://collector.loza.dev"),
    )?;

    // The audit logger inherits the same endpoint, sampler, and redactor.
    let audit = loza::alias("audit-service");

    let mut ctx = audit.start_event(
        loza::Params::new("permission.changed").with_kind("http"),
    );
    audit.enrich(&mut ctx, loza::UserID("u-abc123"));
    audit.finish(&mut ctx, "success")?;
    audit.emit(&ctx)?;

    loza::flush();
    loza::shutdown();
    Ok(())
}
```

## Connecting to a Collector

In production, send events to the LOZA collector with authentication:

```rust
loza::configure(
    loza::Config::production("checkout-service")
        .with_collector_endpoint("https://collector.loza.dev")
        .with_api_key(std::env::var("LOZA_API_KEY").unwrap_or_default())
        .with_sampler(loza::SampleErrors()),
)?;
```

The SDK automatically sets `Authorization: Bearer <key>` headers.

### Authentication

| Config Field | Env Var | Description |
|---|---|---|
| `api_key` | `LOZA_API_KEY` | Ingest API key (`lz_sec_live_k_xxx_yyyy`) |

```rust
// Production
loza::Config::production("my-service")
    .with_api_key("lz_sec_live_k_xxx_yyyy")

// Local dev
loza::Config::dev("my-service")
    .with_api_key("lz_local_dev_mytoken")
```

See [Security](../../docs/security.md) for key types and RBAC roles.

## Cross-Language Parity

The same event lifecycle is available in every LOZA SDK:

| Operation | Rust | Go | Python | JavaScript |
|---|---|---|---|---|
| Configure | `loza::configure(config)` | `loza.Configure(config)` | `loza.configure(config)` | `configure(config)` |
| Start event | `loza::start_event(None, params)` | `loza.StartEvent(ctx, params)` | `loza.start_event(event=...)` | `startEvent(params)` |
| Enrich | `loza::enrich(&mut ctx, attrs)` | `loza.Enrich(ctx, attrs...)` | `loza.enrich(ctx, attrs...)` | `enrich(ctx, attrs...)` |
| Finish | `loza::finish(&mut ctx)` | `loza.Finish(ctx, outcome)` | `loza.finish(ctx, outcome)` | `finish(ctx, outcome)` |
| Emit | `loza::emit(&mut ctx)` | `loza.Emit(ctx)` | `loza.emit(ctx)` | `emit(ctx)` |
| Flush | `loza::flush()` | `loza.Flush()` | `loza.flush()` | `flush()` |
| Shutdown | `loza::shutdown()` | `loza.Shutdown()` | `loza.shutdown()` | `shutdown()` |
| Custom instance | `loza::create_loza(config)` | `loza.New(config)` | `loza.create_loza(config)` | `createLoza(config)` |
| Alias | `loza::alias("name")` | `loza.Alias("name")` | `loza.alias("name")` | `alias("name")` |

## Event Lifecycle

```
start_event -> enrich -> finish -> emit
     |           |        |       |
  creates     adds      marks   serializes
  context    attrs     done     + sends
```

## Next Steps

- [Authentication](../../docs/authentication.md) -- API keys, RBAC roles, and ABAC boundaries
- [Business Instrumentation](../../docs/business-instrumentation.md) -- Checkout, payments, auth, jobs, queues, cron patterns
- [Benchmarking](benchmarking.md) -- Running Rust benchmarks
