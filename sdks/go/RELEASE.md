# Release Process

This document describes how to publish a new release of the LOXA Go SDK.

## Prerequisites

- Push access to the `astraive/loxa` repository.
- Go 1.25+ installed locally.
- All tests passing: `go test ./... -race`.
- Changelog updated in `CHANGELOG.md`.

## Steps

### 1. Update the Changelog

Edit `CHANGELOG.md` at the repository root. Add a new section for the release version with the date and a summary of changes. Follow the existing format:

```
## [X.Y.Z] - YYYY-MM-DD

### Added
- ...

### Changed
- ...

### Breaking
- ...
```

### 2. Tag the Release

Create a Git tag following the `go/vX.Y.Z` convention required by Go modules:

```bash
git tag -a sdks/go/v1.0.0 -m "Go SDK v1.0.0"
git push origin sdks/go/v1.0.0
```

The `go/` prefix is required because the Go SDK lives in a subdirectory of a monorepo. The Go module proxy at `proxy.golang.org` will pick up the new tag automatically.

### 3. Verify the Module Proxy

After pushing the tag, verify that the module proxy has indexed the new version:

```bash
GOPROXY=https://proxy.golang.org go list -m github.com/Astraive/loxa/sdks/go@v1.0.0
```

If the proxy has not yet indexed the version, wait a few minutes and retry. You can also force a re-fetch:

```bash
curl https://proxy.golang.org/github.com/Astraive/loxa/sdks/go/@v/v1.0.0.info
```

### 4. Verify Downstream Consumers

Confirm that the new version resolves cleanly in a fresh module:

```bash
mkdir /tmp/loxa-verify && cd /tmp/loxa-verify
go mod init verify
go get github.com/Astraive/loxa/sdks/go@v1.0.0
go build ./...
```

### 5. Tag Submodules (if applicable)

If middleware or integration submodules also changed, tag them independently:

```bash
git tag -a sdks/go/src/middleware/v1.0.0 -m "Go SDK middleware v1.0.0"
git push origin sdks/go/src/middleware/v1.0.0
```

### 6. Create a GitHub Release

Create a GitHub Release from the tag with release notes copied from the changelog:

```bash
gh release create sdks/go/v1.0.0 --title "Go SDK v1.0.0" --notes-file release-notes.md
```

## Version Policy

The Go SDK follows Semantic Versioning:

- **Major** (v1.0.0): Breaking changes to the public API.
- **Minor** (v1.0.0): New features, backward-compatible.
- **Patch** (v1.0.0): Bug fixes, backward-compatible.

## Go Module Proxy Notes

- The module proxy caches versions permanently. Do not delete or re-tag versions.
- If a tag is pushed with errors, increment to the next patch version instead of re-tagging.
- The `+incompatible` suffix is not expected for this module since it uses Go module layout.

## Checklist

- [ ] All tests pass (`go test ./... -race`)
- [ ] `go vet` clean
- [ ] Changelog updated
- [ ] Tag pushed with `go/vX.Y.Z` format
- [ ] Module proxy verified
- [ ] GitHub Release created
- [ ] Submodules tagged (if changed)
