# Getting Started

A 5-minute quickstart for the LOXA JS SDK (`loxa-js`). By the end you will have a working application that creates, enriches, finishes, and emits a wide-event using the `loxa` default instance.

## Default Client

Use `loxa.<method>()` for quick starts and single-client applications.

## Custom Client / Alias

Use `createLoxa(config)` for an independent client and `loxa.alias("name")` for a same-config child that emits `loxa.alias`.

## Cross-Language Parity

JS maps to the v0.0.2 parity family as `loxa`, `createLoxa`, optional `new Loxa`, and user-defined variables such as `logger.info(...)`.

## Install

```bash
npm install loxa-js
```

Or from the monorepo:

```bash
cd sdks/js
npm install
npm run build
```

## Full Working Example

```typescript
import {
  loxa,
  configure,
  production,
  httpBatchSink,
  sampleErrors,
  defaultRedactor,
  userId,
  tenantId,
  string,
  int,
} from 'loxa-js';

async function main() {
  // 1. Configure the default loxa instance with a service name, sink, and sampler.
  configure(
    production('checkout-service')
      .withSink(httpBatchSink({ endpoint: 'http://collector:9090/events' }))
      .withSampler(sampleErrors())
      .withRedactor(defaultRedactor())
  );

  // 2. Start an event on the loxa instance.
  const ctx = loxa.startEvent({
    event: 'checkout.request',
    kind: 'http',
  });

  // 3. Enrich with typed attributes.
  loxa.enrich(ctx,
    userId('u-abc123'),
    tenantId('tenant-acme'),
    string('cart.id', 'cart-42'),
    int('item_count', 3),
  );

  // 4. Record a checkpoint mid-flow.
  loxa.checkpoint(ctx, 'payment.charged', { amount: 5999 });

  // 5. Finish the event with an outcome.
  loxa.finish(ctx, 'success', int('status_code', 200));

  // 6. Emit the event to the configured sink.
  const result = await loxa.emit(ctx);
  console.log('Emitted:', result);

  // 7. Flush buffered events and shut down.
  await loxa.flush();
  await loxa.shutdown();
}

main().catch(console.error);
```

## What Each Step Does

| Step | Call | Purpose |
|------|------|---------|
| configure | `configure(production(...))` | Sets up the default `loxa` instance with service name, sink, sampler, and redactor. |
| startEvent | `loxa.startEvent({ event, kind })` | Creates a new event with a UUIDv7 ID and stores it in async context. |
| enrich | `loxa.enrich(ctx, ...attrs)` | Adds typed attributes (userId, string, int, etc.) to the event. |
| checkpoint | `loxa.checkpoint(ctx, name)` | Records a named checkpoint with a timestamp for mid-flow milestones. |
| finish | `loxa.finish(ctx, outcome, ...attrs)` | Marks the event as finished, records outcome and duration. |
| emit | `await loxa.emit(ctx)` | Sanitizes, encodes, applies redaction, and delivers the event to the sink. Returns the encoded payload or `null` if sampled/dropped. |
| flush | `await loxa.flush()` | Flushes any buffered events in the sink. |
| shutdown | `await loxa.shutdown()` | Closes the sink and releases resources. |

## Connecting to a Collector

In production, send events to the LOXA collector with authentication:

```typescript
import { configure, production, httpBatchSink, sampleErrors } from 'loxa-js';

configure(
  production('checkout-service')
    .withApiKey(process.env.LOXA_API_KEY!)
    .withCollectorUrl('https://collector.loxa.dev')
    .withSampler(sampleErrors())
);
```

The SDK automatically sets `Authorization: Bearer <key>` headers.

### Authentication

| Config Field | Env Var | Description |
|---|---|---|
| `apiKey` | `LOXA_API_KEY` | Ingest API key (`lx_sec_live_k_xxx_yyyy`) |

```typescript
import { configure, production, dev } from 'loxa-js';

// Production
configure(production('my-service').withApiKey('lx_sec_live_k_xxx_yyyy'));

// Local dev
configure(dev('my-service').withApiKey('lx_local_dev_mytoken'));
```

See [Security](../../docs/security.md) for key types and RBAC roles.

## Immediate Log Helpers

For simple log lines that do not need the full start/enrich/finish/emit lifecycle, use the convenience methods directly on `loxa`:

