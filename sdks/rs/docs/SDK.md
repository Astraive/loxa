# SDK

The Rust crate currently exposes:

- `Config`
- `Logger`
- `Params`
- `EventContext`
- `SinkConfig::HttpBatch`

Collector-first guidance:

1. build one event per operation
2. emit to `loza-collector` over `/events`
3. keep heavy delivery, fanout, durability, and storage concerns in the collector

Implemented Rust-facing modules today:

- middleware: `actix`, `axum`, `hyper`, `tower`
- integrations: `log`, `tracing`, `otel`
