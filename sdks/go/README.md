# LOZA-Go

[![CI](https://github.com/astraive/loza/actions/workflows/sdks-go-ci.yml/badge.svg)](https://github.com/astraive/loza/actions/workflows/sdks-go-ci.yml)

**Status**: 🟢 **STABLE** (v0.3.2) - Production-ready, collector-first stable-v1 SDK

Full emitter SDK conformance is tracked through `spec/`:

- `spec/docs/SDK_CONFORMANCE_CONTRACT.md`
- `spec/docs/SDK_CONFORMANCE_TEST_SUITE.md`
- `spec/docs/SDK_COMPLETION_MATRIX.md`

LOZA-Go is a canonical wide-event SDK for Go.
It builds one structured event per operation (request, job, queue message, CLI run, cron run), then emits to your log/analytics backend.

For the shared event contract, see `spec/`.
For the collector runtime, see `collector/`.
For the operations CLI, see `cli/`.

`slog`/`zap`/`zerolog` still fit: LOZA is the operation event layer above line-by-line logs.

## Install

Core:

```bash
go get github.com/astraive/loza/sdks/go
```

Optional modules:

```bash
go get github.com/astraive/loza/sdks/go/middleware
go get github.com/astraive/loza/sdks/go/integrations
go get github.com/astraive/loza/sdks/go/sinks/httpbatch
```

## Quick Start

```go
package main

import (
	"context"

	"github.com/astraive/loza/sdks/go"
)

func main() {
	_ = loza.Configure(loza.Production().WithService("checkout"))
	defer loza.Shutdown(context.Background())

	ctx := loza.StartEvent(context.Background(), loza.Params{
		Event:  "checkout.request",
		Method: "POST",
		Path:   "/checkout",
		Route:  "/checkout",
	})
	defer loza.Emit(ctx)

	loza.Enrich(ctx, loza.UserID("u-1"), loza.String("payment.provider", "stripe"))
	loza.Finish(ctx, "success", loza.Int("status_code", 200))
}
```

Lifecycle:

```text
StartEvent -> Enrich -> Finish / FinishError -> Emit
```

Sample event:

```json
{
  "timestamp": "2026-05-11T10:30:00Z",
  "event_id": "evt_abc",
  "request_id": "req_123",
  "trace_id": "trace_456",
  "service": "checkout",
  "event": "checkout.request",
  "method": "POST",
  "path": "/checkout",
  "route": "/checkout",
  "status_code": 200,
  "duration_ms": 42,
  "outcome": "success",
  "user": {
    "id": "u-1"
  },
  "payment": {
    "provider": "stripe"
  }
}
```

## Custom Instances

```go
// Create a custom logger with its own config
logger, _ := loza.CreateLoza(loza.Config{
    Service:      "checkout-api",
    CollectorURL: "http://localhost:9308",
})
logger.Info(ctx, "payment processed")

// Or use the idiomatic Go alias
logger, _ := loza.New(loza.Config{Service: "checkout-api"})
logger.Info(ctx, "payment processed")

// Alias -- same config as default, loza.alias metadata
audit, _ := loza.Alias("audit-service")
audit.Info(ctx, "permission changed")
```

## API Stability (v1)

The stable core surface for `v1.0.x` is:

- `StartEvent`, `StartHTTPEvent`, `StartJobEvent`, `StartQueueEvent`, `StartCLIEvent`, `StartCronEvent`
- `Enrich`, `Set`, `Merge`, `Delete`
- `Finish`, `FinishError`, `Emit`, `Flush`, `Shutdown`
- `CustomSchema`, `DuplicateFieldPolicy`, `Sampler`, `Redactor`, `StatsHandler`

For cross-language stable-v1 parity, treat `docs/sdk-parity-manifest.json` as the authoritative shared API surface. Go may still expose language-specific helpers outside that manifest, but those helpers are not part of the cross-language stable-v1 promise unless the manifest is updated.

## Package Boundaries

- `github.com/astraive/loza/sdks/go`: lifecycle, attrs, config, schema, sampler, security, core sinks.
- `github.com/astraive/loza/sdks/go/middleware/*`: HTTP/RPC adapters.
- `github.com/astraive/loza/sdks/go/integrations/*`: slog/zap/zerolog/otel bridges.
- `github.com/astraive/loza/sdks/go/sinks/httpbatch`: collector HTTP batch transport.
- `github.com/astraive/loza/sdks/go/testkit`: capture/assert helpers for tests.

Repository boundaries:

- `spec/`: protocol, schemas, compatibility rules
- `collector/`: ingest server, worker, durability, fanout, deployment, and heavy sinks
- `cli/`: operator and developer CLI

## Migration Pattern

1. Keep existing `slog`/`zap`/`zerolog`.
2. Add LOZA lifecycle around business operations.
3. Emit one canonical event per operation for analytics and support workflows.

## Examples

- [examples/nethttp/README.md](examples/nethttp/README.md)
- [examples/slog-bridge/README.md](examples/slog-bridge/README.md)
- [examples/custom-schema/README.md](examples/custom-schema/README.md)
- [examples/httpbatch-to-collector/README.md](examples/httpbatch-to-collector/README.md)

Heavy production sinks such as Kafka, ClickHouse, Postgres, DuckDB, OTLP, S3, GCS, and Loki are collector-owned. Applications should emit to the collector using the SDK HTTP batch sink.

## Documentation

- [Instrumentation Guide](docs/business-instrumentation.md) — instrumenting checkout, payments, auth, jobs, queues, cron
- [docs/sdk.md](docs/sdk.md)
- [docs/public-api.md](docs/public-api.md)
- [docs/event-lifecycle.md](docs/event-lifecycle.md)
- [docs/migration.md](docs/migration.md)
- `../../spec/docs/SDK_CONFORMANCE_CONTRACT.md`
- `../../spec/docs/SDK_CONFORMANCE_TEST_SUITE.md`

## Breaking Changes in Current Refactor

- Root testing helpers moved to `github.com/astraive/loza/sdks/go/testkit`.
- Root `net/http` middleware wrapper removed; use `github.com/astraive/loza/sdks/go/middleware/nethttp`.

## Current Focus

- SDK lifecycle and canonical event emission for Go applications
- app-side middleware, integrations, and sinks
- compatibility with the shared LOZA spec and public collector ingest API

## SDK Helpers

- shutdown helpers:
  - `loza.ShutdownTimeout(10 * time.Second)`
  - `loza.MustShutdown(10 * time.Second)`
- config ergonomics:
  - `ApplyConfig(...)`
  - `WithAsyncQueue`
  - `WithWorkers`
  - `WithAsyncFlushInterval`
  - `WithAsyncMaxBatchBytes`
  - `WithBackpressure`
  - `WithDuplicatePolicy`
  - `WithStrict`
- HTTP context propagation:
  - `RequestIDFromHTTP`
  - `TraceFromOTel`
  - `InjectHTTPHeaders`
  - `InjectHTTPHeaderCarrier`
  - `ExtractHTTPHeaders`
  - `ExtractHTTPHeaderAttrs`

`WithStrict(true)` enables stronger runtime checks for missing service/event, invalid attr keys, canonical key collisions from custom attrs, and unsupported custom values.
It also enables strict config validation (`Config.Validate` / `New` / `Configure`) for required service identity and async/security bounds.
