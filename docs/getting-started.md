# Getting Started with LOXA

![Version](https://img.shields.io/badge/version-0.2.0-blue)
![License](https://img.shields.io/badge/license-MIT-green)

> v0.2.0 makes the lifecycle primitives first class across every SDK. For the canonical product plan and full method catalog, see [instrumentation-and-sdk-idea.md](./instrumentation-and-sdk-idea.md) and [business-instrumentation.md](./business-instrumentation.md).

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Start the Collector](#1-start-the-collector)
3. [Emit Your First Event](#2-emit-your-first-event)
4. [Query Events](#3-query-events)
5. [Next Steps](#4-next-steps)

## Prerequisites

| Component | Minimum Version | Check Command |
|-----------|----------------|---------------|
| Go        | 1.22+          | `go version`  |
| Python    | 3.10+          | `python3 --version` |
| Rust      | 1.75+          | `rustc --version`   |
| Node.js   | 20+            | `node --version`    |

## Happy Path

```mermaid
sequenceDiagram
    participant App as Application
    participant SDK as LOXA SDK
    participant Collector as Collector
    participant Sink as Storage Sink
    participant CLI as loxa CLI

    App->>SDK: StartEvent("order.created")
    SDK->>SDK: Enrich with attributes
    SDK->>SDK: Finish event
    SDK->>Collector: POST /ingest (batched, gzipped)
    Collector->>Collector: Validate, deduplicate, redact PII
    Collector->>Sink: Fanout to configured sinks
    Collector-->>SDK: 202 Accepted
    CLI->>Collector: GET /core/query?event_name=order.created
    Collector-->>CLI: Matching events
    CLI-->>App: Display results
```

## 1. Start the Collector

### Build from Source

```bash
cd collector
go build -o loxa-collector ./cmd/loxa-collector
./loxa-collector --config configs/loxa.local.yaml
```

### Run with Default Config

```bash
cd collector
go run ./cmd/loxa-collector --config loxa-collector.defaults.yaml
```

The collector starts on port 9308 by default. Verify it is running:

```bash
curl http://localhost:9308/healthz
# Expected: {"status":"ok"}
```

### Docker Compose Alternative

```bash
cd collector/deploy
docker compose up -d
```

This starts the collector with a DuckDB sink and local configuration.

## 2. Emit Your First Event

### Go

```go
package main

import (
    "context"
    "fmt"

    "github.com/astraive/loxa/sdks/go"
)

func main() {
    loxa.Configure(loxa.Dev("my-service").WithCollectorEndpoint("http://localhost:9308"))
    defer loxa.Shutdown(context.Background())

    ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "order.created"})
    loxa.Enrich(ctx,
        loxa.String("order_id", "ORD-12345"),
        loxa.String("customer_id", "CUST-789"),
        loxa.Int("total_cents", 5999),
    )
    loxa.Finish(ctx, "success")
    if err := loxa.Emit(ctx); err != nil {
        fmt.Printf("emit error: %v\n", err)
    }
}
```

### Python

```python
import loxa

loxa.configure(loxa.dev("my-service").with_collector_endpoint("http://localhost:9308"))

ctx = loxa.start_event(event="order.created")
loxa.enrich(ctx,
    loxa.String("order_id", "ORD-12345"),
    loxa.String("customer_id", "CUST-789"),
    loxa.Int("total_cents", 5999),
)
loxa.finish(ctx, "success")
loxa.emit(ctx)
loxa.shutdown()
```

### Rust

```rust
fn main() -> Result<(), Box<dyn std::error::Error>> {
    loxa::configure(
        loxa::Config::dev("my-service").with_collector_endpoint("http://localhost:9308"),
    )?;

    let mut ctx = loxa::start_event(loxa::Params::new("order.created"));
    loxa::enrich(&mut ctx, loxa::String("order_id", "ORD-12345"));
    loxa::enrich(&mut ctx, loxa::String("customer_id", "CUST-789"));
    loxa::enrich(&mut ctx, loxa::Int("total_cents", 5999));
    loxa::finish(&mut ctx, "success");
    loxa::emit(&mut ctx)?;
    loxa::shutdown();
    Ok(())
}
```

### JavaScript

```typescript
import { loxa } from "loxa-js";

loxa.configure(loxa.dev("my-service").withCollectorEndpoint("http://localhost:9308"));

const ctx = loxa.startEvent({ event: "order.created" });
loxa.enrich(ctx,
    loxa.string("order_id", "ORD-12345"),
    loxa.string("customer_id", "CUST-789"),
    loxa.int("total_cents", 5999),
);
loxa.finish(ctx, "success");
await loxa.emit(ctx);
await loxa.shutdown();
```

## Default API (`loxa.*`)

All SDKs export a default logger for quick usage. No explicit construction needed:

### Go
```go
loxa.Configure(loxa.Config{Service: "my-service", CollectorURL: "http://localhost:9308"})
loxa.Info("server started", loxa.String("port", "8080"))
```

### Python
```python
import loxa
loxa.configure(loxa.Config(service="my-service", collector_endpoint="http://localhost:9308"))
loxa.info("server started", port="8080")
```

### Rust
```rust
loxa::configure(loxa::Config::dev("my-service").with_collector_endpoint("http://localhost:9308")).unwrap();
loxa::info("server started");
```

### JavaScript
```typescript
import { loxa } from "loxa-js";
loxa.configure(loxa.production("my-service").withCollectorEndpoint("http://localhost:9308"));
loxa.info("server started");
```

## Custom Instances (`createLoxa` / `loxa.New`)

Create isolated logger instances for different services or contexts:

### Go
```go
logger, _ := loxa.New(loxa.Config{Service: "checkout-api", CollectorURL: "http://localhost:9308"})
logger.Info(ctx, "payment processed")
```

### Python
```python
logger = loxa.create_loxa(service="checkout-api", collector_endpoint="http://localhost:9308")
logger.info("payment processed")
```

### Rust
```rust
let logger = loxa::create_loxa(loxa::Config::dev("checkout-api").with_collector_endpoint("http://localhost:9308"));
logger.info("payment processed");
```

### JavaScript
```typescript
import { loxa } from "loxa-js";
const logger = loxa.createLoxa({ service: "checkout-api", collectorUrl: "http://localhost:9308" });
logger.info("payment processed");
```

## Aliases (`loxa.alias`)

Create a same-config variant that preserves `service` and emits `loxa.alias` metadata:

### Go
```go
audit, _ := loxa.Alias("audit-service")
audit.Info(ctx, "permission changed")
```

### Python
```python
audit = loxa.alias("audit-service")
audit.info("permission changed")
```

### Rust
```rust
let audit = loxa::alias("audit-service");
audit.info("permission changed");
```

### JavaScript
```typescript
import { loxa } from "loxa-js";
const audit = loxa.alias("audit-service");
audit.info("permission changed");
```

## 3. Query Events

Use the CLI to query events stored by the collector:

```bash
loxa query --event order.created --last 10
```

Example output:

```
EVENT            SERVICE       DURATION   ATTRS
order.created    my-service    12ms       order_id=ORD-12345 customer_id=CUST-789 total_cents=5999
order.created    my-service    8ms        order_id=ORD-12346 customer_id=CUST-790 total_cents=2499
```

Query with filters:

```bash
loxa query --event order.created --attr "total_cents>5000" --last 5 --format json
```

Tail live events as they arrive:

```bash
loxa tail --event order.created
```

## 4. Next Steps

- **Authentication**: [Authentication](authentication.md) and [Authorization](authorization.md) for API keys, RBAC, and ABAC
- **SDK Documentation**: See `sdks/go/docs`, `sdks/py/docs`, `sdks/rs/docs`, `sdks/js/docs` for framework-specific guides
- **Collector Configuration**: Review `collector/configs/` for production, queue, and fanout configurations
- **CLI Reference**: Run `loxa --help` or see `cli/docs/` for all available commands
- **Cortex**: Enable the Persistent Context Engine for incident reconstruction and service graph analysis
- **Conformance**: Run `cd spec && python conformance/runner.py` to verify SDK behavior
