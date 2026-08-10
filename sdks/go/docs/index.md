# loza Skill

A skill for AI coding assistants to apply loza wide-event logging when writing or reviewing
observability and instrumentation code.

## Overview

loza replaces scattered `log.Println` / `slog.Info` calls with **one canonical event per operation**,
emitted once when the outcome is known. This makes logs queryable, debuggable, and useful for analytics.

> One event. The whole story.

## Key Concepts

- **Wide Events**: One comprehensive event per service hop, emitted at completion
- **loza SDK**: `loza.StartEvent()`, `loza.Enrich()`, `loza.Checkpoint()`, `loza.Finish()` / `loza.FinishError()`, `loza.Emit()` lifecycle
- **Dot-separated fields**: `user.id`, `cart.total_cents`, `payment.provider`, `error.code`
- **Tail sampling**: Decisions made after outcome is known — errors always kept
- **Business context**: `user.plan`, `cart.total_cents`, `feature.*` flags always included
- **Privacy-first**: Redaction and hashing configured at SDK level via `loza.ComposeRedactors()`

## Structure

```
loza/
├── skill.md              # Agent instructions + quick reference
└── rules/
    ├── wide-events.md    # Core patterns: lifecycle, middleware, checkpoints, jobs (CRITICAL)
    ├── context.md        # Cardinality, dimensionality, business & env context (CRITICAL)
    ├── structure.md      # SDK setup, sinks, sampling, field limits (HIGH)
    └── pitfalls.md       # Common mistakes and fixes (MEDIUM)
```

## Rules Summary

1. **Wide Events** (CRITICAL) — One event per operation, `defer loza.Emit(ctx)`, propagate `request.id`
2. **Context** (CRITICAL) — High cardinality IDs, 30–100 fields, business context, env context
3. **Structure** (HIGH) — Single loza instance, middleware pattern, schema file, two log levels
4. **Pitfalls** (MEDIUM) — No scattered enrich calls without an event, always call Finish/FinishError, dot-separated names

## References

- [Stripe Blog — Canonical Log Lines](https://stripe.com/blog/canonical-log-lines)
- [Boris Tane — Observability Wide Events 101](https://boristane.com/blog/observability-wide-events-101/)
