---
title: Context — Cardinality, Dimensionality, Business & Environment
impact: CRITICAL
---

## Context in loxa Events

A wide event is only useful if it contains enough context to answer questions you haven't thought of yet.
Two properties make that possible: **high cardinality** and **high dimensionality**.

---

### High Cardinality

Fields like `user.id`, `request.id`, `cart.id`, `tenant.id`, `order.id`, `trace.id` can have millions of
unique values. loxa is designed for this. Never aggregate away these IDs — they are the point.

```json
{
  "user.id": "user_7f3k2",
  "request.id": "req_abc123",
  "cart.id": "cart_9x7",
  "tenant.id": "tenant_acme",
  "trace.id": "trace_4b2f",
  "deployment.id": "deploy_2026-05-10-v3"
}
```

These fields let you answer: *"What happened to this specific user's request?"*

---

### High Dimensionality

loxa events should comfortably hold 30–100 fields. More fields = more questions answerable without
redeploying code. When in doubt, add the field.

A well-structured checkout event:

```go
loxa.Enrich(ctx,
    loxa.UserID(user.ID),
    loxa.String("user.plan", user.Plan),                     // "free" | "premium" | "enterprise"
    loxa.Int("user.account_age_days", user.AccountAgeDays),
    loxa.Int("user.lifetime_value_cents", user.LTV),

    loxa.CartID(cart.ID),
    loxa.Int("cart.total_cents", cart.Total),
    loxa.Int("cart.item_count", len(cart.Items)),
    loxa.Any("cart.coupon_applied", cart.Coupon),            // nil if none

    loxa.String("payment.provider", "stripe"),
    loxa.String("payment.method", "card"),
    loxa.Int("payment.latency_ms", paymentLatency),
    loxa.Int("payment.attempt", attempt),

    loxa.FeatureFlag("new_checkout", flags.NewCheckout),
    loxa.Experiment("checkout_redesign", "B"),
)
```

---

### Always Include Business Context

Technical context tells you *what broke*. Business context tells you *why it matters*.

```go
// ❌ Technical only — incomplete picture
loxa.FinishError(ctx, err,
    loxa.Int("http.status_code", 500),
    loxa.String("error.code", "card_declined"),
)

// ✅ Business context included — actionable picture
loxa.FinishError(ctx, err,
    loxa.Int("http.status_code", 500),
    loxa.String("error.code", "card_declined"),
    loxa.String("user.plan", "enterprise"),
    loxa.Int("user.lifetime_value_cents", 4850000),  // $48,500 LTV
    loxa.Int("cart.total_cents", 249900),             // $2,499 order at stake
    loxa.Bool("feature.new_payment_flow", true),      // Was new code involved?
)
```

Now you know: *Enterprise customer, $48k LTV, losing a $2.5k order, new code enabled.*

**Fields to always consider:**
- `user.plan`, `user.account_age_days`, `user.lifetime_value_cents`
- `cart.total_cents`, `cart.item_count`, `cart.coupon_applied`
- `order.id`, `order.type`, `order.contains_annual_plan`
- `feature.*` flags (crucial for rollout debugging) — use `loxa.FeatureFlag()`
- `experiment.*.variant` (A/B test assignments) — use `loxa.Experiment()`

---

### Always Include Environment Context

Every event should carry the deployment and runtime context. Set this once at startup via
`loxa.Configure()` — it attaches automatically to every event.

```go
// main.go — configure once at startup
loxa.Configure(loxa.Config{
    Service:     os.Getenv("SERVICE_NAME"),
    Version:     os.Getenv("SERVICE_VERSION"),
    Environment: os.Getenv("ENV"),
    Region:      os.Getenv("AWS_REGION"),
})
```

These fields appear automatically on every event:

```json
{
  "service.name": "checkout-service",
  "service.version": "2.4.1",
  "environment": "production",
  "region": "ap-south-1",
  "host.name": "host-22",
  "deployment.id": "deploy_456"
}
```

**Why each matters:**
- `service.version` + `deployment.id` → correlate errors with deploys
- `region` → identify region-specific failures
- `host.name` → debug issues on specific instances

---

### AI Agent Context

For `agent` kind events, include model and tool fields:

```go
ctx = loxa.StartEvent(ctx, loxa.Params{
    Service: "agent-runner",
    Name:    "agent.tool_call",
    Kind:    "agent",
})
defer loxa.Emit(ctx)

loxa.Enrich(ctx,
    loxa.String("agent.id", agent.ID),
    loxa.String("agent.run_id", run.ID),
    loxa.String("model.name", "claude-sonnet-4-20250514"),
    loxa.String("model.provider", "anthropic"),
    loxa.Int("tokens.input", usage.InputTokens),
    loxa.Int("tokens.output", usage.OutputTokens),
    loxa.String("tool.name", tool.Name),
    loxa.String("tool.call_id", tool.CallID),
    loxa.Int("tool.latency_ms", tool.LatencyMs),
    loxa.Bool("safety.blocked", false),
)
```

These fields enable queries like:
```sql
-- Which tool causes the most agent failures?
SELECT tool_name, count(*) FROM loxa_events
WHERE event_kind = 'agent' AND event_outcome = 'error'
GROUP BY tool_name ORDER BY count(*) DESC;

-- Model cost by workflow
SELECT model_name, sum(tokens_input + tokens_output) FROM loxa_events
WHERE event_kind = 'agent' GROUP BY model_name;
```

---

### Privacy: Fields That Must Never Be Set

loxa will warn or block these at the SDK level via `loxa.DefaultRedactor()`. Do not work around it.

| Field pattern | Why |
|---------------|-----|
| `password`, `token`, `authorization` | Secrets |
| `cookie`, `card.number` | Sensitive auth / PCI |
| `user.email` | PII — use `loxa.SensitiveString("user.email", v)` to hash automatically |
| `http.client_ip` | PII — hash or omit |
| Raw request/response bodies | Too large, likely contains secrets |

```go
// ✅ Hash PII automatically via SensitiveString
loxa.Enrich(ctx,
    loxa.SensitiveString("user.email", user.Email),  // stored as hash, never raw
)

// ✅ Configure redaction once at startup
loxa.Configure(loxa.Config{
    Redactor: loxa.ComposeRedactors(
        loxa.DefaultRedactor(),
        loxa.HashKeys("user.email", "http.client_ip"),
        loxa.RedactKeys("password", "token", "authorization", "cookie"),
    ),
})
```
