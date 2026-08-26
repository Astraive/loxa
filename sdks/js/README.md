# loza

LOZA wide-event SDK for JavaScript/TypeScript — lightweight bridge connector to loza-collector.

**Status**: STABLE (v0.3.4) - Production-ready, full feature conformance

## Installation

```bash
npm install @astraive/loza
```

## Quick Start

```typescript
import { loza } from '@astraive/loza';

// Configure
loza.configure(
  loza.production('checkout')
    .withCollectorEndpoint('http://localhost:9308')
);

// Lifecycle
const ctx = loza.startEvent({
  event: 'checkout.request',
  kind: 'http',
  method: 'POST',
  path: '/checkout',
});

loza.append(ctx,
  loza.userId('u_123'),
  loza.cartId('cart_456'),
  loza.featureFlag('checkout_v2', 'on'),
);

loza.checkpoint(ctx, 'payment_started');

try {
  loza.finish(ctx, 'success', loza.int('status_code', 200));
} catch (err) {
  loza.finishError(ctx, err, loza.retryable(true));
} finally {
  await loza.emit(ctx);
}

await loza.shutdown();
```

## Core Lifecycle API

The main flow: `startEvent` -> `enrich` -> `checkpoint` -> `finish`/`finishError` -> `emit`

```typescript
import {
  startEvent, startHttpEvent, startJobEvent, startQueueEvent,
  startCliEvent, startCronEvent,
  append, enrich, set, merge, del, get, getGroup,
  checkpoint, finish, finishError, emit, flush, shutdown,
} from '@astraive/loza';

// Start an event
const ctx = startEvent({
  event: 'checkout.request',
  kind: 'http',
  method: 'POST',
  path: '/checkout',
  service: 'checkout',
});

// Typed starters
const httpCtx = startHttpEvent({ event: 'http.request', method: 'GET', path: '/health' });
const jobCtx = startJobEvent({ event: 'job.send_email' });
const queueCtx = startQueueEvent({ event: 'queue.process' });
const cliCtx = startCliEvent({ event: 'cli.run' });
const cronCtx = startCronEvent({ event: 'cron.tick' });

// Enrich (add attributes)
enrich(ctx, string('user.id', 'u_123'), int('cart.items', 3));

// Append (alias for enrich)
append(ctx, string('key', 'value'));

// Set (override)
set(ctx, 'status', 'processing');

// Merge into group
merge(ctx, { payment: { provider: 'stripe', attempt: 1 } });

// Delete
del(ctx, 'temp_field');

// Get
const val = get(ctx, 'user.id');
const group = getGroup(ctx, 'payment');

// Checkpoint
checkpoint(ctx, 'payment_started');
checkpoint(ctx, 'payment_finished', { provider: 'stripe' });

// Finish
finish(ctx, 'success', int('status_code', 200));

// Finish with error
try {
  processPayment();
} catch (err) {
  finishError(ctx, err, int('status_code', 500));
}

// Emit
await emit(ctx);

// Flush
await flush();

// Shutdown
await shutdown();
```

## Attribute Constructors

```typescript
import {
  string, int, int64, uint64, float64, bool, time, duration, any, group, nullAttr,
  String, Int, Int64, Uint64, Float64, Bool, Time, Duration, Any, Group, Null,
} from '@astraive/loza';

enrich(ctx,
  string('user.id', 'u_123'),
  int('cart.items', 3),
  int64('big_number', 9999999999),
  float64('price', 49.99),
  bool('premium', true),
  duration('timeout', 30000),
  any('metadata', { key: 'value' }),
  nullAttr('optional_field'),
);

// Groups (nested objects)
enrich(ctx,
  group('user',
    string('id', 'u_123'),
    string('email', 'user@example.com'),
  ),
  group('payment',
    string('provider', 'stripe'),
    int('attempt', 1),
  ),
);
```

Both camelCase and PascalCase are exported. Use whichever fits your style.

## Canonical Helpers

```typescript
import {
  userId, tenantId, workspaceId, organizationId, sessionId,
  requestId, traceId, spanId,
  featureFlag, featureFlagBool, experiment,
  // PascalCase aliases:
  UserID, TenantID, RequestID, TraceID, FeatureFlag, Experiment,
} from '@astraive/loza';

enrich(ctx,
  userId('u_123'),
  tenantId('t_456'),
  workspaceId('w_789'),
  organizationId('org_abc'),
  sessionId('sess_xyz'),
  requestId('req_123'),
  traceId('trace_abc'),
  spanId('span_def'),
  featureFlag('checkout_v2', 'enabled'),
  featureFlagBool('new_ui', true),
  experiment('pricing_test', 'variant_b'),
);
```

