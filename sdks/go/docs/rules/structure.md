---
title: Structure — SDK Setup, Middleware, Sinks, Field Limits
impact: HIGH
---

## loxa Structure

### Single loxa Instance

Configure loxa once at startup. Never create multiple instances or call `loxa.Configure()` more than once.

```go
// main.go — configure once
package main

import (
    "os"
    "github.com/yourorg/loxa"
    "github.com/yourorg/loxa/loxahttp"
)

func main() {
    loxa.Configure(loxa.Config{
        Service:     os.Getenv("SERVICE_NAME"),
        Version:     os.Getenv("SERVICE_VERSION"),
        Environment: os.Getenv("ENV"),
        Region:      os.Getenv("AWS_REGION"),

        Encoder: loxa.JSONEncoder(),
        Sampler: loxa.AnySampler(
            loxa.SampleErrors(),
            loxa.SampleSlowRequests(2000 * time.Millisecond),
            loxa.SampleRandom(0.05),
        ),
        Redactor: loxa.ComposeRedactors(
            loxa.DefaultRedactor(),
            loxa.HashKeys("user.email", "http.client_ip"),
            loxa.RedactKeys("password", "token", "authorization", "cookie"),
        ),
        Sinks: []loxa.Sink{
            loxa.StdoutSink(),
        },
    })
    defer loxa.Shutdown()

    r := chi.NewRouter()
    r.Use(loxahttp.Middleware())
    // ...
}

// All other files — just call loxa directly, no import of a configured instance needed
// loxa.Enrich(ctx, ...) uses the global configured instance
```

---

### Middleware for HTTP Services

Middleware handles: event creation, timing, HTTP field population, tail sampling, and emission.
Handlers are responsible only for business context.

Framework adapters available:

```go
loxahttp.Middleware()               // net/http compatible
loxachi.Middleware()                // chi
loxagin.Middleware()                // gin
loxafiber.Middleware()              // fiber
loxagrpc.UnaryInterceptor()         // gRPC unary
loxagrpc.StreamInterceptor()        // gRPC streaming
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

Configure in `loxa.Config.Sinks`. Multiple sinks run in parallel.

```go
Sinks: []loxa.Sink{
    // Local / dev
    loxa.StdoutSink(),

    // File with rotation
    loxa.RotatingFileSink(loxa.RotatingFileConfig{
        Path:       "/var/log/app/events.ndjson",
        MaxSizeMB:  100,
        MaxBackups: 5,
    }),

    // HTTP batch to the LOXA collector
    loxahttpbatch.New(loxahttpbatch.Config{
        Endpoint: os.Getenv("LOXA_ENDPOINT"),
        Headers:  map[string]string{"Authorization": "Bearer " + os.Getenv("LOXA_TOKEN")},
    }),
}
```

Heavy delivery sinks such as Kafka, ClickHouse, Postgres, DuckDB, OTLP, S3, GCS, and Loki belong in `loxa-collector`, not in application SDKs.

---

### Local Dev Output

Use `loxa.Dev()` preset in development — prints human-readable events to stdout:

```go
if os.Getenv("ENV") == "development" {
    loxa.Configure(loxa.Dev().WithService(os.Getenv("SERVICE_NAME")))
} else {
    loxa.Configure(loxa.Production(). /* ... */ )
}
```

Dev output:
```
loxa checkout.completed error 1247ms
  request.id=req_1 trace.id=trace_abc
  user.id=user_7 user.plan=premium
  cart.total_cents=15999
  payment.provider=stripe payment.attempt=3
  error.code=card_declined
```

In production it emits structured JSON / OTLP. Same application code, different sink.

---

### Immediate Logs

loxa supports one-shot log events without `StartEvent` for startup messages, background warnings, etc.

```go
loxa.Info("worker started", loxa.String("queue", "emails"))
loxa.Warn("rate limit approaching", loxa.Int("requests_remaining", 10))
loxa.Error("sink flush failed", loxa.String("sink", "clickhouse"))
```

These emit immediately — no `Emit()` needed. Use sparingly; prefer enriching the active wide event.

---

### Two Application-Facing Levels Only

loxa exposes only two levels to application code:

- **`loxa.Info` / `loxa.Finish`** — all normal events, including those with `outcome=error`
- **`loxa.Error`** — SDK-level failures only (sink unavailable, schema panic)

Do not reach for `loxa.Debug`, `loxa.Warn`, or `loxa.Trace` in hot paths.
If you want more detail, **add fields to the event** rather than adding log lines.

---

### Field Count and Safety Limits

The SDK enforces these by default (configurable via `loxa.SecurityConfig`):

| Limit | Default |
|-------|---------|
| Max attrs per event | 512 |
| Max field value bytes | 4 KB |
| Max object depth | 5 |
| Max array length | 50 |
| Max event bytes | 256 KB |

Events exceeding limits are truncated (warn mode) or dropped (strict mode) and never crash the app.
