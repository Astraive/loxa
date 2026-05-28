# LOXA-RS

**Status**: STABLE (v0.2.5) - Production-ready, full feature conformance

`loxa-rs` is a collector-first Rust SDK for wide events. It builds one structured event per operation (request, job, queue message, CLI run, cron run), then emits to your log/analytics backend.

## Quick Start

```rust
fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Configure the default logger
    loxa::configure(
        loxa::Config::production("checkout")
            .with_collector_endpoint("http://127.0.0.1:9308"),
    )?;

    // Lifecycle
    let mut ctx = loxa::start_event(loxa::Params::new("checkout.request").with_kind("http"));
    loxa::enrich(&mut ctx, loxa::UserID("u_123"));
    loxa::enrich(&mut ctx, loxa::String("payment.provider", "stripe"));
    loxa::finish(&mut ctx, "success")?;
    loxa::emit(&mut ctx)?;
    loxa::shutdown();
    Ok(())
}
```

## Core Lifecycle API

The main flow: `start_event` -> `enrich` -> `checkpoint` -> `finish`/`finish_error` -> `emit`

```rust
use loxa::{Params, EventContext};

// Start an event
let mut ctx = loxa::start_event(Params::new("checkout.request")
    .with_kind("http")
    .with_service("checkout"));

// Typed starters
let mut ctx = loxa::start_http_event(Params::new("http.request"));
let mut ctx = loxa::start_job_event(Params::new("job.send_email"));
let mut ctx = loxa::start_queue_event(Params::new("queue.process"));
let mut ctx = loxa::start_cli_event(Params::new("cli.run"));
let mut ctx = loxa::start_cron_event(Params::new("cron.tick"));

// Enrich (add attributes)
loxa::enrich(&mut ctx, loxa::String("user.id", "u_123"));
loxa::enrich(&mut ctx, loxa::Int("cart.items", 3));

// Set (override)
loxa::set(&mut ctx, loxa::String("status", "processing"));

// Merge into group
loxa::merge(&mut ctx, "payment", loxa::String("provider", "stripe"));

// Delete
loxa::delete(&mut ctx, "temp_field");

// Get
let val = loxa::get(&ctx, "user.id");
let group = loxa::get_group(&ctx, "payment");

// Checkpoint
loxa::checkpoint(&mut ctx, "payment_started");
loxa::checkpoint(&mut ctx, "payment_finished");

// Finish
loxa::finish(&mut ctx, "success").unwrap();

// Finish with error
loxa::finish_error(&mut ctx, "payment failed").unwrap();

// Emit
loxa::emit(&ctx).unwrap();

// Flush
loxa::flush().unwrap();

// Shutdown
loxa::shutdown().unwrap();
```

## Attribute Constructors

```rust
use loxa::{
    String, Int, Int64, Uint64, Float64, Bool, Time, Duration, Any, Null, Group,
};

loxa::enrich(&mut ctx,
    String("user.id", "u_123"),
    Int("cart.items", 3),
    Int64("big_number", 9999999999),
    Float64("price", 49.99),
    Bool("premium", true),
    Duration("timeout", std::time::Duration::from_secs(30)),
    Any("metadata", serde_json::json!({"key": "value"})),
    Null("optional_field"),
);

// Groups (nested objects)
loxa::enrich(&mut ctx,
    Group("user", vec![
        String("id", "u_123"),
        String("email", "user@example.com"),
    ]),
);
```

## Canonical Helpers

```rust
use loxa::{
    UserID, TenantID, WorkspaceID, OrganizationID, SessionID,
    RequestID, TraceID, SpanID,
    FeatureFlag, FeatureFlagBool, Experiment,
};

loxa::enrich(&mut ctx,
    UserID("u_123"),
    TenantID("t_456"),
    WorkspaceID("w_789"),
    OrganizationID("org_abc"),
    SessionID("sess_xyz"),
    RequestID("req_123"),
    TraceID("trace_abc"),
    SpanID("span_def"),
    FeatureFlag("checkout_v2", "enabled"),
    FeatureFlagBool("new_ui", true),
    Experiment("pricing_test", "variant_b"),
);
```

## Business/Domain Helpers

```rust
use loxa::{
    OrderID, CartID, ProductID, CustomerID,
    Plan, Currency, Amount, Country, Device, Platform, AppVersion,
};

loxa::enrich(&mut ctx,
    OrderID("ord_123"),
    CartID("cart_456"),
    ProductID("prod_789"),
    CustomerID("cust_abc"),
    Plan("pro"),
    Currency("INR"),
    Amount(4999),
    Country("IN"),
    Device("mobile"),
    Platform("ios"),
    AppVersion("2.1.0"),
);
```

## Error Helpers