```typescript
import { loxa, configure, production, string } from 'loxa-js';

configure(production('my-service'));

await loxa.info('server started', string('port', '8080'));
await loxa.warn('cache miss', string('key', 'user:abc'));
await loxa.error('payment failed', string('provider', 'stripe'));
```

These create an event internally, set the level, enrich with any extra attrs, finish with `success`, and emit in a single call.

## Custom Instances

Use `createLoxa` when you need a logger with a different config than the default:

```typescript
import { createLoxa, stdoutSink, string } from 'loxa-js';

const logger = createLoxa({
  service: 'checkout-api',
  sink: stdoutSink(),
});

const ctx = logger.startEvent({ event: 'checkout.begin' });
logger.enrich(ctx, string('cart.id', 'cart-99'));
logger.finish(ctx, 'success');
await logger.emit(ctx);
```

Custom instances are independent -- they do not share config or state with the default `loxa` instance.

## Aliases

Use `alias` to create a logger that shares the same config as `loxa` and adds `loxa.alias` metadata. This is useful for emitting events from a logical subsystem without duplicating configuration:

```typescript
import { loxa, configure, production, httpBatchSink, string } from 'loxa-js';

configure(
  production('checkout-service')
    .withSink(httpBatchSink({ endpoint: 'http://collector:9090/events' }))
);

const audit = loxa.alias('audit-service');

// Regular checkout event
const ctx = loxa.startEvent({ event: 'checkout.request' });
loxa.finish(ctx, 'success');
await loxa.emit(ctx);

// Audit event with loxa.alias metadata
const auditCtx = audit.startEvent({ event: 'audit.action', kind: 'http' });
audit.enrich(ctx, string('action', 'user.login'));
audit.finish(auditCtx, 'success');
await audit.emit(auditCtx);
```

Aliases inherit the service, sink, sampler, redactor, and schema from the parent. They do not mutate the parent logger.

## Event Lifecycle

```
startEvent -> enrich -> checkpoint -> finish -> emit
     |           |          |           |        |
  creates     adds      records      marks    sanitizes
  context    attrs     milestone     done     + delivers
```

## Cross-Language Parity

All LOXA SDKs expose the same conceptual API. Here is how the JS patterns map to the other languages:

| Operation | JS | Python | Go | Rust |
|-----------|-----|--------|----|------|
| Configure default | `configure(production('svc'))` | `loxa.configure(loxa.production('svc'))` | `loxa.Configure(loxa.Production("svc"))` | `loxa::configure(loxa::production("svc"))` |
| Start event | `loxa.startEvent({...})` | `loxa.start_event({...})` | `loxa.StartEvent(ctx, {...})` | `loxa::start_event({...})` |
| Enrich | `loxa.enrich(ctx, ...)` | `loxa.enrich(ctx, ...)` | `loxa.Enrich(ctx, ...)` | `loxa::enrich(&ctx, ...)` |
| Checkpoint | `loxa.checkpoint(ctx, name)` | `loxa.checkpoint(ctx, name)` | `loxa.Checkpoint(ctx, name)` | `loxa::checkpoint(&ctx, name)` |
| Finish | `loxa.finish(ctx, 'success')` | `loxa.finish(ctx, 'success')` | `loxa.Finish(ctx, "success")` | `loxa::finish(&ctx, "success")` |
| Emit | `await loxa.emit(ctx)` | `loxa.emit(ctx)` | `loxa.Emit(ctx)` | `loxa::emit(&ctx)` |
| Info log | `await loxa.info('msg')` | `loxa.info('msg')` | `loxa.Info(ctx, "msg")` | `loxa::info("msg")` |
| Custom instance | `createLoxa({service:'x'})` | `loxa.create_loxa(service='x')` | `loxa.New(loxa.WithService("x"))` | `loxa::create_loxa(config)` |
| Alias | `loxa.alias('audit')` | `loxa.alias('audit')` | `loxa.Alias("audit")` | `loxa::alias("audit")` |

## Next Steps

- [Public API](public-api.md) -- Full API reference.
- [Middleware](middleware.md) -- Express, http, and AsyncLocalStorage integration.
- [Integrations](integrations.md) -- Logging and tracing integrations.
- [Instrumentation](instrumentation.md) -- Business event instrumentation patterns.
