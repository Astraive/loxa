# Release Process

This document describes how to publish a new release of the LOXA Rust SDK.

## Prerequisites

- crates.io account with publish access to the `loxa` crate.
- Rust toolchain installed (stable channel).
- All tests passing: `cargo test --all`.
- Clippy clean: `cargo clippy -- -D warnings`.
- Changelog updated in `CHANGELOG.md`.

## Steps

### 1. Update the Changelog

Edit `CHANGELOG.md` at the SDK root. Add a new section for the release version with the date and a summary of changes.

### 2. Update Version in Cargo.toml

Edit `Cargo.toml` to set the new version:

```toml
[package]
name = "loxa"
version = "1.0.0"
```

### 3. Verify the Build

```bash
cargo build --release
cargo test --all
cargo clippy -- -D warnings
cargo doc --no-deps
```

### 4. Publish to crates.io

```bash
cargo publish --dry-run  # verify packaging
cargo publish             # publish to crates.io
```

### 5. Tag the Release

Create a Git tag following the `rs/vX.Y.Z` convention:

```bash
git tag -a rs/v1.0.0 -m "loxa-rs v1.0.0"
git push origin rs/v1.0.0
```

### 6. Create a GitHub Release

```bash
gh release create rs/v1.0.0 --title "loxa-rs v1.0.0" --notes-file release-notes.md
```

## Version Policy

The Rust SDK follows Semantic Versioning:

- **Major** (v1.0.0): Breaking changes to the public API.
- **Minor** (v1.0.0): New features, backward-compatible.
- **Patch** (v1.0.0): Bug fixes, backward-compatible.

## crates.io Notes

- Published crates are permanent. Do not yank versions unless absolutely necessary.
- If a release has issues, publish a new patch version instead of yanking.
- The `Cargo.lock` file is not published; consumers resolve their own dependencies.

## Checklist

- [ ] All tests pass (`cargo test --all`)
- [ ] Clippy clean (`cargo clippy -- -D warnings`)
- [ ] Docs build (`cargo doc --no-deps`)
- [ ] Changelog updated
- [ ] Version bumped in `Cargo.toml`
- [ ] Published to crates.io
- [ ] Tag pushed with `rs/vX.Y.Z` format
- [ ] GitHub Release created
