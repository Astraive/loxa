# Integrations

Logging and tracing integrations for the LOXA JS SDK (`loxa`).

## Logging Framework Bridges

The JS SDK can be used as a structured logging backend. Events follow the LOXA wide-event spec, which includes standard log fields (level, message, timestamp).

### Using LOXA as a Logging Backend

```typescript
import { loxa } from 'loxa';

// These create and emit events with the appropriate level
loxa.info('User signed up', loxa.userId('u-123'), loxa.string('method', 'oauth'));
loxa.error('Payment failed', loxa.errorCode('PAYMENT_DECLINED'), loxa.string('order_id', 'ord-456'));
```

### Bridging console.log

Replace `console.log` with LOXA structured events:

```typescript
import { loxa } from 'loxa';

function structuredLog(message: string, meta?: Record<string, any>) {
  const attrs = Object.entries(meta || {}).map(([k, v]) => loxa.string(k, String(v)));
  loxa.info(message, ...attrs);
}
```

## OpenTelemetry Integration

The SDK supports OpenTelemetry trace context propagation. When an OTel span is active, the SDK automatically extracts trace IDs:

```typescript
import { startEvent, traceId, spanId } from 'loxa';

// If OTel context is available, traceId/spanId are auto-populated
const ctx = startEvent({ event: 'my.operation' });
```

The SDK reads:
- `traceparent` header for trace ID and span ID
- `tracestate` header for vendor-specific trace state
- `x-request-id` header for request ID
- `baggage` header for baggage items

## Winston Integration

Create a Winston transport that emits LOXA events:

```typescript
import winston from 'winston';
import { loxa } from 'loxa';

const loxaTransport = new winston.transports.Console({
  log: (logInfo: any) => {
    const level = logInfo.level;
    const message = logInfo.message;
    const attrs = Object.entries(logInfo).filter(([k]) => !['level', 'message'].includes(k));
    const loxaAttrs = attrs.map(([k, v]) => loxa.string(k, String(v)));

    if (level === 'error') loxa.error(message, ...loxaAttrs);
    else if (level === 'warn') loxa.warn(message, ...loxaAttrs);
    else loxa.info(message, ...loxaAttrs);
  },
});
```

## Pino Integration

Pino is a popular fast JSON logger for Node.js. Bridge it to LOXA:

```typescript
import pino from 'pino';
import { loxa } from 'loxa';

const pinoLogger = pino({
  hooks: {
    logMethod(inputArgs: any[], method: any) {
      const [msg, ...rest] = inputArgs;
      const attrs = rest.map((v: any, i: number) => loxa.string(`extra_${i}`, String(v)));
      loxa.info(msg, ...attrs);
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
