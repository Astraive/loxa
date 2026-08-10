# LOZA-RS

**Status**: STABLE (v0.2.6) - Production-ready, full feature conformance

`loza-rs` is a collector-first Rust SDK for wide events. It builds one structured event per operation (request, job, queue message, CLI run, cron run), then emits to your log/analytics backend.

## Quick Start

```rust
fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Configure the default logger
    loza::configure(
        loza::Config::production("checkout")
            .with_collector_endpoint("http://127.0.0.1:9308"),
    )?;

    // Lifecycle
    let mut ctx = loza::start_event(loza::Params::new("checkout.request").with_kind("http"));
    loza::enrich(&mut ctx, loza::UserID("u_123"));
    loza::enrich(&mut ctx, loza::String("payment.provider", "stripe"));
    loza::finish(&mut ctx, "success")?;
    loza::emit(&mut ctx)?;
    loza::shutdown();
    Ok(())
}
```

## Core Lifecycle API

The main flow: `start_event` -> `enrich` -> `checkpoint` -> `finish`/`finish_error` -> `emit`

```rust
use loza::{Params, EventContext};

// Start an event
let mut ctx = loza::start_event(Params::new("checkout.request")
    .with_kind("http")
    .with_service("checkout"));

// Typed starters
let mut ctx = loza::start_http_event(Params::new("http.request"));
let mut ctx = loza::start_job_event(Params::new("job.send_email"));
let mut ctx = loza::start_queue_event(Params::new("queue.process"));
let mut ctx = loza::start_cli_event(Params::new("cli.run"));
let mut ctx = loza::start_cron_event(Params::new("cron.tick"));

// Enrich (add attributes)
loza::enrich(&mut ctx, loza::String("user.id", "u_123"));
loza::enrich(&mut ctx, loza::Int("cart.items", 3));

// Set (override)
loza::set(&mut ctx, loza::String("status", "processing"));

// Merge into group
loza::merge(&mut ctx, "payment", loza::String("provider", "stripe"));

// Delete
loza::delete(&mut ctx, "temp_field");

// Get
let val = loza::get(&ctx, "user.id");
let group = loza::get_group(&ctx, "payment");

// Checkpoint
loza::checkpoint(&mut ctx, "payment_started");
loza::checkpoint(&mut ctx, "payment_finished");

// Finish
loza::finish(&mut ctx, "success").unwrap();

// Finish with error
loza::finish_error(&mut ctx, "payment failed").unwrap();

// Emit
loza::emit(&ctx).unwrap();

// Flush
loza::flush().unwrap();

// Shutdown
loza::shutdown().unwrap();
```

## Attribute Constructors

```rust
use loza::{
    String, Int, Int64, Uint64, Float64, Bool, Time, Duration, Any, Null, Group,
};

loza::enrich(&mut ctx,
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
loza::enrich(&mut ctx,
    Group("user", vec![
        String("id", "u_123"),
        String("email", "user@example.com"),
    ]),
);
```

## Canonical Helpers

```rust
use loza::{
    UserID, TenantID, WorkspaceID, OrganizationID, SessionID,
    RequestID, TraceID, SpanID,
    FeatureFlag, FeatureFlagBool, Experiment,
};

loza::enrich(&mut ctx,
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
use loza::{
    OrderID, CartID, ProductID, CustomerID,
    Plan, Currency, Amount, Country, Device, Platform, AppVersion,
};

loza::enrich(&mut ctx,
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
use loza::{ErrorType, ErrorCode, ErrorMessage, ErrorStack, Retryable};

match process() {
    Ok(_) => loza::finish(&mut ctx, "success").unwrap(),
    Err(e) => {
        loza::finish_error(&mut ctx, &e.to_string()).unwrap();
        loza::enrich(&mut ctx, ErrorType("ValidationError"));
        loza::enrich(&mut ctx, ErrorCode("INVALID_INPUT"));
        loza::enrich(&mut ctx, Retryable(false));
    }
}
```

## Immediate Logs

One-shot events without requiring `start_event`:

```rust
loza::info("worker started").unwrap();
loza::error("payment failed").unwrap();
```

## Logger Instances

```rust
// Default API -- configure once, use everywhere
loza::configure(loza::Config::production("checkout").with_collector_endpoint("http://127.0.0.1:9308"))?;
loza::info("server started");

// Custom instance
let logger = loza::create_loza(loza::Config::dev("checkout-api").with_collector_endpoint("http://127.0.0.1:9308"));
logger.info("custom instance ready");

// Alias -- same config, loza.alias metadata
let audit = loza::alias("audit-service");
audit.info("audit trail started");

// Presets
let cfg = loza::Dev("checkout");       // pretty JSON, stdout, sync, debug level
let cfg = loza::Production("checkout"); // compact JSON, stdout, async, info level
let cfg = loza::Test("checkout");       // sync, no sinks, debug level

// Get default
let logger = loza::default();

// Instance methods
let mut ctx = logger.start_event(loza::Params::new("checkout.request"));
logger.finish(&mut ctx, "success").unwrap();
logger.emit(&ctx).unwrap();
logger.flush().unwrap();
logger.shutdown().unwrap();
```

