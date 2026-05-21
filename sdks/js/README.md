# loxa-js

LOXA wide-event SDK for JavaScript/TypeScript — lightweight bridge connector to loxa-collector.

**Status**: 🟢 **STABLE** (v1.0.0) - Production-ready, full feature conformance

Stable-v1 parity and release gates are tracked through:

- `../loxa-spec/docs/sdk-parity-manifest.json`
- `docs/sdk-parity-manifest.json`
- `../loxa-spec/docs/SDK_CONFORMANCE_CONTRACT.md`

## Installation

```bash
npm install loxa-js
```

## Quick Start — Default Facade

```typescript
import * as loxa from 'loxa-js';

loxa.configure(
  loxa.production('checkout')
    .withSink(loxa.httpBatchSink({
      endpoint: 'http://localhost:9090/v1/events',
    }))
);

const ctx = loxa.startEvent({
  event: 'checkout.request',
  kind: 'http',
  method: 'POST',
  path: '/checkout',
});

loxa.append(ctx,
  loxa.userId('u_123'),
  loxa.cartId('cart_456'),
  loxa.featureFlag('checkout_v2', 'on'),
);

loxa.checkpoint(ctx, 'payment_started');

try {
  loxa.finish(ctx, 'success', loxa.int('status_code', 200));
} catch (err) {
  loxa.finishError(ctx, err, loxa.retryable(true));
} finally {
  await loxa.emit(ctx);
}

await loxa.shutdown();
```

## Advanced — Logger Instance

```typescript
import { Logger, production, memorySink, userId } from 'loxa-js';

const logger = new Logger(
  production('checkout')
    .withSink(memorySink())
);

const ctx = logger.startEvent({ event: 'job.send_email', kind: 'job' });
logger.append(ctx, userId('u_123'));
logger.finish(ctx, 'success');
await logger.emit(ctx);
await logger.shutdown();
```

## Features

- **Wide events** — structured events with rich context
- **Default facade** — `import * as loxa from 'loxa-js'` for zero-config usage
- **Lightweight** — zero external dependencies (Node.js stdlib only)
- **Type-safe** — full TypeScript support with camelCase API
- **Stable aliases** — PascalCase parity exports match Go, Python, and Rust
- **Async delivery** — configurable batching and retry
- **Redaction** — sensitive attr sanitization + safety-net key redaction
- **Sampling** — client-side traffic reduction
- **Middleware** — Express
- **Spec-backed** — generated from loxa-spec JSON Schema

## Architecture

```
SDK (loxa-js) → HTTP → Collector → gRPC → Cortex
```

SDKs are lightweight bridge connectors. Heavy processing (dedup, schema validation, PII redaction, sampling) happens in the collector.

Collector-owned sinks and storage (`Kafka`, `DuckDB`, `ClickHouse`, `Postgres`, `Loki`, `OTLP`, `S3`, and `GCS`) are intentionally outside SDK parity scope.

## Verification

```bash
npm test
npm run build
python ../loxa-spec/conformance/runner.py --sdk javascript --group all
```

## License

MIT
