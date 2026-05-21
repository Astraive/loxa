# Conformance

Central conformance testing for the LOXA monorepo. Validates that all SDKs and components conform to the shared specification.

## Quick Start

```bash
./conformance/run-all.sh              # Run all conformance tests
./conformance/run-sdk.sh --sdk go     # Run Go SDK conformance only
./conformance/run-collector.sh        # Run collector sink conformance only
```

## What is Tested

### Cross-SDK Conformance (12 groups x 4 SDKs = 48 checks)

| Group | Description |
|-------|-------------|
| state_machine | Event lifecycle state transitions |
| canonical_fields | Required fields always present and immutable |
| duplicate_policy | Canonical fields silently dropped from attrs |
| sampling | All sampler types produce correct filtering |
| delivery_semantics | Emit returns valid JSON, idempotent behavior |
| panic_error_safety | Error paths do not panic |
| config_precedence | Test/Dev/Production presets work correctly |
| metrics | Emission counters increment |
| golden_fixtures | Golden event fixtures validate against schema |
| collector_integration | Events accepted by collector |
| cortex_emitted_shape | Emitted events match expected shape |
| parity | All public APIs exported |

### Collector Sink Conformance

Reusable test suite for all sink implementations:
- open/close lifecycle
- single event write
- batch serial write
- flush with timeout
- large payload handling
- context cancellation

### Comprehensive Python Verifier

105 subchecks across 12 categories (Python SDK only):
- Lifecycle, Fields, Wire Format, Sampling, Redaction, Delivery, Config, Schemas, Cortex, Collector, Timing, Parity

## Results

Results are saved to `results/<suite>-<timestamp>.json`.
