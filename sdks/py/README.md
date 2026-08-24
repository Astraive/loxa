# LOZA Python SDK

**Status**: STABLE (v0.3.2) - Production-ready, full feature conformance

Full API conformance with specification is complete. See [SDK_CONFORMANCE_CONTRACT.md](../../spec/docs/SDK_CONFORMANCE_CONTRACT.md) for detailed guarantees.

`loza-py` is a collector-first Python SDK for wide events. It builds one structured event per operation (request, job, queue message, CLI run, cron run), then emits to your log/analytics backend.

## Install

```bash
pip install -e .
```

## Quick Start

```python
import loza

# Configure the default logger
loza.configure(
    loza.production("checkout")
    .with_collector_endpoint("http://127.0.0.1:9308")
)

# Lifecycle
ctx = loza.start_event(loza.Params(event="checkout.request", kind="http", method="POST", path="/checkout"))
loza.enrich(ctx, loza.UserID("u_123"), loza.String("payment.provider", "stripe"))
loza.finish(ctx, "success", loza.Int("status_code", 200))
loza.emit(ctx)
loza.shutdown()
```

## Core Lifecycle API

The main flow: `start_event` -> `enrich` -> `checkpoint` -> `finish`/`finish_error` -> `emit`

```python
import loza

# Start an event (returns context carrying the event)
ctx = loza.start_event(loza.Params(
    event="checkout.request",
    kind="http",
    method="POST",
    path="/checkout",
    service="checkout",
))

# Typed starters
ctx = loza.start_http_event(loza.Params(event="http.request", method="GET", path="/health"))
ctx = loza.start_job_event(loza.Params(event="job.send_email"))
ctx = loza.start_queue_event(loza.Params(event="queue.process"))
ctx = loza.start_cli_event(loza.Params(event="cli.run"))
ctx = loza.start_cron_event(loza.Params(event="cron.tick"))

# Enrich (add attributes)
loza.enrich(ctx, loza.String("user.id", "u_123"), loza.Int("cart.items", 3))

# Append (alias for enrich)
loza.append(ctx, loza.String("key", "value"))

# Set (override)
loza.set(ctx, loza.String("status", "processing"))

# Merge into group
loza.merge(ctx, "payment", loza.String("provider", "stripe"), loza.Int("attempt", 1))

# Delete
loza.delete(ctx, "temp_field")

# Get
val = loza.get(ctx, "user.id")
group = loza.get_group(ctx, "payment")

# Checkpoint (timeline marker)
loza.checkpoint(ctx, "payment_started")
loza.checkpoint(ctx, "payment_finished", loza.String("provider", "stripe"))

# Finish
loza.finish(ctx, "success", loza.Int("status_code", 200))

# Or finish with error
try:
    process_payment()
except Exception as e:
    loza.finish_error(ctx, e, loza.Int("status_code", 500))

# Emit (sends to sink)
loza.emit(ctx)

# Flush (force buffered events)
loza.flush()

# Shutdown (drain pipeline)
loza.shutdown()
```

## Attribute Constructors

```python
import loza

loza.enrich(ctx,
    loza.String("user.id", "u_123"),
    loza.Int("cart.items", 3),
    loza.Int64("big_number", 9999999999),
    loza.Float64("price", 49.99),
    loza.Bool("premium", True),
    loza.Duration("timeout", timedelta(seconds=30)),
    loza.Any("metadata", {"key": "value"}),
    loza.Null("optional_field"),
)

# Groups (nested objects)
loza.enrich(ctx,
    loza.Group("user",
        loza.String("id", "u_123"),
        loza.String("email", "user@example.com"),
    ),
    loza.Group("payment",
        loza.String("provider", "stripe"),
        loza.Int("attempt", 1),
    ),
)
```

Dot keys expand into nested JSON:

```python
loza.String("user.id", "u_123")  # -> {"user": {"id": "u_123"}}
```

## Canonical Helpers

