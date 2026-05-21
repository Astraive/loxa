# Getting Started

A 5-minute quickstart for the LOXA JS SDK (loxa-js). By the end you will have a working application that creates, enriches, finishes, and emits a wide-event.

## Install

```bash
npm install loxa-js
```

Or from the repository:

```bash
cd sdks/js
npm install
npm run build
```

## Full Working Example

```typescript
import {
  production,
  httpBatchSink,
  configure,
  startEvent,
  enrich,
  finish,
  emit,
  flush,
  shutdown,
  userId,
  tenantId,
  string,
  int,
} from 'loxa-js';

async function main() {
  // 1. Configure a logger with an HTTP batch sink.
  const logger = configure(
    production('checkout-service').withSink(
      httpBatchSink({ endpoint: 'http://collector:9090/v1/events' })
    )
  );

  // 2. Start an event.
  const ctx = startEvent({
    event: 'checkout.request',
    kind: 'http',
  });

  // 3. Enrich with attributes.
  enrich(ctx,
    userId('u-abc123'),
    tenantId('tenant-acme'),
    string('cart.id', 'cart-42'),
    int('item_count', 3),
  );

  // 4. Finish the event with an outcome.
  finish(ctx, 'success', int('status_code', 200));

  // 5. Emit the event to the configured sink.
  const eventId = await emit(ctx);
  console.log('Emitted event:', eventId);

  // 6. Flush and shut down.
  await flush();
  await shutdown();
}

main().catch(console.error);
```

## What Each Step Does

| Step | Function | Purpose |
|------|----------|---------|
| configure | `configure(config)` | Sets the global default logger with service name, sink, sampler. |
| startEvent | `startEvent(params)` | Creates a new `Event` with a UUIDv7 event ID. |
| enrich | `enrich(ctx, ...attrs)` | Adds typed attributes to the event. |
| finish | `finish(ctx, outcome, ...attrs)` | Marks the event as finished, records outcome and duration. |
| emit | `await emit(ctx)` | Serializes and sends the event to the sink. Returns event ID. |
| flush | `await flush()` | Flushes buffered events. |
| shutdown | `await shutdown()` | Shuts down the logger. |

## Connecting to a Collector

In production, send events to the LOXA collector:

```typescript
import { production, httpBatchSink, sampleErrors, configure } from 'loxa-js';

configure(
  production('checkout-service')
    .withSink(httpBatchSink({ endpoint: 'http://collector:9090/v1/events' }))
    .withSampler(sampleErrors())
);
```

## Using the Logger Directly

Instead of the module-level facade, you can use a Logger instance:

```typescript
import { Logger, production, stdoutSink } from 'loxa-js';

const logger = new Logger(production('my-service').withSink(stdoutSink()));

const ctx = logger.startEvent({ event: 'my.event' });
logger.enrich(ctx, string('key', 'value'));
logger.finish(ctx, 'success');
await logger.emit(ctx);
```

## Event Lifecycle

```
startEvent -> enrich -> finish -> emit
     |            |        |       |
  creates     adds      marks   serializes
  context    attrs     done     + sends
```

## Next Steps

- [Public API](public-api.md) -- Full API reference.
- [Middleware](middleware.md) -- Express, http, and AsyncLocalStorage integration.
- [Integrations](integrations.md) -- Logging and tracing integrations.
