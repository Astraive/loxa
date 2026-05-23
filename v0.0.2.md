# Loxa Instrumentation + Full Product Idea Doc

## Recommended file

`docs/instrumentation-and-sdk-idea.md`

## One-line idea

Loxa is a collector-first business observability stack for wide events, lifecycle instrumentation, durable ingestion, safe enrichment, cross-language SDK parity, and queryable production telemetry.

It should not be “just logs,” “just traces,” or “just span attributes.” It should be the layer where application developers describe what actually happened in the business flow, while OpenTelemetry describes where the request moved through the system.

---

# 1. Core positioning

## What Loxa is

Loxa is a structured event lifecycle system.

It lets developers write code like this:

```ts
const event = loxa.startEvent({
  event: "checkout.request",
  kind: "http",
  route: "/checkout",
});

event.append(
  loxa.userId(user.id),
  loxa.int("cart.item_count", req.body.items.length),
  loxa.money("cart.total", req.body.total, "INR"),
);

event.checkpoint("cart_loaded");

const payment = event.process("payment.authorize");

try {
  const order = await processOrder(req.body);

  payment.finish(
    loxa.string("payment.provider", order.paymentProvider),
    loxa.string("payment.status", order.paymentStatus),
  );

  event.finish("success", loxa.string("order.id", order.id));
} catch (err) {
  payment.finishError(err);
  event.finishError(err, loxa.string("error.stage", "payment"));
} finally {
  await event.emit();
}
```

And get this automatically:

```txt
canonical event schema
safe attribute handling
automatic event duration
process/group/timer duration
checkpoints
OpenTelemetry correlation
collector validation
redaction
sampling
dedupe
spool
DLQ
replay
sink fanout
queryability
retention
deletion
cross-SDK conformance
```

## What Loxa is not

Loxa is not only a logging SDK.

Loxa is not a replacement for OpenTelemetry.

Loxa is not a metrics-only system.

Loxa is not a vendor-specific wrapper around `console.log`.

Loxa is the business event layer that sits beside traces, logs, and metrics.

---

# 2. The mental model

## OpenTelemetry answers

```txt
Which service called which service?
Where did latency happen?
Which span failed?
What trace connects this request across systems?
```

## Loxa answers

```txt
What business event happened?
Which user, tenant, order, cart, plan, payment, feature, or agent context mattered?
Which checkpoints happened?
Which ordered processes ran?
Which groups/phases took time?
Which timers measured latency?
What outcome did the event finish with?
Was it emitted, rejected, quarantined, dropped, replayed, or deleted?
Can I query this later with SQL?
Can I safely fan it out to sinks?
```

## Recommended architecture

```txt
Application
  ├─ OpenTelemetry spans
  │    └─ distributed tracing
  │
  └─ Loxa SDK
       └─ business-wide events
             ↓
        Loxa Collector
             ├─ auth / API keys / RBAC
             ├─ validation
             ├─ redaction
             ├─ schema policy
             ├─ sampling
             ├─ dedupe
             ├─ cardinality policy
             ├─ durable spool
             ├─ DLQ
             ├─ replay
             ├─ query
             ├─ tail
             ├─ delete
             └─ sink fanout
```

---

# 3. Product thesis

The main Loxa thesis:

> Developers already know what happened in their app, but current observability tools force them to scatter that knowledge across logs, spans, metrics, random attributes, and ad-hoc JSON blobs. Loxa gives them one lifecycle-native, schema-aware, collector-governed way to describe the full business story.

The strongest use cases are:

```txt
checkout flows
payment flows
auth flows
subscription flows
background jobs
AI agents
RAG pipelines
webhooks
internal tools
data sync jobs
billing operations
long-running workflows
```

The product should feel like:

```txt
Stripe-level DX for business observability.
OpenTelemetry-friendly, not OpenTelemetry-hostile.
Collector-first like real infra.
SDK-simple like a logger.
Wide-event-native like modern event analytics.
```

---

# 4. Core event lifecycle

The canonical Loxa lifecycle should be:

```txt
startEvent
  → append / enrich
  → checkpoint
  → process / group / timer
  → finish / finishError
  → emit
```

Full state machine:

```txt
INIT
  ↓
STARTED
  ├─ append attrs
  ├─ enrich context
  ├─ checkpoint
  ├─ start process
  ├─ start group
  ├─ start timer
  ↓
FINISHED
  ↓
EMITTED
```

Error states:

```txt
STARTED → INVALID
STARTED → DROPPED
FINISHED → EMIT_FAILED
FINISHED → SPOOLED
FINISHED → DLQ_WRITTEN
```

Canonical outcomes:

```txt
success
error
partial
abandoned
retried
cancelled
timeout
skipped
rejected
quarantined
```

Minimum stable outcomes:

```txt
success | error | partial | abandoned | retried
```

---

# 5. Canonical event schema

Every SDK should normalize to the same collector payload.

```json
{
  "schema_version": "v1",
  "event_version": "v1",
  "event_id": "018f7f3a-...",
  "timestamp": "2026-05-22T06:10:42.120Z",
  "service": "checkout",
  "environment": "production",
  "release": "1.2.3",
  "event": "checkout.request",
  "kind": "http",
  "level": "info",
  "outcome": "success",
  "duration_ms": 842,
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "request_id": "req_123",
  "tenant": {
    "id": "tenant_123"
  },
  "user": {
    "id": "user_456"
  },
  "http": {
    "method": "POST",
    "route": "/checkout",
    "path": "/checkout",
    "status_code": 200
  },
  "attrs": {
    "cart.item_count": 3,
    "cart.total_cents": 1399900,
    "checkout.payment_method": "card",
    "payment.provider": "stripe",
    "order.id": "ord_83k2"
  },
  "checkpoints": [],
  "processes": [],
  "groups": [],
  "timers": [],
  "links": [],
  "sdk": {
    "name": "loxa-js",
    "version": "0.1.0",
    "language": "javascript"
  },
  "collector": {
    "received_at": "2026-05-22T06:10:42.511Z"
  }
}
```

