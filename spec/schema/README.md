# schema/ — Compatibility Mirror

This folder is a **read-only compatibility mirror** of the canonical event schemas.

## Canonical Source
The authoritative schemas live in:
```
spec/schemas/json/
```

## Why This Mirror Exists
For backwards compatibility with code and systems that reference schemas from this old location. New code should import from `spec/schemas/json/`.

## Do Not Edit Directly
Changes to this folder will be overwritten by:
```bash
python scripts/check_mirrors.py
```

If you need to update schemas, edit the canonical sources under `spec/schemas/json/`, then the mirror will be synchronized by CI.

## File Mapping
- `schema/event.schema.json` ← `spec/schemas/json/event.schema.json`
- `schema/event.strict.schema.json` ← `spec/schemas/json/event.strict.schema.json`
- `schema/event.loose.schema.json` ← `spec/schemas/json/event.loose.schema.json`
- `schema/ingest.schema.json` ← `spec/schemas/json/ingest-envelope.schema.json`
- `schema/collector-response.schema.json` ← `spec/schemas/json/collector-response.schema.json`