```python
import loza

loza.enrich(ctx,
    loza.UserID("u_123"),
    loza.TenantID("t_456"),
    loza.WorkspaceID("w_789"),
    loza.OrganizationID("org_abc"),
    loza.SessionID("sess_xyz"),
    loza.RequestID("req_123"),
    loza.TraceID("trace_abc"),
    loza.SpanID("span_def"),
    loza.FeatureFlag("checkout_v2", "enabled"),
    loza.FeatureFlagBool("new_ui", True),
    loza.Experiment("pricing_test", "variant_b"),
)
```

## Business/Domain Helpers

```python
import loza

loza.enrich(ctx,
    loza.OrderID("ord_123"),
    loza.CartID("cart_456"),
    loza.ProductID("prod_789"),
    loza.CustomerID("cust_abc"),
    loza.Plan("pro"),
    loza.Currency("INR"),
    loza.Amount(4999),
    loza.Country("IN"),
    loza.Device("mobile"),
    loza.Platform("ios"),
    loza.AppVersion("2.1.0"),
)
```

## Error Helpers

```python
import loza

try:
    process()
except Exception as e:
    loza.finish_error(ctx, e,
        loza.ErrorType("ValidationError"),
        loza.ErrorCode("INVALID_INPUT"),
        loza.ErrorMessage(str(e)),
        loza.ErrorStack(traceback.format_exc()),
        loza.Retryable(False),
    )
```

Error output:

```json
{
  "outcome": "error",
  "level": "error",
  "error": {
    "type": "ValidationError",
    "message": "...",
    "code": "INVALID_INPUT",
    "retryable": false
  }
}
```

## Immediate Logs

One-shot events without requiring `StartEvent`:

```python
import loza

loza.info("worker started", queue="emails")
loza.error("payment failed", provider="stripe", amount=4999)
```

## Logger Instances

```python
import loza

# Default API — configure once, use everywhere
loza.configure(loza.production("checkout").with_collector_endpoint("http://127.0.0.1:9308"))
loza.info("server started")

# Custom instance
logger = loza.create_loza(service="checkout-api", collector_endpoint="http://127.0.0.1:9308")
logger.info("custom instance ready")

# Alias -- same config, loza.alias metadata
audit = loza.alias("audit-service")
audit.info("audit trail started")

# Presets
cfg = loza.dev("checkout")       # pretty JSON, stdout, sync, debug level
cfg = loza.production("checkout") # compact JSON, stdout, async, info level
cfg = loza.test("checkout")       # sync, no sinks, debug level

# Instance methods
ctx = logger.start_event(loza.Params(event="checkout.request"))
logger.enrich(ctx, loza.String("key", "value"))
logger.finish(ctx, "success")
logger.emit(ctx)
logger.flush()
logger.shutdown()

# Immediate logs on instance
logger.info("started")
logger.error("failed")
```

## Config API

```python
import loza

cfg = loza.Config(
    service="checkout",
    version="1.2.0",
    environment="prod",
    region="ap-south-1",
    level=loza.LevelInfo,
    sinks=[loza.StdoutSink()],
    async_config=loza.AsyncConfig(enabled=True, queue_size=8192, workers=2),
    field_naming=loza.FieldNamingConfig(expand_dot_keys=True),
    security=loza.SecurityConfig(max_field_bytes=4096, max_event_bytes=262144),
)

# Builder style
cfg = (loza.production("checkout")
    .with_version("1.2.0")
    .with_environment("prod")
    .with_sink(loza.StdoutSink())
    .with_sampler(loza.SampleErrors())
    .with_redactor(loza.DefaultRedactor())
    .with_async(True)
    .with_duplicate_policy(loza.LastWins)
)
```

## Levels

```python
import loza

level = loza.ParseLevel("info")  # loza.LevelInfo
```

## Sinks

```python
import loza

cfg = loza.production("checkout").with_sink(loza.StdoutSink())
cfg = loza.production("checkout").with_sink(loza.FileSink("/var/log/app.log"))
cfg = loza.production("checkout").with_sink(loza.HTTPBatchSink("http://collector:9308/events"))

# For testing
sink, store = loza.MemorySink()
logger = loza.create_loza(service="checkout", sink=sink)
# ... use logger ...
events = store.events()
```