## Config API

```rust
use loza::{
    Config, Production, Dev, Test,
    WithService, WithVersion, WithEnvironment, WithSink, WithSampler,
    WithRedactor, WithSchema, WithAsync, WithDuplicatePolicy,
};

let cfg = Config::production("checkout")
    .with_version("0.0.2")
    .with_environment("prod")
    .with_sink(loza::StdoutSink)
    .with_sampler(loza::SampleErrors)
    .with_redactor(loza::DefaultRedactor)
    .with_async(true);
```

## Levels

```rust
use loza::{LevelDebug, LevelInfo, LevelWarn, LevelError, LevelFatal};
```

## Sinks

```rust
use loza::{StdoutSink, StderrSink, FileSink, MemorySink, NoopSink, HttpBatchSink};

let cfg = Config::production("checkout").with_sink(StdoutSink);
let cfg = Config::production("checkout").with_sink(FileSink("/var/log/app.log"));
let cfg = Config::production("checkout").with_sink(HttpBatchSink("http://collector:9308/events"));
```

## Sampling

```rust
use loza::{
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
use loza::{
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
loza::enrich(&mut ctx, SensitiveString("user.email", email));
loza::enrich(&mut ctx, HashString("user.ssn", ssn));
```

## Schema

```rust
use loza::{DefaultSchema, FlatSchema, OTelLogSchema, ECSchema, CustomSchema};

let cfg = Config::production("checkout").with_schema(DefaultSchema);
let cfg = Config::production("checkout").with_schema(FlatSchema);
let cfg = Config::production("checkout").with_schema(OTelLogSchema);
```

## Duplicate Policy

```rust
use loza::{CanonicalWins, UserWins, FirstWins, LastWins, KeepBoth, ErrorOnDuplicate};

let cfg = Config::production("checkout").with_duplicate_policy(CanonicalWins);
let cfg = Config::production("checkout").with_duplicate_policy(LastWins);
```

## Context Helpers

```rust
use loza::{FromContext, HasEvent, EventID, RequestIDFromContext, TraceIDFromContext};

let ev = loza::from_context(&ctx);
let has = loza::has_event(&ctx);
let eid = loza::event_id(&ctx);
let rid = loza::request_id_from_context(&ctx);
```

## Feature Flags

```rust
use loza::{FeatureFlag, FeatureFlagBool, Experiment};

loza::enrich(&mut ctx, FeatureFlag("checkout_v2", "enabled"));
loza::enrich(&mut ctx, FeatureFlagBool("new_ui", true));
loza::enrich(&mut ctx, Experiment("pricing_test", "variant_b"));
```

## Middleware

### Tower / Axum (feature-gated)

```toml
[dependencies]
loza = { version = "1.0", features = ["axum"] }
```

```rust
use loza::middleware::axum::axum_impl::{LozaLayer, loza_middleware};

let layer = LozaLayer::new(logger, "checkout");
```

### Actix-web (feature-gated)

```toml
[dependencies]
loza = { version = "1.0", features = ["actix"] }
```

```rust
use loza::middleware::actix::actix_impl::LozaMiddleware;

App::new()
    .wrap(LozaMiddleware::new(logger, "checkout"))
```

### Tower (built-in)

```rust
use loza::middleware::tower::{LozaLayer, MiddlewareConfig, capture_request};

let result = capture_request(&logger, "GET", "/health", 200)?;
```

## Testing

```rust
use loza::testkit::{test_logger, assert_contains, capture, assert_event, assert_redacted, assert_has_checkpoint};

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

The same event lifecycle is available in every LOZA SDK:

| Operation | Rust | Go | Python | JavaScript |
|---|---|---|---|---|
| Configure | `loza::configure(config)` | `loza.Configure(config)` | `loza.configure(config)` | `loza.configure(config)` |
| Start event | `loza::start_event(params)` | `loza.StartEvent(ctx, params)` | `loza.start_event(params)` | `loza.startEvent(params)` |
| Enrich | `loza::enrich(&mut ctx, attrs)` | `loza.Enrich(ctx, attrs...)` | `loza.enrich(ctx, attrs...)` | `loza.enrich(ctx, attrs...)` |
| Checkpoint | `loza::checkpoint(&mut ctx, name)` | `loza.Checkpoint(ctx, name)` | `loza.checkpoint(ctx, name)` | `loza.checkpoint(ctx, name)` |
| Finish | `loza::finish(&mut ctx, outcome)` | `loza.Finish(ctx, outcome)` | `loza.finish(ctx, outcome)` | `loza.finish(ctx, outcome)` |
| Emit | `loza::emit(&ctx)` | `loza.Emit(ctx)` | `loza.emit(ctx)` | `await loza.emit(ctx)` |
| Info (immediate) | `loza::info("msg")` | `loza.Info("msg")` | `loza.info("msg")` | `await loza.info("msg")` |
| Custom instance | `loza::create_loza(config)` | `loza.New(config)` | `loza.create_loza(config)` | `createLoza(config)` |
| Alias | `loza::alias("name")` | `loza.Alias("name")` | `loza.alias("name")` | `loza.alias("name")` |

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
