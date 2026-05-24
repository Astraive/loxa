# LOXA Python SDK

**Status**: STABLE (v0.0.1) - Production-ready, full feature conformance

Full API conformance with specification is complete. See [SDK_CONFORMANCE_CONTRACT.md](../../spec/docs/SDK_CONFORMANCE_CONTRACT.md) for detailed guarantees.

`loxa-py` is a collector-first Python SDK for wide events. It builds one structured event per operation (request, job, queue message, CLI run, cron run), then emits to your log/analytics backend.

## Install

```bash
pip install -e .
```

## Quick Start

```python
import loxa

# Configure the default logger
loxa.configure(
    loxa.production("checkout")
    .with_collector_endpoint("http://127.0.0.1:9090")
)

# Lifecycle
ctx = loxa.start_http_event(event="checkout.request", method="POST", path="/checkout")
loxa.enrich(ctx, loxa.UserID("u_123"), loxa.String("payment.provider", "stripe"))
loxa.finish(ctx, "success", loxa.Int("status_code", 200))
loxa.emit(ctx)
loxa.shutdown()
```

## Core Lifecycle API

The main flow: `start_event` -> `enrich` -> `checkpoint` -> `finish`/`finish_error` -> `emit`

```python
import loxa

# Start an event (returns context carrying the event)
ctx = loxa.start_event(loxa.Params(
    event="checkout.request",
    kind="http",
    method="POST",
    path="/checkout",
    service="checkout",
))

# Typed starters
ctx = loxa.start_http_event(loxa.Params(event="http.request", method="GET", path="/health"))
ctx = loxa.start_job_event(loxa.Params(event="job.send_email"))
ctx = loxa.start_queue_event(loxa.Params(event="queue.process"))
ctx = loxa.start_cli_event(loxa.Params(event="cli.run"))
ctx = loxa.start_cron_event(loxa.Params(event="cron.tick"))

# Enrich (add attributes)
loxa.enrich(ctx, loxa.String("user.id", "u_123"), loxa.Int("cart.items", 3))

# Append (alias for enrich)
loxa.append(ctx, loxa.String("key", "value"))

# Set (override)
loxa.set(ctx, loxa.String("status", "processing"))

# Merge into group
loxa.merge(ctx, "payment", loxa.String("provider", "stripe"), loxa.Int("attempt", 1))

# Delete
loxa.delete(ctx, "temp_field")

# Get
val = loxa.get(ctx, "user.id")
group = loxa.get_group(ctx, "payment")

# Checkpoint (timeline marker)
loxa.checkpoint(ctx, "payment_started")
loxa.checkpoint(ctx, "payment_finished", loxa.String("provider", "stripe"))

# Finish
loxa.finish(ctx, "success", loxa.Int("status_code", 200))

# Or finish with error
try:
    process_payment()
except Exception as e:
    loxa.finish_error(ctx, e, loxa.Int("status_code", 500))

# Emit (sends to sink)
loxa.emit(ctx)

# Flush (force buffered events)
loxa.flush()

# Shutdown (drain pipeline)
loxa.shutdown()
```

## Attribute Constructors

```python
import loxa

loxa.enrich(ctx,
    loxa.String("user.id", "u_123"),
    loxa.Int("cart.items", 3),
    loxa.Int64("big_number", 9999999999),
    loxa.Float64("price", 49.99),
    loxa.Bool("premium", True),
    loxa.Duration("timeout", timedelta(seconds=30)),
    loxa.Any("metadata", {"key": "value"}),
    loxa.Null("optional_field"),
)

# Groups (nested objects)
loxa.enrich(ctx,
    loxa.Group("user",
        loxa.String("id", "u_123"),
        loxa.String("email", "user@example.com"),
    ),
    loxa.Group("payment",
        loxa.String("provider", "stripe"),
        loxa.Int("attempt", 1),
    ),
)
```

Dot keys expand into nested JSON:

```python
loxa.String("user.id", "u_123")  # -> {"user": {"id": "u_123"}}
```

## Canonical Helpers

```python
import loxa

loxa.enrich(ctx,
    loxa.UserID("u_123"),
    loxa.TenantID("t_456"),
    loxa.WorkspaceID("w_789"),
    loxa.OrganizationID("org_abc"),
    loxa.SessionID("sess_xyz"),
    loxa.RequestID("req_123"),
    loxa.TraceID("trace_abc"),
    loxa.SpanID("span_def"),
    loxa.FeatureFlag("checkout_v2", "enabled"),
    loxa.FeatureFlagBool("new_ui", True),
    loxa.Experiment("pricing_test", "variant_b"),
)
```

## Business/Domain Helpers

```python
import loxa

loxa.enrich(ctx,
    loxa.OrderID("ord_123"),
    loxa.CartID("cart_456"),
    loxa.ProductID("prod_789"),
    loxa.CustomerID("cust_abc"),
    loxa.Plan("pro"),
    loxa.Currency("INR"),
    loxa.Amount(4999),
    loxa.Country("IN"),
    loxa.Device("mobile"),
    loxa.Platform("ios"),
    loxa.AppVersion("2.1.0"),
)
```