## Sampling

```python
import loza
from datetime import timedelta

cfg = loza.production("checkout").with_sampler(loza.SampleAll())
cfg = loza.production("checkout").with_sampler(loza.SampleRandom(0.01))  # 1% sampling
cfg = loza.production("checkout").with_sampler(loza.SampleErrors())
cfg = loza.production("checkout").with_sampler(loza.SampleSlowRequests(timedelta(milliseconds=500)))
cfg = loza.production("checkout").with_sampler(loza.SampleStatusCodes(500, 502, 503))
cfg = loza.production("checkout").with_sampler(loza.SampleRoutes("/checkout", "/payment"))
cfg = loza.production("checkout").with_sampler(loza.SampleUsers("u_1", "u_2"))
cfg = loza.production("checkout").with_sampler(loza.SampleRateLimited(100.0, 1.0))  # 100 events/sec

# Combinators
cfg = loza.production("checkout").with_sampler(
    loza.AnySampler(
        loza.SampleErrors(),
        loza.SampleSlowRequests(timedelta(milliseconds=500)),
        loza.SampleRandom(0.01),
    )
)
```

## Redaction

```python
import loza

cfg = loza.production("checkout").with_redactor(loza.DefaultRedactor())
cfg = loza.production("checkout").with_redactor(loza.RedactKeys("password", "token", "authorization"))
cfg = loza.production("checkout").with_redactor(loza.HashKeys("user.email"))

# Compose
cfg = loza.production("checkout").with_redactor(
    loza.ComposeRedactors(
        loza.DefaultRedactor(),
        loza.RedactKeys("password", "token"),
        loza.HashKeys("user.email"),
    )
)

# Mark fields as sensitive
loza.enrich(ctx,
    loza.SensitiveString("user.email", email),  # auto-redacted
    loza.MarkSensitive("credit_card", card_no),
    loza.HashString("user.ssn", ssn),
)
```

## Schema

```python
import loza

cfg = loza.production("checkout").with_schema(loza.DefaultSchema())
cfg = loza.production("checkout").with_schema(loza.FlatSchema())  # good for ClickHouse
cfg = loza.production("checkout").with_schema(loza.OTelLogSchema())
cfg = loza.production("checkout").with_schema(loza.ECSchema())

# Custom schema
def my_schema(ev):
    return {
        "ts": ev.timestamp(),
        "service": ev.service(),
        "requestId": ev.request_id(),
        "took": ev.duration_ms(),
        "fields": ev.attrs(),
    }

cfg = loza.production("checkout").with_schema(loza.CustomSchema(my_schema))
```

## Duplicate Policy

```python
import loza

cfg = loza.production("checkout").with_duplicate_policy(loza.CanonicalWins)  # default
cfg = loza.production("checkout").with_duplicate_policy(loza.LastWins)      # user attrs win
cfg = loza.production("checkout").with_duplicate_policy(loza.ErrorOnDuplicate)  # strict
```

## Middleware

```python
# Flask
from loza.middleware.flask.middleware import LozaMiddleware
app.wsgi_app = LozaMiddleware(app.wsgi_app, service="checkout")

# FastAPI / Starlette (ASGI)
from loza.middleware.asgi.middleware import Middleware as AsgiMiddleware
app.add_middleware(AsgiMiddleware, service="checkout")

# Django
from loza.middleware.django.middleware import LozaMiddleware
# Add to MIDDLEWARE in settings.py
```

## Feature Flags

```python
import loza

loza.enrich(ctx,
    loza.FeatureFlag("checkout_v2", "enabled"),
    loza.FeatureFlagBool("new_ui", True),
    loza.Experiment("pricing_test", "variant_b"),
)
```

## Security

