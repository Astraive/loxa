# LOXA Python SDK

**Status**: STABLE (v1.0.0) - Production-ready, full feature conformance

Full API conformance with specification is complete. See [SDK_CONFORMANCE_CONTRACT.md](../../spec/docs/SDK_CONFORMANCE_CONTRACT.md) for detailed guarantees.

`loxa-py` is a collector-first Python SDK for wide events. It builds one structured event per operation (request, job, queue message, CLI run, cron run), then emits to your log/analytics backend.

## Install

```bash
pip install -e .
```

## Quick Start

```python
from loxa import Config, Logger, Params, StartHTTPEvent, Enrich, Finish, Emit
from loxa import UserID, String, Int, HTTPBatchSink

# Configure
logger = Logger(
    Config.production("checkout").with_sink(
        HTTPBatchSink("http://127.0.0.1:9090/v1/events")
    )
)

# Or use the default facade
from loxa import configure, production
configure(production("checkout").with_sink(
    HTTPBatchSink("http://127.0.0.1:9090/v1/events")
))

# Lifecycle
ctx = StartHTTPEvent(None, Params(event="checkout.request", method="POST", path="/checkout"))
Enrich(ctx, UserID("u_123"), String("payment.provider", "stripe"))
Finish(ctx, "success", Int("status_code", 200))
Emit(ctx)
```

## Core Lifecycle API

The main flow: `StartEvent` -> `Enrich` -> `Checkpoint` -> `Finish`/`FinishError` -> `Emit`

```python
from loxa import (
    start_event, start_http_event, start_job_event, start_queue_event,
    start_cli_event, start_cron_event,
    enrich, append, set, merge, delete, get, get_group,
    checkpoint, finish, finish_error, emit, flush, shutdown,
    Params,
)

# Start an event (returns context carrying the event)
ctx = start_event(Params(
    event="checkout.request",
    kind="http",
    method="POST",
    path="/checkout",
    service="checkout",
))

# Typed starters
ctx = start_http_event(Params(event="http.request", method="GET", path="/health"))
ctx = start_job_event(Params(event="job.send_email"))
ctx = start_queue_event(Params(event="queue.process"))
ctx = start_cli_event(Params(event="cli.run"))
ctx = start_cron_event(Params(event="cron.tick"))

# Enrich (add attributes)
enrich(ctx, String("user.id", "u_123"), Int("cart.items", 3))

# Append (alias for enrich)
append(ctx, String("key", "value"))

# Set (override)
set(ctx, String("status", "processing"))

# Merge into group
merge(ctx, "payment", String("provider", "stripe"), Int("attempt", 1))

# Delete
delete(ctx, "temp_field")

# Get
val = get(ctx, "user.id")
group = get_group(ctx, "payment")

# Checkpoint (timeline marker)
checkpoint(ctx, "payment_started")
checkpoint(ctx, "payment_finished", String("provider", "stripe"))

# Finish
finish(ctx, "success", Int("status_code", 200))

# Or finish with error
try:
    process_payment()
except Exception as e:
    finish_error(ctx, e, Int("status_code", 500))

# Emit (sends to sink)
emit(ctx)

# Flush (force buffered events)
flush()

# Shutdown (drain pipeline)
shutdown()
```

## Attribute Constructors

```python
from loxa import (
    String, Int, Int64, Uint64, Float64, Bool, Time, Duration, Any, Null, Group,
)

enrich(ctx,
    String("user.id", "u_123"),
    Int("cart.items", 3),
    Int64("big_number", 9999999999),
    Float64("price", 49.99),
    Bool("premium", True),
    Duration("timeout", timedelta(seconds=30)),
    Any("metadata", {"key": "value"}),
    Null("optional_field"),
)

# Groups (nested objects)
enrich(ctx,
    Group("user",
        String("id", "u_123"),
        String("email", "user@example.com"),
    ),
    Group("payment",
        String("provider", "stripe"),
        Int("attempt", 1),
    ),
)
```

Dot keys expand into nested JSON:

```python
String("user.id", "u_123")  # -> {"user": {"id": "u_123"}}
```

## Canonical Helpers