## Error Helpers

```python
import loxa

try:
    process()
except Exception as e:
    loxa.finish_error(ctx, e,
        loxa.ErrorType("ValidationError"),
        loxa.ErrorCode("INVALID_INPUT"),
        loxa.ErrorMessage(str(e)),
        loxa.ErrorStack(traceback.format_exc()),
        loxa.Retryable(False),
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
import loxa

loxa.info("worker started", queue="emails")
loxa.error("payment failed", provider="stripe", amount=4999)
```

## Logger Instances

```python
import loxa

# Default API — configure once, use everywhere
loxa.configure(loxa.production("checkout").with_collector_endpoint("http://127.0.0.1:9090"))
loxa.info("server started")

# Custom instance
logger = loxa.create_loxa(service="checkout-api", collector_endpoint="http://127.0.0.1:9090")
logger.info("custom instance ready")

# Alias -- same config, loxa.alias metadata
audit = loxa.alias("audit-service")
audit.info("audit trail started")

# Presets
cfg = loxa.dev("checkout")       # pretty JSON, stdout, sync, debug level
cfg = loxa.production("checkout") # compact JSON, stdout, async, info level
cfg = loxa.test("checkout")       # sync, no sinks, debug level

# Instance methods
ctx = logger.start_event(loxa.Params(event="checkout.request"))
logger.enrich(ctx, loxa.String("key", "value"))
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
import loxa

cfg = loxa.Config(
    service="checkout",
    version="1.2.0",
    environment="prod",
    region="ap-south-1",
    level=loxa.LevelInfo,
    sinks=[loxa.StdoutSink()],
    async_config=loxa.AsyncConfig(enabled=True, queue_size=8192, workers=2),
    field_naming=loxa.FieldNamingConfig(expand_dot_keys=True),
    security=loxa.SecurityConfig(max_field_bytes=4096, max_event_bytes=262144),
)

# Builder style
cfg = (loxa.production("checkout")
    .with_version("1.2.0")
    .with_environment("prod")
    .with_sink(loxa.StdoutSink())
    .with_sampler(loxa.SampleErrors())
    .with_redactor(loxa.DefaultRedactor())
    .with_async(True)
    .with_duplicate_policy(loxa.LastWins)
)
```

## Levels

```python
import loxa

level = loxa.ParseLevel("info")  # loxa.LevelInfo
```

## Sinks

```python
import loxa

cfg = loxa.production("checkout").with_sink(loxa.StdoutSink())
cfg = loxa.production("checkout").with_sink(loxa.FileSink("/var/log/app.log"))
cfg = loxa.production("checkout").with_sink(loxa.HTTPBatchSink("http://collector:9090/events"))

# For testing
sink, store = loxa.MemorySink()
logger = loxa.create_loxa(service="checkout", sink=sink)
# ... use logger ...
events = store.events()
```

## Sampling

```python
import loxa
from datetime import timedelta

cfg = loxa.production("checkout").with_sampler(loxa.SampleAll())
cfg = loxa.production("checkout").with_sampler(loxa.SampleRandom(0.01))  # 1% sampling
cfg = loxa.production("checkout").with_sampler(loxa.SampleErrors())
cfg = loxa.production("checkout").with_sampler(loxa.SampleSlowRequests(timedelta(milliseconds=500)))
cfg = loxa.production("checkout").with_sampler(loxa.SampleStatusCodes(500, 502, 503))
cfg = loxa.production("checkout").with_sampler(loxa.SampleRoutes("/checkout", "/payment"))
cfg = loxa.production("checkout").with_sampler(loxa.SampleUsers("u_1", "u_2"))
cfg = loxa.production("checkout").with_sampler(loxa.SampleRateLimited(100.0, 1.0))  # 100 events/sec

# Combinators
cfg = loxa.production("checkout").with_sampler(
    loxa.AnySampler(
        loxa.SampleErrors(),
        loxa.SampleSlowRequests(timedelta(milliseconds=500)),
        loxa.SampleRandom(0.01),
    )
)
```

## Redaction

```python
import loxa

cfg = loxa.production("checkout").with_redactor(loxa.DefaultRedactor())
cfg = loxa.production("checkout").with_redactor(loxa.RedactKeys("password", "token", "authorization"))
cfg = loxa.production("checkout").with_redactor(loxa.HashKeys("user.email"))

# Compose
cfg = loxa.production("checkout").with_redactor(
    loxa.ComposeRedactors(
        loxa.DefaultRedactor(),
        loxa.RedactKeys("password", "token"),
        loxa.HashKeys("user.email"),
    )
)

# Mark fields as sensitive
loxa.enrich(ctx,
    loxa.SensitiveString("user.email", email),  # auto-redacted
    loxa.MarkSensitive("credit_card", card_no),
    loxa.HashString("user.ssn", ssn),
)
```

## Schema

