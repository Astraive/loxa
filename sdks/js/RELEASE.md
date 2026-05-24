# Release Process

This document describes how to publish a new release of the LOXA JS SDK (loxa-js).

## Prerequisites

- npm account with publish access to the `loxa-js` package.
- Node.js 18+ installed locally.
- All tests passing: `npm test`.
- TypeScript compiles clean: `npm run lint`.
- Changelog updated in `CHANGELOG.md`.

## Steps

### 1. Update the Changelog

Edit `CHANGELOG.md` at the SDK root. Add a new section for the release version with the date and a summary of changes.

### 2. Update Version in package.json

Edit `package.json` to set the new version:

```json
{
  "version": "0.0.2"
}
```

Or use npm version:

```bash
npm version patch  # 0.0.2 -> 1.0.1
npm version minor  # 0.0.2 -> 1.1.0
npm version major  # 0.0.2 -> 1.1.0
```

### 3. Verify the Build

```bash
npm run build
npm test
npm run lint
```

### 4. Publish to npm

```bash
npm pack --dry-run  # verify packaging
npm publish           # publish to npm registry
```

If this is the first publish or a scoped package:

```bash
npm publish --access public
```

### 5. Tag the Release

Create a Git tag following the `js/vX.Y.Z` convention:

```bash
git tag -a js/v0.0.2 -m "loxa-js v0.0.2"
git push origin js/v0.0.2
```

### 6. Create a GitHub Release

```bash
gh release create js/v0.0.2 --title "loxa-js v0.0.2" --notes-file release-notes.md
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

- [ ] All tests pass (`npm test`)
- [ ] TypeScript compiles (`npm run lint`)
- [ ] Build succeeds (`npm run build`)
- [ ] Changelog updated
- [ ] Version bumped in `package.json`
- [ ] Published to npm
- [ ] Tag pushed with `js/vX.Y.Z` format
- [ ] GitHub Release created