```python
from loxa import (
    UserID, TenantID, WorkspaceID, OrganizationID, SessionID,
    RequestID, TraceID, SpanID,
    FeatureFlag, FeatureFlagBool, Experiment,
)

enrich(ctx,
    UserID("u_123"),
    TenantID("t_456"),
    WorkspaceID("w_789"),
    OrganizationID("org_abc"),
    SessionID("sess_xyz"),
    RequestID("req_123"),
    TraceID("trace_abc"),
    SpanID("span_def"),
    FeatureFlag("checkout_v2", "enabled"),
    FeatureFlagBool("new_ui", True),
    Experiment("pricing_test", "variant_b"),
)
```

## Business/Domain Helpers

```python
from loxa import (
    OrderID, CartID, ProductID, CustomerID,
    Plan, Currency, Amount, Country, Device, Platform, AppVersion,
)

enrich(ctx,
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
)
```

## Error Helpers

```python
from loxa import ErrorType, ErrorCode, ErrorMessage, ErrorStack, Retryable

try:
    process()
except Exception as e:
    finish_error(ctx, e,
        ErrorType("ValidationError"),
        ErrorCode("INVALID_INPUT"),
        ErrorMessage(str(e)),
        ErrorStack(traceback.format_exc()),
        Retryable(False),
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
from loxa import debug, info, warn, error, fatal

info("worker started", queue="emails")
error("payment failed", provider="stripe", amount=4999)
```

## Logger Instances

```python
from loxa import Logger, new, configure, default, Config, dev, production, test

# Create logger
logger = Logger(Config.production("checkout"))

# Or use factory
logger = new(production("checkout"))

# Configure default
configure(production("checkout"))

# Get default
logger = default()

# Presets
cfg = dev("checkout")       # pretty JSON, stdout, sync, debug level
cfg = production("checkout") # compact JSON, stdout, async, info level
cfg = test("checkout")       # sync, no sinks, debug level

# Instance methods
ctx = logger.start_event(Params(event="checkout.request"))
logger.enrich(ctx, String("key", "value"))
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
from loxa import (
    Config, SecurityConfig, AsyncConfig, FieldNamingConfig,
    WithService, WithVersion, WithEnvironment, WithSink, WithSampler,
    WithRedactor, WithSchema, WithEventSchema, WithAsync,
    WithCollectorEndpoint, WithDuplicatePolicy, WithStatsHandler,
    WithDeploymentID, WithIncludeHost, WithPanicRecovery, WithExitOnFatal,
)

cfg = Config(
    service="checkout",
    version="1.2.0",
    environment="prod",
    region="ap-south-1",
    level=LevelInfo,
    sinks=[StdoutSink()],
    async_config=AsyncConfig(enabled=True, queue_size=8192, workers=2),
    field_naming=FieldNamingConfig(expand_dot_keys=True),
    security=SecurityConfig(max_field_bytes=4096, max_event_bytes=262144),
)

# Builder style
cfg = (production("checkout")
    .with_version("1.2.0")
    .with_environment("prod")
    .with_sink(StdoutSink())
    .with_sampler(SampleErrors())
    .with_redactor(DefaultRedactor())
    .with_async(True)
    .with_duplicate_policy(LastWins)
)
```

## Levels

```python
from loxa import LevelDebug, LevelInfo, LevelWarn, LevelError, LevelFatal, ParseLevel

level = ParseLevel("info")  # LevelInfo
```

## Sinks

```python
from loxa import (
    StdoutSink, StderrSink, FileSink, RotatingFileSink,
    MemorySink, NoopSink, HTTPBatchSink,
)

cfg = production("checkout").with_sink(StdoutSink())
cfg = production("checkout").with_sink(FileSink("/var/log/app.log"))
cfg = production("checkout").with_sink(HTTPBatchSink("http://collector:9090/v1/events"))

# For testing
sink, store = MemorySink()
logger = Logger(test("checkout").with_sink(sink))
# ... use logger ...
events = store.events()
```

## Sampling

