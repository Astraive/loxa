# Getting Started

A 5-minute quickstart for the LOXA Rust SDK. By the end you will have a working application that creates, enriches, finishes, and emits a wide-event.

## Add Dependency

Add to your `Cargo.toml`:

```toml
[dependencies]
loxa = "1.0"
```

## Full Working Example

```rust
use loxa;

fn main() -> Result<(), loxa::LoxaError> {
    // 1. Configure the global default logger.
    loxa::configure(
        loxa::Config::production("checkout-service")
            .with_collector_endpoint("https://collector.loxa.dev"),
    )?;

    // 2. Start a new wide-event.
    let mut ctx = loxa::start_event(
        None,
        loxa::Params::new("checkout.request").with_kind("http"),
    );

    // 3. Enrich with typed attributes.
    loxa::enrich(&mut ctx, [
        loxa::UserID("u-abc123"),
        loxa::TenantID("tenant-acme"),
        loxa::String("cart.id", "cart-42"),
        loxa::Int("item_count", 3),
    ]);

    // 4. Mark the event as finished.
    loxa::finish(&mut ctx);

    // 5. Serialize and send to the configured sink.
    loxa::emit(&mut ctx)?;

    // 6. Flush buffered events and shut down.
    loxa::flush();
    loxa::shutdown();
    Ok(())
}
```

## What Each Step Does

| Step | Call | Purpose |
|------|------|---------|
| Configure | `loxa::configure(config)` | Installs a global default logger with service name, sink, sampler, and redactor. |
| Start event | `loxa::start_event(None, params)` | Creates a new `EventContext` with a UUIDv7 event ID and records the start timestamp. Pass a parent context or `None`. |
| Enrich | `loxa::enrich(&mut ctx, attrs)` | Adds one or more typed attributes to the event context. |
| Finish | `loxa::finish(&mut ctx)` | Marks the event as finished with a "success" outcome and records the duration. |
| Emit | `loxa::emit(&mut ctx)` | Serializes the event, applies redaction and sampling, and sends it to the sink. |
| Flush | `loxa::flush()` | Forces any buffered events to be written to the sink. |
| Shutdown | `loxa::shutdown()` | Flushes and closes the logger and all sinks. |

## Custom Instances

Use `loxa::create_loxa(config)` to create an isolated logger with its own config. This is useful when you need separate sinks, samplers, or service names for different subsystems.

```rust
use loxa;

fn main() -> Result<(), loxa::LoxaError> {
    // Create a dedicated logger for the billing subsystem.
    let billing = loxa::create_loxa(
        loxa::Config::production("billing-worker")
            .with_collector_endpoint("https://collector.loxa.dev")
            .with_sampler(loxa::SampleErrors()),
    );

    let mut ctx = billing.start_event(
        loxa::Params::new("invoice.generated").with_kind("job"),
    );
    billing.enrich(&mut ctx, loxa::OrderID("ORD-9876"));
    billing.finish(&mut ctx, "success")?;
    billing.emit(&ctx)?;
    billing.shutdown()?;
    Ok(())
}
```

Custom instances do not affect the global default logger. Call `loxa::configure(...)` separately if you also want module-level calls like `loxa::info(...)` to work.

## Aliases

Use `loxa::alias("service-name")` to create a logger that shares the global config but uses a different service name. This is useful for emitting events from a logical subsystem without duplicating configuration.

```rust
use loxa;

fn main() -> Result<(), loxa::LoxaError> {
    loxa::configure(
        loxa::Config::production("checkout-service")
            .with_collector_endpoint("https://collector.loxa.dev"),
    )?;

    // The audit logger inherits the same endpoint, sampler, and redactor.
    let audit = loxa::alias("audit-service");

    let mut ctx = audit.start_event(
        loxa::Params::new("permission.changed").with_kind("http"),
    );
    audit.enrich(&mut ctx, loxa::UserID("u-abc123"));
    audit.finish(&mut ctx, "success")?;
    audit.emit(&ctx)?;

    loxa::flush();
    loxa::shutdown();
    Ok(())
}
```

## Connecting to a Collector

In production, send events to the LOXA collector with authentication:

```rust
loxa::configure(
    loxa::Config::production("checkout-service")
        .with_collector_endpoint("https://collector.loxa.dev")
        .with_api_key(std::env::var("LOXA_API_KEY").unwrap_or_default())
        .with_sampler(loxa::SampleErrors()),
)?;
```

The SDK automatically sets `Authorization: Bearer <key>` headers.

### Authentication

| Config Field | Env Var | Description |
|---|---|---|
| `api_key` | `LOXA_API_KEY` | Ingest API key (`lx_sec_live_k_xxx_yyyy`) |

```rust
// Production
loxa::Config::production("my-service")
    .with_api_key("lx_sec_live_k_xxx_yyyy")

// Local dev
loxa::Config::dev("my-service")
    .with_api_key("lx_local_dev_mytoken")
```

See [Security](../../docs/security.md) for key types and RBAC roles.

## Cross-Language Parity

The same event lifecycle is available in every LOXA SDK:

| Operation | Rust | Go | Python | JavaScript |
|---|---|---|---|---|
| Configure | `loxa::configure(config)` | `loxa.Configure(config)` | `loxa.configure(config)` | `configure(config)` |
| Start event | `loxa::start_event(None, params)` | `loxa.StartEvent(ctx, params)` | `loxa.start_event(event=...)` | `startEvent(params)` |
| Enrich | `loxa::enrich(&mut ctx, attrs)` | `loxa.Enrich(ctx, attrs...)` | `loxa.enrich(ctx, attrs...)` | `enrich(ctx, attrs...)` |
| Finish | `loxa::finish(&mut ctx)` | `loxa.Finish(ctx, outcome)` | `loxa.finish(ctx, outcome)` | `finish(ctx, outcome)` |
| Emit | `loxa::emit(&mut ctx)` | `loxa.Emit(ctx)` | `loxa.emit(ctx)` | `emit(ctx)` |
| Flush | `loxa::flush()` | `loxa.Flush()` | `loxa.flush()` | `flush()` |
| Shutdown | `loxa::shutdown()` | `loxa.Shutdown()` | `loxa.shutdown()` | `shutdown()` |
| Custom instance | `loxa::create_loxa(config)` | `loxa.New(config)` | `loxa.create_loxa(config)` | `createLoxa(config)` |
| Alias | `loxa::alias("name")` | `loxa.Alias("name")` | `loxa.alias("name")` | `alias("name")` |

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