```python
import loxa

cfg = loxa.production("checkout").with_schema(loxa.DefaultSchema())
cfg = loxa.production("checkout").with_schema(loxa.FlatSchema())  # good for ClickHouse
cfg = loxa.production("checkout").with_schema(loxa.OTelLogSchema())
cfg = loxa.production("checkout").with_schema(loxa.ECSchema())

# Custom schema
def my_schema(ev):
    return {
        "ts": ev.timestamp(),
        "service": ev.service(),
        "requestId": ev.request_id(),
        "took": ev.duration_ms(),
        "fields": ev.attrs(),
    }

cfg = loxa.production("checkout").with_schema(loxa.CustomSchema(my_schema))
```

## Duplicate Policy

```python
import loxa

cfg = loxa.production("checkout").with_duplicate_policy(loxa.CanonicalWins)  # default
cfg = loxa.production("checkout").with_duplicate_policy(loxa.LastWins)      # user attrs win
cfg = loxa.production("checkout").with_duplicate_policy(loxa.ErrorOnDuplicate)  # strict
```

## Middleware

```python
# Flask
from loxa.middleware.flask.middleware import LoxaMiddleware
app.wsgi_app = LoxaMiddleware(app.wsgi_app, service="checkout")

# FastAPI / Starlette (ASGI)
from loxa.middleware.asgi.middleware import Middleware as AsgiMiddleware
app.add_middleware(AsgiMiddleware, service="checkout")

# Django
from loxa.middleware.django.middleware import LoxaMiddleware
# Add to MIDDLEWARE in settings.py
```

## Feature Flags

```python
import loxa

loxa.enrich(ctx,
    loxa.FeatureFlag("checkout_v2", "enabled"),
    loxa.FeatureFlagBool("new_ui", True),
    loxa.Experiment("pricing_test", "variant_b"),
)
```

## Security

```python
import loxa

cfg = loxa.production("checkout").with_security(loxa.SecurityConfig(
    redact_by_default=True,
    allow_pii=False,
    max_field_bytes=4096,
    max_event_bytes=262144,
    max_attr_count=512,
    drop_oversized_events=True,
))

loxa.enrich(ctx,
    loxa.SensitiveString("user.email", email),
    loxa.HashString("user.ssn", ssn),
)
```

## Context Helpers

```python
import loxa

ev, ok = loxa.FromContext(ctx)
if loxa.HasEvent(ctx):
    eid = loxa.EventID(ctx)
    rid = loxa.RequestIDFromContext(ctx)
```

## Testing

```python
from loxa.testkit.helpers import test_logger, capture, assert_event, assert_redacted, assert_has_checkpoint, CapturingLogger

# Test logger with memory sink
logger, store = test_logger("test")

# Capture events
events = capture(lambda: some_function())

# Assert
assert_event(events[0], "user.id", "u_123")
assert_redacted(events[0], "password")
assert_has_checkpoint(events[0], "payment_started")

# Context manager
with CapturingLogger("test") as (logger, store):
    ctx = logger.start_event(loxa.Params(event="test"))
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
| Module facade             | `import loxa`        | `import "loxa"`      | `import loxa`        | `use loxa`           |
| Configure                 | `loxa.configure()`   | `loxa.Configure()`   | `loxa.configure()`   | `loxa::configure()`  |
| Start event               | `loxa.start_event()` | `loxa.StartEvent()`  | `loxa.startEvent()`  | `loxa::start_event()`|
| Enrich                    | `loxa.enrich()`      | `loxa.Enrich()`      | `loxa.enrich()`      | `loxa::enrich()`     |
| Checkpoint                | `loxa.checkpoint()`  | `loxa.Checkpoint()`  | `loxa.checkpoint()`  | `loxa::checkpoint()` |
| Finish                    | `loxa.finish()`      | `loxa.Finish()`      | `loxa.finish()`      | `loxa::finish()`     |
| Emit                      | `loxa.emit()`        | `loxa.Emit()`        | `loxa.emit()`        | `loxa::emit()`       |
| Shutdown                  | `loxa.shutdown()`    | `loxa.Shutdown()`    | `loxa.shutdown()`    | `loxa::shutdown()`   |
| Create instance           | `loxa.create_loxa()` | `loxa.NewLogger()`   | `loxa.createLoxa()`  | `loxa::new()`        |
| Alias                     | `loxa.alias()`       | `loxa.Alias()`       | `loxa.alias()`       | `loxa::alias()`      |
| String attr               | `loxa.String()`      | `loxa.String()`      | `loxa.string()`      | `loxa::String()`     |
| Int attr                  | `loxa.Int()`         | `loxa.Int()`         | `loxa.int()`         | `loxa::Int()`        |
| User ID                   | `loxa.UserID()`      | `loxa.UserID()`      | `loxa.userID()`      | `loxa::UserID()`     |
| Feature flag              | `loxa.FeatureFlag()` | `loxa.FeatureFlag()` | `loxa.featureFlag()` | `loxa::FeatureFlag()`|
| Middleware                 | `loxa.middleware.*`  | `loxa/http`          | `loxa/middleware`    | `loxa::middleware`   |
| Test kit                  | `loxa.testkit`       | `loxa/testkit`       | `loxa/testkit`       | `loxa::testkit`      |
