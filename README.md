# LOXA

<p align="center">
  <img src="http://github.com/Astraive/loxa/blob/main/assets/branding/loxa-horizontal.png?raw=true" alt="Loxa" width="320">
</p>

<h3 align="center">
  Collector-first wide-event observability stack
</h3>

<p align="center">
  <a href="https://github.com/astraive/loxa/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT">
  </a>
  <a href="https://github.com/astraive/loxa/releases">
    <img src="https://img.shields.io/badge/version-v0.2.6-green.svg" alt="Version">
  </a>
  <br>
  <a href="https://github.com/astraive/loxa/actions/workflows/collector-ci.yml">
    <img src="https://github.com/astraive/loxa/actions/workflows/collector-ci.yml/badge.svg" alt="Collector CI">
  </a>
  <a href="https://github.com/astraive/loxa/actions/workflows/sdks-go.yml">
    <img src="https://github.com/astraive/loxa/actions/workflows/sdks-go.yml/badge.svg" alt="Go SDK CI">
  </a>
  <a href="https://github.com/astraive/loxa/actions/workflows/sdks-py-ci.yml">
    <img src="https://github.com/astraive/loxa/actions/workflows/sdks-py-ci.yml/badge.svg" alt="Python SDK CI">
  </a>
  <a href="https://github.com/astraive/loxa/actions/workflows/sdks-rs-ci.yml">
    <img src="https://github.com/astraive/loxa/actions/workflows/sdks-rs-ci.yml/badge.svg" alt="Rust SDK CI">
  </a>
  <a href="https://github.com/Astraive/loxana/actions/workflows/loxana-ci.yml">
    <img src="https://github.com/Astraive/loxana/actions/workflows/loxana-ci.yml/badge.svg" alt="Loxana CI">
  </a>
  <a href="https://hub.docker.com/r/astraive/loxa">
  <br>
    <img src="https://img.shields.io/docker/pulls/astraive/loxa?label=collector%20pulls" alt="Collector Docker Pulls">
  </a>
  <a href="https://hub.docker.com/r/astraive/loxa-cortex">
    <img src="https://img.shields.io/docker/pulls/astraive/loxa-cortex?label=cortex%20pulls" alt="Cortex Docker Pulls">
  </a>
</p>

---

LOXA is a collector-first wide-event observability stack.

Applications emit canonical structured events through lightweight SDKs to the LOXA Collector. The collector owns the heavy production pipeline: validation, deduplication, durable storage, querying, replay, deletion, and fanout to downstream sinks.

Cortex adds incident intelligence and graph analysis on top of the event stream. Loxana provides a dashboard for querying, visualization, alerting, and operator workflows.

The design principle is simple:

> **SDKs stay thin. The collector owns the pipeline.**

## Install Loxa CLI

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/astraive/loxa/main/cli/install/install.sh | bash
```

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/astraive/loxa/main/cli/install/install.ps1 | iex
```

## Install SDKs

```bash
npm install loxa
pip install loxa
cargo add loxa
go get github.com/astraive/loxa/sdks/go
```

## Repository Layout

```txt
.
├── collector/   # Ingest runtime, query surface, durability layer, and sink fanout
├── cortex/      # Incident intelligence, graph reconstruction, and analysis
├── sdks/
│   ├── go/      # Reference SDK for emitting canonical events
│   ├── py/      # Lightweight Python SDK using HTTPBatchSink
│   ├── rs/      # Lightweight Rust SDK using HttpBatch
│   └── js/      # Lightweight JavaScript/TypeScript SDK using HTTPBatchSink
├── cli/         # Local operator CLI
│   └── install/ # CLI-only install scripts
├── lql/         # LOXA Query Language compiler / tooling
├── spec/        # Shared wire and schema contract
├── docs/        # Architecture, configuration, deployment, and security docs
└── deploy/      # Docker, Kubernetes, and Helm deployment assets
```

For the current release contract, treat [`spec/docs/MVP_CUT.md`](spec/docs/MVP_CUT.md) plus the package READMEs as authoritative.

