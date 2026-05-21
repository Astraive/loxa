---
title: Wide Events — the loxa Foundation
impact: CRITICAL
---

## Wide Events in loxa

One event per service hop. Build it during execution. Emit once via `defer loxa.Emit(ctx)`.

loxa provides three patterns — choose based on context.

---

### Pattern 1: Explicit Lifecycle (handlers, any code)

```go
import "github.com/yourorg/loxa"

func CheckoutHandler(w http.ResponseWriter, r *http.Request) {
    ctx := loxa.StartEvent(r.Context(), loxa.Params{
        Service: "checkout-service",
        Name:    "checkout.completed",
        Kind:    "request",
    })
    defer loxa.Emit(ctx)

    user, err := getUser(ctx, r.UserID)
    if err != nil {
        loxa.FinishError(ctx, err)
        http.Error(w, "unauthorized", 401)
        return
    }
    loxa.Enrich(ctx,
        loxa.UserID(user.ID),
        loxa.String("user.plan", user.Plan),
        loxa.Int("user.account_age_days", user.AccountAgeDays),
    )

    cart, err := getCart(ctx, user.ID)
    if err != nil {
        loxa.FinishError(ctx, err)
        http.Error(w, "cart error", 500)
        return
    }
    loxa.Enrich(ctx,
        loxa.CartID(cart.ID),
        loxa.Int("cart.total_cents", cart.Total),
        loxa.Int("cart.item_count", len(cart.Items)),
    )

    payment, err := charge(ctx, cart)
    if err != nil {
        loxa.FinishError(ctx, err,
            loxa.String("payment.provider", payment.Provider),
            loxa.Int("payment.attempt", payment.Attempt),
        )
        http.Error(w, "payment failed", 402)
        return
    }
    loxa.Finish(ctx, "success",
        loxa.String("payment.provider", payment.Provider),
        loxa.Int("payment.latency_ms", payment.LatencyMs),
        loxa.Int("payment.attempt", payment.Attempt),
    )
    json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}
```

---

### Pattern 2: HTTP Middleware (auto-emits)

The middleware creates the event, captures timing and HTTP fields, and emits on the way out.
Handlers only add **business context**.

```go
// main.go — apply once at startup
r.Use(loxahttp.Middleware())

// Handler: only enrich
func CheckoutHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    loxa.Enrich(ctx,
        loxa.UserID(ctx.Value("userID").(string)),
        loxa.String("user.plan", ctx.Value("plan").(string)),
        loxa.Int("cart.total_cents", cart.Total),
        loxa.FeatureFlag("new_checkout", "on"),
    )

    payment, err := charge(ctx, cart)
    if err != nil {
        // middleware sees the status code and marks outcome=error automatically
        http.Error(w, "payment failed", 402)
        return
    }
    loxa.Enrich(ctx,
        loxa.String("payment.provider", payment.Provider),
        loxa.Int("payment.latency_ms", payment.LatencyMs),
    )
    // middleware emits — http.method, http.status_code, duration_ms,
    // request.id, trace.id, service.* all populated automatically
}
```

Emitted event includes all standard HTTP fields automatically:

```json
{
  "event.name": "http.request",
  "event.kind": "request",
  "event.outcome": "success",
  "http.method": "POST",
  "http.route": "/checkout",
  "http.status_code": 200,
  "duration_ms": 243,
  "request.id": "req_abc",
  "trace.id": "trace_xyz",
  "service.name": "checkout-service",
  "user.id": "user_7",
  "user.plan": "premium",
  "cart.total_cents": 15999,
  "feature.checkout_v2": "on",
  "payment.provider": "stripe",
  "payment.latency_ms": 189,
  "sample.kept": true,
  "sample.reason": "error"
}
```

---

### Pattern 3: Job / Worker (explicit start + defer emit)

```go
func SendEmailDigestJob(ctx context.Context, job Job) error {
    ctx = loxa.StartJobEvent(ctx, loxa.Params{
        Service: "mailer",
        Name:    "email_digest.sent",
    })
    defer loxa.Emit(ctx)

    loxa.Enrich(ctx,
        loxa.String("job.id", job.ID),
        loxa.String("job.queue", "default"),
        loxa.UserID(job.UserID),
        loxa.Int("email.count", len(job.Recipients)),
    )

    result, err := sendDigest(ctx, job.Recipients)
    if err != nil {
        loxa.FinishError(ctx, err)
        return err
    }
    loxa.Finish(ctx, "success",
        loxa.Int("email.sent", result.Sent),
        loxa.Int("email.failed", result.Failed),
        loxa.String("email.provider", result.Provider),
    )
    return nil
}
```

---

### Checkpoints

Use checkpoints for external calls, DB queries, retries, or AI tool calls.
**Do not use them as debug breadcrumbs.** One or two per event is fine; many is a smell.

```go
loxa.Checkpoint(ctx, "payment_started",
    loxa.String("payment.provider", "stripe"),
    loxa.Int("payment.amount_cents", 15999),
    loxa.String("payment.currency", "usd"),
)

charge, err := stripeCharge(amount)

loxa.Checkpoint(ctx, "payment_finished",
    loxa.String("payment.charge_id", charge.ID),
    loxa.Int("payment.attempt", attempt),
)
// Checkpoints appear as: { "checkpoints": [{"name":"payment_started","at_ms":42}, ...] }
```

---

### Propagate request.id Across Services

Every event that calls a downstream service must forward `request.id` as a header.

```go
// Outbound — forward the request ID
req, _ := http.NewRequestWithContext(ctx, "POST", "http://inventory-service/reserve", body)
req.Header.Set("x-request-id", loxa.RequestIDFromContext(ctx))
req.Header.Set("traceparent", loxa.TraceIDFromContext(ctx)) // OTel propagation
client.Do(req)

// Inbound (inventory-service) — pick it up and start a linked event
func ReserveHandler(w http.ResponseWriter, r *http.Request) {
    ctx = loxa.StartEvent(r.Context(), loxa.Params{
        Service:   "inventory-service",
        Name:      "inventory.reserve",
        Kind:      "request",
        RequestID: r.Header.Get("x-request-id"),
    })
    defer loxa.Emit(ctx)
    // Both events now share the same request.id — queryable together
}
```

---

### Error Fields (structured, not strings)

```go
// loxa.FinishError() auto-populates structured fields from the error:
// {
//   "error.type":          "PaymentError",
//   "error.message":       "Card declined by issuer",
//   "error.code":          "card_declined",
//   "error.retryable":     false,
//   "error.stack":         "...",
// }

// Pass additional attrs alongside the error:
loxa.FinishError(ctx, err,
    loxa.String("payment.provider", "stripe"),
    loxa.Int("payment.attempt", 3),
    loxa.Bool("retryable", false),
)
```

Never do: `loxa.Enrich(ctx, loxa.String("error", err.Error()))` — that loses structure.
