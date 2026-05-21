---
title: Common Pitfalls
impact: MEDIUM
---

## loxa Pitfalls

### Pitfall 1: Calling `loxa.Enrich` without a live event in context

Enriching without a started event is a no-op or panics in strict mode.

```go
// ❌ No event in context
loxa.Enrich(ctx, loxa.UserID(userID))  // Which event? Unknown.

// ✅ Always start an event first
ctx = loxa.StartEvent(ctx, loxa.Params{Name: "checkout.completed", Kind: "request"})
defer loxa.Emit(ctx)
loxa.Enrich(ctx, loxa.UserID(userID))

// ✅ Or use loxahttp.Middleware() — it starts the event for you, handlers just enrich
```

---

### Pitfall 2: Not using `defer loxa.Emit(ctx)`

If `Emit` is called only on the happy path, failures before it mean no event is recorded —
the worst moment to go dark.

```go
// ❌ Emit only on success
ctx = loxa.StartEvent(ctx, loxa.Params{Name: "checkout.completed", Kind: "request"})
// ... logic ...
loxa.Finish(ctx, "success")
loxa.Emit(ctx)  // never called if an early return fired above

// ✅ Defer emit — always runs
ctx = loxa.StartEvent(ctx, loxa.Params{Name: "checkout.completed", Kind: "request"})
defer loxa.Emit(ctx)  // guaranteed regardless of how the function exits

// ... logic ...
if err != nil {
    loxa.FinishError(ctx, err)
    return err
}
loxa.Finish(ctx, "success")
```

---

### Pitfall 3: Using `loxa.String("error", err.Error())` instead of `loxa.FinishError()`

```go
// ❌ Loses structure
loxa.Enrich(ctx,
    loxa.String("error", err.Error()),
    loxa.String("error.details", fmt.Sprintf("%+v", err)),
)

// ✅ Structured error — auto-populates error.type, error.message, error.code, error.retryable, error.stack
loxa.FinishError(ctx, err)

// ✅ With additional attrs
loxa.FinishError(ctx, err,
    loxa.String("payment.provider", "stripe"),
    loxa.Int("payment.attempt", 3),
)
```

---

### Pitfall 4: Using checkpoints as debug breadcrumbs

```go
// ❌ Checkpoint per internal step — defeats the purpose
loxa.Checkpoint(ctx, "loaded user")
loxa.Checkpoint(ctx, "loaded cart")
loxa.Checkpoint(ctx, "computed total")

// ✅ Enrich the parent event with fields, not checkpoints
loxa.Enrich(ctx,
    loxa.UserID(user.ID),
    loxa.CartID(cart.ID),
    loxa.Int("cart.total_cents", cart.Total),
)
```

Checkpoints are for external service calls, DB calls, retries, and AI tool calls — not internal steps.

---

### Pitfall 5: Camel-case or inconsistent field names

```go
// ❌ Inconsistent names across services
loxa.Enrich(ctx, loxa.String("userId", user.ID))    // camelCase
loxa.Enrich(ctx, loxa.String("user_id", user.ID))   // snake_root
loxa.Enrich(ctx, loxa.String("uid", user.ID))        // alias
loxa.Enrich(ctx, loxa.String("actor.id", user.ID))  // wrong namespace

// ✅ Dot-separated lowercase everywhere — or use the canonical helper
loxa.Enrich(ctx, loxa.UserID(user.ID))               // canonical
loxa.Enrich(ctx, loxa.String("user.id", user.ID))   // equivalent
```

---

### Pitfall 6: Not propagating `request.id` across services

```go
// ❌ Downstream call without request ID — events can't be linked
resp, _ := http.Post("http://inventory-service/reserve", "application/json", body)

// ✅ Always forward request ID (and trace ID for OTel)
req, _ := http.NewRequestWithContext(ctx, "POST", "http://inventory-service/reserve", body)
req.Header.Set("x-request-id", loxa.RequestIDFromContext(ctx))
req.Header.Set("traceparent", loxa.TraceIDFromContext(ctx))
client.Do(req)
```

---

### Pitfall 7: Logging sensitive fields directly

```go
// ❌ PII / secrets in the event
loxa.Enrich(ctx,
    loxa.String("user.email", user.Email),              // PII
    loxa.String("payment.card_number", card.Number),    // PCI
    loxa.String("authorization", r.Header.Get("Authorization")), // Secret
)

// ✅ Use SensitiveString (hashes at SDK level) or omit entirely
loxa.Enrich(ctx,
    loxa.SensitiveString("user.email", user.Email),  // stored as hash
)
// card.number and authorization → blocked by loxa.DefaultRedactor()
```

---

### Pitfall 8: Forgetting to call `loxa.Finish` or `loxa.FinishError`

An event without `event.outcome` is hard to query and triggers schema warnings.

```go
// ❌ No outcome set — outcome field missing from event
ctx = loxa.StartEvent(ctx, loxa.Params{Name: "order.refunded", Kind: "workflow"})
defer loxa.Emit(ctx)
loxa.Enrich(ctx, loxa.OrderID(order.ID))
// function returns — outcome never set

// ✅ Always call Finish or FinishError before returning
ctx = loxa.StartEvent(ctx, loxa.Params{Name: "order.refunded", Kind: "workflow"})
defer loxa.Emit(ctx)
loxa.Enrich(ctx, loxa.OrderID(order.ID))

if err := processRefund(ctx, order); err != nil {
    loxa.FinishError(ctx, err)
    return err
}
loxa.Finish(ctx, "success")
```

---

### Pitfall 9: Multiple loxa instances

```go
// ❌ New instance per file — misconfigured, different sinks
logger := loxa.New(loxa.Config{Service: "checkout"})  // wrong sink config
logger.Enrich(ctx, loxa.UserID(userID))

// ✅ Call loxa.Configure() once at startup; call loxa.* directly everywhere else
// main.go
loxa.Configure(loxa.Config{ /* ... */ })

// handlers/checkout.go — no import of a logger, just use the package
loxa.Enrich(ctx, loxa.UserID(userID))
```

---

### Pitfall 10: Enriching after Emit

```go
// ❌ Enrich after emit — silently ignored or panics
defer loxa.Emit(ctx)
// ...
loxa.Emit(ctx)                                      // explicit early emit
loxa.Enrich(ctx, loxa.String("payment.provider", "stripe"))  // too late

// ✅ All Enrich calls before the function exits (defer handles timing)
ctx = loxa.StartEvent(ctx, loxa.Params{Name: "checkout.completed", Kind: "request"})
defer loxa.Emit(ctx)

loxa.Enrich(ctx, loxa.String("payment.provider", "stripe"))
loxa.Finish(ctx, "success")
// defer fires here — event emitted with all fields
```
