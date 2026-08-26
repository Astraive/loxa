# LOZA SDK instrumentation

Use SDKs as thin connectors to the Collector. The SDK owns event lifecycle, batching, compression, sampling, redaction hints, and graceful delivery; application code owns business context and error decisions.

## Lifecycle invariant

For one logical operation:

```text
start → enrich → checkpoint/process/group/timer/link → finish|finish-error → emit → flush → shutdown
```

Start exactly one canonical event at the request/job/queue/CLI boundary. Lower-level functions should enrich the existing event or create a separate event only when they represent a separate operation. Middleware and manual instrumentation must not both emit the same request event.

## Cross-language names

| Concept | Go | Python | JavaScript/TypeScript | Rust |
|---|---|---|---|---|
| Configure | `Configure` | `configure` | `configure` | `configure` |
| Start generic | `StartEvent` | `start_event` | `startEvent` | `start_event` |
| Start HTTP | `StartHTTPEvent`, `StartHTTPEventFromRequest` | `start_http_event` | `startHttpEvent` | HTTP adapter where provided |
| Background work | `StartJobEvent`, `StartQueueEvent`, `StartCronEvent` | matching snake_case helpers | matching camelCase helpers | matching snake_case helpers |
| Add fields | `Enrich` | `enrich` | `enrich` | `enrich` |
| Close | `Finish`, `FinishError` | `finish`, `finish_error` | `finish`, `finishError` | `finish`, `finish_error` |
| Deliver | `Emit`, `Flush`, `Shutdown` | `emit`, `flush`, `shutdown` | `emit`, `flush`, `shutdown` | `emit`, `flush`, `shutdown` |

## Go request pattern

```go
ctx := loza.StartHTTPEventFromRequest(r, loza.Params{
    Event: "checkout.request",
})
loza.Enrich(ctx,
    loza.RequestID(requestID),
    loza.Service("checkout"),
    loza.MarkSensitive("payment_token"),
)

if err := handleCheckout(ctx); err != nil {
    loza.FinishError(ctx, err)
    return err
}
loza.Finish(ctx, "success")
return nil
```

`StartHTTPEventFromRequest` takes the `*http.Request` first and derives its context. Verify the exact `Params` fields and return behavior against the installed Go SDK release.

## Attributes and privacy

Prefer typed constructors such as `String`, `Int`, `Duration`, `RequestID`, `TraceID`, `StatusCode`, `Outcome`, `UserID`, `OrderID`, `SensitiveString`, and `HashString` where the SDK provides them. Do not record raw authorization headers, access tokens, passwords, payment-card data, or unnecessary personal data. Hash or mark sensitive values before emission and rely on Collector policy for final enforcement.

## Configuration and shutdown

Configure once during process startup. Use production settings and a server-only credential for backend services; use origin-restricted public credentials only in browser/client contexts. On shutdown, stop accepting new work, call `Flush` with a bounded context, then call `Shutdown`. If the sink is asynchronous, a returned event handle or successful `Finish` is not proof that the Collector persisted the event.

## Framework adapters

Use the release-provided framework adapter (`nethttp`, `gin`, `echo`, `fiber`, `chi`, `grpc`, or equivalent) when it owns the request/RPC boundary. Do not wrap an adapter with duplicate manual start/finish calls. Check package feature flags and the installed release README before importing an adapter.
