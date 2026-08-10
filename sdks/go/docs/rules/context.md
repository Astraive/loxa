---
title: Context — Cardinality, Dimensionality, Business & Environment
impact: CRITICAL
---

## Context in loza Events

A wide event is only useful if it contains enough context to answer questions you haven't thought of yet.
Two properties make that possible: **high cardinality** and **high dimensionality**.

---

### High Cardinality

Fields like `user.id`, `request.id`, `cart.id`, `tenant.id`, `order.id`, `trace.id` can have millions of
unique values. loza is designed for this. Never aggregate away these IDs — they are the point.

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

loza events should comfortably hold 30–100 fields. More fields = more questions answerable without
redeploying code. When in doubt, add the field.

A well-structured checkout event:

```go
loza.Enrich(ctx,
    loza.UserID(user.ID),
    loza.String("user.plan", user.Plan),                     // "free" | "premium" | "enterprise"
    loza.Int("user.account_age_days", user.AccountAgeDays),
    loza.Int("user.lifetime_value_cents", user.LTV),

    loza.CartID(cart.ID),
    loza.Int("cart.total_cents", cart.Total),
    loza.Int("cart.item_count", len(cart.Items)),
    loza.Any("cart.coupon_applied", cart.Coupon),            // nil if none

    loza.String("payment.provider", "stripe"),
    loza.String("payment.method", "card"),
    loza.Int("payment.latency_ms", paymentLatency),
    loza.Int("payment.attempt", attempt),

    loza.FeatureFlag("new_checkout", flags.NewCheckout),
    loza.Experiment("checkout_redesign", "B"),
)
```

---

### Always Include Business Context

Technical context tells you *what broke*. Business context tells you *why it matters*.

```go
// ❌ Technical only — incomplete picture
loza.FinishError(ctx, err,
    loza.Int("http.status_code", 500),
    loza.String("error.code", "card_declined"),
)

// ✅ Business context included — actionable picture
loza.FinishError(ctx, err,
    loza.Int("http.status_code", 500),
    loza.String("error.code", "card_declined"),
    loza.String("user.plan", "enterprise"),
    loza.Int("user.lifetime_value_cents", 4850000),  // $48,500 LTV
    loza.Int("cart.total_cents", 249900),             // $2,499 order at stake
    loza.Bool("feature.new_payment_flow", true),      // Was new code involved?
)
```

Now you know: *Enterprise customer, $48k LTV, losing a $2.5k order, new code enabled.*

**Fields to always consider:**
- `user.plan`, `user.account_age_days`, `user.lifetime_value_cents`
- `cart.total_cents`, `cart.item_count`, `cart.coupon_applied`
- `order.id`, `order.type`, `order.contains_annual_plan`
- `feature.*` flags (crucial for rollout debugging) — use `loza.FeatureFlag()`
- `experiment.*.variant` (A/B test assignments) — use `loza.Experiment()`

---

### Always Include Environment Context

Every event should carry the deployment and runtime context. Set this once at startup via
`loza.Configure()` — it attaches automatically to every event.

```go
// main.go — configure once at startup
loza.Configure(loza.Config{
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
ctx = loza.StartEvent(ctx, loza.Params{
    Service: "agent-runner",
    Name:    "agent.tool_call",
    Kind:    "agent",
})
defer loza.Emit(ctx)

loza.Enrich(ctx,
    loza.String("agent.id", agent.ID),
    loza.String("agent.run_id", run.ID),
    loza.String("model.name", "claude-sonnet-4-20250514"),
    loza.String("model.provider", "anthropic"),
    loza.Int("tokens.input", usage.InputTokens),
    loza.Int("tokens.output", usage.OutputTokens),
    loza.String("tool.name", tool.Name),
    loza.String("tool.call_id", tool.CallID),
    loza.Int("tool.latency_ms", tool.LatencyMs),
    loza.Bool("safety.blocked", false),
)
```

These fields enable queries like:
```sql
-- Which tool causes the most agent failures?
SELECT tool_name, count(*) FROM loza_events
WHERE event_kind = 'agent' AND event_outcome = 'error'
GROUP BY tool_name ORDER BY count(*) DESC;

-- Model cost by workflow
SELECT model_name, sum(tokens_input + tokens_output) FROM loza_events
WHERE event_kind = 'agent' GROUP BY model_name;
```

---

### Privacy: Fields That Must Never Be Set

loza will warn or block these at the SDK level via `loza.DefaultRedactor()`. Do not work around it.

| Field pattern | Why |
|---------------|-----|
| `password`, `token`, `authorization` | Secrets |
| `cookie`, `card.number` | Sensitive auth / PCI |
| `user.email` | PII — use `loza.SensitiveString("user.email", v)` to hash automatically |
| `http.client_ip` | PII — hash or omit |
| Raw request/response bodies | Too large, likely contains secrets |

```go
// ✅ Hash PII automatically via SensitiveString
loza.Enrich(ctx,
    loza.SensitiveString("user.email", user.Email),  // stored as hash, never raw
)

// ✅ Configure redaction once at startup
loza.Configure(loza.Config{
    Redactor: loza.ComposeRedactors(
        loza.DefaultRedactor(),
        loza.HashKeys("user.email", "http.client_ip"),
        loza.RedactKeys("password", "token", "authorization", "cookie"),
    ),
})
```