The `.kiro` specs and roadmap files are backlog material unless a feature is also claimed in a package README or runnable example.

## Architecture

```mermaid
flowchart TD
    subgraph Apps["Applications"]
        Go["Go SDK"]
        Py["Python SDK"]
        Rs["Rust SDK"]
        Js["JavaScript / TypeScript SDK"]
    end

    Go -->|Canonical Events| Collector
    Py -->|HTTP Batch| Collector
    Rs -->|HTTP Batch| Collector
    Js -->|HTTP Batch| Collector

    subgraph Loxa["LOXA Runtime"]
        Collector["LOXA Collector<br/>Port 9308"]
        Validate["Validate"]
        Dedupe["Deduplicate"]
        Store["Durable Store"]
        Query["Query / Tail / DLQ / Replay / Delete"]
        Fanout["Sink Fanout"]
    end

    Collector --> Validate
    Validate --> Dedupe
    Dedupe --> Store
    Store --> Query
    Dedupe --> Fanout

    subgraph Sinks["Collector-Owned Sinks"]
        DuckDB["DuckDB"]
        Kafka["Kafka"]
        ClickHouse["ClickHouse"]
        Postgres["Postgres"]
        Loki["Loki"]
        OTLP["OTLP"]
        S3["S3"]
        GCS["GCS"]
    end

    Fanout --> DuckDB
    Fanout --> Kafka
    Fanout --> ClickHouse
    Fanout --> Postgres
    Fanout --> Loki
    Fanout --> OTLP
    Fanout --> S3
    Fanout --> GCS

    Store --> Cortex["LOXA Cortex<br/>Incident Intelligence<br/>Port 9312"]
    Cortex --> Loxana["Loxana Dashboard<br/>Vite + React"]
```

## Collector-First Contract

Applications emit canonical events to the collector. Heavy production sinks stay collector-owned.

```mermaid
flowchart LR
    App["Application"] --> SDK["Thin SDK"]
    SDK --> Transport["stdout / file / test / HTTP batch"]
    Transport --> Collector["Collector"]

    Collector --> Durability["Durability"]
    Collector --> Querying["Query Surface"]
    Collector --> Deletion["Deletion APIs"]
    Collector --> Fanout["Production Fanout"]

    Fanout --> DuckDB["DuckDB"]
    Fanout --> Kafka["Kafka"]
    Fanout --> ClickHouse["ClickHouse"]
    Fanout --> Postgres["Postgres"]
    Fanout --> Loki["Loki"]
    Fanout --> OTLP["OTLP"]
    Fanout --> ObjectStorage["S3 / GCS"]
```

### SDK-Owned Delivery

SDKs own only lightweight local delivery:

* stdout sink
* file sink
* test sink
* collector HTTP batch transport

### Collector-Owned Delivery

The collector owns production sinks and durable routing:

* DuckDB
* Kafka
* ClickHouse
* Postgres
* Loki
* OTLP
* S3
* GCS

### Stable Operator Path

The stable operator surface is:

* `query`
* `tail`
* `dlq`
* `replay`
* collector-side deletion endpoints

## Components

| Component                  | Description                                                               |           Language | Docs                                           |
| -------------------------- | ------------------------------------------------------------------------- | -----------------: | ---------------------------------------------- |
| [`collector/`](collector/) | Event ingestion, validation, storage, query, replay, deletion, and fanout |                 Go | [`collector/README.md`](collector/README.md)   |
| [`cortex/`](cortex/)       | Incident intelligence, graph reconstruction, and event analysis           |                 Go | [`cortex/README.md`](cortex/README.md)         |
| [`sdks/go/`](sdks/go/)     | Reference SDK and strongest implementation                                |                 Go | [`sdks/go/README.md`](sdks/go/README.md)       |
| [`sdks/py/`](sdks/py/)     | Lightweight Python SDK                                                    |             Python | [`sdks/py/README.md`](sdks/py/README.md)       |
| [`sdks/rs/`](sdks/rs/)     | Lightweight Rust SDK                                                      |               Rust | [`sdks/rs/README.md`](sdks/rs/README.md)       |
| [`sdks/js/`](sdks/js/)     | JavaScript / TypeScript SDK                                               |         TypeScript | [`sdks/js/README.md`](sdks/js/README.md)       |
| [`cli/`](cli/)             | Local operator CLI                                                        |                 Go | [`cli/README.md`](cli/README.md)               |
| [`lql/`](lql/)             | LOXA Query Language tooling                                               |        Rust / WASM | [`lql/README.md`](lql/README.md)               |
| [`spec/`](spec/)           | Shared wire, schema, and conformance contract                             | Markdown / schemas | [`spec/docs/MVP_CUT.md`](spec/docs/MVP_CUT.md) |