```python
from loxa import (
    SampleAll, SampleNone, SampleRandom, SampleErrors,
    SampleSlowRequests, SampleStatusCodes, SampleRoutes,
    SampleUsers, SampleTenants, SampleFeatureFlag,
    SampleRateLimited, SampleByHeader,
    AnySampler, AllSampler, NotSampler,
)
from datetime import timedelta

cfg = production("checkout").with_sampler(SampleAll())
cfg = production("checkout").with_sampler(SampleRandom(0.01))  # 1% sampling
cfg = production("checkout").with_sampler(SampleErrors())
cfg = production("checkout").with_sampler(SampleSlowRequests(timedelta(milliseconds=500)))
cfg = production("checkout").with_sampler(SampleStatusCodes(500, 502, 503))
cfg = production("checkout").with_sampler(SampleRoutes("/checkout", "/payment"))
cfg = production("checkout").with_sampler(SampleUsers("u_1", "u_2"))
cfg = production("checkout").with_sampler(SampleRateLimited(100.0, 1.0))  # 100 events/sec

# Combinators
cfg = production("checkout").with_sampler(
    AnySampler(
        SampleErrors(),
        SampleSlowRequests(timedelta(milliseconds=500)),
        SampleRandom(0.01),
    )
)
```

## Redaction

```python
from loxa import (
    DefaultRedactor, RedactKeys, RedactPatterns, HashKeys, MaskKeys, DropKeys,
    ComposeRedactors, SensitiveString, MarkSensitive, HashString,
)

cfg = production("checkout").with_redactor(DefaultRedactor())
cfg = production("checkout").with_redactor(RedactKeys("password", "token", "authorization"))
cfg = production("checkout").with_redactor(HashKeys("user.email"))

# Compose
cfg = production("checkout").with_redactor(
    ComposeRedactors(
        DefaultRedactor(),
        RedactKeys("password", "token"),
        HashKeys("user.email"),
    )
)

# Mark fields as sensitive
enrich(ctx,
    SensitiveString("user.email", email),  # auto-redacted
    MarkSensitive("credit_card", card_no),
    HashString("user.ssn", ssn),
)
```

## Schema

```python
from loxa import (
    DefaultSchema, FlatSchema, NestedSchema, OTelLogSchema, ECSchema,
    DatadogSchema, CustomSchema, SchemaFunc,
)

cfg = production("checkout").with_schema(DefaultSchema())
cfg = production("checkout").with_schema(FlatSchema())  # good for ClickHouse
cfg = production("checkout").with_schema(OTelLogSchema())
cfg = production("checkout").with_schema(ECSchema())

# Custom schema
def my_schema(ev):
    return {
        "ts": ev.timestamp(),
        "service": ev.service(),
        "requestId": ev.request_id(),
        "took": ev.duration_ms(),
        "fields": ev.attrs(),
    }

cfg = production("checkout").with_schema(CustomSchema(my_schema))
```

## Duplicate Policy

```python
from loxa import CanonicalWins, UserWins, FirstWins, LastWins, KeepBoth, ErrorOnDuplicate

cfg = production("checkout").with_duplicate_policy(CanonicalWins)  # default
cfg = production("checkout").with_duplicate_policy(LastWins)      # user attrs win
cfg = production("checkout").with_duplicate_policy(ErrorOnDuplicate)  # strict
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
from loxa import FeatureFlag, FeatureFlagBool, Experiment

enrich(ctx,
    FeatureFlag("checkout_v2", "enabled"),
    FeatureFlagBool("new_ui", True),
    Experiment("pricing_test", "variant_b"),
)
```

## Security

```python
from loxa import SecurityConfig, SensitiveString, MarkSensitive, HashString

cfg = production("checkout").with_security(SecurityConfig(
    redact_by_default=True,
    allow_pii=False,
    max_field_bytes=4096,
    max_event_bytes=262144,
    max_attr_count=512,
    drop_oversized_events=True,
))

enrich(ctx,
    SensitiveString("user.email", email),
    HashString("user.ssn", ssn),
)
```

## Context Helpers

```python
from loxa import FromContext, HasEvent, EventID, RequestIDFromContext

ev, ok = FromContext(ctx)
if HasEvent(ctx):
    eid = EventID(ctx)
    rid = RequestIDFromContext(ctx)
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
    ctx = logger.start_event(Params(event="test"))
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