## Business/Domain Helpers

```typescript
import {
  orderId, cartId, productId, customerId,
  plan, currency, amount, country, device, platform, appVersion,
} from '@astraive/loza';

enrich(ctx,
  orderId('ord_123'),
  cartId('cart_456'),
  productId('prod_789'),
  customerId('cust_abc'),
  plan('pro'),
  currency('INR'),
  amount(4999),
  country('IN'),
  device('mobile'),
  platform('ios'),
  appVersion('2.1.0'),
);
```

## Error Helpers

```typescript
import {
  errorType, errorCode, errorMessage, errorStack, retryable,
} from '@astraive/loza';

try {
  process();
} catch (err) {
  finishError(ctx, err,
    errorType('ValidationError'),
    errorCode('INVALID_INPUT'),
    retryable(false),
  );
}
```

## Immediate Logs

One-shot events without requiring `startEvent`:

```typescript
import { loza } from '@astraive/loza';

await loza.info('worker started', loza.string('queue', 'emails'));
await loza.error('payment failed', loza.string('provider', 'stripe'));
```

## Logger Instances

```typescript
import { loza, createLoza } from '@astraive/loza';

// Default API — configure once, use everywhere
loza.configure(loza.production('checkout').withCollectorEndpoint('http://127.0.0.1:9308'));
loza.info('server started');

// Custom instance
const logger = createLoza({ service: 'checkout-api', collectorUrl: 'http://127.0.0.1:9308' });
logger.info('custom instance ready');

// Alias -- same config, loza.alias metadata
const audit = loza.alias('audit-service');
audit.info('audit trail started');

// Presets
const cfg = loza.dev('checkout');       // pretty JSON, stdout, sync, debug level
const cfg2 = loza.production('checkout'); // compact JSON, stdout, async, info level
const cfg3 = loza.test('checkout');       // sync, no sinks, debug level

// Instance methods
const ctx = logger.startEvent({ event: 'checkout.request' });
logger.enrich(ctx, loza.string('key', 'value'));
logger.finish(ctx, 'success');
await logger.emit(ctx);
await logger.flush();
await logger.shutdown();

// Immediate logs
await logger.info('started');
await logger.error('failed');
```

## Config API

```typescript
import {
  production, dev, test,
  stdoutSink, fileSink, memorySink, httpBatchSink,
  sampleAll, sampleErrors, sampleRandom, sampleSlowRequests,
  defaultRedactor, redactKeys, hashKeys, dropKeys,
} from '@astraive/loza';

const cfg = production('checkout')
  .withVersion('0.0.2')
  .withEnvironment('prod')
  .withSink(stdoutSink())
  .withSampler(sampleErrors())
  .withRedactor(defaultRedactor())
  .withAsync(true);
```

## Levels

```typescript
import { LevelDebug, LevelInfo, LevelWarn, LevelError, LevelFatal, parseLevel } from '@astraive/loza';

const level = parseLevel('info'); // LevelInfo
```

## Sinks

```typescript
import {
  stdoutSink, stderrSink, fileSink, memorySink, noopSink, httpBatchSink,
} from '@astraive/loza';

const cfg = production('checkout').withSink(stdoutSink());
const cfg2 = production('checkout').withSink(fileSink('/var/log/app.log'));
const cfg3 = production('checkout').withSink(httpBatchSink({
  endpoint: 'http://collector:9308/events',
}));

// For testing
const sink = memorySink();
const logger = createLoza({ service: 'checkout', sink });
// ... use logger ...
const events = sink.getEvents();
```

## Sampling

```typescript
import {
  sampleAll, sampleNone, sampleRandom, sampleErrors,
  sampleSlowRequests, sampleStatusCodes, sampleRoutes,
  sampleUsers, sampleTenants, sampleFeatureFlag,
  sampleRateLimited, sampleByHeader,
  anySampler, allSampler, notSampler,
} from '@astraive/loza';

const cfg = production('checkout').withSampler(sampleAll());
const cfg2 = production('checkout').withSampler(sampleRandom(0.01));
const cfg3 = production('checkout').withSampler(sampleErrors());
const cfg4 = production('checkout').withSampler(sampleSlowRequests(500));
const cfg5 = production('checkout').withSampler(sampleStatusCodes(500, 502, 503));
const cfg6 = production('checkout').withSampler(sampleRateLimited(100, 1000));

// Combinators
const cfg7 = production('checkout').withSampler(
  anySampler(
    sampleErrors(),
    sampleSlowRequests(500),
    sampleRandom(0.01),
  )
);
```

## Redaction