```rust
use loxa::{ErrorType, ErrorCode, ErrorMessage, ErrorStack, Retryable};

match process() {
    Ok(_) => loxa::finish(&mut ctx, "success").unwrap(),
    Err(e) => {
        loxa::finish_error(&mut ctx, &e.to_string()).unwrap();
        loxa::enrich(&mut ctx, ErrorType("ValidationError"));
        loxa::enrich(&mut ctx, ErrorCode("INVALID_INPUT"));
        loxa::enrich(&mut ctx, Retryable(false));
    }
}
```

## Immediate Logs

One-shot events without requiring `start_event`:

```rust
loxa::info("worker started").unwrap();
loxa::error("payment failed").unwrap();
```

## Logger Instances

```rust
// Default API -- configure once, use everywhere
loxa::configure(loxa::Config::production("checkout").with_collector_endpoint("http://127.0.0.1:9308"))?;
loxa::info("server started");

// Custom instance
let logger = loxa::create_loxa(loxa::Config::dev("checkout-api").with_collector_endpoint("http://127.0.0.1:9308"));
logger.info("custom instance ready");

// Alias -- same config, loxa.alias metadata
let audit = loxa::alias("audit-service");
audit.info("audit trail started");

// Presets
let cfg = loxa::Dev("checkout");       // pretty JSON, stdout, sync, debug level
let cfg = loxa::Production("checkout"); // compact JSON, stdout, async, info level
let cfg = loxa::Test("checkout");       // sync, no sinks, debug level

// Get default
let logger = loxa::default();

// Instance methods
let mut ctx = logger.start_event(loxa::Params::new("checkout.request"));
logger.finish(&mut ctx, "success").unwrap();
logger.emit(&ctx).unwrap();
logger.flush().unwrap();
logger.shutdown().unwrap();
```

## Config API

```rust
use loxa::{
    Config, Production, Dev, Test,
    WithService, WithVersion, WithEnvironment, WithSink, WithSampler,
    WithRedactor, WithSchema, WithAsync, WithDuplicatePolicy,
};

let cfg = Config::production("checkout")
    .with_version("0.0.2")
    .with_environment("prod")
    .with_sink(loxa::StdoutSink)
    .with_sampler(loxa::SampleErrors)
    .with_redactor(loxa::DefaultRedactor)
    .with_async(true);
```

## Levels

```rust
use loxa::{LevelDebug, LevelInfo, LevelWarn, LevelError, LevelFatal};
```

## Sinks

```rust
use loxa::{StdoutSink, StderrSink, FileSink, MemorySink, NoopSink, HttpBatchSink};

let cfg = Config::production("checkout").with_sink(StdoutSink);
let cfg = Config::production("checkout").with_sink(FileSink("/var/log/app.log"));
let cfg = Config::production("checkout").with_sink(HttpBatchSink("http://collector:9308/events"));
```

## Sampling

```rust
use loxa::{
    SampleAll, SampleNone, SampleRandom, SampleErrors,
    SampleSlowRequests, SampleStatusCodes, SampleRoutes,
    SampleUsers, SampleTenants, SampleFeatureFlag,
    AnySampler, AllSampler, NotSampler,
};

let cfg = Config::production("checkout").with_sampler(SampleAll);
let cfg = Config::production("checkout").with_sampler(SampleRandom(0.01));
let cfg = Config::production("checkout").with_sampler(SampleErrors);

// Combinators
let cfg = Config::production("checkout").with_sampler(
    AnySampler(vec![SampleErrors, SampleRandom(0.01)])
);
```

## Redaction

```rust
use loxa::{
    DefaultRedactor, RedactKeys, HashKeys, DropKeys, ComposeRedactors,
    SensitiveString, MarkSensitive, HashString,
};

let cfg = Config::production("checkout").with_redactor(DefaultRedactor);
let cfg = Config::production("checkout").with_redactor(RedactKeys(vec!["password", "token"]));
let cfg = Config::production("checkout").with_redactor(HashKeys(vec!["user.email"]));

// Compose
let cfg = Config::production("checkout").with_redactor(
    ComposeRedactors(vec![DefaultRedactor, RedactKeys(vec!["password"])])
);

// Mark fields
loxa::enrich(&mut ctx, SensitiveString("user.email", email));
loxa::enrich(&mut ctx, HashString("user.ssn", ssn));
```

## Schema

```rust
use loxa::{DefaultSchema, FlatSchema, OTelLogSchema, ECSchema, CustomSchema};

let cfg = Config::production("checkout").with_schema(DefaultSchema);
let cfg = Config::production("checkout").with_schema(FlatSchema);
let cfg = Config::production("checkout").with_schema(OTelLogSchema);
```

## Duplicate Policy