## Quick Start

### 1. Start the Collector

The tracked Collector configuration is secure by default. Generate distinct runtime secrets; do not put them in YAML or source control.

```bash
cd collector
export COLLECTOR_AUTH_SERVER_SECRET="$(openssl rand -hex 32)"
export COLLECTOR_INGEST_KEY_SECRET="$(openssl rand -hex 24)"
export COLLECTOR_ADMIN_KEY_SECRET="$(openssl rand -hex 24)"
export LOXA_STORAGE_ENCRYPTION_KEY="$(openssl rand -hex 32)"
export LOXA_API_KEY="lx_sec_live_kingest_${COLLECTOR_INGEST_KEY_SECRET}"
go run ./cmd/loxa-collector run -c configs/loxa.yaml
```

Verify the public health endpoint, then send an authenticated event to the actual ingest route:

```bash
curl http://127.0.0.1:9308/health
curl -X POST http://127.0.0.1:9308/events \
  -H "Authorization: Bearer ${LOXA_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '[{"event":"checkout.request","service":"checkout"}]'
```

The admin token uses `lx_sec_live_kadmin_${COLLECTOR_ADMIN_KEY_SECRET}` and is intended for query and administrative routes.

### 2. Emit an Event from an SDK

#### Go

```go
package main

import (
	"context"

	"github.com/astraive/loxa/sdks/go"
)

func main() {
	loxa.Configure(loxa.Production("checkout").
		WithCollectorEndpoint("http://127.0.0.1:9308"))
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

#### Python

```python
import loxa

loxa.configure(
    loxa.production("checkout")
    .with_collector_endpoint("http://127.0.0.1:9308")
)

ctx = loxa.start_event(event="checkout.request", kind="http")
loxa.enrich(ctx, loxa.UserID("u-1"))
loxa.finish(ctx, "success", loxa.Int("status_code", 200))
loxa.emit(ctx)
loxa.shutdown()
```

#### Rust

```rust
fn main() -> Result<(), Box<dyn std::error::Error>> {
    loxa::configure(
        loxa::Config::production("checkout")
            .with_collector_endpoint("http://127.0.0.1:9308"),
    )?;

    let mut ctx = loxa::start_event(
        loxa::Params::new("checkout.request").with_kind("http"),
    );

    loxa::enrich(&mut ctx, loxa::UserID("u-1"));
    loxa::finish(&mut ctx, "success");
    loxa::emit(&mut ctx)?;
    loxa::shutdown();

    Ok(())
}
```

#### JavaScript / TypeScript

```ts
import { loxa } from 'loxa';

loxa.configure(
  loxa.production('checkout')
    .withCollectorEndpoint('http://127.0.0.1:9308')
);

const ctx = loxa.startEvent({
  event: 'checkout.request',
  kind: 'http',
});

loxa.enrich(ctx, { 'user.id': 'u-1' });
loxa.finish(ctx, 'success');

await loxa.emit(ctx);
await loxa.shutdown();
```

### 3. Query Stored Events

```bash
cd cli
go run ./cmd/loxa query --sql "SELECT * FROM events LIMIT 10"
```

## Docker

### Collector

The collector image is:

```txt
astraive/loxa
```

Run it locally:

```bash
docker run --rm \
  -p 9308:9308 \
  -v "$PWD/collector/configs:/configs:ro" \
  astraive/loxa:latest \
  run -c /configs/loxa.local.yaml
