# Getting Started

A 5-minute quickstart for the LOZA JS SDK (`loza`). By the end you will have a working application that creates, enriches, finishes, and emits a wide-event using the `loza` default instance.

## Default Client

Use `loza.<method>()` for quick starts and single-client applications.

## Custom Client / Alias

Use `createLoza(config)` for an independent client and `loza.alias("name")` for a same-config child that emits `loza.alias`.

## Cross-Language Parity

JS maps to the v0.0.2 parity family as `loza`, `createLoza`, optional `new Loza`, and user-defined variables such as `logger.info(...)`.

## Install

```bash
npm install @astraive/loza
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
  loza,
  configure,
  production,
  httpBatchSink,
  sampleErrors,
  defaultRedactor,
  userId,
  tenantId,
  string,
  int,
} from '@astraive/loza';

async function main() {
  // 1. Configure the default loza instance with a service name, sink, and sampler.
  configure(
    production('checkout-service')
      .withSink(httpBatchSink({ endpoint: 'http://collector:9308/events' }))
      .withSampler(sampleErrors())
      .withRedactor(defaultRedactor())
  );

  // 2. Start an event on the loza instance.
  const ctx = loza.startEvent({
    event: 'checkout.request',
    kind: 'http',
  });

  // 3. Enrich with typed attributes.
  loza.enrich(ctx,
    userId('u-abc123'),
    tenantId('tenant-acme'),
    string('cart.id', 'cart-42'),
    int('item_count', 3),
  );

  // 4. Record a checkpoint mid-flow.
  loza.checkpoint(ctx, 'payment.charged', { amount: 5999 });

  // 5. Finish the event with an outcome.
  loza.finish(ctx, 'success', int('status_code', 200));

  // 6. Emit the event to the configured sink.
  const result = await loza.emit(ctx);
  console.log('Emitted:', result);

  // 7. Flush buffered events and shut down.
  await loza.flush();
  await loza.shutdown();
}

main().catch(console.error);
```

## What Each Step Does

| Step | Call | Purpose |
|------|------|---------|
| configure | `configure(production(...))` | Sets up the default `loza` instance with service name, sink, sampler, and redactor. |
| startEvent | `loza.startEvent({ event, kind })` | Creates a new event with a UUIDv7 ID and stores it in async context. |
| enrich | `loza.enrich(ctx, ...attrs)` | Adds typed attributes (userId, string, int, etc.) to the event. |
| checkpoint | `loza.checkpoint(ctx, name)` | Records a named checkpoint with a timestamp for mid-flow milestones. |
| finish | `loza.finish(ctx, outcome, ...attrs)` | Marks the event as finished, records outcome and duration. |
| emit | `await loza.emit(ctx)` | Sanitizes, encodes, applies redaction, and delivers the event to the sink. Returns the encoded payload or `null` if sampled/dropped. |
| flush | `await loza.flush()` | Flushes any buffered events in the sink. |
| shutdown | `await loza.shutdown()` | Closes the sink and releases resources. |

## Connecting to a Collector

In production, send events to the LOZA collector with authentication:

```typescript
import { configure, production, httpBatchSink, sampleErrors } from '@astraive/loza';

configure(
  production('checkout-service')
    .withApiKey(process.env.LOZA_API_KEY!)
    .withCollectorUrl('https://collector.loza.dev')
    .withSampler(sampleErrors())
);
```

The SDK automatically sets `Authorization: Bearer <key>` headers.

### Authentication

| Config Field | Env Var | Description |
|---|---|---|
| `apiKey` | `LOZA_API_KEY` | Ingest API key (`lz_sec_live_k_xxx_yyyy`) |

```typescript
import { configure, production, dev } from '@astraive/loza';

// Production
configure(production('my-service').withApiKey('lz_sec_live_k_xxx_yyyy'));

// Local dev
configure(dev('my-service').withApiKey('lz_local_dev_mytoken'));
```

See [Security](../../docs/security.md) for key types and RBAC roles.

## Immediate Log Helpers

For simple log lines that do not need the full start/enrich/finish/emit lifecycle, use the convenience methods directly on `loza`:

```typescript
import { loza, configure, production, string } from '@astraive/loza';

configure(production('my-service'));

await loza.info('server started', string('port', '8080'));
await loza.warn('cache miss', string('key', 'user:abc'));
await loza.error('payment failed', string('provider', 'stripe'));
```

These create an event internally, set the level, enrich with any extra attrs, finish with `success`, and emit in a single call.

## Custom Instances

Use `createLoza` when you need a logger with a different config than the default:

```typescript
import { createLoza, stdoutSink, string } from '@astraive/loza';

const logger = createLoza({
  service: 'checkout-api',
  sink: stdoutSink(),
});

const ctx = logger.startEvent({ event: 'checkout.begin' });
logger.enrich(ctx, string('cart.id', 'cart-99'));
logger.finish(ctx, 'success');
await logger.emit(ctx);
```

Custom instances are independent -- they do not share config or state with the default `loza` instance.

## Aliases

Use `alias` to create a logger that shares the same config as `loza` and adds `loza.alias` metadata. This is useful for emitting events from a logical subsystem without duplicating configuration:

```typescript
import { loza, configure, production, httpBatchSink, string } from '@astraive/loza';

configure(
  production('checkout-service')
    .withSink(httpBatchSink({ endpoint: 'http://collector:9308/events' }))
);

const audit = loza.alias('audit-service');

// Regular checkout event
const ctx = loza.startEvent({ event: 'checkout.request' });
loza.finish(ctx, 'success');
await loza.emit(ctx);

// Audit event with loza.alias metadata
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

All LOZA SDKs expose the same conceptual API. Here is how the JS patterns map to the other languages:

| Operation | JS | Python | Go | Rust |
|-----------|-----|--------|----|------|
| Configure default | `configure(production('svc'))` | `loza.configure(loza.production('svc'))` | `loza.Configure(loza.Production("svc"))` | `loza::configure(loza::production("svc"))` |
| Start event | `loza.startEvent({...})` | `loza.start_event({...})` | `loza.StartEvent(ctx, {...})` | `loza::start_event({...})` |
| Enrich | `loza.enrich(ctx, ...)` | `loza.enrich(ctx, ...)` | `loza.Enrich(ctx, ...)` | `loza::enrich(&ctx, ...)` |
| Checkpoint | `loza.checkpoint(ctx, name)` | `loza.checkpoint(ctx, name)` | `loza.Checkpoint(ctx, name)` | `loza::checkpoint(&ctx, name)` |
| Finish | `loza.finish(ctx, 'success')` | `loza.finish(ctx, 'success')` | `loza.Finish(ctx, "success")` | `loza::finish(&ctx, "success")` |
| Emit | `await loza.emit(ctx)` | `loza.emit(ctx)` | `loza.Emit(ctx)` | `loza::emit(&ctx)` |
| Info log | `await loza.info('msg')` | `loza.info('msg')` | `loza.Info(ctx, "msg")` | `loza::info("msg")` |
| Custom instance | `createLoza({service:'x'})` | `loza.create_loza(service='x')` | `loza.New(loza.WithService("x"))` | `loza::create_loza(config)` |
| Alias | `loza.alias('audit')` | `loza.alias('audit')` | `loza.Alias("audit")` | `loza::alias("audit")` |

## Next Steps

- [Public API](public-api.md) -- Full API reference.
- [Middleware](middleware.md) -- Express, http, and AsyncLocalStorage integration.
- [Integrations](integrations.md) -- Logging and tracing integrations.
- [Instrumentation](instrumentation.md) -- Business event instrumentation patterns.
