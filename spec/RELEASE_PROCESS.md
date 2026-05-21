# Release Process

This document describes how to create, verify, and publish releases of the LOXA specification.

## Release Lifecycle

```
Specification Development
        ↓
CI Verification (generation, mirrors, conformance)
        ↓
Manual QA & Testing
        ↓
CHANGELOG Update
        ↓
Version Bump
        ↓
Tag & Release
        ↓
Snapshot Frozen (immutable)
```

## Pre-Release Checklist

Before creating a release, ensure:

- [ ] All specs under `spec/` and `spec/cortex/` are finalized
- [ ] All conformance fixtures pass validation
- [ ] `codegen/generate.py --check` passes
- [ ] `scripts/check_mirrors.py` passes
- [ ] All SDKs are tested against new contracts
- [ ] Breaking changes (if any) are documented

## Creating a Release

### Step 1: Update Version in Code

Edit `codegen/model.py`:

```python
SPEC_VERSION = "1.1.0"       # Updated
EVENT_VERSION = "1.1.0"      # Updated
API_VERSION = "v1"           # Usually unchanged unless major version
```

### Step 2: Update CHANGELOG.md

Add new section at the top:

```markdown
# Changelog

## [1.1.0] - 2024-01-20

### Added
- New field `trace_context` in event schema
- Cortex conformance fixtures for graph reconstruction

### Fixed
- Strict validation now rejects event_type aliases before normalization
- Ingest envelope validation improved

### Deprecated
- Legacy `schema/` folder paths (use `spec/schemas/json/`)

### Changed
- Increased `max_event_size_bytes` from 1000 to 65536

[1.0.0] - 2024-01-01
```

### Step 3: Generate Release Artifacts

```bash
cd spec

# Ensure generated artifacts are up-to-date
python codegen/generate.py

# Verify everything passes checks
python codegen/generate.py --check
python scripts/check_mirrors.py
python scripts/check_conformance.py
python scripts/check_releases.py
```

### Step 4: Create Release Snapshot

Create release directory with immutable snapshot:

```bash
# For version 1.1.0
mkdir -p releases/v1.1.0

# Copy canonical specs
cp -r spec releases/v1.1.0/
cp -r spec/cortex releases/v1.1.0/

# Copy generated contracts
cp generated/contract/loxa-contract.json releases/v1.1.0/
cp generated/contract/cortex-contract.json releases/v1.1.0/

# Create release manifest
cat > releases/v1.1.0/README.md << 'EOF'
# LOXA Specification v1.1.0

Generated: [ISO timestamp]
Spec Version: 1.1.0
Event Version: 1.1.0
API Version: v1

## Contents

- `spec/` - Canonical specification sources
- `loxa-contract.json` - Generated Loxa contract
- `cortex-contract.json` - Generated Cortex contract

## Verification

To verify this release:

```bash
# Checksums
sha256sum -c manifest.sha256

# Contract validation
python -c "import json; json.load(open('loxa-contract.json'))"
python -c "import json; json.load(open('cortex-contract.json'))"
```

## Deprecations

- Legacy `schema/`, `openapi/`, `proto/` folders (still supported in v1.x)
- Path-based imports (use contract-driven approach)

See [MIGRATION_GUIDE.md](../../MIGRATION_GUIDE.md) for details.
EOF
```

### Step 5: Generate Checksums

```bash
cd releases/v1.1.0

# Generate checksums for verification
sha256sum loxa-contract.json cortex-contract.json > manifest.sha256

# Include checksums in README
echo "
## Integrity Verification

```bash
sha256sum -c manifest.sha256
```

Checksums:
" >> README.md

cat manifest.sha256 >> README.md
```

### Step 6: Commit & Tag

```bash
# Commit changes
git add codegen/model.py CHANGELOG.md releases/v1.1.0/
git commit -m "release: loxa-spec v1.1.0

Features:
- New trace_context field in event schema
- Cortex graph reconstruction conformance fixtures

Fixes:
- Strict validation now rejects event_type aliases
- Ingest envelope validation improved

Changes:
- Increased max_event_size_bytes to 65536 bytes

See releases/v1.1.0/ for immutable snapshot.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"

# Create annotated tag
git tag -a v1.1.0 -m "LOXA Spec v1.1.0

Spec Version: 1.1.0
Event Version: 1.1.0

See CHANGELOG.md and releases/v1.1.0/ for details."

# Push
git push origin main v1.1.0
```

