# Getting Started

A 5-minute quickstart for the LOZA Go SDK. By the end you will have a working application that creates, enriches, finishes, and emits a wide-event.

## Default Client

Use package-level `loza.<Method>(ctx, ...)` calls for quick starts and single-client applications.

## Custom Client / Alias

Use `loza.CreateLoza(config)` for an independent client, `loza.New(config)` as the idiomatic Go alias, and `loza.Alias("name")` for a same-config child that emits `loza.alias`.

## Cross-Language Parity

Go maps to the v0.0.2 parity family as package-level `loza`, `CreateLoza`, `New`, and user-defined variables such as `logger.Info(ctx, ...)`.

## Install

```bash
go get github.com/astraive/loza/sdks/go@latest
```

## Full Working Example

```go
package main

import (
	"context"
	"fmt"

	loza "github.com/astraive/loza/sdks/go"
)

func main() {
	// 1. Configure the global logger for development.
	loza.Configure(loza.Dev().WithService("checkout-service"))

	// 2. Start an event.
	ctx := loza.StartEvent(context.Background(), loza.Params{
		Event: "checkout.request",
		Kind:  "http",
	})

	// 3. Enrich with attributes.
	loza.Enrich(ctx,
		loza.UserID("u-abc123"),
		loza.TenantID("tenant-acme"),
		loza.String("cart.id", "cart-42"),
		loza.Int("item_count", 3),
	)

	// 4. Finish the event with an outcome.
	loza.Finish(ctx, "success", loza.Int("status_code", 200))

	// 5. Emit the event to the configured sink.
	if err := loza.Emit(ctx); err != nil {
		fmt.Printf("emit error: %v\n", err)
	}

	// 6. Flush any buffered events and shut down.
	loza.Flush(context.Background())
	loza.Shutdown(context.Background())
}
```

## What Each Step Does

| Step | Function | Purpose |
|------|----------|---------|
| Configure | `loza.Configure(cfg)` | Sets the global logger with service name, sink, sampler, and redactor. |
| StartEvent | `loza.StartEvent(ctx, params)` | Creates a new `EventContext` with a UUIDv7 event ID and initial metadata. |
| Enrich | `loza.Enrich(ctx, attrs...)` | Adds typed attributes to the event. Attributes are merged into the event map. |
| Finish | `loza.Finish(ctx, outcome, attrs...)` | Marks the event as finished, records the outcome, and calculates duration. |
| Emit | `loza.Emit(ctx)` | Serializes the event and sends it to the configured sink. |
| Flush | `loza.Flush(ctx)` | Flushes any buffered events in async sinks. |
| Shutdown | `loza.Shutdown(ctx)` | Shuts down the logger, flushing remaining events. |

## Connecting to a Collector

In production, send events to the LOZA collector with authentication:

```go
loza.Configure(
	loza.Production().
		WithService("checkout-service").
		WithAPIKey(os.Getenv("LOZA_API_KEY")).
		WithCollectorEndpoint("https://collector.loza.dev").
		WithSampler(loza.SampleErrors()),
)
```

The SDK automatically sets `Authorization: Bearer <key>`, `X-Loza-Service`, and `X-Loza-Env` headers.

### Authentication

| Config Field | Env Var | Description |
|---|---|---|
| `APIKey` | `LOZA_API_KEY` | Ingest API key (`lz_sec_live_k_xxx_yyyy`) |
| `Insecure` | -- | Allow plain HTTP (local dev only) |

```go
// Production (HTTPS required)
loza.Configure(
	loza.Production().
		WithService("my-service").
		WithAPIKey("lz_sec_live_k_xxx_yyyy"),
)

// Local dev (HTTP allowed)
loza.Configure(
	loza.Dev().
		WithService("my-service").
		WithAPIKey("lz_local_dev_mytoken").
		WithInsecure(true),
)
```

See [Security](../../docs/security.md) for key types, RBAC roles, and ABAC restrictions.

## Custom Instances

Use `loza.CreateLoza` when you need an isolated logger that does not share the global default. Each instance has its own config, sinks, and buffers.

```go
// Create an independent logger.
logger, err := loza.CreateLoza(loza.Config{
	Service:     "payment-api",
	CollectorURL: "https://collector.loza.dev",
	APIKey:      os.Getenv("LOZA_API_KEY"),
})
if err != nil {
	log.Fatal(err)
}

// Use the instance the same way as the global API.
ctx := logger.StartEvent(context.Background(), loza.Params{
	Event: "payment.charge",
	Kind:  "http",
})

logger.Enrich(ctx, loza.String("payment.provider", "stripe"))
logger.Finish(ctx, "success")
logger.Emit(ctx)
```

`loza.New` is an idiomatic Go alias for `loza.CreateLoza` -- both do the same thing:

```go
logger, err := loza.New(loza.Config{Service: "api"})
```

## Aliases

Use `loza.Alias` to create a second logger that inherits the default logger's config and emits `loza.alias` metadata. This is useful for emitting events on behalf of a logical subsystem without duplicating configuration.

```go
// Create an alias from the global default.
audit, err := loza.Alias("audit")
if err != nil {
	log.Fatal(err)
}

// Use the alias -- same API as the default logger.
ctx := audit.StartEvent(context.Background(), loza.Params{
	Event: "audit.record",
	Kind:  "job",
})

audit.Enrich(ctx, loza.String("action", "user.delete"))
audit.Finish(ctx, "success")
audit.Emit(ctx)
```

The key difference between `CreateLoza` and `Alias`:

| Factory | Config Source | Use Case |
|---------|-------------|----------|
| `loza.CreateLoza(cfg)` | Fully independent config | Different sinks, samplers, or endpoints |
| `loza.Alias("name")` | Inherits default logger config plus `loza.alias` | Same infrastructure, logical alias metadata |

## Immediate Logging

For events that do not need the full start/enrich/finish/emit lifecycle, use the immediate logging API:

```go
// Package-level (uses the global default logger).
loza.Info("server started", loza.String("addr", ":8080"))
loza.Warn("high memory usage", loza.Float64("usage_mb", 512.0))
loza.Error("connection failed", loza.String("host", "db.example.com"))

// On a custom instance.
logger, _ := loza.New(loza.Config{Service: "api"})
logger.Info("ready to serve")
```

## Cross-Language Parity

The Go SDK mirrors the same API pattern across all LOZA SDKs:

| Operation | Go | JavaScript | Python | Rust |
|-----------|----|------------|--------|------|
| Configure | `loza.Configure(cfg)` | `loza.configure(cfg)` | `loza.configure(cfg)` | `loza::configure(cfg)` |
| Start Event | `loza.StartEvent(ctx, p)` | `loza.startEvent(p)` | `loza.start_event(p)` | `loza::start_event(p)` |
| Enrich | `loza.Enrich(ctx, ...)` | `loza.enrich(...)` | `loza.enrich(...)` | `loza::enrich(...)` |
| Finish | `loza.Finish(ctx, ...)` | `loza.finish(...)` | `loza.finish(...)` | `loza::finish(...)` |
| Emit | `loza.Emit(ctx)` | `loza.emit()` | `loza.emit()` | `loza::emit()` |
| Info (immediate) | `loza.Info(msg, ...)` | `loza.info(msg, ...)` | `loza.info(msg, ...)` | `loza::info(msg, ...)` |
| Custom Instance | `loza.CreateLoza(cfg)` | `createLoza(cfg)` | `loza.create_loza(cfg)` | `loza::create_loza(cfg)` |
| Alias | `loza.Alias("name")` | `loza.alias("name")` | `loza.alias("name")` | `loza::alias("name")` |

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
