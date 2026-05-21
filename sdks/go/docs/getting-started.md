# Getting Started

A 5-minute quickstart for the LOXA Go SDK. By the end you will have a working application that creates, enriches, finishes, and emits a wide-event.

## Install

```bash
go get github.com/astraive/loxa-go@latest
```

## Full Working Example

```go
package main

import (
	"context"
	"fmt"

	"github.com/astraive/loxa-go"
)

func main() {
	// 1. Configure a logger with a stdout sink (dev mode).
	cfg := loxa.Dev("checkout-service").
		WithSink(loxa.StdoutSink())

	_ = loxa.Configure(cfg)

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
	loxa.Flush()
	loxa.Shutdown()
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
| Flush | `loxa.Flush()` | Flushes any buffered events in async sinks. |
| Shutdown | `loxa.Shutdown()` | Shuts down the logger, flushing remaining events. |

## Connecting to a Collector

In production, send events to the LOXA collector via HTTPBatchSink:

```go
cfg := loxa.Production("checkout-service").
    WithSink(loxa.HTTPBatchSink("http://collector:9090/v1/events")).
    WithSampler(loxa.SampleErrors())
```

## Event Lifecycle

```
StartEvent -> Enrich -> Finish -> Emit
     |            |        |       |
  creates     adds      marks   serializes
  context    attrs     done     + sends
```

## Next Steps

- [Public API](public-api.md) -- Full API reference.
- [Middleware](middleware.md) -- HTTP framework integration.
- [Security](security.md) -- Redaction and field controls.
- [Testing](testing.md) -- Using testkit and MemorySink.
