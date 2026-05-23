# LOXA

LOXA is a collector-first wide-event stack:

- `collector/`: the ingest runtime, query surface, durability layer, and sink fanout
- `sdks/go/`: the reference SDK for emitting canonical events
- `sdks/py/`: a lightweight Python SDK that emits to the collector through `HTTPBatchSink`
- `sdks/rs/`: a lightweight Rust SDK that emits to the collector through `HttpBatch`
- `sdks/js/`: a lightweight JavaScript/TypeScript SDK that emits to the collector through `HTTPBatchSink`
- `cli/`: the local operator CLI
- `spec/`: the shared wire and schema contract

For the current release contract, treat [spec/docs/MVP_CUT.md](spec/docs/MVP_CUT.md) plus the package READMEs as authoritative. The `.kiro` specs and roadmap files are backlog material unless a feature is also claimed in a package README or runnable example.

## Quick Start

### 1. Start the collector

From this repo:

```bash
cd collector
go run ./cmd/loxa-collector run -c configs/loxa.local.yaml
```

Or through the CLI when `collector_repo_path` points at `collector`:

```bash
cd cli
go run ./cmd/loxa collector run -c configs/loxa.local.yaml
```

### 2. Emit an event from an SDK

Go:

```go
package main

import (
	"context"

	"github.com/astraive/loxa/sdks/go"
)

func main() {
	loxa.Configure(loxa.Production("checkout").
		WithCollectorEndpoint("http://127.0.0.1:9090"))
	defer loxa.Shutdown(context.Background())

	ctx := loxa.StartEvent(context.Background(), loxa.Params{
		Event:  "checkout.request",
		Method: "POST",
		Path:   "/checkout",
	})
	loxa.Enrich(ctx, loxa.UserID("u-1"))
	loxa.Finish(ctx, "success", loxa.Int("status_code", 200))
	_ = loxa.Emit(ctx)
}
```

Python:

```python
import loxa

loxa.configure(
    loxa.production("checkout")
    .with_collector_endpoint("http://127.0.0.1:9090")
)

ctx = loxa.start_event(event="checkout.request", kind="http")
loxa.enrich(ctx, loxa.UserID("u-1"))
loxa.finish(ctx, "success", loxa.Int("status_code", 200))
loxa.emit(ctx)
loxa.shutdown()
```

Rust:

```rust
fn main() -> Result<(), Box<dyn std::error::Error>> {
    loxa::configure(
        loxa::Config::production("checkout")
            .with_collector_endpoint("http://127.0.0.1:9090"),
    )?;

    let mut ctx = loxa::start_event(loxa::Params::new("checkout.request").with_kind("http"));
    loxa::enrich(&mut ctx, loxa::UserID("u-1"));
    loxa::finish(&mut ctx, "success");
    loxa::emit(&mut ctx)?;
    loxa::shutdown();
    Ok(())
}
```

JavaScript:

```ts
import { loxa } from 'loxa-js';

loxa.configure(
  loxa.production('checkout')
    .withCollectorEndpoint('http://127.0.0.1:9090')
);

const ctx = loxa.startEvent({ event: 'checkout.request', kind: 'http' });
loxa.enrich(ctx, { 'user.id': 'u-1' });
loxa.finish(ctx, 'success');
await loxa.emit(ctx);
await loxa.shutdown();
```

### 3. Query stored events

```bash
cd cli
go run ./cmd/loxa query --sql "SELECT * FROM events LIMIT 10"
```

## Collector-First Contract

Applications emit canonical events to the collector. Heavy production sinks stay collector-owned.

- SDK-owned delivery: stdout/file/test sinks plus collector HTTP batch transport
- Collector-owned delivery: DuckDB, Kafka, ClickHouse, Postgres, Loki, OTLP, S3, and GCS fanout
- Stable operator path: `query`, `tail`, `dlq`, `replay`, and collector-side deletion endpoints

## Current Status

- **Go SDK**: 🟢 **STABLE** - Strongest implementation, full conformance, production-ready
- **Python SDK**: 🟢 **STABLE** - Collector-capable, full conformance, production-ready
- **Rust SDK**: 🟢 **STABLE** - Collector-capable, full conformance, production-ready
- **JavaScript SDK**: 🟢 **STABLE** - TypeScript-capable, full conformance, production-ready
- **Collector**: 🟢 **STABLE** - Direct/spool/queue modes, gzip ingestion, query/tail/DLQ/delete endpoints
- **CLI**: Mixed maturity - use `loxa maturity` to see per-command status

For conformance guarantees and required SDK behaviors, see [spec/docs/SDK_CONFORMANCE_CONTRACT.md](spec/docs/SDK_CONFORMANCE_CONTRACT.md).

See [CLI Command Maturity](#cli-command-maturity) below for per-command stability levels.

Not current release guarantees:

- SIGHUP hot reload
- zstd or response compression
- hybrid reliability mode
- Redis-backed distributed dedupe
- enforced JWT or mTLS auth for HTTP ingest

## Documentation

- [docs/architecture.md](docs/architecture.md)
- [docs/configuration.md](docs/configuration.md)
- [docs/deployment.md](docs/deployment.md)
- [collector/README.md](collector/README.md)
- [sdks/go/README.md](sdks/go/README.md)
- [sdks/py/README.md](sdks/py/README.md)
- [sdks/rs/README.md](sdks/rs/README.md)
- [sdks/js/README.md](sdks/js/README.md)
- [cli/README.md](cli/README.md)
- [spec/docs/MVP_CUT.md](spec/docs/MVP_CUT.md) - Release contract
- [spec/docs/SDK_CONFORMANCE_CONTRACT.md](spec/docs/SDK_CONFORMANCE_CONTRACT.md) - SDK canonical behaviors
- [spec/docs/DUPLICATE_FIELDS.md](spec/docs/DUPLICATE_FIELDS.md) - Reserved field policies

## CLI Command Maturity

Use `loxa maturity` to view current command stability levels:

| Command | Maturity | Notes |
|---------|----------|-------|
| init | stable | v0.0.1: Core initialization working reliably |
| dev | stable | v0.0.1: Development server fully tested |
| config | stable | v0.0.1: Configuration management solid |
| schema | stable | v0.0.1: Schema validation complete |
| collector | stable | v0.0.1: Collector CLI fully functional |
| emit | stable | v0.0.1: Event emission tested |
| query | stable | v0.0.1: SQL query support production-ready |
| tail | stable | v0.0.1: Log tail functionality complete |
| bench | stable | v0.0.1: Load generation validated |
| worker | beta | v0.0.1: Worker mode working, may refine distribution |
| dlq | beta | v0.0.1: DLQ replay available, refinements planned |
| replay | beta | v0.0.1: DLQ/spool replay working |
| delete | beta | v0.0.1: GDPR deletion available, SLOs TBD |
| audit | beta | v0.0.1: Audit/PII discovery available |
| deploy | beta | v0.0.1: Docker/Helm/K8s support, multi-cloud TBD |
| export | experimental | v0.0.1: Format support may change |
| doctor | experimental | v0.0.1: Environment checks in progress |
| dashboard | experimental | v0.0.1: Web UI under active development |

**Stability Definitions**:
- **stable**: Production-ready, covered by release tests, API stable until major version
- **beta**: Working implementation, being refined, minor API changes possible in v1.x
- **experimental**: Under active development, subject to significant change, may be removed

## Verification

Current verification baseline after this closure pass:

- `cd sdks/go && go test ./...`
- `cd sdks/py && python -m pytest -q`
- `cd sdks/rs && cargo test -q`
- `cd sdks/js && npm test && npm run build`
- `cd collector && go test ./...`
