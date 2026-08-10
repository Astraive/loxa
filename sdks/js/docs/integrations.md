# Integrations

Logging and tracing integrations for the LOZA JS SDK (`loza`).

## Logging Framework Bridges

The JS SDK can be used as a structured logging backend. Events follow the LOZA wide-event spec, which includes standard log fields (level, message, timestamp).

### Using LOZA as a Logging Backend

```typescript
import { loza } from 'loza';

// These create and emit events with the appropriate level
loza.info('User signed up', loza.userId('u-123'), loza.string('method', 'oauth'));
loza.error('Payment failed', loza.errorCode('PAYMENT_DECLINED'), loza.string('order_id', 'ord-456'));
```

### Bridging console.log

Replace `console.log` with LOZA structured events:

```typescript
import { loza } from 'loza';

function structuredLog(message: string, meta?: Record<string, any>) {
  const attrs = Object.entries(meta || {}).map(([k, v]) => loza.string(k, String(v)));
  loza.info(message, ...attrs);
}
```

## OpenTelemetry Integration

The SDK supports OpenTelemetry trace context propagation. When an OTel span is active, the SDK automatically extracts trace IDs:

```typescript
import { startEvent, traceId, spanId } from 'loza';

// If OTel context is available, traceId/spanId are auto-populated
const ctx = startEvent({ event: 'my.operation' });
```

The SDK reads:
- `traceparent` header for trace ID and span ID
- `tracestate` header for vendor-specific trace state
- `x-request-id` header for request ID
- `baggage` header for baggage items

## Winston Integration

Create a Winston transport that emits LOZA events:

```typescript
import winston from 'winston';
import { loza } from 'loza';

const lozaTransport = new winston.transports.Console({
  log: (logInfo: any) => {
    const level = logInfo.level;
    const message = logInfo.message;
    const attrs = Object.entries(logInfo).filter(([k]) => !['level', 'message'].includes(k));
    const lozaAttrs = attrs.map(([k, v]) => loza.string(k, String(v)));

    if (level === 'error') loza.error(message, ...lozaAttrs);
    else if (level === 'warn') loza.warn(message, ...lozaAttrs);
    else loza.info(message, ...lozaAttrs);
  },
});
```

## Pino Integration

Pino is a popular fast JSON logger for Node.js. Bridge it to LOZA:

```typescript
import pino from 'pino';
import { loza } from 'loza';

const pinoLogger = pino({
  hooks: {
    logMethod(inputArgs: any[], method: any) {
      const [msg, ...rest] = inputArgs;
      const attrs = rest.map((v: any, i: number) => loza.string(`extra_${i}`, String(v)));
      loza.info(msg, ...attrs);
      method.apply(this, inputArgs);
    },
  },
});
```

## Design Notes

- Integrations are optional; the core SDK has no external dependencies.
- The SDK's event lifecycle (start -> enrich -> finish -> emit) maps naturally to request-scoped logging.
- For application-wide logging (not request-scoped), use the immediate log functions (`debug`, `info`, `warn`, `error`, `fatal`).
- AsyncLocalStorage ensures event context flows through async boundaries without explicit passing.
