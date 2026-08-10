# Public API

The Python SDK public API surface, aligned with the cross-language parity manifest at `docs/sdk-parity-manifest.json`.

## Lifecycle

| Function | Description |
|----------|-------------|
| `start_event(params)` | Create a new event context. |
| `start_http_event(params)` | Create an HTTP-kind event. |
| `start_job_event(params)` | Create a job-kind event. |
| `start_queue_event(params)` | Create a queue-kind event. |
| `start_cli_event(params)` | Create a CLI-kind event. |
| `start_cron_event(params)` | Create a cron-kind event. |
| `append(ctx, *attrs)` | Append attributes to an event. |
| `enrich(ctx, *attrs)` | Enrich an event with attributes (alias for append). |
| `set(ctx, *attrs)` | Set attributes, overwriting existing keys. |
| `merge(ctx, group, *attrs)` | Merge attributes into a named group. |
| `delete(ctx, *keys)` | Remove keys from an event. |
| `get(ctx, key)` | Get a single attribute value. |
| `get_group(ctx, name)` | Get all attributes in a named group. |
| `checkpoint(ctx, name, *attrs)` | Record a named checkpoint with timestamp. |
| `finish(ctx, outcome, *attrs)` | Finish the event with an outcome. |
| `finish_error(ctx, error, *attrs)` | Finish the event with an error outcome. |
| `emit(ctx)` | Serialize and send the event to the sink. Returns event ID. |
| `flush()` | Flush buffered events. |
| `shutdown()` | Shut down the logger. |

## Immediate Logs

| Function | Level |
|----------|-------|
| `debug(message, **attrs)` | debug |
| `info(message, **attrs)` | info |
| `warn(message, **attrs)` | warn |
| `error(message, **attrs)` | error |
| `fatal(message, **attrs)` | fatal |

## Logger Configuration

| Function | Description |
|----------|-------------|
| `configure(config)` | Set the global default logger. |
| `new(config)` | Create a new Logger instance. |
| `dev(service)` | Dev-mode config preset. |
| `production(service)` | Production config preset. |
| `test(service)` | Test config preset. |

### Config Options

| Method | Description |
|--------|-------------|
| `with_service(name)` | Set the service name. |
| `with_version(ver)` | Set the service version. |
| `with_environment(env)` | Set the deployment environment. |
| `with_sink(sink)` | Set the output sink. |
| `with_sampler(sampler)` | Set the sampling strategy. |
| `with_redactor(redactor)` | Set the redaction strategy. |
| `with_schema(schema)` | Set the output schema. |
| `with_async(enabled)` | Enable async delivery. |
| `with_collector_endpoint(url)` | Set collector HTTP endpoint. |
| `with_duplicate_policy(policy)` | Set canonical field duplicate policy. |

## Attribute Constructors

| Constructor | Canonical Key |
|-------------|---------------|
| `String(key, value)` | custom |
| `Int(key, value)` | custom |
| `Int64(key, value)` | custom |
| `Uint64(key, value)` | custom |
| `Float64(key, value)` | custom |
| `Bool(key, value)` | custom |
| `Time(key, value)` | custom |
| `Duration(key, value)` | custom |
| `Any(key, value)` | custom |
| `Null(key)` | custom |
| `Group(name, attrs)` | custom |
| `SensitiveString(key, value)` | custom |
| `HashString(key, value)` | custom |
| `MarkSensitive(attr)` | marks any attr sensitive |
| `UserID(value)` | user.id |
| `TenantID(value)` | tenant.id |
| `WorkspaceID(value)` | tenant.workspace_id |
| `OrganizationID(value)` | tenant.organization_id |
| `SessionID(value)` | session.id |
| `RequestID(value)` | request_id |
| `TraceID(value)` | trace_id |
| `SpanID(value)` | span_id |
| `FeatureFlag(name, value)` | feature.{name} |
| `FeatureFlagBool(name, value)` | feature.{name} |
| `Experiment(name, variant)` | experiment.{name} |
| `OrderID(value)` | order.id |
| `CartID(value)` | cart.id |
| `ProductID(value)` | product.id |
| `CustomerID(value)` | customer.id |
| `Plan(value)` | customer.plan |
| `Currency(value)` | payment.currency |
| `Amount(value)` | payment.amount |
| `Country(value)` | geo.country |
| `Device(value)` | device.name |
| `Platform(value)` | device.platform |
| `AppVersion(value)` | app.version |
| `ErrorType(value)` | error.type |
| `ErrorCode(value)` | error.code |
| `ErrorMessage(value)` | error.message |
| `ErrorStack(value)` | error.stack |
| `Retryable(value)` | error.retryable |

