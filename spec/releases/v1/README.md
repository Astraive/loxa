# v1/ — Release Snapshot (Immutable)

This folder contains the **immutable snapshot of Loxa v1**.

## Do Not Edit This Folder
The v1 release is frozen. Any changes require:
1. Explicit version bump (e.g., to v1.1, v2.0)
2. CHANGELOG entry documenting changes
3. Full conformance test suite passing
4. Explicit approval in the commit message

## Canonical Location
Active development happens under:
```
spec/loxa/
spec/cortex/
```

## Why This Snapshot Exists
- **Immutability guarantee**: SDKs and services can safely pin to v1 knowing it won't change
- **Reproducibility**: Consumers can verify their version matches exactly
- **Release hygiene**: Prevents accidental mutations to released code

## Migration Path
If you're still using v1 directly, plan to migrate to:
- `spec/loxa/` for the latest Loxa specification
- `spec/cortex/` for the latest Cortex specification

See `CHANGELOG.md` and `compatibility.md` for detailed migration guidance.

## Version Bumping
To create a new release:
```bash
# Update version in codegen/model.py
# Verify all CI checks pass
# Create new releases/v{N}/ folder
# Update CHANGELOG.md with release notes
# Tag and release
```