## Reserved top-level fields

Application attributes must not overwrite these:

```txt
schema_version
event_version
timestamp
event_id
service
environment
release
event
kind
level
outcome
duration_ms
request_id
trace_id
span_id
trace_flags
http
user
tenant
sdk
collector
attrs
checkpoints
processes
groups
timers
links
errors
sampling
redaction
```

---

# 6. Event primitives

## 6.1 Event

An event is the full business story.

Use it for:

```txt
checkout.request
payment.authorize
agent.run
job.invoice_generate
webhook.stripe.request
subscription.upgraded
```

An event owns:

```txt
canonical fields
business attrs
checkpoints
processes
groups
timers
links
final outcome
full duration
emit state
```

## 6.2 Attribute

An attribute is a typed fact.

Examples:

```txt
cart.item_count = 3
cart.total_cents = 1399900
payment.provider = stripe
risk.score_bucket = low
agent.model = gpt-5.5
```

Attributes should be typed and safe by default.

## 6.3 Enrichment

Enrichment adds shared context.

Examples:

```txt
service.version
deployment.id
commit.sha
tenant.id
user.plan
feature.checkout_v2
runtime.language
```

Enrichment can happen in:

```txt
SDK global context
SDK event context
framework middleware
collector ingest
sink adapter
```

## 6.4 Checkpoint

A checkpoint is a breadcrumb at a point in time.

It has offset time but no duration.

```ts
event.checkpoint("cart_loaded");
event.checkpoint("risk_checked", loxa.string("risk.bucket", "low"));
```

Use checkpoint for:

```txt
request_validated
cart_loaded
risk_checked
payment_started
email_queued
prompt_built
tool_selected
```

Do not use checkpoint when duration matters.

## 6.5 Process

A process is an ordered numbered step inside an event.

It has:

```txt
step number
name
started_at_ms
ended_at_ms
duration_ms
status_code
attrs
outcome
error
```

Use process for the main ordered story:

```txt
validate_checkout
load_cart
reserve_inventory
authorize_payment
create_order
send_receipt
agent_plan
agent_tool_call
agent_summarize
```

## 6.6 Group

A group is a named phase/block.

Use it when one larger phase contains multiple smaller operations.

Examples:

```txt
payment_flow
inventory_flow
fulfillment_flow
notification_flow
agent_reasoning_phase
agent_tool_phase
rag_retrieval_phase
```

## 6.7 Timer

A timer measures duration without implying ordered business sequence.

Use it for:

```txt
db.cart_lookup
cache.get_user
api.stripe_authorize
model.inference
embedding.generate
file.upload
```

## 6.8 Stopwatch

A stopwatch is local elapsed time measurement.

Use it when you need a duration before deciding whether to attach it as an attr, timer, process, or group.

## 6.9 Link

A link connects one Loxa event to another event, trace, span, job, message, request, or external ID.

Examples:

```txt
checkout.request → payment.authorize
agent.run → agent.tool_call
webhook.stripe.request → order.updated
job.sync_customer → external.crm.customer
```

---

# 7. Primitive decision table

| Primitive  | Has duration |        Ordered |    Has attrs | Best for             |
| ---------- | -----------: | -------------: | -----------: | -------------------- |
| Event      |          Yes | Full lifecycle |          Yes | Full business story  |
| Attribute  |           No |             No | Value itself | Business facts       |
| Enrichment |           No |             No |          Yes | Shared context       |
| Checkpoint |  Offset only |       Timeline |          Yes | Milestone/breadcrumb |
| Process    |          Yes |  Yes, numbered |          Yes | Main ordered steps   |
| Group      |          Yes | Phase timeline |          Yes | Larger named phase   |
| Timer      |          Yes |             No |          Yes | Latency measurement  |
| Stopwatch  |          Yes |             No |       Manual | Local elapsed time   |
| Link       |           No |             No |          Yes | Correlation          |

Rule:

```txt
Breadcrumb? checkpoint.
Main ordered business step? process.
Large phase? group.
Only latency? timer.
Local measurement? stopwatch.
Cross-event relationship? link.
```

---

# 8. Instrumentation examples

## 8.1 Checkout request

```ts
app.post("/checkout", async (req, res) => {
  const event = loxa.startEvent({
    event: "checkout.request",
    kind: "http",
    method: "POST",
    route: "/checkout",
  });

  event.append(
    loxa.userId(req.user.id),
    loxa.tenantId(req.user.tenantId),
    loxa.string("user.plan", req.user.plan),
    loxa.int("cart.item_count", req.body.items.length),
    loxa.money("cart.total", req.body.total, "INR"),
    loxa.string("checkout.payment_method", req.body.paymentMethod),
  );

  const checkout = event.group("checkout_flow");

  try {
    event.checkpoint("request_validated");

    const loadCart = event.process("load_cart");
    const cart = await loadCartFromDb(req.body.cartId);
    loadCart.finish(loxa.int("cart.item_count", cart.items.length));

    const payment = event.process("payment.authorize");
    const auth = await authorizePayment(cart);
    payment.finish(
      loxa.string("payment.provider", auth.provider),
      loxa.string("payment.status", auth.status),
    );

    const orderProcess = event.process("order.create");
    const order = await createOrder(cart, auth);
    orderProcess.finish(loxa.string("order.id", order.id));

    checkout.finish(loxa.string("checkout.status", "completed"));

    event.finish("success", loxa.httpStatus(200));
    res.json(order);
  } catch (err) {
    checkout.finishError(err);
    event.finishError(err, loxa.httpStatus(500));
    res.status(500).json({ error: "Failed" });
  } finally {
    await event.emit();
  }
});
```

## 8.2 Payment retry flow