### Step 7: Release on GitHub

Go to GitHub → Releases → Create Release:

1. Tag: `v1.1.0`
2. Title: `LOXA Spec v1.1.0`
3. Description:

```markdown
## LOXA Specification v1.1.0

See [CHANGELOG.md](CHANGELOG.md) and [releases/v1.1.0/](releases/v1.1.0/) for full details.

### Key Changes

- New field `trace_context` in event schema
- Cortex graph reconstruction conformance fixtures
- Increased `max_event_size_bytes` to 65536

### Migration

See [MIGRATION_GUIDE.md](MIGRATION_GUIDE.md) for path updates if using legacy locations.

### Files

- `loxa-contract.json` - Authoritative contract
- `cortex-contract.json` - Cortex contract
- Full snapshot in `releases/v1.1.0/`
```

4. Mark as "Latest release" if applicable
5. Publish

## Release Validation

After release, verify:

```bash
# Verify tag exists
git tag -v v1.1.0

# Verify snapshot immutability
ls -la releases/v1.1.0/

# Verify contracts are valid JSON
python -c "import json; json.load(open('releases/v1.1.0/loxa-contract.json'))"
python -c "import json; json.load(open('releases/v1.1.0/cortex-contract.json'))"

# Verify checksums
cd releases/v1.1.0/ && sha256sum -c manifest.sha256
```

## After Release

### Notify SDKs

Update each SDK to consume new release:

```bash
# For loxa-go
go get github.com/astraive/loxa-spec@v1.1.0

# For loxa-py
pip install loxa-spec==1.1.0

# For loxa-rs
cargo update loxa-spec
```

### Update Documentation

- Update SDK READMEs with new spec version
- Update client library release notes
- Announce breaking changes (if any)

### Monitor for Issues

- Watch GitHub issues for compatibility problems
- Be ready to patch with v1.1.1 if needed
- Document any errata in CHANGELOG.md

## Minor Release (e.g., 1.1.1)

For patch releases with critical fixes only:

1. Make minimal changes to `spec/`
2. Update `codegen/model.py` with patch version
3. Update CHANGELOG.md with "Fixed" section only
4. Create `releases/v1.1.1/` snapshot
5. Tag, release, notify SDKs

**Note**: Patch releases should not add new fields or break compatibility.

## Major Release (e.g., 2.0.0)

Major releases allow breaking changes:

1. Update `codegen/model.py` with major version bump
2. Document breaking changes in CHANGELOG.md
3. Add migration section in MIGRATION_GUIDE.md
4. Create `releases/v2.0.0/` snapshot
5. Remove deprecated paths and features
6. Tag, release, notify SDKs with migration timeline

**Minimum migration period**: 90 days overlap with v1.x still being accepted by collectors.

## Rollback (Emergency Only)

If a release has critical issues:

```bash
# Mark current release as deprecated in releases/
echo "⚠️ DEPRECATED: Use v1.1.1 or later" > releases/v1.1.0/DEPRECATED

# Fix the issue
# ... make changes ...

# Release patch version immediately
git tag v1.1.1
git push origin v1.1.1

# Document rollback in CHANGELOG.md
```

## Release Protection in CI

The CI pipeline (`.github/workflows/ci.yml`) protects releases:

```yaml
release-protection:
  # Prevents accidental changes to releases/ folder
  # Requires explicit PR description and maintainer approval
```

To modify releases (emergency only):

1. Create PR with clear justification
2. Update CHANGELOG.md in same PR
3. Add `[release-override]` to commit message
4. Request approval from maintainers

## Questions?

- **Version bumping?** See [semantic versioning](https://semver.org/)
- **Compatibility?** See [compatibility.md](compatibility.md)
- **Contract format?** See `generated/contract/loxa-contract.json`
- **Issues?** File an issue with `[release]` tag
