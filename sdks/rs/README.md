# LOXA-RS

**Status**: STABLE (v1.0.0) - Production-ready, full feature conformance

`loxa-rs` is a collector-first Rust SDK for wide events. It builds one structured event per operation (request, job, queue message, CLI run, cron run), then emits to your log/analytics backend.

## Quick Start

```rust
use loxa::{Config, Logger, Params, String as LoxaString, Int};

fn main() {
    let logger = Logger::new(
        Config::production("checkout")
            .with_sink(loxa::StdoutSink),
    );

    let mut ctx = logger.start_event(Params::new("checkout.request").with_kind("http"));
    loxa::Append(&mut ctx, loxa::UserID("u_123"));
    loxa::Append(&mut ctx, loxa::String("payment.provider", "stripe"));
    logger.finish(&mut ctx, "success").unwrap();
    logger.emit(&ctx).unwrap();
}
```

## Core Lifecycle API

The main flow: `StartEvent` -> `Enrich` -> `Checkpoint` -> `Finish`/`FinishError` -> `Emit`

```rust
use loxa::{
    StartEvent, StartHTTPEvent, StartJobEvent, StartQueueEvent,
    StartCLIEvent, StartCronEvent,
    Append, Enrich, Set, Merge, Delete, Get, GetGroup,
    Checkpoint, Finish, FinishError, Emit, Flush, Shutdown,
    Params, EventContext,
};

// Start an event (PascalCase - Go-style)
let mut ctx = StartEvent(Params::new("checkout.request")
    .with_kind("http")
    .with_service("checkout"));

// Or snake_case (Rust-idiomatic)
let mut ctx = start_event(Params::new("checkout.request"));

// Typed starters
let mut ctx = StartHTTPEvent(Params::new("http.request"));
let mut ctx = StartJobEvent(Params::new("job.send_email"));
let mut ctx = StartQueueEvent(Params::new("queue.process"));
let mut ctx = StartCLIEvent(Params::new("cli.run"));
let mut ctx = StartCronEvent(Params::new("cron.tick"));

// Enrich (add attributes)
Append(&mut ctx, String("user.id", "u_123"));
Append(&mut ctx, Int("cart.items", 3));

// Set (override)
Set(&mut ctx, String("status", "processing"));

// Merge into group
Merge(&mut ctx, "payment", String("provider", "stripe"));

// Delete
Delete(&mut ctx, "temp_field");

// Get
let val = Get(&ctx, "user.id");
let group = GetGroup(&ctx, "payment");

// Checkpoint
Checkpoint(&mut ctx, "payment_started");
Checkpoint(&mut ctx, "payment_finished");

// Finish
Finish(&mut ctx, "success").unwrap();

// Finish with error
FinishError(&mut ctx, "payment failed").unwrap();

// Emit
Emit(&ctx).unwrap();

// Flush
Flush().unwrap();

// Shutdown
Shutdown().unwrap();
```

## Attribute Constructors

```rust
use loxa::{
    String, Int, Int64, Uint64, Float64, Bool, Time, Duration, Any, Null, Group,
};

Append(&mut ctx,
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
Append(&mut ctx,
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

Append(&mut ctx,
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

Append(&mut ctx,
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
    Ok(_) => loxa::Finish(&mut ctx, "success").unwrap(),
    Err(e) => {
        loxa::FinishError(&mut ctx, &e.to_string()).unwrap();
        Append(&mut ctx, ErrorType("ValidationError"));
        Append(&mut ctx, ErrorCode("INVALID_INPUT"));
        Append(&mut ctx, Retryable(false));
    }
}
```

## Immediate Logs

One-shot events without requiring `StartEvent`:

```rust
use loxa::{Debug, Info, Warn, Error, Fatal};

Info("worker started").unwrap();
Error("payment failed").unwrap();
```

## Logger Instances

```rust
use loxa::{Logger, Config, Dev, Production, Test, Default};

// Create logger
let logger = Logger::new(Config::production("checkout"));

// Presets
let cfg = Dev("checkout");       // pretty JSON, stdout, sync, debug level
let cfg = Production("checkout"); // compact JSON, stdout, async, info level
let cfg = Test("checkout");       // sync, no sinks, debug level

// Get default
let logger = Default();

// Instance methods
let mut ctx = logger.start_event(Params::new("checkout.request"));
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
    .with_version("1.0.0")
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
let cfg = Config::production("checkout").with_sink(HttpBatchSink("http://collector:9090/v1/events"));
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
Append(&mut ctx, SensitiveString("user.email", email));
Append(&mut ctx, HashString("user.ssn", ssn));
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

let ev = FromContext(&ctx);
let has = HasEvent(&ctx);
let eid = EventID(&ctx);
let rid = RequestIDFromContext(&ctx);
```

## Feature Flags

```rust
use loxa::{FeatureFlag, FeatureFlagBool, Experiment};

Append(&mut ctx, FeatureFlag("checkout_v2", "enabled"));
Append(&mut ctx, FeatureFlagBool("new_ui", true));
Append(&mut ctx, Experiment("pricing_test", "variant_b"));
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

## Documentation

- [Instrumentation Guide](docs/business-instrumentation.md) — instrumenting checkout, payments, auth, jobs, queues, cron
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