```ts
const event = loxa.startEvent({
  event: "payment.retry",
  kind: "job",
});

event.append(
  loxa.string("payment.id", payment.id),
  loxa.string("order.id", order.id),
  loxa.int("payment.retry_attempt", attempt),
);

const retryGroup = event.group("retry_flow");

try {
  const wait = event.timer("retry.backoff_wait");
  await sleep(backoffMs);
  wait.stop(loxa.duration("backoff.ms", backoffMs));

  const authorize = event.process("payment.authorize_retry");
  const result = await gateway.authorize(payment);
  authorize.finish(loxa.string("payment.status", result.status));

  retryGroup.finish(loxa.string("retry.status", "authorized"));
  event.finish("success");
} catch (err) {
  retryGroup.finishError(err);

  if (isRetriable(err)) {
    event.finish("retried", loxa.string("error.code", safeErrorCode(err)));
  } else {
    event.finishError(err);
  }
} finally {
  await event.emit();
}
```

## 8.3 Auth login

```ts
const event = loxa.startEvent({
  event: "auth.login.request",
  kind: "http",
  method: "POST",
  route: "/login",
});

try {
  event.append(
    loxa.string("auth.method", "password"),
    loxa.string("auth.identifier_hash", hashEmail(req.body.email)),
  );

  const verify = event.process("auth.verify_credentials");
  const user = await verifyCredentials(req.body.email, req.body.password);
  verify.finish(loxa.bool("auth.credentials_valid", true));

  event.append(loxa.userId(user.id));
  event.finish("success", loxa.httpStatus(200));
} catch (err) {
  event.finishError(err, loxa.httpStatus(401), loxa.string("auth.failure_reason", "invalid_credentials"));
} finally {
  await event.emit();
}
```

## 8.4 Background job

```ts
await loxa.runEvent({ event: "job.invoice_generate", kind: "job" }, async event => {
  event.append(
    loxa.string("job.id", job.id),
    loxa.string("tenant.id", job.tenantId),
  );

  await event.withProcess("load_customer", () => loadCustomer(job.customerId));
  await event.withProcess("generate_invoice_pdf", () => generateInvoicePdf(job));
  await event.withProcess("send_invoice_email", () => sendInvoiceEmail(job));

  event.finish("success");
});
```

## 8.5 AI agent run

```ts
await loxa.runEvent({ event: "agent.run", kind: "agent" }, async event => {
  event.append(
    loxa.string("agent.name", "checkout-support-agent"),
    loxa.string("agent.provider", "openai"),
    loxa.string("agent.model", "gpt-5.5"),
    loxa.string("agent.run_type", "customer_support"),
    loxa.userId(user.id),
  );

  event.checkpoint("prompt_built");

  const reasoning = event.group("agent_reasoning_phase");
  const result = await runAgent(input, {
    onToolCallStart(tool) {
      event.process(`agent.tool.${tool.name}`).append(
        loxa.string("agent.tool.name", tool.name),
      );
    },
  });
  reasoning.finish(
    loxa.int("agent.steps", result.steps.length),
    loxa.int("agent.tool_calls", result.toolCalls.length),
  );

  event.append(
    loxa.int("agent.tokens.input", result.usage.inputTokens),
    loxa.int("agent.tokens.output", result.usage.outputTokens),
    loxa.money("agent.cost", result.costCents, "USD"),
  );

  event.finish("success");
});
```

## 8.6 RAG pipeline

```ts
await loxa.runEvent({ event: "rag.query", kind: "ai" }, async event => {
  event.append(
    loxa.string("rag.index", "docs-v3"),
    loxa.string("rag.embedding_model", "text-embedding-3-large"),
  );

  const retrieval = event.group("rag_retrieval_phase");

  const embed = event.timer("embedding.generate");
  const queryEmbedding = await embedQuery(input);
  embed.stop();

  const search = event.process("vector.search");
  const chunks = await vectorDb.search(queryEmbedding);
  search.finish(
    loxa.int("rag.chunks.retrieved", chunks.length),
    loxa.float("rag.top_score", chunks[0]?.score ?? 0),
  );

  retrieval.finish();

  const generate = event.process("llm.generate_answer");
  const answer = await generateAnswer(input, chunks);
  generate.finish(
    loxa.int("agent.tokens.input", answer.usage.inputTokens),
    loxa.int("agent.tokens.output", answer.usage.outputTokens),
  );

  event.finish("success");
});
```

---

# 9. Field naming conventions

Use dot-separated business keys:

```txt
cart.item_count
cart.total_cents
checkout.payment_method
payment.provider
payment.failure_code
order.status
feature.checkout_v2
risk.score_bucket
agent.model
agent.tool.name
rag.chunks.retrieved
```

Avoid vague keys:

```txt
metadata
data
payload
raw
extra
stuff
context
obj
json
```

Avoid raw blobs by default:

```ts
// Bad
loxa.string("request.body", JSON.stringify(req.body));

// Good
loxa.int("cart.item_count", req.body.items.length);
loxa.money("cart.total", req.body.total, "INR");
```

Feature flags should be normalized:

```ts
// Bad
loxa.string("feature_flags", JSON.stringify(user.flags));

// Good
loxa.featureFlag("checkout_v2", "on");
loxa.featureFlag("risk_engine_v2", "off");
```

---

# 10. Cardinality policy

## Usually safe to aggregate

```txt
service
environment
release
event
kind
level
outcome
http.method
http.route
http.status_code
user.plan
checkout.payment_method
payment.provider
error.code
feature.checkout_v2
agent.model
agent.provider
```

## High-cardinality fields

```txt
user.id
order.id
cart.id
payment.id
request_id
trace_id
span_id
session.id
email
ip_address
```

These can be stored for lookup and correlation, but should not be blindly indexed or grouped by default.

## Prefer buckets

```ts
loxa.string("user.ltv_bucket", bucketLtv(user.ltv));
loxa.string("risk.score_bucket", bucketRisk(score));
loxa.string("cart.total_bucket", bucketMoney(cart.total));
```

---

# 11. Redaction and privacy model

## SDK safety net