```rust
use loxa::{CanonicalWins, UserWins, FirstWins, LastWins, KeepBoth, ErrorOnDuplicate};

let cfg = Config::production("checkout").with_duplicate_policy(CanonicalWins);
let cfg = Config::production("checkout").with_duplicate_policy(LastWins);
```

## Context Helpers

```rust
use loxa::{FromContext, HasEvent, EventID, RequestIDFromContext, TraceIDFromContext};

let ev = loxa::from_context(&ctx);
let has = loxa::has_event(&ctx);
let eid = loxa::event_id(&ctx);
let rid = loxa::request_id_from_context(&ctx);
```

## Feature Flags

```rust
use loxa::{FeatureFlag, FeatureFlagBool, Experiment};

loxa::enrich(&mut ctx, FeatureFlag("checkout_v2", "enabled"));
loxa::enrich(&mut ctx, FeatureFlagBool("new_ui", true));
loxa::enrich(&mut ctx, Experiment("pricing_test", "variant_b"));
```

## Middleware

### Tower / Axum (feature-gated)

```toml
[dependencies]
loxa = { version = "1.0", features = ["axum"] }
```

```rust
use loxa::middleware::axum::axum_impl::{LoxaLayer, loxa_middleware};

let layer = LoxaLayer::new(logger, "checkout");
```

### Actix-web (feature-gated)

```toml
[dependencies]
loxa = { version = "1.0", features = ["actix"] }
```

```rust
use loxa::middleware::actix::actix_impl::LoxaMiddleware;

App::new()
    .wrap(LoxaMiddleware::new(logger, "checkout"))
```

### Tower (built-in)

```rust
use loxa::middleware::tower::{LoxaLayer, MiddlewareConfig, capture_request};

let result = capture_request(&logger, "GET", "/health", 200)?;
```

## Testing

```rust
use loxa::testkit::{test_logger, assert_contains, capture, assert_event, assert_redacted, assert_has_checkpoint};

// Test logger
let logger = test_logger("test");

// Capture events
let events = capture(|logger| {
    let mut ctx = logger.start_event(Params::new("test"));
    logger.finish(&mut ctx, "success").unwrap();
    logger.emit(&ctx).unwrap();
});

// Assert
assert_contains(&events[0], "test");
assert_event(&events[0], "outcome", "success");
assert_redacted(&events[0], "password");
assert_has_checkpoint(&events[0], "payment_started");
```

## Cross-Language Parity

The same event lifecycle is available in every LOXA SDK:

| Operation | Rust | Go | Python | JavaScript |
|---|---|---|---|---|
| Configure | `loxa::configure(config)` | `loxa.Configure(config)` | `loxa.configure(config)` | `loxa.configure(config)` |
| Start event | `loxa::start_event(params)` | `loxa.StartEvent(ctx, params)` | `loxa.start_event(params)` | `loxa.startEvent(params)` |
| Enrich | `loxa::enrich(&mut ctx, attrs)` | `loxa.Enrich(ctx, attrs...)` | `loxa.enrich(ctx, attrs...)` | `loxa.enrich(ctx, attrs...)` |
| Checkpoint | `loxa::checkpoint(&mut ctx, name)` | `loxa.Checkpoint(ctx, name)` | `loxa.checkpoint(ctx, name)` | `loxa.checkpoint(ctx, name)` |
| Finish | `loxa::finish(&mut ctx, outcome)` | `loxa.Finish(ctx, outcome)` | `loxa.finish(ctx, outcome)` | `loxa.finish(ctx, outcome)` |
| Emit | `loxa::emit(&ctx)` | `loxa.Emit(ctx)` | `loxa.emit(ctx)` | `await loxa.emit(ctx)` |
| Info (immediate) | `loxa::info("msg")` | `loxa.Info("msg")` | `loxa.info("msg")` | `await loxa.info("msg")` |
| Custom instance | `loxa::create_loxa(config)` | `loxa.New(config)` | `loxa.create_loxa(config)` | `createLoxa(config)` |
| Alias | `loxa::alias("name")` | `loxa.Alias("name")` | `loxa.alias("name")` | `loxa.alias("name")` |

## Documentation

- [Instrumentation Guide](docs/business-instrumentation.md) -- instrumenting checkout, payments, auth, jobs, queues, cron
- [docs/SDK.md](docs/SDK.md)
- [docs/getting-started.md](docs/getting-started.md)

## Run Tests

```bash
cargo test
python ../../spec/conformance/runner.py --sdk rust --group all
```

## Scope

This crate emits events to the collector. Heavy delivery and storage remain collector-owned:

- Kafka
- OTLP fanout
- ClickHouse/Postgres/DuckDB ownership
- S3/GCS archival