```python
import loza

cfg = loza.production("checkout").with_security(loza.SecurityConfig(
    redact_by_default=True,
    allow_pii=False,
    max_field_bytes=4096,
    max_event_bytes=262144,
    max_attr_count=512,
    drop_oversized_events=True,
))

loza.enrich(ctx,
    loza.SensitiveString("user.email", email),
    loza.HashString("user.ssn", ssn),
)
```

## Context Helpers

```python
import loza

ev, ok = loza.FromContext(ctx)
if loza.HasEvent(ctx):
    eid = loza.EventID(ctx)
    rid = loza.RequestIDFromContext(ctx)
```

## Testing

```python
from loza.testkit.helpers import TestLogger, Capture, AssertEvent, AssertRedacted, AssertHasCheckpoint, CapturingLogger

# Test logger with memory sink
logger, store = TestLogger("test")

# Capture events
events = Capture(lambda: some_function())

# Assert
AssertEvent(events[0], user__id="u_123")
AssertRedacted(events[0], "password")
AssertHasCheckpoint(events[0], "payment_started")

# Context manager
with CapturingLogger("test") as (logger, store):
    ctx = logger.start_event(loza.Params(event="test"))
    logger.finish(ctx, "success")
    logger.emit(ctx)
events = store.events()
```

## Testing / Conformance

```bash
python -m pytest -q
python ../../spec/conformance/runner.py --sdk python --group all
```

## Docs

- [Instrumentation Guide](docs/business-instrumentation.md) — instrumenting checkout, payments, auth, jobs, queues, cron
- [docs/public-api.md](docs/public-api.md)
- [docs/getting-started.md](docs/getting-started.md)
- [docs/middleware.md](docs/middleware.md)
- [docs/integrations.md](docs/integrations.md)
- [docs/security.md](docs/security.md)
- [docs/testing.md](docs/testing.md)

## Cross-Language Parity

| Feature                   | Python               | Go                   | JavaScript           | Rust                 |
|---------------------------|----------------------|----------------------|----------------------|----------------------|
| Module facade             | `import loza`        | `import "loza"`      | `import loza`        | `use loza`           |
| Configure                 | `loza.configure()`   | `loza.Configure()`   | `loza.configure()`   | `loza::configure()`  |
| Start event               | `loza.start_event()` | `loza.StartEvent()`  | `loza.startEvent()`  | `loza::start_event()`|
| Enrich                    | `loza.enrich()`      | `loza.Enrich()`      | `loza.enrich()`      | `loza::enrich()`     |
| Checkpoint                | `loza.checkpoint()`  | `loza.Checkpoint()`  | `loza.checkpoint()`  | `loza::checkpoint()` |
| Finish                    | `loza.finish()`      | `loza.Finish()`      | `loza.finish()`      | `loza::finish()`     |
| Emit                      | `loza.emit()`        | `loza.Emit()`        | `loza.emit()`        | `loza::emit()`       |
| Shutdown                  | `loza.shutdown()`    | `loza.Shutdown()`    | `loza.shutdown()`    | `loza::shutdown()`   |
| Create instance           | `loza.create_loza()` | `loza.NewLogger()`   | `loza.createLoza()`  | `loza::new()`        |
| Alias                     | `loza.alias()`       | `loza.Alias()`       | `loza.alias()`       | `loza::alias()`      |
| String attr               | `loza.String()`      | `loza.String()`      | `loza.string()`      | `loza::String()`     |
| Int attr                  | `loza.Int()`         | `loza.Int()`         | `loza.int()`         | `loza::Int()`        |
| User ID                   | `loza.UserID()`      | `loza.UserID()`      | `loza.userID()`      | `loza::UserID()`     |
| Feature flag              | `loza.FeatureFlag()` | `loza.FeatureFlag()` | `loza.featureFlag()` | `loza::FeatureFlag()`|
| Middleware                 | `loza.middleware.*`  | `loza/http`          | `loza/middleware`    | `loza::middleware`   |
| Test kit                  | `loza.testkit`       | `loza/testkit`       | `loza/testkit`       | `loza::testkit`      |