SDKs should block obvious secrets before data leaves the process:

```txt
password
passwd
secret
token
api_key
apikey
authorization
cookie
set_cookie
private_key
client_secret
access_token
refresh_token
```

This should be lightweight and consistent across SDKs.

## Collector policy

The collector owns the real policy:

```txt
PII detection
allowlist/blocklist
schema validation
tenant-specific policy
quarantine mode
audit logs
deletion support
DLQ context
```

## Final default

```txt
SDK: minimal key-based redaction safety net
Collector: full PII/security policy enforcement
```

Do not remove SDK redaction completely. The collector is the authority, but the SDK is the first safety layer.

---

# 12. OpenTelemetry bridge

Loxa should integrate with OpenTelemetry without copying everything into spans.

## Recommended behavior

When an active OTel span exists, Loxa should:

```txt
read trace_id
read span_id
read trace_flags/sample state
set these on the Loxa event
add loxa.event_id to the span
optionally add safe allowlisted attrs to the span
```

## Do not copy all Loxa attrs to OTel

Wide events can have many business fields. Copying all of them to span attributes can cause:

```txt
high cardinality
privacy leaks
cost increase
index bloat
vendor lock-in
```

## Recommended config

```ts
loxa.configure(
  loxa.production("checkout")
    .withOtelBridge({
      mode: "link",
      spanAttributeAllowlist: [
        "loxa.event_id",
        "loxa.event",
        "loxa.outcome",
        "cart.item_count",
        "checkout.payment_method",
        "payment.provider",
      ],
    })
);
```

---

# 13. Collector ingest contract

Default endpoint:

```txt
POST /v1/events
```

Batch request:

```json
{
  "events": [
    {
      "schema_version": "v1",
      "event_version": "v1",
      "event_id": "018f7f...",
      "timestamp": "2026-05-22T06:10:42.120Z",
      "service": "checkout",
      "event": "checkout.request",
      "kind": "http",
      "level": "info",
      "outcome": "success",
      "duration_ms": 842,
      "attrs": {}
    }
  ]
}
```

Response:

```json
{
  "accepted": 1,
  "rejected": 0,
  "quarantined": 0,
  "errors": []
}
```

Validation modes:

```txt
off         accept everything, still normalize
warn        accept and report schema issues
enforce     reject invalid events
quarantine  store invalid events separately
```

---

# 14. Collector responsibilities

The collector should own production complexity.

## Ingest

```txt
HTTP ingest
gRPC ingest
batch ingest
compressed payloads
API key auth
mTLS optional
rate limiting
body size limits
```

## Policy

```txt
schema validation
reserved-field collision handling
redaction
PII detection
cardinality warnings
attribute size limits
allowed event names
tenant/project resolution
```

## Durability

```txt
spool to disk
retry queue
DLQ
idempotency
dedupe
replay
backpressure
```

## Query

```txt
tail events
SQL query
filter by service/event/outcome/tenant
inspect DLQ
inspect quarantined events
export query result
```

## Fanout

```txt
DuckDB
ClickHouse
Postgres
Kafka
NATS
Loki
OTLP
S3
GCS
BigQuery
Snowflake
stdout
file
webhook
```

---

# 15. API keys and RBAC

Public collector ingestion should use scoped API keys.

Key model:

```txt
key_id
key_prefix
hashed_secret
tenant_id
project_id
service_allowlist
allowed_events
allowed_envs
rate_limit
expires_at
created_by
revoked_at
last_used_at
```

Scopes:

```txt
ingest:write
query:read
dlq:read
dlq:replay
events:delete
keys:create
keys:revoke
admin:read
admin:write
```

Rules:

```txt
SDK keys usually only need ingest:write.
Querying needs query:read.
Deletion needs events:delete.
DLQ replay needs dlq:replay.
Collector should derive tenant/project from API key.
Payload tenant fields should not override authenticated tenant by default.
```

---

# 16. Cross-language parity

Loxa should feel like the same product in every language.

## Required model

```txt
Default/global client:
  loxa.<method>()

Cross-language factory:
  createLoxa / create_loxa / CreateLoxa

Custom alias:
  logger = createLoxa(...)
  logger.<method>()

Same-config alias:
  logger = loxa.alias("logger")
```

## Language mapping

| Concept               | JS/TS          | Python          | Go               | Rust            |
| --------------------- | -------------- | --------------- | ---------------- | --------------- |
| Default client        | `loxa.info()`  | `loxa.info()`   | `loxa.Info(ctx)` | `loxa::info()`  |
| Factory               | `createLoxa()` | `create_loxa()` | `CreateLoxa()`   | `create_loxa()` |
| Idiomatic constructor | `new Loxa()`   | `Loxa()`        | `New()`          | `Loxa::new()`   |
| Alias                 | `loxa.alias()` | `loxa.alias()`  | `loxa.Alias()`   | `loxa::alias()` |

## Go rule

Go should support both:

```go
logger := loxa.CreateLoxa(loxa.Config{Service: "api"})
```

and:

```go
logger := loxa.New(loxa.Config{Service: "api"})
```

But `CreateLoxa` should be documented as the cross-language parity constructor, and `New` should be the idiomatic Go alias.

```txt
CreateLoxa == New
```

---

# 17. Method surface: 100+ methods

This section defines the full API family. Not every method must ship on day one, but the shape should be reserved so the SDKs grow consistently.

## 17.1 Client creation and configuration

