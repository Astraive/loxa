---
title: Wide Events — the loza Foundation
impact: CRITICAL
---

## Wide Events in loza

One event per service hop. Build it during execution. Emit once via `defer loza.Emit(ctx)`.

loza provides three patterns — choose based on context.

---

### Pattern 1: Explicit Lifecycle (handlers, any code)

```go
import "github.com/yourorg/loza"

func CheckoutHandler(w http.ResponseWriter, r *http.Request) {
    ctx := loza.StartEvent(r.Context(), loza.Params{
        Service: "checkout-service",
        Name:    "checkout.completed",
        Kind:    "request",
    })
    defer loza.Emit(ctx)

    user, err := getUser(ctx, r.UserID)
    if err != nil {
        loza.FinishError(ctx, err)
        http.Error(w, "unauthorized", 401)
        return
    }
    loza.Enrich(ctx,
        loza.UserID(user.ID),
        loza.String("user.plan", user.Plan),
        loza.Int("user.account_age_days", user.AccountAgeDays),
    )

    cart, err := getCart(ctx, user.ID)
    if err != nil {
        loza.FinishError(ctx, err)
        http.Error(w, "cart error", 500)
        return
    }
    loza.Enrich(ctx,
        loza.CartID(cart.ID),
        loza.Int("cart.total_cents", cart.Total),
        loza.Int("cart.item_count", len(cart.Items)),
    )

    payment, err := charge(ctx, cart)
    if err != nil {
        loza.FinishError(ctx, err,
            loza.String("payment.provider", payment.Provider),
            loza.Int("payment.attempt", payment.Attempt),
        )
        http.Error(w, "payment failed", 402)
        return
    }
    loza.Finish(ctx, "success",
        loza.String("payment.provider", payment.Provider),
        loza.Int("payment.latency_ms", payment.LatencyMs),
        loza.Int("payment.attempt", payment.Attempt),
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
r.Use(lozahttp.Middleware())

// Handler: only enrich
func CheckoutHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    loza.Enrich(ctx,
        loza.UserID(ctx.Value("userID").(string)),
        loza.String("user.plan", ctx.Value("plan").(string)),
        loza.Int("cart.total_cents", cart.Total),
        loza.FeatureFlag("new_checkout", "on"),
    )

    payment, err := charge(ctx, cart)
    if err != nil {
        // middleware sees the status code and marks outcome=error automatically
        http.Error(w, "payment failed", 402)
        return
    }
    loza.Enrich(ctx,
        loza.String("payment.provider", payment.Provider),
        loza.Int("payment.latency_ms", payment.LatencyMs),
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
    ctx = loza.StartJobEvent(ctx, loza.Params{
        Service: "mailer",
        Name:    "email_digest.sent",
    })
    defer loza.Emit(ctx)

    loza.Enrich(ctx,
        loza.String("job.id", job.ID),
        loza.String("job.queue", "default"),
        loza.UserID(job.UserID),
        loza.Int("email.count", len(job.Recipients)),
    )

    result, err := sendDigest(ctx, job.Recipients)
    if err != nil {
        loza.FinishError(ctx, err)
        return err
    }
    loza.Finish(ctx, "success",
        loza.Int("email.sent", result.Sent),
        loza.Int("email.failed", result.Failed),
        loza.String("email.provider", result.Provider),
    )
    return nil
}
```

---

### Checkpoints

Use checkpoints for external calls, DB queries, retries, or AI tool calls.
**Do not use them as debug breadcrumbs.** One or two per event is fine; many is a smell.

```go
loza.Checkpoint(ctx, "payment_started",
    loza.String("payment.provider", "stripe"),
    loza.Int("payment.amount_cents", 15999),
    loza.String("payment.currency", "usd"),
)

charge, err := stripeCharge(amount)

loza.Checkpoint(ctx, "payment_finished",
    loza.String("payment.charge_id", charge.ID),
    loza.Int("payment.attempt", attempt),
)
// Checkpoints appear as: { "checkpoints": [{"name":"payment_started","at_ms":42}, ...] }
```

---

### Propagate request.id Across Services

Every event that calls a downstream service must forward `request.id` as a header.

```go
// Outbound — forward the request ID
req, _ := http.NewRequestWithContext(ctx, "POST", "http://inventory-service/reserve", body)
req.Header.Set("x-request-id", loza.RequestIDFromContext(ctx))
req.Header.Set("traceparent", loza.TraceIDFromContext(ctx)) // OTel propagation
client.Do(req)

// Inbound (inventory-service) — pick it up and start a linked event
func ReserveHandler(w http.ResponseWriter, r *http.Request) {
    ctx = loza.StartEvent(r.Context(), loza.Params{
        Service:   "inventory-service",
        Name:      "inventory.reserve",
        Kind:      "request",
        RequestID: r.Header.Get("x-request-id"),
    })
    defer loza.Emit(ctx)
    // Both events now share the same request.id — queryable together
}
```

---

### Error Fields (structured, not strings)

```go
// loza.FinishError() auto-populates structured fields from the error:
// {
//   "error.type":          "PaymentError",
//   "error.message":       "Card declined by issuer",
//   "error.code":          "card_declined",
//   "error.retryable":     false,
//   "error.stack":         "...",
// }

// Pass additional attrs alongside the error:
loza.FinishError(ctx, err,
    loza.String("payment.provider", "stripe"),
    loza.Int("payment.attempt", 3),
    loza.Bool("retryable", false),
)
```

Never do: `loza.Enrich(ctx, loza.String("error", err.Error()))` — that loses structure.
