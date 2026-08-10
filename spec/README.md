# LOZA Spec

LOZA Spec defines the shared contract between SDKs and collectors.

- `sdks/go`, `sdks/py`, `sdks/rs`, and `sdks/js` emit LOZA events that follow this spec.
- `collector` accepts LOZA events that follow this spec.
- This directory is the source of truth for schemas, ingest payload formats, compatibility rules, and wire-level examples.

## Current Versions

- **Schema Version**: `v0.0.1`
- **Ingest API Version**: `v1`
- **Status**: Active, stable-v1 emitter contract

## Stable-v1 SDK Contract

For the stable emitter SDK scope, use these documents first:

- [docs/SDK_CONFORMANCE_CONTRACT.md](docs/SDK_CONFORMANCE_CONTRACT.md)
- [docs/SDK_CONFORMANCE_TEST_SUITE.md](docs/SDK_CONFORMANCE_TEST_SUITE.md)
- [docs/SDK_COMPLETION_MATRIX.md](docs/SDK_COMPLETION_MATRIX.md)
- [docs/SDK_RELEASE_CHECKLIST.md](docs/SDK_RELEASE_CHECKLIST.md)

The SDK scope is collector-first. Kafka, DuckDB, ClickHouse, Postgres, Loki, OTLP fanout, S3, and GCS remain collector-owned rather than SDK parity requirements.

## Repository Structure (Canonical)

The specification is now organized under `spec/` with deterministic contract generation:

```
spec/                          # ← Canonical sources
│   ├── schemas/json/            # LOZA JSON schemas
│   ├── openapi/                 # LOZA OpenAPI specs
│   ├── proto/                   # LOZA protocol buffers
│   ├── docs/                    # LOZA documentation
│   ├── cortex/                  # Cortex specification
│   │   ├── CORTEX_*.md          # Documentation
│   │   ├── schemas/json/        # JSON schemas
│   │   ├── openapi/             # OpenAPI specs
│   │   └── proto/               # Protocol buffers
│   │
│   └── shared/                  # Shared definitions
│
├── generated/                   # ← Generated artifacts
│   ├── contract/
│   │   ├── loza-contract.json   # Authoritative contract
│   │   └── cortex-contract.json # Authoritative contract
│   ├── go/
│   ├── python/
│   └── rust/
│
├── fixtures/                    # Conformance fixtures
│   ├── valid/
│   ├── invalid/
│   └── cortex/{valid,invalid}/
│
├── schema/                      # [Deprecated] Compatibility mirror
├── openapi/                     # [Deprecated] Compatibility mirror
├── proto/                       # [Deprecated] Compatibility mirror
├── v1/                          # [Deprecated] Version 1 archive
├── examples/                    # Additional examples
├── docs/                        # Documentation
├── CHANGELOG.md                 # Version history
├── compatibility.md             # Compatibility guide
└── README.md                    # This file
```

**See [MIGRATION_GUIDE.md](MIGRATION_GUIDE.md) for path updates if using legacy locations.**

## Quick Start

### Validate an Event

```bash
# Validate against v1 schema
loza schema validate --file event.json --schema-version v1

# Validate from stdin
echo '{"event_id":"...","event_type":"user.login",...}' | loza schema validate
```

### View Schema Versions

```bash
# List all available schema versions
loza schema list

# Show differences between versions
loza schema diff --from v0.0.1 --to v0.0.1
```

## Schema Overview

### Required Fields (v1)

All LOZA events must include:

- `event_id` (string, UUID): Unique event identifier
- `event_type` (string): Event type in dot notation (e.g., `user.login`)
- `timestamp` (string, ISO 8601): Event timestamp
- `service` (object): Service identity with `name` and `version`
- `deployment` (object): Deployment context with `environment`

### Example Event

```json
{
  "event_id": "01934a5e-8f2c-7b3d-9e4f-5a6b7c8d9e0f",
  "event_type": "user.login",
  "timestamp": "2024-01-15T10:30:00.123Z",
  "service": {
    "name": "auth-service",
    "version": "2.1.0"
  },
  "deployment": {
    "environment": "production",
    "region": "us-east-1"
  },
  "data": {
    "login_method": "password",
    "success": true
  }
}
```