1. `configure(config)` — configure the global client.
2. `createLoxa(config)` / `create_loxa(config)` / `CreateLoxa(config)` — create independent client.
3. `new Loxa(config)` / `Loxa(config)` / `New(config)` / `Loxa::new(config)` — idiomatic constructor.
4. `production(service)` — production preset.
5. `development(service)` — development preset.
6. `test(service)` — test preset.
7. `disabled()` — no-op preset.
8. `fromEnv()` — load config from environment.
9. `withService(name)` — set service.
10. `withEnvironment(env)` — set environment.
11. `withRelease(version)` — set release.
12. `withNamespace(namespace)` — set namespace.
13. `withEndpoint(url)` — set collector endpoint.
14. `withApiKey(key)` — set API key.
15. `withSink(sink)` — attach sink.
16. `withSampler(sampler)` — attach sampler.
17. `withRedaction(policy)` — attach redaction config.
18. `withOtelBridge(config)` — enable OTel bridge.
19. `withFlushInterval(ms)` — set periodic flush.
20. `withBatchSize(size)` — set batch size.
21. `withQueueSize(size)` — set queue size.
22. `withRetry(config)` — configure retries.
23. `withTimeout(ms)` — set request timeout.
24. `withLogger(logger)` — internal SDK diagnostic logger.
25. `reset()` — reset global config in tests.

## 17.2 Basic logging and event methods

26. `debug(message, attrs?)` — debug log event.
27. `info(message, attrs?)` — info log event.
28. `notice(message, attrs?)` — notice level event.
29. `warn(message, attrs?)` — warning event.
30. `error(errorOrMessage, attrs?)` — error event.
31. `fatal(errorOrMessage, attrs?)` — fatal event.
32. `event(name, attrs?)` — simple named business event.
33. `track(name, attrs?)` — analytics-style alias for event.
34. `audit(name, attrs?)` — audit event helper.
35. `security(name, attrs?)` — security event helper.
36. `metric(name, value, attrs?)` — simple measurement event.
37. `count(name, value?, attrs?)` — count helper.
38. `gauge(name, value, attrs?)` — gauge helper.
39. `histogram(name, value, attrs?)` — histogram-style helper.
40. `breadcrumb(name, attrs?)` — lightweight checkpoint-style event.

## 17.3 Lifecycle event methods

41. `startEvent(params)` — create lifecycle event.
42. `append(ctx, ...attrs)` — append attrs to event.
43. `enrich(ctxOrAttrs, attrs?)` — enrich event/client context.
44. `checkpoint(ctx, name, attrs?)` — add checkpoint.
45. `finish(ctx, outcome, attrs?)` — finish lifecycle event.
46. `finishError(ctx, error, attrs?)` — finish with error.
47. `emit(ctx)` — emit lifecycle event.
48. `drop(ctx, reason)` — mark event dropped.
49. `cancel(ctx, reason?)` — cancel event.
50. `abandon(ctx, reason?)` — mark abandoned.
51. `retry(ctx, attrs?)` — mark retried.
52. `partial(ctx, attrs?)` — mark partial success.
53. `cloneEvent(ctx)` — clone event context.
54. `linkEvent(ctx, target)` — link event to another event/trace/job.
55. `currentEvent()` — get current async-local event.
56. `fromRequest(req)` — get event from framework request.
57. `bindEvent(ctx, fn)` — run function with active event context.
58. `runEvent(params, fn)` — start, run, finish, emit automatically.
59. `run(ctx, fn)` — run with existing event and auto finish/emit.
60. `wrap(name, fn)` — wrap function with event instrumentation.

## 17.4 Process, group, timer, stopwatch methods

61. `process(ctx, name, attrs?)` — start ordered process.
62. `startProcess(ctx, name, attrs?)` — explicit process start.
63. `finishProcess(handle, attrs?)` — finish process.
64. `finishProcessError(handle, error, attrs?)` — finish process with error.
65. `withProcess(ctx, name, fn, attrs?)` — automatic process wrapper.
66. `group(ctx, name, attrs?)` — start group.
67. `startGroup(ctx, name, attrs?)` — explicit group start.
68. `finishGroup(handle, attrs?)` — finish group.
69. `finishGroupError(handle, error, attrs?)` — finish group with error.
70. `withGroup(ctx, name, fn, attrs?)` — automatic group wrapper.
71. `timer(ctx, name, attrs?)` — start event-attached timer.
72. `startTimer(ctx, name, attrs?)` — explicit timer start.
73. `stopTimer(handle, attrs?)` — stop timer.
74. `withTimer(ctx, name, fn, attrs?)` — automatic timer wrapper.
75. `stopwatch(name?)` — standalone stopwatch.
76. `duration(key, ms)` — duration attr helper.
77. `measure(name, fn)` — local measurement helper.
78. `step(ctx, name, fn, attrs?)` — sugar over process.
79. `phase(ctx, name, fn, attrs?)` — sugar over group.
80. `span(ctx, name, fn, attrs?)` — local span-like block, Loxa-owned.

## 17.5 Typed attribute helpers

81. `attr(key, value)` — generic typed attr.
82. `string(key, value)` — string attr.
83. `int(key, value)` — integer attr.
84. `float(key, value)` — float attr.
85. `bool(key, value)` — boolean attr.
86. `json(key, value)` — JSON attr with size policy.
87. `list(key, values)` — list attr.
88. `map(key, value)` — map/object attr.
89. `enum(key, value, allowed?)` — enum attr.
90. `id(key, value)` — high-cardinality ID attr.
91. `hash(key, value)` — hashed attr.
92. `redacted(key)` — explicit redacted marker.
93. `masked(key, value)` — masked value.
94. `url(key, value)` — URL attr with query stripping.
95. `emailHash(key, email)` — hashed email helper.
96. `ipHash(key, ip)` — hashed IP helper.
97. `money(key, amount, currency)` — monetary attr.
98. `percent(key, value)` — percent attr.
99. `bytes(key, value)` — bytes attr.
100. `statusCode(code)` — generic status code helper.
101. `httpStatus(code)` — HTTP status helper.
102. `errorCode(code)` — error code helper.
103. `featureFlag(name, value)` — feature flag helper.
104. `bucket(key, value)` — bucket helper.
105. `tags(values)` — tag list helper.

## 17.6 Identity and domain helpers

