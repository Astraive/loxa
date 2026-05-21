# LOXA-RS

**Status**: 🟢 **STABLE** (v1.0.0) - Production-ready, full feature conformance

Full API conformance with specification is complete. See [SDK_CONFORMANCE_CONTRACT.md](../loxa-spec/docs/SDK_CONFORMANCE_CONTRACT.md) for detailed guarantees.

Stable-v1 parity and release gates are tracked through:

- [SDK_CONFORMANCE_TEST_SUITE.md](../loxa-spec/docs/SDK_CONFORMANCE_TEST_SUITE.md)
- [SDK_COMPLETION_MATRIX.md](../loxa-spec/docs/SDK_COMPLETION_MATRIX.md)

`loxa-rs` is a collector-first Rust SDK for wide events.

It provides:

- event lifecycle primitives
- canonical JSON encoding
- local sinks for stdout/file/tests
- collector delivery through `SinkConfig::HttpBatch`
- middleware modules for `actix`, `axum`, `hyper`, and `tower`
- integrations for `log`, `tracing`, and `otel`

## Quick Start

```rust
use loxa::{Config, HttpBatchSink, Logger, Params};

fn main() {
    let logger = Logger::new(
        Config::production("checkout")
            .with_sink(HttpBatchSink("http://127.0.0.1:9090/v1/events")),
    );

    let mut ctx = logger.start_event(Params::new("checkout.request").with_kind("http"));
    logger.finish(&mut ctx, "success").unwrap();
    logger.emit(&ctx).unwrap();
}
```

If your collector requires an API key, set:

```bash
export LOXA_COLLECTOR_API_KEY=your-key
export LOXA_COLLECTOR_API_KEY_HEADER=X-API-Key
```

## Scope

This crate emits events to the collector. Heavy delivery and storage remain collector-owned:

- Kafka
- OTLP fanout
- ClickHouse/Postgres/DuckDB ownership
- S3/GCS archival

## Examples

- `examples/basic/main.rs`
- `examples/custom-schema/main.rs`
- `examples/httpbatch-to-collector/main.rs`
- `examples/nethttp/main.rs`

## Run Tests

```bash
cargo test
python ../loxa-spec/conformance/runner.py --sdk rust --group all
```
