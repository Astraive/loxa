# Getting Started

A 5-minute quickstart for the LOXA Rust SDK. By the end you will have a working application that creates, enriches, finishes, and emits a wide-event.

## Add Dependency

Add to your `Cargo.toml`:

```toml
[dependencies]
loxa = "1.0"
serde_json = "1"
```

## Full Working Example

```rust
use loxa::{Config, Logger, Params};

fn main() {
    // 1. Create a logger with a stdout sink (dev mode).
    let logger = Logger::new(Config::dev("checkout-service"));

    // 2. Start an event.
    let mut ctx = logger.start_event(Params::new("checkout.request").with_kind("http"));

    // 3. Enrich with attributes.
    logger.append(&mut ctx, loxa::UserID("u-abc123"));
    logger.append(&mut ctx, loxa::TenantID("tenant-acme"));
    logger.append(&mut ctx, loxa::String("cart.id", "cart-42"));
    logger.append(&mut ctx, loxa::Int("item_count", 3));

    // 4. Finish the event with an outcome.
    logger.finish(&mut ctx, "success");

    // 5. Emit the event to the configured sink.
    match logger.emit(&ctx) {
        Ok(encoded) => println!("Emitted {} bytes", encoded.len()),
        Err(e) => eprintln!("Emit error: {}", e),
    }

    // 6. Flush and shut down.
    logger.flush();
    logger.shutdown();
}
```

## What Each Step Does

| Step | Function | Purpose |
|------|----------|---------|
| Logger::new | `Logger::new(config)` | Creates a logger with service name, sink, sampler, and redactor. |
| start_event | `logger.start_event(params)` | Creates a new `EventContext` with a UUIDv7 event ID. |
| append | `logger.append(&mut ctx, attr)` | Adds a typed attribute to the event. |
| finish | `logger.finish(&mut ctx, outcome)` | Marks the event as finished, records outcome and duration. |
| emit | `logger.emit(&ctx)` | Serializes the event and sends it to the sink. Returns encoded JSON. |
| flush | `logger.flush()` | Flushes buffered events. |
| shutdown | `logger.shutdown()` | Shuts down the logger. |

## Connecting to a Collector

In production, send events to the LOXA collector:

```rust
use loxa::{Config, Logger, HTTPBatchSink, SampleErrors};

let logger = Logger::new(
    Config::production("checkout-service")
        .with_sink(HTTPBatchSink("http://collector:9090/v1/events"))
        .with_sampler(SampleErrors()),
);
```

## Using the Global Default Logger

The SDK provides free functions that delegate to a global default logger:

```rust
use loxa::{configure, production, start_event, enrich, finish, emit, flush, shutdown};

fn main() {
    let _logger = configure(production("my-service")).unwrap();

    let mut ctx = start_event(None, loxa::Params::new("my.event"));
    enrich(&mut ctx, vec![loxa::String("key", "value")]);
    finish(&mut ctx);
    emit(&mut ctx).unwrap();
    flush();
    shutdown();
}
```

## Event Lifecycle

```
start_event -> append -> finish -> emit
     |            |        |       |
  creates     adds      marks   serializes
  context    attrs     done     + sends
```

## Next Steps

- [Benchmarking](benchmarking.md) -- Running Rust benchmarks.