```

### Cortex

The Cortex image is:

```txt
astraive/loxa-cortex
```

Run it locally:

```bash
docker run --rm \
  -p 9312:9312 \
  astraive/loxa-cortex:latest
```

### Docker Compose

```bash
cd collector/deploy
docker compose up -d
```

Typical local services:

| Service    |   Port | Purpose                                    |
| ---------- | -----: | ------------------------------------------ |
| Collector  | `9308` | Ingest, query, tail, DLQ, replay, deletion |
| Cortex     | `9312` | Incident intelligence and graph analysis   |
| Prometheus | `9090` | Metrics                                    |
| Loki       | `3100` | Logs                                       |
| Grafana    | `3000` | Dashboards                                 |

## Deployment Flow

```mermaid
flowchart TD
    Dev["Local Development"] --> Compose["Docker Compose"]
    Compose --> Staging["Staging"]
    Staging --> K8s["Kubernetes / Helm"]
    K8s --> Prod["Production"]

    Prod --> Collector["astraive/loxa"]
    Prod --> Cortex["astraive/loxa-cortex"]
    Prod --> Dashboard["Loxana Dashboard"]
```

## SDK Installation

| Language                | Command                                          |
| ----------------------- | ------------------------------------------------ |
| Go                      | `go get github.com/astraive/loxa/sdks/go@v0.2.6` |
| Python                  | `pip install loxa`                               |
| Rust                    | `cargo add loxa`                                 |
| JavaScript / TypeScript | `npm install loxa`                               |

## Current Status

| Component      | Status         | Notes                                                                     |
| -------------- | -------------- | ------------------------------------------------------------------------- |
| Go SDK         | 🟢 Stable      | Strongest implementation, full conformance, production-ready              |
| Python SDK     | 🟢 Stable      | Collector-capable, full conformance, production-ready                     |
| Rust SDK       | 🟢 Stable      | Collector-capable, full conformance, production-ready                     |
| JavaScript SDK | 🟢 Stable      | TypeScript-capable, full conformance, production-ready                    |
| Collector      | 🟢 Stable      | Direct/spool/queue modes, gzip ingestion, query/tail/DLQ/delete endpoints |
| CLI            | Mixed maturity | Use `loxa maturity` to see per-command status                             |
| Cortex         | Beta           | Incident intelligence and graph analysis are active surfaces              |
| Loxana         | Experimental   | Dashboard UI under active development                                     |

For conformance guarantees and required SDK behaviors, see [`spec/docs/SDK_CONFORMANCE_CONTRACT.md`](spec/docs/SDK_CONFORMANCE_CONTRACT.md).

## Not Current Release Guarantees

The following are not guaranteed by the current release contract unless claimed in a package README or runnable example:

* SIGHUP hot reload
* zstd or response compression
* hybrid reliability mode
* Redis-backed distributed dedupe
* enforced JWT or mTLS auth for HTTP ingest

## CLI Command Maturity

Use `loxa maturity` to view current command stability levels.

| Command     | Maturity     | Notes                                            |
| ----------- | ------------ | ------------------------------------------------ |
| `init`      | stable       | Core initialization working reliably             |
| `dev`       | stable       | Development server fully tested                  |
| `config`    | stable       | Configuration management solid                   |
| `schema`    | stable       | Schema validation complete                       |
| `collector` | stable       | Collector CLI fully functional                   |
| `emit`      | stable       | Event emission tested                            |
| `query`     | stable       | SQL query support production-ready               |
| `tail`      | stable       | Log tail functionality complete                  |
| `bench`     | stable       | Load generation validated                        |
| `worker`    | beta         | Worker mode working, distribution may be refined |
| `dlq`       | beta         | DLQ replay available, refinements planned        |
| `replay`    | beta         | DLQ/spool replay working                         |
| `delete`    | beta         | GDPR deletion available, SLOs TBD                |
| `audit`     | beta         | Audit/PII discovery available                    |
| `deploy`    | beta         | Docker/Helm/K8s support, multi-cloud TBD         |
| `export`    | experimental | Format support may change                        |
| `doctor`    | experimental | Environment checks in progress                   |
| `dashboard` | experimental | Web UI under active development                  |

### Stability Definitions

| Level        | Meaning                                                                    |
| ------------ | -------------------------------------------------------------------------- |
| stable       | Production-ready, covered by release tests, API stable until major version |
| beta         | Working implementation, being refined, minor API changes possible in v1.x  |
| experimental | Under active development, subject to significant change, may be removed    |

## Verification

Current verification baseline after this closure pass:

```bash
cd sdks/go && go test ./...
cd sdks/py && python -m pytest -q
cd sdks/rs && cargo test -q
cd sdks/js && npm test && npm run build
cd collector && go test ./...
```

## Documentation

| Topic                       | Link                                                                             |
| --------------------------- | -------------------------------------------------------------------------------- |
| Architecture                | [`docs/architecture.md`](docs/architecture.md)                                   |
| Configuration               | [`docs/configuration.md`](docs/configuration.md)                                 |
| Deployment                  | [`docs/deployment.md`](docs/deployment.md)                                       |
| Collector                   | [`collector/README.md`](collector/README.md)                                     |
| Go SDK                      | [`sdks/go/README.md`](sdks/go/README.md)                                         |
| Python SDK                  | [`sdks/py/README.md`](sdks/py/README.md)                                         |
| Rust SDK                    | [`sdks/rs/README.md`](sdks/rs/README.md)                                         |
| JavaScript / TypeScript SDK | [`sdks/js/README.md`](sdks/js/README.md)                                         |
| CLI                         | [`cli/README.md`](cli/README.md)                                                 |
| MVP Cut / Release Contract  | [`spec/docs/MVP_CUT.md`](spec/docs/MVP_CUT.md)                                   |
| SDK Conformance             | [`spec/docs/SDK_CONFORMANCE_CONTRACT.md`](spec/docs/SDK_CONFORMANCE_CONTRACT.md) |
| Duplicate Fields Policy     | [`spec/docs/DUPLICATE_FIELDS.md`](spec/docs/DUPLICATE_FIELDS.md)                 |

## Supported Sinks

| Sink       | Ownership       | Status    |
| ---------- | --------------- | --------- |
| DuckDB     | Collector-owned | Supported |
| Kafka      | Collector-owned | Supported |
| ClickHouse | Collector-owned | Supported |
| Postgres   | Collector-owned | Supported |
| Loki       | Collector-owned | Supported |
| OTLP       | Collector-owned | Supported |
| S3         | Collector-owned | Supported |
| GCS        | Collector-owned | Supported |

## Development

```bash
git clone https://github.com/astraive/loxa.git
cd loxa

