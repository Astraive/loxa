# Public API

The JS SDK (loxa-js) public API surface, aligned with the cross-language parity manifest at `docs/sdk-parity-manifest.json`.

## Module Exports

The SDK exports from `loxa-js` (main entry) and sub-path exports for middleware:

```typescript
import { ... } from 'loxa-js';
import { loxaMiddleware } from 'loxa-js/middleware/express';
```

## Lifecycle

| Function | Description |
|----------|-------------|
| `startEvent(params)` | Create a new event. |
| `startHttpEvent(params)` | Create an HTTP-kind event. |
| `startJobEvent(params)` | Create a job-kind event. |
| `startQueueEvent(params)` | Create a queue-kind event. |
| `startCliEvent(params)` | Create a CLI-kind event. |
| `startCronEvent(params)` | Create a cron-kind event. |
| `append(ctx, ...attrs)` | Append attributes to an event. |
| `enrich(ctx, ...attrs)` | Enrich an event with attributes. |
| `set(ctx, key, value)` | Set a single attribute, overwriting if exists. |
| `merge(ctx, obj)` | Merge a record of key-value pairs into the event. |
| `del(ctx, key)` | Remove a key from the event. |
| `get(ctx, key)` | Get a single attribute value. |
| `getGroup(ctx, prefix)` | Get all attributes with a common prefix. |
| `checkpoint(ctx, name, attrs?)` | Record a named checkpoint with timestamp. |
| `finish(ctx, outcome, ...attrs)` | Finish the event with an outcome. |
| `finishError(ctx, err, ...attrs)` | Finish the event with an error outcome. |
| `emit(ctx)` | Serialize and send the event. Returns event ID. |
| `runEvent(params, fn, finishAttrs?)` | Run a function within an event context. |
| `flush()` | Flush buffered events. |
| `shutdown()` | Shut down the logger. |

## Immediate Logs

| Function | Level |
|----------|-------|
| `debug(message, ...attrs)` | debug |
| `info(message, ...attrs)` | info |
| `warn(message, ...attrs)` | warn |
| `error(message, ...attrs)` | error |
| `fatal(message, ...attrs)` | fatal |

## Logger Configuration

| Function | Description |
|----------|-------------|
| `configure(config)` | Set the global default logger. |
| `getDefault()` | Get the global default logger. |
| `reset()` | Reset the global default logger. |
| `dev(service)` | Dev-mode config preset. |
| `production(service)` | Production config preset. |
| `test(service)` | Test config preset. |

### ConfigBuilder

```typescript
import { production, httpBatchSink, sampleErrors } from 'loxa-js';

const config = production('my-service')
  .withSink(httpBatchSink({ endpoint: 'http://collector:9308/events' }))
  .withSampler(sampleErrors())
  .withRedactor(defaultRedactor())
  .withAsync(true);
```

## Attribute Constructors

### PascalCase (canonical)

`String`, `Int`, `Int64`, `Uint64`, `Float64`, `Bool`, `Null`, `Any`, `Group`, `Time`, `Duration`, `SensitiveString`, `HashString`, `MarkSensitive`

### camelCase (primary v1 API)

`string`, `int`, `int64`, `uint64`, `float64`, `bool`, `any`, `group`, `time`, `duration`, `sensitiveString`, `hashString`, `markSensitive`

### Domain Helpers

`userId`, `tenantId`, `workspaceId`, `organizationId`, `sessionId`, `requestId`, `traceId`, `spanId`, `featureFlag`, `featureFlagBool`, `experiment`, `orderId`, `cartId`, `productId`, `customerId`, `plan`, `currency`, `amount`, `country`, `device`, `platform`, `appVersion`, `errorType`, `errorCode`, `errorMessage`, `errorStack`, `retryable`

## Sinks

| Sink | Description |
|------|-------------|
| `stdoutSink()` | Writes events to stdout. |
| `stderrSink()` | Writes events to stderr. |
| `fileSink(path)` | Writes events to a file. |
| `rotatingFileSink(path)` | Writes events to a rotating file. |
| `memorySink()` | Stores events in memory for testing. |
| `noopSink()` | Discards all events. |
| `httpBatchSink(opts)` | Batches and sends events via HTTP. |
| `collectorSink()` | Alias for httpBatchSink with default options. |

## Samplers

| Sampler | Description |
|---------|-------------|
| `sampleAll()` | Sample 100% of events. |
| `sampleNone()` | Drop all events. |
| `sampleRandom(rate)` | Sample at the given rate (0.0-1.0). |
| `sampleErrors()` | Always sample error events. |
| `sampleSlowRequests(threshold)` | Sample requests slower than threshold. |
| `sampleStatusCodes(...codes)` | Sample specific HTTP status codes. |
| `sampleRoutes(...routes)` | Sample specific routes. |
| `sampleUsers(...ids)` | Sample specific user IDs. |
| `sampleTenants(...ids)` | Sample specific tenant IDs. |
| `sampleFeatureFlag(name, value)` | Sample based on feature flag. |
| `sampleByHeader(header, value)` | Sample based on HTTP header. |
| `anySampler(...samplers)` | Sample if any sub-sampler matches. |
| `allSampler(...samplers)` | Sample if all sub-samplers match. |
| `notSampler(sampler)` | Invert a sampler. |

## Redactors

| Redactor | Description |
|----------|-------------|
| `defaultRedactor()` | 14-key safety-net redactor. |
| `redactKeys(...keys)` | Replace values with `[REDACTED]`. |
| `hashKeys(...keys)` | Replace values with SHA-256 hashes. |
| `dropKeys(...keys)` | Remove keys from the event. |
| `maskKeys(...keys)` | Mask values, showing prefix/suffix. |
| `redactPatterns(...patterns)` | Redact values matching regex. |
| `composeRedactors(...redactors)` | Chain multiple redactors. |

## Schemas

| Schema | Description |
|--------|-------------|
| `DefaultSchema` | Default JSON output. |
| `FlatSchema` | Flat key-value output. |
| `NestedSchema` | Nested JSON output. |
| `ECSchema` | Elastic Common Schema. |
| `OTelSchema` | OpenTelemetry log schema. |
| `OTelLogSchema` | OpenTelemetry log schema variant. |
| `DatadogSchema` | Datadog log schema. |
| `CustomSchema(fn)` | User-defined schema function. |

## Timing

| Function | Description |
|----------|-------------|
| `process(ctx, name, ...attrs)` | Start a named process handle. |
| `startTimer(ctx, name, ...attrs)` | Start a named timer handle. |
| `startGroup(ctx, name, ...attrs)` | Start a named group handle. |
| `stopwatch()` | Create a standalone stopwatch. |

## Encoder

| Function | Description |
|----------|-------------|
| `encodeJSON(event)` | Encode event to JSON string. |
| `encodePrettyJSON(event)` | Encode event to pretty-printed JSON. |

## Cortex Client

| Class | Description |
|-------|-------------|
| `CortexClient` | Client for the LOXA cortex PCE. |

## Collector Client

| Class | Description |
|-------|-------------|
| `CollectorClient` | Client for the LOXA collector API. |
