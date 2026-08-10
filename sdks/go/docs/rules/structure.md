---
title: Structure — SDK Setup, Middleware, Sinks, Field Limits
impact: HIGH
---

## loza Structure

### Single loza Instance

Configure loza once at startup. Never create multiple instances or call `loza.Configure()` more than once.

```go
// main.go — configure once
package main

import (
    "os"
    "github.com/yourorg/loza"
    "github.com/yourorg/loza/lozahttp"
)

func main() {
    loza.Configure(loza.Config{
        Service:     os.Getenv("SERVICE_NAME"),
        Version:     os.Getenv("SERVICE_VERSION"),
        Environment: os.Getenv("ENV"),
        Region:      os.Getenv("AWS_REGION"),

        Encoder: loza.JSONEncoder(),
        Sampler: loza.AnySampler(
            loza.SampleErrors(),
            loza.SampleSlowRequests(2000 * time.Millisecond),
            loza.SampleRandom(0.05),
        ),
        Redactor: loza.ComposeRedactors(
            loza.DefaultRedactor(),
            loza.HashKeys("user.email", "http.client_ip"),
            loza.RedactKeys("password", "token", "authorization", "cookie"),
        ),
        Sinks: []loza.Sink{
            loza.StdoutSink(),
        },
    })
    defer loza.Shutdown()

    r := chi.NewRouter()
    r.Use(lozahttp.Middleware())
    // ...
}

// All other files — just call loza directly, no import of a configured instance needed
// loza.Enrich(ctx, ...) uses the global configured instance
```

---

### Middleware for HTTP Services

Middleware handles: event creation, timing, HTTP field population, tail sampling, and emission.
Handlers are responsible only for business context.

Framework adapters available:

```go
lozahttp.Middleware()               // net/http compatible
lozachi.Middleware()                // chi
lozagin.Middleware()                // gin
lozafiber.Middleware()              // fiber
lozagrpc.UnaryInterceptor()         // gRPC unary
lozagrpc.StreamInterceptor()        // gRPC streaming
```

Fields automatically populated by middleware:

```json
{
  "event.name": "http.request",
  "event.kind": "request",
  "event.outcome": "success | error",
  "http.method": "POST",
  "http.route": "/checkout",
  "http.path": "/checkout?coupon=SAVE20",
  "http.status_code": 200,
  "http.user_agent": "Mozilla/5.0 ...",
  "request.id": "req_abc",
  "trace.id": "trace_xyz",
  "span.id": "span_123",
  "duration_ms": 243,
  "timestamp": "2026-05-10T12:00:00.000Z",
  "service.name": "checkout-service",
  "service.version": "2.4.1",
  "environment": "production",
  "region": "ap-south-1"
}
```

---

### Sinks

Configure in `loza.Config.Sinks`. Multiple sinks run in parallel.

```go
Sinks: []loza.Sink{
    // Local / dev
    loza.StdoutSink(),

    // File with rotation
    loza.RotatingFileSink(loza.RotatingFileConfig{
        Path:       "/var/log/app/events.ndjson",
        MaxSizeMB:  100,
        MaxBackups: 5,
    }),

    // HTTP batch to the LOZA collector
    lozahttpbatch.New(lozahttpbatch.Config{
        Endpoint: os.Getenv("LOZA_ENDPOINT"),
        Headers:  map[string]string{"Authorization": "Bearer " + os.Getenv("LOZA_TOKEN")},
    }),
}
```

Heavy delivery sinks such as Kafka, ClickHouse, Postgres, DuckDB, OTLP, S3, GCS, and Loki belong in `loza-collector`, not in application SDKs.

---

### Local Dev Output

Use `loza.Dev()` preset in development — prints human-readable events to stdout:

```go
if os.Getenv("ENV") == "development" {
    loza.Configure(loza.Dev().WithService(os.Getenv("SERVICE_NAME")))
} else {
    loza.Configure(loza.Production(). /* ... */ )
}
```

Dev output:
```
loza checkout.completed error 1247ms
  request.id=req_1 trace.id=trace_abc
  user.id=user_7 user.plan=premium
  cart.total_cents=15999
  payment.provider=stripe payment.attempt=3
  error.code=card_declined
```

In production it emits structured JSON / OTLP. Same application code, different sink.

---

### Immediate Logs

loza supports one-shot log events without `StartEvent` for startup messages, background warnings, etc.

```go
loza.Info("worker started", loza.String("queue", "emails"))
loza.Warn("rate limit approaching", loza.Int("requests_remaining", 10))
loza.Error("sink flush failed", loza.String("sink", "clickhouse"))
```

These emit immediately — no `Emit()` needed. Use sparingly; prefer enriching the active wide event.

---

### Two Application-Facing Levels Only

loza exposes only two levels to application code:

- **`loza.Info` / `loza.Finish`** — all normal events, including those with `outcome=error`
- **`loza.Error`** — SDK-level failures only (sink unavailable, schema panic)

Do not reach for `loza.Debug`, `loza.Warn`, or `loza.Trace` in hot paths.
If you want more detail, **add fields to the event** rather than adding log lines.

---

### Field Count and Safety Limits

The SDK enforces these by default (configurable via `loza.SecurityConfig`):

| Limit | Default |
|-------|---------|
| Max attrs per event | 512 |
| Max field value bytes | 4 KB |
| Max object depth | 5 |
| Max array length | 50 |
| Max event bytes | 256 KB |

Events exceeding limits are truncated (warn mode) or dropped (strict mode) and never crash the app.