See [v1/examples/](v1/examples/) for complete examples.

## Compatibility Promise

- **Backward Compatible**: Minor versions (v1.x) are backward compatible
- **Forward Compatible**: Collectors accept events with unknown optional fields
- **Breaking Changes**: Require major version increment (v1 → v2)
- **Migration Period**: 90 days minimum overlap for major versions

See [compatibility.md](compatibility.md) for details.

## Generated Contracts

The specification generates authoritative contracts for both Loza and Cortex:

### Loza Contract
```json
{
  "spec_version": "0.0.2",
  "event_version": "0.0.2",
  "api_version": "v1",
  "kinds": ["event"],
  "levels": ["debug", "info", "warn", "error"],
  "limits": {
    "max_event_size_bytes": 65536,
    "max_batch_events": 100,
    "max_batch_size_bytes": 10485760
  },
  "schema_paths": {
    "event": "spec/schemas/json/event.schema.json",
    ...
  },
  "validation_modes": {
    "strict": { ... },
    "loose": { ... }
  }
}
```

Generated at: `generated/contract/loza-contract.json`

### Cortex Contract
```json
{
  "spec_version": "0.0.2",
  "api_version": "v1",
  "kinds": ["event", "metric", "log", ...],
  "provenance_types": ["loza", "collector", "otlp", "jsonl", "manual", "replay"],
  "graph_node_types": ["service", "event", "trace", "span", ...],
  "graph_edge_types": ["depends_on", "same_trace", "parent_span", ...],
  "routes": {
    "/cortex.CortexService/IngestEvent": { ... },
    ...
  }
}
```

Generated at: `generated/contract/cortex-contract.json`

### Using Generated Contracts

SDKs and services should consume generated contracts instead of duplicating constants:

```python
import json
from pathlib import Path

contract = json.loads(Path("generated/contract/loza-contract.json").read_text())

# Access contract metadata
max_size = contract["limits"]["max_event_size_bytes"]
allowed_levels = contract["levels"]
validation_schema = contract["schema_paths"]["event"]
```

See [MIGRATION_GUIDE.md](MIGRATION_GUIDE.md) for detailed integration steps.

## Key Locations

- `v1/`: Version 1 schema definitions and examples
- `schema/`: JSON Schema for canonical LOZA events (legacy)
- `openapi/`: Collector ingest API description
- `examples/`: Event and ingest payload examples
- `docs/`: Language-neutral contract and compatibility documentation
- `proto/`: Canonical protobuf source files

## Documentation

- [v1 Schema Documentation](v1/README.md)
- [Compatibility Matrix](compatibility.md)
- [Schema Evolution Policy](docs/SCHEMA_EVOLUTION_POLICY.md)
- [Event State Machine](docs/EVENT_STATE_MACHINE.md)
- [Identity Model](docs/IDENTITY_MODEL.md)
- [Privacy & Compliance](docs/PRIVACY_COMPLIANCE.md)

## Versioning

LOZA schemas follow [Semantic Versioning](https://semver.org/):

- **MAJOR** (v1 → v2): Breaking changes (field removal, type changes)
- **MINOR** (v0.0.1): Backward-compatible additions (new optional fields)
- **PATCH** (v0.0.1 → v1.0.1): Documentation updates

See [CHANGELOG.md](CHANGELOG.md) for version history.

## Contributing

1. Review [Schema Evolution Policy](docs/SCHEMA_EVOLUTION_POLICY.md)
2. Submit GitHub issue with proposed changes
3. Create PR with schema updates and tests
4. Update CHANGELOG.md and compatibility docs

## Testing

### Conformance Fixtures

Use golden test files for validation:

```bash
# Run validation checks
python scripts/check_conformance.py
```

### Schema Validation

```bash
# Validate schema syntax
loza schema validate-schema --file v1/event.schema.json

# Check backward compatibility
loza schema check-compatibility --base v0.0.1 --target v0.0.1
```

## License

See [LICENSE](../LICENSE) file.

## Contact

- GitHub Issues: https://github.com/astraive/loza/issues
- Documentation: https://github.com/astraive/loza/tree/main/spec/docs
