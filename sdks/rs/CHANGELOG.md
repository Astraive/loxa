# Changelog

All notable changes to the LOXA Rust SDK are documented in this file.

## [0.0.2] - 2026-05-20

### Added

- Initial release of the LOXA Rust SDK.
- Core event lifecycle: `start_event` -> `enrich` -> `finish` -> `emit`.
- Logger with async/sync delivery via configurable sinks.
- `StdoutSink`, `StderrSink`, `FileSink`, `MemorySink`, `NoopSink`, `HTTPBatchSink`.
- 14-key safety-net redactor (`DefaultRedactor`).
- Full sampler suite: `SampleAll`, `SampleNone`, `SampleRandom`, `SampleErrors`, `SampleSlowRequests`, `SampleStatusCodes`, `SampleRoutes`, `SampleUsers`, `SampleTenants`, `SampleFeatureFlag`, `SampleByHeader`.
- Sampler combinators: `AnySampler`, `AllSampler`, `NotSampler`.
- Schema support: `DefaultSchema`, `FlatSchema`, `NestedSchema`, `ECSchema`, `OTelSchema`, `DatadogSchema`, `CustomSchema`.
- Redaction: `RedactKeys`, `HashKeys`, `DropKeys`, `MaskKeys`, `RedactPatterns`, `ComposeRedactors`.
- Attribute constructors: `String`, `Int`, `Int64`, `Uint64`, `Float64`, `Bool`, `Time`, `Duration`, `Any`, `Null`, `Group`, `SensitiveString`, `HashString`, `MarkSensitive`.
- Canonical attribute helpers: `UserID`, `TenantID`, `RequestID`, `TraceID`, `SpanID`, and 20+ domain-specific helpers.
- `MetricsCollector` and Prometheus rendering.
- `CortexClient` for Persistent Context Engine integration.
- Tower middleware for HTTP request capture.
- Spec contract from `spec/`.
- Both PascalCase (Go-style) and snake_case (Rust-idiomatic) public API.
