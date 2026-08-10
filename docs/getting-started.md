# Getting Started with LOZA

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
    participant SDK as LOZA SDK
    participant Collector as Collector
    participant Sink as Storage Sink
    participant CLI as loza CLI

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
go build -o loza-collector ./cmd/loza-collector
./loza-collector --config configs/loza.local.yaml
```

### Run with Default Config

```bash
cd collector
go run ./cmd/loza-collector --config loza-collector.defaults.yaml
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

    "github.com/astraive/loza/sdks/go"
)

func main() {
    loza.Configure(loza.Dev("my-service").WithCollectorEndpoint("http://localhost:9308"))
    defer loza.Shutdown(context.Background())

    ctx := loza.StartEvent(context.Background(), loza.Params{Event: "order.created"})
    loza.Enrich(ctx,
        loza.String("order_id", "ORD-12345"),
        loza.String("customer_id", "CUST-789"),
        loza.Int("total_cents", 5999),
    )
    loza.Finish(ctx, "success")
    if err := loza.Emit(ctx); err != nil {
        fmt.Printf("emit error: %v\n", err)
    }
}
```

### Python

```python
import loza

loza.configure(loza.dev("my-service").with_collector_endpoint("http://localhost:9308"))

ctx = loza.start_event(event="order.created")
loza.enrich(ctx,
    loza.String("order_id", "ORD-12345"),
    loza.String("customer_id", "CUST-789"),
    loza.Int("total_cents", 5999),
)
loza.finish(ctx, "success")
loza.emit(ctx)
loza.shutdown()
```

### Rust

```rust
fn main() -> Result<(), Box<dyn std::error::Error>> {
    loza::configure(
        loza::Config::dev("my-service").with_collector_endpoint("http://localhost:9308"),
    )?;

    let mut ctx = loza::start_event(loza::Params::new("order.created"));
    loza::enrich(&mut ctx, loza::String("order_id", "ORD-12345"));
    loza::enrich(&mut ctx, loza::String("customer_id", "CUST-789"));
    loza::enrich(&mut ctx, loza::Int("total_cents", 5999));
    loza::finish(&mut ctx, "success");
    loza::emit(&mut ctx)?;
    loza::shutdown();
    Ok(())
}
```

### JavaScript

```typescript
import { loza } from "@astraive/loza";

loza.configure(loza.dev("my-service").withCollectorEndpoint("http://localhost:9308"));

const ctx = loza.startEvent({ event: "order.created" });
loza.enrich(ctx,
    loza.string("order_id", "ORD-12345"),
    loza.string("customer_id", "CUST-789"),
    loza.int("total_cents", 5999),
);
loza.finish(ctx, "success");
await loza.emit(ctx);
await loza.shutdown();
```

## Default API (`loza.*`)

All SDKs export a default logger for quick usage. No explicit construction needed:

### Go
```go
loza.Configure(loza.Config{Service: "my-service", CollectorURL: "http://localhost:9308"})
loza.Info("server started", loza.String("port", "8080"))
```

### Python
```python
import loza
loza.configure(loza.Config(service="my-service", collector_endpoint="http://localhost:9308"))
loza.info("server started", port="8080")
```

### Rust
```rust
loza::configure(loza::Config::dev("my-service").with_collector_endpoint("http://localhost:9308")).unwrap();
loza::info("server started");
```

### JavaScript
```typescript
import { loza } from "@astraive/loza";
loza.configure(loza.production("my-service").withCollectorEndpoint("http://localhost:9308"));
loza.info("server started");
```

## Custom Instances (`createLoza` / `loza.New`)

Create isolated logger instances for different services or contexts:

### Go
```go
logger, _ := loza.New(loza.Config{Service: "checkout-api", CollectorURL: "http://localhost:9308"})
logger.Info(ctx, "payment processed")
```

### Python
```python
logger = loza.create_loza(service="checkout-api", collector_endpoint="http://localhost:9308")
logger.info("payment processed")
```

### Rust
```rust
let logger = loza::create_loza(loza::Config::dev("checkout-api").with_collector_endpoint("http://localhost:9308"));
logger.info("payment processed");
```

### JavaScript
```typescript
import { loza } from "@astraive/loza";
const logger = loza.createLoza({ service: "checkout-api", collectorUrl: "http://localhost:9308" });
logger.info("payment processed");
```

## Aliases (`loza.alias`)

Create a same-config variant that preserves `service` and emits `loza.alias` metadata:

### Go
```go
audit, _ := loza.Alias("audit-service")
audit.Info(ctx, "permission changed")
```

### Python
```python
audit = loza.alias("audit-service")
audit.info("permission changed")
```

### Rust
```rust
let audit = loza::alias("audit-service");
audit.info("permission changed");
```

### JavaScript
```typescript
import { loza } from "@astraive/loza";
const audit = loza.alias("audit-service");
audit.info("permission changed");
```

## 3. Query Events

Use the CLI to query events stored by the collector:

```bash
loza query --event order.created --last 10
```

Example output:

```
EVENT            SERVICE       DURATION   ATTRS
order.created    my-service    12ms       order_id=ORD-12345 customer_id=CUST-789 total_cents=5999
order.created    my-service    8ms        order_id=ORD-12346 customer_id=CUST-790 total_cents=2499
```

Query with filters:

```bash
loza query --event order.created --attr "total_cents>5000" --last 5 --format json
```

Tail live events as they arrive:

```bash
loza tail --event order.created
```

## 4. Next Steps

- **Authentication**: [Authentication](authentication.md) and [Authorization](authorization.md) for API keys, RBAC, and ABAC
- **SDK Documentation**: See `sdks/go/docs`, `sdks/py/docs`, `sdks/rs/docs`, `sdks/js/docs` for framework-specific guides
- **Collector Configuration**: Review `collector/configs/` for production, queue, and fanout configurations
- **CLI Reference**: Run `loza --help` or see `cli/docs/` for all available commands
- **Cortex**: Enable the Persistent Context Engine for incident reconstruction and service graph analysis
- **Conformance**: Run `cd spec && python conformance/runner.py` to verify SDK behavior