106. `userId(id)` — canonical user ID.
107. `tenantId(id)` — canonical tenant ID.
108. `orgId(id)` — organization ID.
109. `accountId(id)` — account ID.
110. `sessionId(id)` — session ID.
111. `requestId(id)` — request ID.
112. `correlationId(id)` — correlation ID.
113. `traceId(id)` — trace ID.
114. `spanId(id)` — span ID.
115. `orderId(id)` — order ID.
116. `cartId(id)` — cart ID.
117. `paymentId(id)` — payment ID.
118. `subscriptionId(id)` — subscription ID.
119. `invoiceId(id)` — invoice ID.
120. `jobId(id)` — background job ID.
121. `messageId(id)` — message/queue ID.
122. `deploymentId(id)` — deployment ID.
123. `commitSha(sha)` — commit SHA.
124. `release(version)` — release attr.
125. `region(region)` — region attr.

## 17.7 HTTP and framework helpers

126. `httpRequest(req)` — extract safe request context.
127. `httpResponse(res)` — extract safe response context.
128. `httpRoute(route)` — set route pattern.
129. `httpMethod(method)` — method helper.
130. `httpPath(path)` — path helper.
131. `httpUserAgent(ua)` — sanitized user agent.
132. `httpReferer(ref)` — sanitized referer.
133. `expressMiddleware(config)` — Express middleware.
134. `fastifyPlugin(config)` — Fastify plugin.
135. `nextMiddleware(config)` — Next.js middleware.
136. `honoMiddleware(config)` — Hono middleware.
137. `koaMiddleware(config)` — Koa middleware.
138. `djangoMiddleware(config)` — Django middleware.
139. `flaskMiddleware(config)` — Flask middleware.
140. `goHTTPMiddleware(config)` — Go net/http middleware.
141. `axumLayer(config)` — Rust Axum layer.
142. `towerLayer(config)` — Rust Tower layer.

## 17.8 Sink, queue, flush, shutdown methods

143. `httpSink(config)` — HTTP sink.
144. `httpBatchSink(config)` — HTTP batch sink.
145. `stdoutSink(config?)` — stdout sink.
146. `fileSink(config)` — file sink.
147. `memorySink(config?)` — memory sink for tests.
148. `noopSink()` — no-op sink.
149. `multiSink(...sinks)` — fanout from SDK if needed.
150. `otlpSink(config)` — OTLP sink.
151. `kafkaSink(config)` — Kafka sink.
152. `flush()` — flush pending events.
153. `shutdown()` — flush and stop workers.
154. `drain()` — drain queue without accepting new events.
155. `pause()` — pause emission.
156. `resume()` — resume emission.
157. `queueSize()` — inspect queue size.
158. `health()` — SDK health status.

## 17.9 Sampling and policy methods

159. `sampleRate(rate)` — fixed sample rate.
160. `alwaysSample()` — sample all.
161. `neverSample()` — sample none.
162. `sampleByEvent(rules)` — event-based sampling.
163. `sampleByOutcome(rules)` — outcome-based sampling.
164. `sampleErrors(rate)` — sample errors separately.
165. `shouldSample(event)` — evaluate sampling.
166. `redact(keysOrPolicy)` — configure redaction.
167. `allowFields(fields)` — allowlist fields.
168. `blockFields(fields)` — blocklist fields.
169. `maxAttrLength(length)` — truncate long attrs.
170. `maxEventBytes(bytes)` — max event size.
171. `maxAttrs(count)` — max attr count.
172. `cardinalityPolicy(policy)` — cardinality config.
173. `validateEvent(event)` — SDK-side validation.
174. `normalizeEvent(event)` — normalize payload.
175. `sanitizeEvent(event)` — redact/truncate event.

## 17.10 Testing and conformance methods

176. `testkit()` — create SDK testkit.
177. `capture(fn)` — capture emitted events in tests.
178. `lastEvent()` — get last emitted event.
179. `events()` — get all captured events.
180. `clearEvents()` — clear test events.
181. `expectEvent(name)` — assertion helper.
182. `expectAttr(key, value)` — assertion helper.
183. `snapshotEvent(event)` — stable event snapshot.
184. `goldenTest(path)` — compare against golden schema.
185. `conformanceSuite()` — cross-SDK conformance runner.
186. `mockSink()` — mock sink.
187. `fakeClock()` — deterministic time.
188. `setClock(clock)` — injectable clock.
189. `setIdGenerator(fn)` — deterministic IDs.
190. `resetForTest()` — reset globals.

## 17.11 Collector API / CLI method families

191. `collector.run(config)` — run collector.
192. `collector.validate(config)` — validate config.
193. `collector.ingest(events)` — internal ingest.
194. `collector.query(sql)` — SQL query.
195. `collector.tail(filter)` — live tail.
196. `collector.delete(filter)` — delete events.
197. `collector.replay(filter)` — replay events.
198. `collector.dlq.list(filter)` — list DLQ.
199. `collector.dlq.read(id)` — read DLQ item.
200. `collector.dlq.replay(id)` — replay DLQ item.
201. `collector.keys.create(config)` — create API key.
202. `collector.keys.revoke(id)` — revoke key.
203. `collector.keys.rotate(id)` — rotate key.
204. `collector.sinks.test(name)` — test sink.
205. `collector.sinks.list()` — list sinks.
206. `collector.policy.validate(policy)` — validate policy.
207. `collector.schema.check(event)` — check schema.
208. `collector.schema.publish(schema)` — publish schema.
209. `collector.retention.apply(policy)` — apply retention.
210. `collector.health()` — collector health.

---

# 18. MVP method subset

The full list is intentionally huge. The first production SDK should ship a smaller clean core.

## SDK v0.1 must-have

```txt
configure
createLoxa / create_loxa / CreateLoxa
alias
info
warn
error
event
startEvent
append
checkpoint
process
startProcess
finishProcess
finishProcessError
group
startGroup
finishGroup
finishGroupError
timer
startTimer
stopTimer
stopwatch
finish
finishError
emit
flush
shutdown
string
int
float
bool
json
duration
money
httpStatus
userId
tenantId
requestId
traceId
spanId
featureFlag
httpBatchSink
stdoutSink
memorySink
noopSink
```