# Collector tests
cd collector
go test ./...

# Go SDK tests
cd ../sdks/go
go test ./...

# Python SDK tests
cd ../py
python -m pytest -q

# Rust SDK tests
cd ../rs
cargo test -q

# JavaScript SDK tests
cd ../js
npm test
npm run build
```

## Contributing

Contributions are welcome.

1. Fork the repository.
2. Create a feature branch.

```bash
git checkout -b feat/my-feature
```

3. Commit your changes.

```bash
git commit -m "feat: add my feature"
```

4. Push the branch.

```bash
git push origin feat/my-feature
```

5. Open a pull request.

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for development setup, coding standards, and PR guidelines.

## Security

For security vulnerabilities, see [`SECURITY.md`](SECURITY.md).

Do not open a public issue for security reports.

## License

LOXA is licensed under the [`MIT License`](LICENSE).

```txt
MIT License - Copyright (c) 2026 Astraive
```

---

<p align="center">
  Built by <a href="https://astraive.com"><strong>Astraive</strong></a>
  &nbsp;&middot;&nbsp;
  <br>
  <a href="https://github.com/astraive/loxa">GitHub</a>
  &nbsp;&middot;&nbsp;
  <a href="https://github.com/astraive/loxa/releases">Releases</a>
  &nbsp;&middot;&nbsp;
  <a href="https://github.com/astraive/loxa/issues">Issues</a>
</p>
