# golden/ — Compatibility Mirror

This folder is a **read-only compatibility mirror** of conformance fixtures and test cases.

## Canonical Source
The authoritative conformance fixtures live in:
```
fixtures/loxa/valid/
fixtures/loxa/invalid/
fixtures/cortex/valid/
fixtures/cortex/invalid/
```

## Why This Mirror Exists
For backwards compatibility with older test suites that reference fixtures from this location. New tests should import from `fixtures/`.

## Do Not Edit Directly
Changes to this folder should be kept in sync with the canonical fixtures directory. If you need to add or modify fixtures, update the canonical sources under `fixtures/`, not here.

## File Mapping
- Files in this folder correspond to subset of fixtures used for testing conformance.
- For new fixtures, add them to `fixtures/loxa/` or `fixtures/cortex/` instead.