## Sinks

| Sink | Description |
|------|-------------|
| `StdoutSink()` | Writes events to stdout. |
| `StderrSink()` | Writes events to stderr. |
| `FileSink(path)` | Writes events to a file. |
| `RotatingFileSink(path)` | Writes events to a rotating file. |
| `MemorySink()` | Stores events in memory for testing. |
| `NoopSink()` | Discards all events. |
| `HTTPBatchSink(endpoint)` | Batches and sends events via HTTP to a collector. |
| `CollectorSink()` | Alias for HTTPBatchSink with default endpoint. |

## Samplers

| Sampler | Description |
|---------|-------------|
| `SampleAll()` | Sample 100% of events. |
| `SampleNone()` | Drop all events. |
| `SampleRandom(rate)` | Sample at the given rate (0.0-1.0). |
| `SampleErrors()` | Always sample error events. |
| `SampleSlowRequests(duration)` | Sample requests slower than threshold. |
| `SampleStatusCodes(*codes)` | Sample specific HTTP status codes. |
| `SampleRoutes(*routes)` | Sample specific routes. |
| `SampleUsers(*ids)` | Sample specific user IDs. |
| `SampleTenants(*ids)` | Sample specific tenant IDs. |
| `SampleFeatureFlag(name, value)` | Sample based on feature flag. |
| `SampleByHeader(header, value)` | Sample based on HTTP header. |
| `AnySampler(*samplers)` | Sample if any sub-sampler matches. |
| `AllSampler(*samplers)` | Sample if all sub-samplers match. |
| `NotSampler(sampler)` | Invert a sampler. |

## Redactors

| Redactor | Description |
|----------|-------------|
| `DefaultRedactor()` | 14-key safety-net redactor. |
| `RedactKeys(*keys)` | Replace values of specified keys with `[REDACTED]`. |
| `HashKeys(*keys)` | Replace values with SHA-256 hashes. |
| `DropKeys(*keys)` | Remove keys entirely from the event. |
| `MaskKeys(*keys, prefix, suffix)` | Mask values, showing only prefix/suffix characters. |
| `RedactPatterns(*patterns)` | Redact values matching regex patterns. |
| `ComposeRedactors(*redactors)` | Chain multiple redactors. |

## Schemas

| Schema | Description |
|--------|-------------|
| `DefaultSchema()` | Default JSON output. |
| `FlatSchema()` | Flat key-value output. |
| `NestedSchema()` | Nested JSON output. |
| `ECSchema()` | Elastic Common Schema. |
| `OTelSchema()` | OpenTelemetry log schema. |
| `DatadogSchema()` | Datadog log schema. |
| `CustomSchema(fn)` | User-defined schema function. |

## Timing

| Function | Description |
|----------|-------------|
| `process(ctx, name, **attrs)` | Start a named process handle. |
| `start_timer(ctx, name, **attrs)` | Start a named timer handle. |
| `start_group(ctx, name, **attrs)` | Start a named group handle. |
| `stopwatch()` | Create a standalone stopwatch. |

## Cortex Client

| Class | Description |
|-------|-------------|
| `CortexClient(url)` | Client for the LOZA cortex PCE. |
