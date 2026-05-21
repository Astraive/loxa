# Tests

Cross-component and integration tests for the LOXA monorepo.

## Test Categories

| Category | Description | Location |
|----------|-------------|----------|
| Unit Tests | Per-component unit tests | Each component's own test directory |
| Conformance | Cross-SDK behavioral tests | `conformance/` |
| Cross-SDK Equivalence | Same events from all SDKs match | `tests/cross-sdk-equivalence/` |
| Integration Smoke | End-to-end collector + SDK | `tests/integration/` |
| Benchmarks | Performance benchmarks | `bench/` |

## Running Tests

```bash
# Component tests
cd collector && go test ./...
cd sdks/go && go test ./...
cd sdks/py && python -m pytest
cd sdks/rs && cargo test
cd sdks/js && npm test

# Conformance
./conformance/run-all.sh

# Cross-SDK equivalence
python tests/cross-sdk-equivalence/test_equivalence.py

# Integration smoke
./tests/integration/collector-sdk-smoke.sh
```
