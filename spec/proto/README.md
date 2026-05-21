# proto/ — Compatibility Mirror

This folder is a **read-only compatibility mirror** of the canonical Protocol Buffer definitions.

## Canonical Source
The authoritative proto definitions live in:
```
spec/proto/
spec/cortex/proto/
```

## Why This Mirror Exists
For backwards compatibility with code and systems that reference proto definitions from this old location. New code should import from `spec/proto/` or `spec/cortex/proto/`.

## Do Not Edit Directly
Changes to this folder will be overwritten by:
```bash
python scripts/check_mirrors.py
```

If you need to update proto definitions, edit the canonical sources under `spec/proto/` or `spec/cortex/proto/`, then the mirror will be synchronized by CI.

## Migration Path
Update your `buf.yaml` or proto imports to reference:
- `spec/proto/loxa/v1/` for Loxa proto definitions
- `spec/cortex/proto/loxa/v1/` for Cortex proto definitions
