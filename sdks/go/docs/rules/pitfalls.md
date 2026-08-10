---
title: Common Pitfalls
impact: MEDIUM
---

## loza Pitfalls

### Pitfall 1: Calling `loza.Enrich` without a live event in context

Enriching without a started event is a no-op or panics in strict mode.

```go
// ❌ No event in context
loza.Enrich(ctx, loza.UserID(userID))  // Which event? Unknown.

// ✅ Always start an event first
ctx = loza.StartEvent(ctx, loza.Params{Name: "checkout.completed", Kind: "request"})
defer loza.Emit(ctx)
loza.Enrich(ctx, loza.UserID(userID))

// ✅ Or use lozahttp.Middleware() — it starts the event for you, handlers just enrich
```

---

### Pitfall 2: Not using `defer loza.Emit(ctx)`

If `Emit` is called only on the happy path, failures before it mean no event is recorded —
the worst moment to go dark.

```go
// ❌ Emit only on success
ctx = loza.StartEvent(ctx, loza.Params{Name: "checkout.completed", Kind: "request"})
// ... logic ...
loza.Finish(ctx, "success")
loza.Emit(ctx)  // never called if an early return fired above

// ✅ Defer emit — always runs
ctx = loza.StartEvent(ctx, loza.Params{Name: "checkout.completed", Kind: "request"})
defer loza.Emit(ctx)  // guaranteed regardless of how the function exits

// ... logic ...
if err != nil {
    loza.FinishError(ctx, err)
    return err
}
loza.Finish(ctx, "success")
```

---

### Pitfall 3: Using `loza.String("error", err.Error())` instead of `loza.FinishError()`

```go
// ❌ Loses structure
loza.Enrich(ctx,
    loza.String("error", err.Error()),
    loza.String("error.details", fmt.Sprintf("%+v", err)),
)

// ✅ Structured error — auto-populates error.type, error.message, error.code, error.retryable, error.stack
loza.FinishError(ctx, err)

// ✅ With additional attrs
loza.FinishError(ctx, err,
    loza.String("payment.provider", "stripe"),
    loza.Int("payment.attempt", 3),
)
```

---

### Pitfall 4: Using checkpoints as debug breadcrumbs

```go
// ❌ Checkpoint per internal step — defeats the purpose
loza.Checkpoint(ctx, "loaded user")
loza.Checkpoint(ctx, "loaded cart")
loza.Checkpoint(ctx, "computed total")

// ✅ Enrich the parent event with fields, not checkpoints
loza.Enrich(ctx,
    loza.UserID(user.ID),
    loza.CartID(cart.ID),
    loza.Int("cart.total_cents", cart.Total),
)
```

Checkpoints are for external service calls, DB calls, retries, and AI tool calls — not internal steps.

---

### Pitfall 5: Camel-case or inconsistent field names

```go
// ❌ Inconsistent names across services
loza.Enrich(ctx, loza.String("userId", user.ID))    // camelCase
loza.Enrich(ctx, loza.String("user_id", user.ID))   // snake_root
loza.Enrich(ctx, loza.String("uid", user.ID))        // alias
loza.Enrich(ctx, loza.String("actor.id", user.ID))  // wrong namespace

// ✅ Dot-separated lowercase everywhere — or use the canonical helper
loza.Enrich(ctx, loza.UserID(user.ID))               // canonical
loza.Enrich(ctx, loza.String("user.id", user.ID))   // equivalent
```

---

### Pitfall 6: Not propagating `request.id` across services

```go
// ❌ Downstream call without request ID — events can't be linked
resp, _ := http.Post("http://inventory-service/reserve", "application/json", body)

// ✅ Always forward request ID (and trace ID for OTel)
req, _ := http.NewRequestWithContext(ctx, "POST", "http://inventory-service/reserve", body)
req.Header.Set("x-request-id", loza.RequestIDFromContext(ctx))
req.Header.Set("traceparent", loza.TraceIDFromContext(ctx))
client.Do(req)
```

---

### Pitfall 7: Logging sensitive fields directly

```go
// ❌ PII / secrets in the event
loza.Enrich(ctx,
    loza.String("user.email", user.Email),              // PII
    loza.String("payment.card_number", card.Number),    // PCI
    loza.String("authorization", r.Header.Get("Authorization")), // Secret
)

// ✅ Use SensitiveString (hashes at SDK level) or omit entirely
loza.Enrich(ctx,
    loza.SensitiveString("user.email", user.Email),  // stored as hash
)
// card.number and authorization → blocked by loza.DefaultRedactor()
```

---

### Pitfall 8: Forgetting to call `loza.Finish` or `loza.FinishError`

An event without `event.outcome` is hard to query and triggers schema warnings.

```go
// ❌ No outcome set — outcome field missing from event
ctx = loza.StartEvent(ctx, loza.Params{Name: "order.refunded", Kind: "workflow"})
defer loza.Emit(ctx)
loza.Enrich(ctx, loza.OrderID(order.ID))
// function returns — outcome never set

// ✅ Always call Finish or FinishError before returning
ctx = loza.StartEvent(ctx, loza.Params{Name: "order.refunded", Kind: "workflow"})
defer loza.Emit(ctx)
loza.Enrich(ctx, loza.OrderID(order.ID))

if err := processRefund(ctx, order); err != nil {
    loza.FinishError(ctx, err)
    return err
}
loza.Finish(ctx, "success")
```

---

### Pitfall 9: Multiple loza instances

```go
// ❌ New instance per file — misconfigured, different sinks
logger := loza.New(loza.Config{Service: "checkout"})  // wrong sink config
logger.Enrich(ctx, loza.UserID(userID))

// ✅ Call loza.Configure() once at startup; call loza.* directly everywhere else
// main.go
loza.Configure(loza.Config{ /* ... */ })

// handlers/checkout.go — no import of a logger, just use the package
loza.Enrich(ctx, loza.UserID(userID))
```

---

### Pitfall 10: Enriching after Emit

```go
// ❌ Enrich after emit — silently ignored or panics
defer loza.Emit(ctx)
// ...
loza.Emit(ctx)                                      // explicit early emit
loza.Enrich(ctx, loza.String("payment.provider", "stripe"))  // too late

// ✅ All Enrich calls before the function exits (defer handles timing)
ctx = loza.StartEvent(ctx, loza.Params{Name: "checkout.completed", Kind: "request"})
defer loza.Emit(ctx)

loza.Enrich(ctx, loza.String("payment.provider", "stripe"))
loza.Finish(ctx, "success")
// defer fires here — event emitted with all fields
```
