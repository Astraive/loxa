---
name: loza
description: >
  Apply loza wide-event logging when writing or reviewing any observability, logging, or instrumentation code.
  loza replaces scattered log statements with a single context-rich canonical event per operation, emitted
  once when the outcome is known. Use this skill whenever the user is: adding logging to a service, handler,
  worker, job, or AI agent; reviewing existing log/console/logger calls; designing observability strategy;
  asking about structured logging, canonical log lines, wide events, or OpenTelemetry; importing any logger
  (pino, winston, zap, slog, structlog, log/slog); or asking how to debug production issues with logs.
  Even if the user only says "add some logging here" — apply loza.
---

# loza Skill

loza is a wide-event logging framework. Instead of scattering log lines across a codebase, you build
**one rich event per operation** (request, job, workflow, agent run), enrich it with technical and business
context as execution progresses, then emit it once when the outcome is known.

> One event. The whole story.

## Core Rule

**Never write scattered log lines.** Write one loza event per service hop.

```go
// ❌ Old model — scattered diary
log.Println("payment started")
log.Println("retry attempt 2")
log.Printf("payment failed: %v", err)

// ✅ loza model — one canonical event
ctx = loza.StartEvent(ctx, loza.Params{
    Service: "checkout",
    Name:    "checkout.completed",
    Kind:    "request",
})
defer loza.Emit(ctx)

loza.Enrich(ctx,
    loza.String("payment.provider", "stripe"),
    loza.Int("payment.attempt", 2),
)

if err != nil {
    loza.FinishError(ctx, err)
    return err
}
loza.Finish(ctx, "success", loza.Int("status_code", 200))
```

## When to Read Rule Files

| Task | Read |
|------|------|
| Writing any event / handler / middleware | `rules/wide-events.md` (CRITICAL) |
| Deciding which fields to add | `rules/context.md` (CRITICAL) |
| Setting up logger, middleware, sinks | `rules/structure.md` (HIGH) |
| Reviewing existing logging code for issues | `rules/pitfalls.md` (MEDIUM) |

Always read at least `wide-events.md` and `context.md` before writing any loza code.

## Field Naming Convention

loza uses dot-separated lowercase namespaces. This is non-negotiable.

```
service.name      user.id          cart.total_cents
request.id        trace.id         payment.provider
http.status_code  error.code       feature.new_checkout
agent.run_id      model.name       tokens.input
```

**Never** use camelCase (`userId`), snake_root (`user_id` at top level), or inconsistent aliases.

## Required Fields on Every Event

```json
{
  "event.name": "checkout.completed",
  "event.kind": "request",
  "event.outcome": "success | error | timeout | cancelled | rejected | unknown",
  "timestamp": "2026-05-10T12:00:00.000Z",
  "duration_ms": 123,
  "service.name": "checkout-service",
  "service.version": "1.2.3",
  "environment": "production"
}
```

## SDK Quick Reference

```go
// Explicit lifecycle — handlers, any async code
ctx = loza.StartEvent(ctx, loza.Params{
    Service: "checkout-service",
    Name:    "checkout.completed",
    Kind:    "request",
})
defer loza.Emit(ctx)

loza.Enrich(ctx,
    loza.UserID(user.ID),
    loza.Int("cart.total_cents", cart.Total),
)

if err != nil {
    loza.FinishError(ctx, err)
    return
}
loza.Finish(ctx, "success")

// Job / worker pattern
ctx = loza.StartJobEvent(ctx, loza.Params{
    Service: "mailer",
    Name:    "email_digest.sent",
})
defer loza.Emit(ctx)

loza.Enrich(ctx,
    loza.String("job.id", job.ID),
    loza.String("job.queue", "default"),
    loza.Int("email.count", len(recipients)),
)
result := sendDigest(recipients)
loza.Finish(ctx, "success",
    loza.Int("email.sent", result.Sent),
    loza.Int("email.failed", result.Failed),
    loza.String("email.provider", result.Provider),
)

// HTTP middleware (auto-emits — handlers only enrich)
r.Use(lozahttp.Middleware())

// Inside handler — just enrich, middleware handles emit
loza.Enrich(r.Context(),
    loza.UserID(ctx.User.ID),
    loza.FeatureFlag("new_checkout", "on"),
    loza.Int("cart.total_cents", ctx.Cart.Total),
)

// Checkpoints for external calls / DB queries / retries
loza.Checkpoint(ctx, "payment_started",
    loza.String("payment.provider", "stripe"),
    loza.Int("payment.amount_cents", 15999),
)
charge, err := stripeCharge(amount)
loza.Checkpoint(ctx, "payment_finished",
    loza.String("payment.charge_id", charge.ID),
    loza.Int("payment.attempt", attempt),
)
```

## Attribute Constructors

```go
loza.String(key, value)    loza.Int(key, value)     loza.Bool(key, value)
loza.Float64(key, value)   loza.Duration(key, value) loza.Any(key, value)

// Canonical helpers
loza.UserID(id)            loza.TenantID(id)         loza.RequestID(id)
loza.TraceID(id)           loza.SpanID(id)
loza.FeatureFlag(name, value)                         loza.Experiment(name, variant)

// Business helpers
loza.OrderID(id)           loza.CartID(id)           loza.Amount(cents)
loza.Currency(code)        loza.Plan(name)

// Groups (nested objects)
loza.Group("user",
    loza.String("id", userID),
    loza.String("plan", "pro"),
)
```

## Sampling (built-in, tail-based)

Sampling decisions happen **after** outcome is known — never before.

```go
loza.Configure(loza.Production().
    WithSampler(loza.AnySampler(
        loza.SampleErrors(),
        loza.SampleSlowRequests(2000 * time.Millisecond),
        loza.SampleRandom(0.05),
    )),
)
```

Every emitted event includes sampling metadata:

```json
{ "sample.kept": true, "sample.rate": 0.05, "sample.reason": "error" }
```

## Event Kinds

| Kind | Use for |
|------|---------|
| `request` | HTTP / RPC / API call — use `loza.StartHTTPEvent` |
| `job` | Background job — use `loza.StartJobEvent` |
| `workflow` | Multi-step business flow |
| `message` | Queue / message processing — use `loza.StartQueueEvent` |
| `agent` | AI agent / tool execution |
| `system` | Runtime / infra events |
| `security` | Auth, access, policy |
| `audit` | Compliance events |