## SDK v0.2 should add

```txt
runEvent
withProcess
withGroup
withTimer
expressMiddleware
fastifyPlugin
withOtelBridge
redact
sampleRate
testkit
capture
expectEvent
```

## SDK v1 should add

```txt
collector auth integration
schema validation
conformance suite
OpenTelemetry bridge stable
framework middleware stable
AI helper pack
payment helper pack
jobs helper pack
```

---

# 19. Cross-language naming table

| Concept      | JS/TS         | Python         | Go            | Rust           |
| ------------ | ------------- | -------------- | ------------- | -------------- |
| Configure    | `configure`   | `configure`    | `Configure`   | `configure`    |
| Factory      | `createLoxa`  | `create_loxa`  | `CreateLoxa`  | `create_loxa`  |
| Constructor  | `new Loxa`    | `Loxa`         | `New`         | `Loxa::new`    |
| Alias        | `alias`       | `alias`        | `Alias`       | `alias`        |
| Info         | `info`        | `info`         | `Info`        | `info`         |
| Warn         | `warn`        | `warn`         | `Warn`        | `warn`         |
| Error        | `error`       | `error`        | `Error`       | `error`        |
| Event        | `event`       | `event`        | `Event`       | `event`        |
| Start event  | `startEvent`  | `start_event`  | `StartEvent`  | `start_event`  |
| Append       | `append`      | `append`       | `Append`      | `append`       |
| Checkpoint   | `checkpoint`  | `checkpoint`   | `Checkpoint`  | `checkpoint`   |
| Process      | `process`     | `process`      | `Process`     | `process`      |
| Group        | `group`       | `group`        | `Group`       | `group`        |
| Timer        | `timer`       | `timer`        | `Timer`       | `timer`        |
| Stopwatch    | `stopwatch`   | `stopwatch`    | `Stopwatch`   | `stopwatch`    |
| Finish       | `finish`      | `finish`       | `Finish`      | `finish`       |
| Finish error | `finishError` | `finish_error` | `FinishError` | `finish_error` |
| Emit         | `emit`        | `emit`         | `Emit`        | `emit`         |
| Flush        | `flush`       | `flush`        | `Flush`       | `flush`        |
| Shutdown     | `shutdown`    | `shutdown`     | `Shutdown`    | `shutdown`     |

Rule:

```txt
Use language conventions for casing.
Keep the concepts identical.
Keep collector payloads identical.
```

---

# 20. Recommended JS/TS API shape

```ts
import { loxa, createLoxa, Loxa } from "@loxa/js";

const logger = createLoxa({
  service: "checkout-api",
  environment: "production",
});

logger.info("server started");

const event = logger.startEvent({
  event: "checkout.request",
  kind: "http",
});

event.append(loxa.userId("u_123"));
event.checkpoint("cart_loaded");
event.finish("success");
await event.emit();
```

Recommended exports:

```ts
export const loxa: LoxaClient;
export function createLoxa(config?: LoxaConfig): LoxaClient;
export class Loxa implements LoxaClient {}

export type LoxaClient = {
  alias(name: string): LoxaClient;
  info(message: string, attrs?: Attrs): void;
  warn(message: string, attrs?: Attrs): void;
  error(error: Error | string, attrs?: Attrs): void;
  event(name: string, attrs?: Attrs): void;
  startEvent(params: StartEventParams): EventHandle;
  runEvent<T>(params: StartEventParams, fn: (event: EventHandle) => Promise<T>): Promise<T>;
  flush(): Promise<void>;
  shutdown(): Promise<void>;
};
```

---

# 21. Recommended Python API shape

```py
from loxa import loxa, create_loxa, Loxa

logger = create_loxa(
    service="checkout-api",
    environment="production",
)

logger.info("server started")

event = logger.start_event(
    event="checkout.request",
    kind="http",
)

event.append(loxa.user_id("u_123"))
event.checkpoint("cart_loaded")
event.finish("success")
event.emit()
```

Python should use snake_case for method names, but should not force user field keys into snake_case.

---

# 22. Recommended Go API shape

```go
package main

import (
    "context"

    "github.com/astraive/loxa-go"
)

func main() {
    ctx := context.Background()

    logger := loxa.CreateLoxa(loxa.Config{
        Service: "checkout-api",
        Environment: "production",
    })

    logger.Info(ctx, "server started")

    event := logger.StartEvent(ctx, loxa.StartEventParams{
        Event: "checkout.request",
        Kind:  "http",
    })

    event.Append(loxa.UserID("u_123"))
    event.Checkpoint("cart_loaded")
    event.Finish("success")
    event.Emit(ctx)
}
```

Go should support:

```go
loxa.CreateLoxa(...)
loxa.New(...)
```

They must behave identically.

---

# 23. Recommended Rust API shape

```rust
use loxa::{create_loxa, Config};

let logger = create_loxa(Config {
    service: Some("checkout-api".into()),
    environment: Some("production".into()),
    ..Default::default()
});

logger.info("server started");

let mut event = logger.start_event(StartEventParams {
    event: "checkout.request".into(),
    kind: "http".into(),
    ..Default::default()
});

event.append(loxa::user_id("u_123"));
event.checkpoint("cart_loaded");
event.finish("success");
event.emit().await?;
```

Rust should expose:

```txt
create_loxa(config)
Loxa::new(config)
loxa::info(...)
logger.info(...)
```

---

# 24. Domain helper packs

Helper packs should be thin, optional, and schema-stable.

## Checkout helper pack

```txt
checkout.cartItemCount(count)
checkout.cartTotal(cents, currency)
checkout.paymentMethod(method)
checkout.coupon(codeHash)
checkout.status(status)
checkout.failureStage(stage)
```

## Payment helper pack