```typescript
import {
  defaultRedactor, redactKeys, redactPatterns, hashKeys, maskKeys, dropKeys,
  composeRedactors, sensitiveString, markSensitive, hashString,
} from '@astraive/loza';

const cfg = production('checkout').withRedactor(defaultRedactor());
const cfg2 = production('checkout').withRedactor(redactKeys('password', 'token'));
const cfg3 = production('checkout').withRedactor(hashKeys('user.email'));

// Compose
const cfg4 = production('checkout').withRedactor(
  composeRedactors(
    defaultRedactor(),
    redactKeys('password', 'token'),
    hashKeys('user.email'),
  )
);

// Mark fields
enrich(ctx,
  sensitiveString('user.email', email),
  markSensitive('credit_card', cardNo),
  hashString('user.ssn', ssn),
);
```

## Schema

```typescript
import {
  defaultSchema, flatSchema, nestedSchema, otelLogSchema, ecSchema,
  datadogSchema, customSchema,
} from '@astraive/loza';

const cfg = production('checkout').withSchema(defaultSchema());
const cfg2 = production('checkout').withSchema(flatSchema());
const cfg3 = production('checkout').withSchema(otelLogSchema());

// Custom schema
const cfg4 = production('checkout').withSchema(
  customSchema((ev) => ({
    ts: ev.timestamp(),
    service: ev.service(),
    requestId: ev.requestId(),
    took: ev.durationMs(),
    fields: ev.attrs(),
  }))
);
```

## Duplicate Policy

```typescript
import {
  CanonicalWins, UserWins, FirstWins, LastWins, KeepBoth, ErrorOnDuplicate,
} from '@astraive/loza';

const cfg = production('checkout').withDuplicatePolicy(CanonicalWins);
const cfg2 = production('checkout').withDuplicatePolicy(LastWins);
```

## Context Mutation

```typescript
import { append, enrich, set, merge, del, get, getGroup } from '@astraive/loza';

append(ctx, string('user.id', 'u_123'), string('cart.id', 'cart_456'));
enrich(ctx, int('payment.attempt', 1));
set(ctx, 'status', 'processing');
merge(ctx, { payment: { provider: 'stripe' } });
del(ctx, 'temp_field');
const val = get(ctx, 'user.id');
const group = getGroup(ctx, 'payment');
```

## Middleware

```typescript
// Express
import { lozaMiddleware } from '@astraive/loza/middleware/express';

app.use(lozaMiddleware({ service: 'checkout' }));
```

## Feature Flags

```typescript
} from '@astraive/loza';

enrich(ctx,
  featureFlag('checkout_v2', 'enabled'),
  featureFlagBool('new_ui', true),
  experiment('pricing_test', 'variant_b'),
);
```

## Security

```typescript
import { sensitiveString, markSensitive, hashString } from '@astraive/loza';

enrich(ctx,
  sensitiveString('user.email', email),
  markSensitive('credit_card', cardNo),
  hashString('user.ssn', ssn),
);
```

## Context Helpers

```typescript
import { getEvent, hasEvent, eventId } from '@astraive/loza';

const ev = getEvent(ctx);
if (hasEvent(ctx)) {
  const id = eventId(ctx);
}
```

## Testing

```typescript
import { testLogger, capture, assertEvent, assertRedacted, assertHasCheckpoint } from '@astraive/loza/testkit';

// Test logger with memory sink
const { logger, sink } = testLogger();

// Capture events
const events = capture(async (logger) => {
  const ctx = logger.startEvent({ event: 'test' });
  logger.finish(ctx, 'success');
  await logger.emit(ctx);
});

// Assert
assertEvent(events[0], 'user.id', 'u_123');
assertRedacted(events[0], 'password');
assertHasCheckpoint(events[0], 'payment_started');
```

## HTTP Client Instrumentation

```typescript
import { wrapHttpClient, newRoundTripper } from '@astraive/loza';

// Wrap fetch or http client
const client = wrapHttpClient(originalClient);
```

## Run Tests

```bash
bun run test
bun run build
python ../../spec/conformance/runner.py --sdk javascript --group all
```

## Documentation

- [Instrumentation Guide](docs/business-instrumentation.md) — instrumenting checkout, payments, auth, jobs, queues, cron
- [docs/public-api.md](docs/public-api.md)
- [docs/middleware.md](docs/middleware.md)
- [docs/integrations.md](docs/integrations.md)

## Architecture

```
SDK (`loza`) -> HTTP -> Collector -> gRPC -> Cortex
```

SDKs are lightweight bridge connectors. Heavy processing (dedup, schema validation, PII redaction, sampling) happens in the collector.

## License

MIT
