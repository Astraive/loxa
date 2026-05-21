# openapi/ — Compatibility Mirror

This folder is a **read-only compatibility mirror** of the canonical OpenAPI specifications.

## Canonical Source
The authoritative OpenAPI specs live in:
```
spec/openapi/
spec/cortex/openapi/
```

## Why This Mirror Exists
For backwards compatibility with code and systems that reference OpenAPI specs from this old location. New code should import from `spec/openapi/` or `spec/cortex/openapi/`.

## Do Not Edit Directly
Changes to this folder will be overwritten by:
```bash
python scripts/check_mirrors.py
```

If you need to update specs, edit the canonical sources under `spec/openapi/` or `spec/cortex/openapi/`, then the mirror will be synchronized by CI.

## File Mapping
- `openapi/collector.openapi.yaml` ← `spec/openapi/collector.openapi.yaml`
- `openapi/cortex.openapi.yaml` ← `spec/cortex/openapi/cortex.openapi.yaml`
