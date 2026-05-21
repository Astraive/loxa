---
name: loxa
description: >
  Apply loxa wide-event logging when writing or reviewing any observability, logging, or instrumentation code.
  loxa replaces scattered log statements with a single context-rich canonical event per operation, emitted
  once when the outcome is known. Use this skill whenever the user is: adding logging to a service, handler,
  worker, job, or AI agent; reviewing existing log/console/logger calls; designing observability strategy;
  asking about structured logging, canonical log lines, wide events, or OpenTelemetry; importing any logger
  (pino, winston, zap, slog, structlog, log/slog); or asking how to debug production issues with logs.
  Even if the user only says "add some logging here" — apply loxa.
---

# loxa Skill

loxa is a wide-event logging framework. Instead of scattering log lines across a codebase, you build
**one rich event per operation** (request, job, workflow, agent run), enrich it with technical and business
context as execution progresses, then emit it once when the outcome is known.

> One event. The whole story.

## Core Rule

**Never write scattered log lines.** Write one loxa event per service hop.

```go
// ❌ Old model — scattered diary
log.Println("payment started")
log.Println("retry attempt 2")
log.Printf("payment failed: %v", err)

// ✅ loxa model — one canonical event
ctx = loxa.StartEvent(ctx, loxa.Params{
    Service: "checkout",
    Name:    "checkout.completed",
    Kind:    "request",
})
defer loxa.Emit(ctx)

loxa.Enrich(ctx,
    loxa.String("payment.provider", "stripe"),
    loxa.Int("payment.attempt", 2),
)

if err != nil {
    loxa.FinishError(ctx, err)
    return err
}
loxa.Finish(ctx, "success", loxa.Int("status_code", 200))
```

## When to Read Rule Files

| Task | Read |
|------|------|
| Writing any event / handler / middleware | `rules/wide-events.md` (CRITICAL) |
| Deciding which fields to add | `rules/context.md` (CRITICAL) |
| Setting up logger, middleware, sinks | `rules/structure.md` (HIGH) |
| Reviewing existing logging code for issues | `rules/pitfalls.md` (MEDIUM) |

Always read at least `wide-events.md` and `context.md` before writing any loxa code.

## Field Naming Convention

loxa uses dot-separated lowercase namespaces. This is non-negotiable.

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
ctx = loxa.StartEvent(ctx, loxa.Params{
    Service: "checkout-service",
    Name:    "checkout.completed",
    Kind:    "request",
})
defer loxa.Emit(ctx)

loxa.Enrich(ctx,
    loxa.UserID(user.ID),
    loxa.Int("cart.total_cents", cart.Total),
)

if err != nil {
    loxa.FinishError(ctx, err)
    return
}
loxa.Finish(ctx, "success")

// Job / worker pattern
ctx = loxa.StartJobEvent(ctx, loxa.Params{
    Service: "mailer",
    Name:    "email_digest.sent",
})
defer loxa.Emit(ctx)

loxa.Enrich(ctx,
    loxa.String("job.id", job.ID),
    loxa.String("job.queue", "default"),
    loxa.Int("email.count", len(recipients)),
)
result := sendDigest(recipients)
loxa.Finish(ctx, "success",
    loxa.Int("email.sent", result.Sent),
    loxa.Int("email.failed", result.Failed),
    loxa.String("email.provider", result.Provider),
)

// HTTP middleware (auto-emits — handlers only enrich)
r.Use(loxahttp.Middleware())

// Inside handler — just enrich, middleware handles emit
loxa.Enrich(r.Context(),
    loxa.UserID(ctx.User.ID),
    loxa.FeatureFlag("new_checkout", "on"),
    loxa.Int("cart.total_cents", ctx.Cart.Total),
)

// Checkpoints for external calls / DB queries / retries
loxa.Checkpoint(ctx, "payment_started",
    loxa.String("payment.provider", "stripe"),
    loxa.Int("payment.amount_cents", 15999),
)
charge, err := stripeCharge(amount)
loxa.Checkpoint(ctx, "payment_finished",
    loxa.String("payment.charge_id", charge.ID),
    loxa.Int("payment.attempt", attempt),
)
```

## Attribute Constructors

```go
loxa.String(key, value)    loxa.Int(key, value)     loxa.Bool(key, value)
loxa.Float64(key, value)   loxa.Duration(key, value) loxa.Any(key, value)

// Canonical helpers
loxa.UserID(id)            loxa.TenantID(id)         loxa.RequestID(id)
loxa.TraceID(id)           loxa.SpanID(id)
loxa.FeatureFlag(name, value)                         loxa.Experiment(name, variant)

// Business helpers
loxa.OrderID(id)           loxa.CartID(id)           loxa.Amount(cents)
loxa.Currency(code)        loxa.Plan(name)

// Groups (nested objects)
loxa.Group("user",
    loxa.String("id", userID),
    loxa.String("plan", "pro"),
)
```

## Sampling (built-in, tail-based)

Sampling decisions happen **after** outcome is known — never before.

```go
loxa.Configure(loxa.Production().
    WithSampler(loxa.AnySampler(
        loxa.SampleErrors(),
        loxa.SampleSlowRequests(2000 * time.Millisecond),
        loxa.SampleRandom(0.05),
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
| `request` | HTTP / RPC / API call — use `loxa.StartHTTPEvent` |
| `job` | Background job — use `loxa.StartJobEvent` |
| `workflow` | Multi-step business flow |
| `message` | Queue / message processing — use `loxa.StartQueueEvent` |
| `agent` | AI agent / tool execution |
| `system` | Runtime / infra events |
| `security` | Auth, access, policy |
| `audit` | Compliance events |
