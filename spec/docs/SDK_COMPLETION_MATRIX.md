# SDK Completion Matrix

This matrix is the authoritative stable-v1 completion bar for the LOXA emission SDKs:

- `loxa-go`
- `loxa-py`
- `loxa-rs`
- `loxa-js`

The scope is intentionally **collector-first**. SDKs emit canonical events and deliver them to the collector over the documented HTTP ingest contract. Heavy sinks and storage remain collector-owned.

## Stable-v1 Scope

| Capability | Go | Python | Rust | JavaScript | Verification |
|------------|----|--------|------|------------|--------------|
| Lifecycle: `Start` / `Enrich` / `Finish` / `Emit` | Required | Required | Required | Required | Conformance `state_machine` |
| Flush / shutdown semantics | Required | Required | Required | Required | Conformance `delivery_semantics` |
| Canonical field ownership | Required | Required | Required | Required | Conformance `canonical_fields` |
| Duplicate field policy | Required | Required | Required | Required | Conformance `duplicate_policy` |
| Sampling hooks | Required | Required | Required | Required | Conformance `sampling` |
| Redaction/stats hooks | Required | Required | Required | Required | Conformance `panic_error_safety`, `metrics` |
| Collector HTTP batch sink | Required | Required | Required | Required | Conformance `delivery_semantics` |
| Config/env precedence | Required | Required | Required | Required | Conformance `config_precedence` |
| Shared golden fixtures | Required | Required | Required | Required | Conformance `golden_fixtures` |
| Collector integration | Required | Required | Required | Required | Conformance `collector_integration` |
| Cortex-consumable emitted shape | Required | Required | Required | Required | Conformance `cortex_emitted_shape` |
| Stable public API parity vs superset manifest | Required | Required | Required | Required | Conformance `parity` |
| `CreateLoxa()` factory | N/A (uses `New`) | Required | Required | Required | Conformance `parity` |
| `Alias("name")` sugar | Required | Required | Required | Required | Conformance `parity` |

## Excluded From SDK Scope

These remain collector-owned and are not part of stable-v1 SDK parity:

- Kafka
- DuckDB
- ClickHouse
- Postgres
- Loki
- OTLP fanout
- S3
- GCS

## Public Surface Policy

- `spec/docs/sdk-parity-manifest.json` defines the cross-language stable-v1 API surface.
- Go, Python, Rust, and JavaScript must match the stable-v1 superset manifest for parity-gated APIs.
- Language-specific helpers outside that manifest are allowed, but they are not part of the cross-language stable-v1 promise unless added to the manifest and documented here.

## Release Interpretation

All four SDKs are considered **stable-v1** only when:

- all grouped conformance checks pass
- shared fixture behavior matches
- collector delivery semantics match
- status/docs/CI all reflect the same stable-v1 scope
