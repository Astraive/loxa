# Cortex Conformance

The cortex conformance suite validates that a Cortex deployment implements the PCE pipeline correctly and produces expected outputs for known inputs.

## What it Tests

- **Event ingestion** -- single, batch, and NDJSON ingestion paths
- **Reconstruction** -- fast and deep reconstruction modes produce correct causal chains
- **Signature matching** -- similar incident lookup returns ranked results
- **Remediation feedback** -- feedback recording and learner weight updates
- **Graph queries** -- service graph and incident graph traversal
- **API contract** -- HTTP status codes, response schemas, error handling

## Running

```bash
cd cortex
go test ./conformance/... -v
```

## Adding Tests

Conformance tests live in the `conformance/` directory. Each test file covers a specific API surface. Tests use the in-memory storage backend for isolation.
