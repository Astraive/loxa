# Getting Started

A 5-minute quickstart for the LOXA Go SDK. By the end you will have a working application that creates, enriches, finishes, and emits a wide-event.

## Default Client

Use package-level `loxa.<Method>(ctx, ...)` calls for quick starts and single-client applications.

## Custom Client / Alias

Use `loxa.CreateLoxa(config)` for an independent client, `loxa.New(config)` as the idiomatic Go alias, and `loxa.Alias("name")` for a same-config child that emits `loxa.alias`.

## Cross-Language Parity

Go maps to the v0.0.2 parity family as package-level `loxa`, `CreateLoxa`, `New`, and user-defined variables such as `logger.Info(ctx, ...)`.

## Install

```bash
go get github.com/astraive/loxa/sdks/go@latest
```

## Full Working Example

```go
package main

import (
	"context"
	"fmt"

	loxa "github.com/astraive/loxa/sdks/go"
)

func main() {
	// 1. Configure the global logger for development.
	loxa.Configure(loxa.Dev().WithService("checkout-service"))

	// 2. Start an event.
	ctx := loxa.StartEvent(context.Background(), loxa.Params{
		Event: "checkout.request",
		Kind:  "http",
	})

	// 3. Enrich with attributes.
	loxa.Enrich(ctx,
		loxa.UserID("u-abc123"),
		loxa.TenantID("tenant-acme"),
		loxa.String("cart.id", "cart-42"),
		loxa.Int("item_count", 3),
	)

	// 4. Finish the event with an outcome.
	loxa.Finish(ctx, "success", loxa.Int("status_code", 200))

	// 5. Emit the event to the configured sink.
	if err := loxa.Emit(ctx); err != nil {
		fmt.Printf("emit error: %v\n", err)
	}

	// 6. Flush any buffered events and shut down.
	loxa.Flush(context.Background())
	loxa.Shutdown(context.Background())
}
```

## What Each Step Does

| Step | Function | Purpose |
|------|----------|---------|
| Configure | `loxa.Configure(cfg)` | Sets the global logger with service name, sink, sampler, and redactor. |
| StartEvent | `loxa.StartEvent(ctx, params)` | Creates a new `EventContext` with a UUIDv7 event ID and initial metadata. |
| Enrich | `loxa.Enrich(ctx, attrs...)` | Adds typed attributes to the event. Attributes are merged into the event map. |
| Finish | `loxa.Finish(ctx, outcome, attrs...)` | Marks the event as finished, records the outcome, and calculates duration. |
| Emit | `loxa.Emit(ctx)` | Serializes the event and sends it to the configured sink. |
| Flush | `loxa.Flush(ctx)` | Flushes any buffered events in async sinks. |
| Shutdown | `loxa.Shutdown(ctx)` | Shuts down the logger, flushing remaining events. |

## Connecting to a Collector

In production, send events to the LOXA collector with authentication:

```go
loxa.Configure(
	loxa.Production().
		WithService("checkout-service").
		WithAPIKey(os.Getenv("LOXA_API_KEY")).
		WithCollectorEndpoint("https://collector.loxa.dev").
		WithSampler(loxa.SampleErrors()),
)
```

The SDK automatically sets `Authorization: Bearer <key>`, `X-Loxa-Service`, and `X-Loxa-Env` headers.

### Authentication

| Config Field | Env Var | Description |
|---|---|---|
| `APIKey` | `LOXA_API_KEY` | Ingest API key (`lx_sec_live_k_xxx_yyyy`) |
| `Insecure` | -- | Allow plain HTTP (local dev only) |

```go
// Production (HTTPS required)
loxa.Configure(
	loxa.Production().
		WithService("my-service").
		WithAPIKey("lx_sec_live_k_xxx_yyyy"),
)

// Local dev (HTTP allowed)
loxa.Configure(
	loxa.Dev().
		WithService("my-service").
		WithAPIKey("lx_local_dev_mytoken").
		WithInsecure(true),
)
```

See [Security](../../docs/security.md) for key types, RBAC roles, and ABAC restrictions.

## Custom Instances

Use `loxa.CreateLoxa` when you need an isolated logger that does not share the global default. Each instance has its own config, sinks, and buffers.