```txt
payment.provider(provider)
payment.method(method)
payment.intentId(id)
payment.authorizationId(id)
payment.failureCode(code)
payment.retryAttempt(n)
payment.retriable(bool)
payment.latency(ms)
```

## Billing helper pack

```txt
billing.plan(plan)
billing.subscriptionId(id)
billing.invoiceId(id)
billing.amount(cents, currency)
billing.interval(interval)
billing.status(status)
```

## AI agent helper pack

```txt
agent.name(name)
agent.provider(provider)
agent.model(model)
agent.runType(type)
agent.toolName(name)
agent.toolOutcome(outcome)
agent.inputTokens(n)
agent.outputTokens(n)
agent.cost(cents, currency)
agent.failureStage(stage)
```

## RAG helper pack

```txt
rag.index(name)
rag.embeddingModel(model)
rag.chunksRetrieved(n)
rag.topScore(score)
rag.queryHash(hash)
rag.citationCount(n)
rag.retrievalLatency(ms)
```

---

# 25. Query examples

## Recent checkout errors

```sql
SELECT
  timestamp,
  service,
  event,
  outcome,
  attrs->>'error.code' AS error_code,
  attrs->>'payment.provider' AS provider
FROM events
WHERE event = 'checkout.request'
  AND outcome = 'error'
ORDER BY timestamp DESC
LIMIT 50;
```

## Payment failures by provider

```sql
SELECT
  attrs->>'payment.provider' AS provider,
  attrs->>'error.code' AS error_code,
  count(*) AS failures
FROM events
WHERE event = 'checkout.request'
  AND outcome = 'error'
GROUP BY provider, error_code
ORDER BY failures DESC;
```

## Checkout latency by payment method

```sql
SELECT
  attrs->>'checkout.payment_method' AS payment_method,
  avg(duration_ms) AS avg_duration_ms,
  approx_quantile(duration_ms, 0.95) AS p95_duration_ms
FROM events
WHERE event = 'checkout.request'
  AND outcome = 'success'
GROUP BY payment_method;
```

## Agent cost by model

```sql
SELECT
  attrs->>'agent.model' AS model,
  sum(CAST(attrs->>'agent.cost_cents' AS INTEGER)) AS total_cost_cents,
  count(*) AS runs
FROM events
WHERE event = 'agent.run'
GROUP BY model
ORDER BY total_cost_cents DESC;
```

## Failed process names

```sql
SELECT
  process.name,
  count(*) AS failures
FROM events,
UNNEST(processes) AS process
WHERE process.outcome = 'error'
GROUP BY process.name
ORDER BY failures DESC;
```

---

# 26. Event taxonomy

## HTTP request events

```txt
checkout.request
auth.login.request
billing.portal.request
webhook.stripe.request
webhook.github.request
admin.action.request
```

## Business state events

```txt
checkout.started
checkout.completed
checkout.failed
payment.authorize.started
payment.authorize.completed
payment.authorize.failed
order.created
order.cancelled
subscription.upgraded
subscription.cancelled
invoice.generated
invoice.paid
```

## Background jobs

```txt
job.email_send
job.invoice_generate
job.reindex_search
job.sync_customer
job.backfill_events
job.cleanup_expired_sessions
```

## AI events

```txt
agent.run
agent.step
agent.tool_call
agent.memory_read
agent.memory_write
agent.handoff
agent.error
rag.query
rag.reindex
rag.citation_check
```

## Collector events

```txt
collector.ingest
collector.validation_failed
collector.redaction_applied
collector.quarantine_written
collector.dlq_written
collector.replay_started
collector.replay_completed
sink.write_failed
sink.write_recovered
```

---

# 27. Documentation structure

Recommended docs tree:

```txt
docs/
  introduction.md
  why-loxa.md
  business-instrumentation.md
  instrumentation-and-sdk-idea.md
  lifecycle.md
  event-schema.md
  fields-and-cardinality.md
  redaction-and-privacy.md
  opentelemetry.md
  collector.md
  api-keys-and-rbac.md
  querying.md
  sinks.md
  dlq-and-replay.md
  sdk-parity.md
  sdk-methods.md
  examples/
    checkout.md
    payments.md
    auth.md
    jobs.md
    ai-agents.md
    rag.md
  sdks/
    javascript.md
    python.md
    go.md
    rust.md
  conformance.md
```

---

# 28. Implementation roadmap

## Phase 1 — SDK core

```txt
canonical event type
start/append/checkpoint/process/group/timer/finish/emit
minimal redaction
HTTP batch sink
stdout sink
memory sink
flush/shutdown
testkit
```

## Phase 2 — Collector core

```txt
/v1/events ingest
batch accept/reject response
API key auth
validation modes
local durable store
query command
tail command
DLQ
```

## Phase 3 — Cross-SDK parity

```txt
JS/Python/Go/Rust same method concepts
same emitted JSON fixtures
golden conformance tests
same redaction defaults
same naming docs
```

## Phase 4 — OTel and framework integration

```txt
OpenTelemetry bridge
Express/Fastify middleware
Go net/http middleware
Python Flask/Django middleware
Rust Tower/Axum layer
```

## Phase 5 — Production infra

```txt
RBAC keys
spool/replay
ClickHouse/Postgres/Kafka/S3 sinks
retention/deletion
collector metrics
collector health
multi-tenant controls
```

## Phase 6 — Domain packs

```txt
checkout helpers
payment helpers
billing helpers
agent helpers
RAG helpers
job helpers
```

---

# 29. Final product rule

Loxa should make this easy:

```ts
const event = loxa.startEvent({ event: "checkout.request", kind: "http" });
event.append(loxa.userId(user.id), loxa.money("cart.total", total, "INR"));
event.checkpoint("payment_started");
event.finish("success");
await event.emit();
```

And produce this outcome:

```txt
safe structured wide event
business lifecycle captured
durations measured automatically
trace correlation preserved
collector policy enforced
event queryable later
fanout handled centrally
SDKs consistent across languages
```

That is the difference between scattered logging and a real business observability stack.
