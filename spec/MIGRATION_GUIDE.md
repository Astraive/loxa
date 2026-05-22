# Migration Guide: Canonical Specification Paths

## Overview

The LOXA specification has been restructured to use canonical sources under `spec/` with deterministic contract generation. Legacy mirror folders (`schema/`, `openapi/`, `proto/`) will continue to work for backward compatibility but are no longer the sources of truth.

This guide helps you migrate to the new structure.

## New Directory Structure

```
spec/
│   ├── schemas/json/          # ← NEW: Canonical LOXA JSON schemas
│   ├── openapi/               # ← NEW: Canonical LOXA OpenAPI specs
│   ├── proto/                 # ← NEW: Canonical LOXA protocol buffers
│   ├── docs/                  # ← NEW: Canonical LOXA docs
│   ├── cortex/                # ← NEW: Canonical Cortex specification
│   │   ├── CORTEX_*.md        # Documentation
│   │   ├── schemas/json/      # JSON schemas
│   │   ├── openapi/           # OpenAPI specs
│   │   └── proto/             # Protocol buffers
│   │
│   └── shared/                # Shared definitions
│
├── generated/
│   ├── contract/
│   │   ├── loxa-contract.json    # ← Authoritative contract
│   │   └── cortex-contract.json  # ← Authoritative contract
│   ├── go/
│   ├── python/
│   └── rust/
│
├── schema/                  # ← OLD: Compatibility mirror (read-only)
├── openapi/                 # ← OLD: Compatibility mirror (read-only)
├── proto/                   # ← OLD: Compatibility mirror (read-only)
└── examples/golden/         # ← OLD: Compatibility mirror (read-only)
```

## Migration Paths

### For Code That Imports Schemas

**OLD** (deprecated):
```python
from schema import event_schema
import schema.event_schema

# Or reading files
with open("schema/event.schema.json") as f:
    schema = json.load(f)
```

**NEW** (recommended):
```python
# Option 1: Read canonical schema files directly
from pathlib import Path
import json

# Option 2: Import from generated contract
from generated.contract import loxa_contract

# Option 3: Read canonical paths
with open("spec/schemas/json/event.schema.json") as f:
    schema = json.load(f)
```

### For Code That Imports OpenAPI

**OLD** (deprecated):
```yaml
# Reference in code/config
openapi: $ref: openapi/collector.openapi.yaml
```

**NEW** (recommended):
```yaml
openapi: $ref: spec/openapi/collector.openapi.yaml
# Or for Cortex:
openapi: $ref: spec/cortex/openapi/cortex.openapi.yaml
```

### For Code That Imports Proto Definitions

**OLD** (deprecated):
```bash
protoc --proto_path=proto \
  proto/loxa/v1/loxa.proto
```

**NEW** (recommended):
```bash
protoc --proto_path=spec/proto \
  spec/proto/loxa/v1/event.proto

# Or for Cortex:
protoc --proto_path=spec/cortex/proto \
  spec/cortex/proto/loxa/v1/cortex.proto
```

### For Code That Validates Against Generated Contracts

**OLD** (manual schema validation):
```python
from jsonschema import validate

with open("schema/event.schema.json") as f:
    schema = json.load(f)

validate(event_data, schema)
```

**NEW** (contract-driven validation):
```python
import json
from pathlib import Path

# Load the authoritative contract
contract_path = Path("generated/contract/loxa-contract.json")
contract = json.loads(contract_path.read_text())

# Use contract.schema_paths or contract.validation_modes
# Contracts are generated from codegen/model.py
```

## For SDK Maintainers

If you maintain an SDK (`sdks/go`, `sdks/py`, `sdks/rs`, `cli/`, `collector/`):

### Phase 1: Stop Hardcoding Constants
Instead of hardcoding enums and limits:

```go
// OLD: hardcoded
const AllowedKinds = []string{"event", "metric", ...}
const MaxEventSize = 65536
```

```go
// NEW: consume generated contract
import "generated/contract"

func init() {
    contract := contract.LoadLoxa()
    allowedKinds := contract.Kinds
    maxEventSize := contract.Limits.MaxEventSizeBytes
}
```

### Phase 2: Use Canonical Schema Paths
Update configuration to point to canonical locations:

```python
# config.json
{
    "schema_base_path": "spec/schemas/json",
    "contract_path": "generated/contract/loxa-contract.json",
    "openapi_path": "spec/openapi/collector.openapi.yaml"
}
```

### Phase 3: Test Against Conformance Fixtures
Use canonical fixture locations:

```bash
# OLD: examples/golden/
pytest tests/conformance.py --fixtures examples/golden/valid/

# NEW: fixtures/
pytest tests/conformance.py --fixtures fixtures/valid/
```

## Deprecation Timeline

| Phase | When | Action |
|-------|------|--------|
| **Phase 1** | Now | New code uses `spec/` and `spec/cortex/` |
| **Phase 2** | v1.0.0 releases | Mirror folders remain for backward compat, but undocumented |
| **Phase 3** | v1.0.0 | Mirror folders removed |
| **Phase 4** | v1.0.0 | Legacy path support removed from SDKs |

## Common Questions

### Q: Will the legacy folders be maintained?
**A:** Yes, through the v1.0.0 release cycle. CI enforces via `scripts/check_mirrors.py` that mirrors stay in sync with canonical sources.

### Q: When should I migrate?
**A:** Ideally before the next major version (v1.0.0). Minor version v1.0.0 can use either path.

### Q: What if I still reference the old paths?
**A:** They'll continue to work until v1.0.0, but you'll get warnings in logs and may miss updates.

### Q: How do I consume the generated contract?
**A:** Load `generated/contract/loxa-contract.json` or `generated/contract/cortex-contract.json` at startup. These are deterministic artifacts generated from `spec/`.

### Q: Are schemas still under `schema/`?
**A:** For now (backward compat), but the canonical source is `spec/schemas/json/`. Reference the canonical path in new code.

## Validation

To verify your migration, run:

```bash
# Check that your code points to canonical paths
grep -r "schema/\|openapi/\|proto/" src/  # Should be minimal
grep -r "spec/\|spec/cortex" src/           # Should be common

# Verify contracts are being used
grep -r "generated/contract" src/           # Should see this

# Run mirror check to ensure sync
python scripts/check_mirrors.py
```

## Getting Help

- **Questions?** Check `spec/docs/` and `spec/cortex/CORTEX_*.md`
- **Contract format?** See `generated/contract/loxa-contract.json`
- **Fixtures?** See `fixtures/` and `fixtures/cortex/`
- **Issues?** File an issue with your migration path question

## Related Documentation

- [Spec README](README.md)
- [Compatibility Policy](compatibility.md)
- [LOXA docs](spec/docs/)
- [Cortex Architecture](spec/cortex/CORTEX_ARCHITECTURE.md)