```go
// Create an independent logger.
logger, err := loxa.CreateLoxa(loxa.Config{
	Service:     "payment-api",
	CollectorURL: "https://collector.loxa.dev",
	APIKey:      os.Getenv("LOXA_API_KEY"),
})
if err != nil {
	log.Fatal(err)
}

// Use the instance the same way as the global API.
ctx := logger.StartEvent(context.Background(), loxa.Params{
	Event: "payment.charge",
	Kind:  "http",
})

logger.Enrich(ctx, loxa.String("payment.provider", "stripe"))
logger.Finish(ctx, "success")
logger.Emit(ctx)
```

`loxa.New` is an idiomatic Go alias for `loxa.CreateLoxa` -- both do the same thing:

```go
logger, err := loxa.New(loxa.Config{Service: "api"})
```

## Aliases

Use `loxa.Alias` to create a second logger that inherits the default logger's config and emits `loxa.alias` metadata. This is useful for emitting events on behalf of a logical subsystem without duplicating configuration.

```go
// Create an alias from the global default.
audit, err := loxa.Alias("audit")
if err != nil {
	log.Fatal(err)
}

// Use the alias -- same API as the default logger.
ctx := audit.StartEvent(context.Background(), loxa.Params{
	Event: "audit.record",
	Kind:  "job",
})

audit.Enrich(ctx, loxa.String("action", "user.delete"))
audit.Finish(ctx, "success")
audit.Emit(ctx)
```

The key difference between `CreateLoxa` and `Alias`:

| Factory | Config Source | Use Case |
|---------|-------------|----------|
| `loxa.CreateLoxa(cfg)` | Fully independent config | Different sinks, samplers, or endpoints |
| `loxa.Alias("name")` | Inherits default logger config plus `loxa.alias` | Same infrastructure, logical alias metadata |

## Immediate Logging

For events that do not need the full start/enrich/finish/emit lifecycle, use the immediate logging API:

```go
// Package-level (uses the global default logger).
loxa.Info("server started", loxa.String("addr", ":8080"))
loxa.Warn("high memory usage", loxa.Float64("usage_mb", 512.0))
loxa.Error("connection failed", loxa.String("host", "db.example.com"))

// On a custom instance.
logger, _ := loxa.New(loxa.Config{Service: "api"})
logger.Info("ready to serve")
```

## Cross-Language Parity

The Go SDK mirrors the same API pattern across all LOXA SDKs:

| Operation | Go | JavaScript | Python | Rust |
|-----------|----|------------|--------|------|
| Configure | `loxa.Configure(cfg)` | `loxa.configure(cfg)` | `loxa.configure(cfg)` | `loxa::configure(cfg)` |
| Start Event | `loxa.StartEvent(ctx, p)` | `loxa.startEvent(p)` | `loxa.start_event(p)` | `loxa::start_event(p)` |
| Enrich | `loxa.Enrich(ctx, ...)` | `loxa.enrich(...)` | `loxa.enrich(...)` | `loxa::enrich(...)` |
| Finish | `loxa.Finish(ctx, ...)` | `loxa.finish(...)` | `loxa.finish(...)` | `loxa::finish(...)` |
| Emit | `loxa.Emit(ctx)` | `loxa.emit()` | `loxa.emit()` | `loxa::emit()` |
| Info (immediate) | `loxa.Info(msg, ...)` | `loxa.info(msg, ...)` | `loxa.info(msg, ...)` | `loxa::info(msg, ...)` |
| Custom Instance | `loxa.CreateLoxa(cfg)` | `createLoxa(cfg)` | `loxa.create_loxa(cfg)` | `loxa::create_loxa(cfg)` |
| Alias | `loxa.Alias("name")` | `loxa.alias("name")` | `loxa.alias("name")` | `loxa::alias("name")` |

## Event Lifecycle

```
StartEvent -> Enrich -> Finish -> Emit
     |            |        |       |
  creates      adds     marks   serializes
  context     attrs     done    + sends
```

## Next Steps

- [Public API](public-api.md) -- Full API reference.
- [Middleware](middleware.md) -- HTTP framework integration.
- [Security](security.md) -- Redaction and field controls.
- [Testing](testing.md) -- Using testkit and MemorySink.
