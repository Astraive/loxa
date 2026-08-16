# Release Process

This document describes how to publish a new release of the LOZA JS SDK (`loza`).

## Prerequisites

- npm registry account with publish access to the `@astraive/loza` package.
- Bun 1.3.14 installed locally.
- All tests passing: `bun run test`.
- TypeScript compiles clean: `bun run lint`.
- Changelog updated in `CHANGELOG.md`.

## Steps

### 1. Update the Changelog

Edit `CHANGELOG.md` at the SDK root. Add a new section for the release version with the date and a summary of changes.

### 2. Update Version in package.json

Edit `package.json` to set the new version:

```json
{
  "version": "0.2.0"
}
```

Or use Bun to update the version:

```bash
bun pm version patch  # 0.2.0 -> 1.0.1
bun pm version minor  # 0.2.0 -> 1.1.0
bun pm version major  # 0.2.0 -> 1.1.0
```

### 3. Verify the Build

```bash
bun run build
bun run test
bun run lint
```

### 4. Publish to npm

```bash
bun pm pack --dry-run  # verify packaging
bun publish --access public
```

### 5. Tag the Release

Create a Git tag following the `js/vX.Y.Z` convention:

```bash
git tag -a js/v0.2.0 -m "loza v0.2.0"
git push origin js/v0.2.0
```

### 6. Create a GitHub Release

```bash
gh release create js/v0.2.0 --title "loza v0.2.0" --notes-file release-notes.md
```

## Version Policy

The JS SDK follows Semantic Versioning:

- **Major** (v0.0.1): Breaking changes to the public API.
- **Minor** (v0.0.1): New features, backward-compatible.
- **Patch** (v0.0.1): Bug fixes, backward-compatible.

## npm Registry Notes

- Published versions are permanent. Do not unpublish unless absolutely necessary.
- If a release has issues, publish a new patch version instead of unpublishing.
- The `files` field in `package.json` controls what gets published (`dist/`, `README.md`).

## Checklist

- [ ] All tests pass (`bun run test`)
- [ ] TypeScript compiles (`bun run lint`)
- [ ] Build succeeds (`bun run build`)
- [ ] Changelog updated
- [ ] Version bumped in `package.json`
- [ ] Published to npm
- [ ] Tag pushed with `js/vX.Y.Z` format
- [ ] GitHub Release created
