# LOXA

LOXA is a collector-first wide-event stack:

- `loxa-collector`: the ingest runtime, query surface, durability layer, and sink fanout
- `loxa-go`: the reference SDK for emitting canonical events
- `loxa-py`: a lightweight Python SDK that emits to the collector through `HTTPBatchSink`
- `loxa-rs`: a lightweight Rust SDK that emits to the collector through `HttpBatch`
- `loxa-js`: a lightweight JavaScript/TypeScript SDK that emits to the collector through `HTTPBatchSink`
- `loxa-cli`: the local operator CLI
- `loxa-spec`: the shared wire and schema contract

For the current release contract, treat [loxa-spec/docs/MVP_CUT.md](E:/astraive/loxa/loxa-spec/docs/MVP_CUT.md) plus the package READMEs as authoritative. The `.kiro` specs and roadmap files are backlog material unless a feature is also claimed in a package README or runnable example.

## Quick Start

### 1. Start the collector

From this repo:

```bash
cd loxa-collector
go run ./cmd/loxa-collector run -c configs/loxa.local.yaml
```

Or through the CLI when `collector_repo_path` points at `loxa-collector`:

```bash
cd loxa-cli
go run ./cmd/loxa collector run -c configs/loxa.local.yaml
```

### 2. Emit an event from an SDK

Go:

```go
package main

import (
	"context"

	"github.com/astraive/loxa-go"
	"github.com/astraive/loxa-go/sinks/httpbatch"
)

func main() {
	sink, _ := httpbatch.New(httpbatch.Config{
		URL: "http://127.0.0.1:9090/v1/events",
	})

	_ = loxa.Configure(
		loxa.Production().
			WithService("checkout").
			WithSink(sink),
	)
	defer loxa.Shutdown(context.Background())

	ctx := loxa.StartEvent(context.Background(), loxa.Params{
		Event:  "checkout.request",
		Method: "POST",
		Path:   "/checkout",
		Route:  "/checkout",
	})
	loxa.Enrich(ctx, loxa.UserID("u-1"))
	loxa.Finish(ctx, "success", loxa.Int("status_code", 200))
	_ = loxa.Emit(ctx)
}
```

Python:

```python
from loxa import Config, HTTPBatchSink, Logger, Params

logger = Logger(
    Config.production("checkout").with_sink(
        HTTPBatchSink("http://127.0.0.1:9090/v1/events")
    )
)

ctx = logger.start_event(Params(event="checkout.request", method="POST", path="/checkout"))
logger.enrich(ctx, "user.id", "u-1")
logger.finish(ctx, outcome="success")
logger.emit(ctx)
```

Rust:

```rust
use loxa::{Config, Logger, Params, SinkConfig};

fn main() {
    let logger = Logger::new(
        Config::production("checkout").with_sink(SinkConfig::HttpBatch {
            endpoint: "http://127.0.0.1:9090/v1/events".to_string(),
        }),
    );

    let mut ctx = logger.start_event(Params::new("checkout.request").with_kind("http"));
    logger.finish(&mut ctx, "success").unwrap();
    logger.emit(&ctx).unwrap();
}
```

JavaScript:

```ts
import { Logger, HTTPBatchSink, UserID } from 'loxa-js';

const sink = new HTTPBatchSink({ endpoint: 'http://127.0.0.1:9090/v1/events' });
const logger = new Logger({ service: 'checkout', sink });

const ctx = logger.startEvent({ event: 'checkout.request', kind: 'http' });
logger.enrich(ctx, UserID('u-1'));
logger.finish(ctx, 'success');
await logger.emit(ctx);
```

### 3. Query stored events

```bash
cd loxa-cli
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

For conformance guarantees and required SDK behaviors, see [loxa-spec/docs/SDK_CONFORMANCE_CONTRACT.md](loxa-spec/docs/SDK_CONFORMANCE_CONTRACT.md).

See [CLI Command Maturity](#cli-command-maturity) below for per-command stability levels.

Not current release guarantees:

- SIGHUP hot reload
- zstd or response compression
- hybrid reliability mode
- Redis-backed distributed dedupe
- enforced JWT or mTLS auth for HTTP ingest

## Documentation

- [docs/architecture.md](E:/astraive/loxa/docs/architecture.md)
- [docs/configuration.md](E:/astraive/loxa/docs/configuration.md)
- [docs/deployment.md](E:/astraive/loxa/docs/deployment.md)
- [loxa-collector/README.md](E:/astraive/loxa/loxa-collector/README.md)
- [loxa-go/README.md](E:/astraive/loxa/loxa-go/README.md)
- [loxa-py/README.md](E:/astraive/loxa/loxa-py/README.md)
- [loxa-rs/README.md](E:/astraive/loxa/loxa-rs/README.md)
- [loxa-js/README.md](E:/astraive/loxa/loxa-js/README.md)
- [loxa-cli/README.md](E:/astraive/loxa/loxa-cli/README.md)
- [loxa-spec/docs/MVP_CUT.md](E:/astraive/loxa/loxa-spec/docs/MVP_CUT.md) - Release contract
- [loxa-spec/docs/SDK_CONFORMANCE_CONTRACT.md](E:/astraive/loxa/loxa-spec/docs/SDK_CONFORMANCE_CONTRACT.md) - SDK canonical behaviors
- [loxa-spec/docs/DUPLICATE_FIELDS.md](E:/astraive/loxa/loxa-spec/docs/DUPLICATE_FIELDS.md) - Reserved field policies

## CLI Command Maturity

Use `loxa maturity` to view current command stability levels:

| Command | Maturity | Notes |
|---------|----------|-------|
| init | stable | v1.0.0: Core initialization working reliably |
| dev | stable | v1.0.0: Development server fully tested |
| config | stable | v1.0.0: Configuration management solid |
| schema | stable | v1.0.0: Schema validation complete |
| collector | stable | v1.0.0: Collector CLI fully functional |
| emit | stable | v1.0.0: Event emission tested |
| query | stable | v1.0.0: SQL query support production-ready |
| tail | stable | v1.0.0: Log tail functionality complete |
| bench | stable | v1.0.0: Load generation validated |
| worker | beta | v1.0.0: Worker mode working, may refine distribution |
| dlq | beta | v1.0.0: DLQ replay available, refinements planned |
| replay | beta | v1.0.0: DLQ/spool replay working |
| delete | beta | v1.0.0: GDPR deletion available, SLOs TBD |
| audit | beta | v1.0.0: Audit/PII discovery available |
| deploy | beta | v1.0.0: Docker/Helm/K8s support, multi-cloud TBD |
| export | experimental | v1.0.0: Format support may change |
| doctor | experimental | v1.0.0: Environment checks in progress |
| dashboard | experimental | v1.0.0: Web UI under active development |

**Stability Definitions**:
- **stable**: Production-ready, covered by release tests, API stable until major version
- **beta**: Working implementation, being refined, minor API changes possible in v1.x
- **experimental**: Under active development, subject to significant change, may be removed

## Verification

Current verification baseline after this closure pass:

- `cd loxa-go && go test ./...`
- `cd loxa-py && python -m pytest -q`
- `cd loxa-rs && cargo test -q`
- `cd loxa-js && npm test && npm run build`
- `cd loxa-collector && go test ./...`
