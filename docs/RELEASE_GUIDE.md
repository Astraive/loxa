# LOZA Release Guide

LOZA is a multi-component monorepo. A component is released only when the `version` field in that component's YAML manifest increases. Editing source files, docs, or manifest metadata without increasing the version does not publish anything.

## Manifests

The release registry is `release.yaml`:

```txt
loza     -> loza.yaml
spec      -> spec/loza-spec.yaml
collector -> collector/loza.yaml
cortex    -> cortex/loza-cortex.yaml
cli       -> cli/loza-cli.yaml
sdk-go    -> sdks/go/loza-go.yaml
sdk-js    -> sdks/js/loza-js.yaml
sdk-py    -> sdks/py/loza-py.yaml
sdk-rs    -> sdks/rs/loza-rs.yaml
```

Manifest versions are plain semantic versions like `0.2.6`. Do not include a leading `v` in YAML.

Native package metadata must match the manifest:

```txt
sdks/js/package.json version == sdks/js/loza-js.yaml version
sdks/py/pyproject.toml version == sdks/py/loza-py.yaml version
sdks/rs/Cargo.toml version == sdks/rs/loza-rs.yaml version
```

For Go modules, the manifest version must match the component tag.

SDK installs are independent:

```txt
npm install @astraive/loza
pip install loza
cargo add loza
go get github.com/astraive/loza/sdks/go
```

## Pull Requests

On pull requests, `Release Detect`:

- Validates `release.yaml` and every component manifest.
- Compares manifest versions against the PR base.
- Fails if a changed version is invalid or decreased.
- Runs affected component tests.
- Runs SDK and dependent conformance checks when `spec/loza-spec.yaml` changes.

Pull requests never publish.

## Pushes To Main

On pushes to `main`, `Release Publish` compares the pushed commit with the previous `main` commit. It publishes only components whose manifest version increased.

If `spec/loza-spec.yaml` changes, CI runs spec validation, collector contract checks, CLI schema checks, and SDK conformance checks. It publishes/tags only the spec unless SDK manifests also changed.

If root `loza.yaml` changes, CI creates the umbrella GitHub Release `vX.Y.Z`. It does not publish every component package.

## Manual Publish

Run `Release Publish` with:

```txt
components: collector,sdk-js
version: 0.2.6
dry_run: false
```

The workflow validates that every selected component manifest already has that exact version. Manual dispatch does not bypass manifest validation.

## Examples

To release only the collector:

1. Change `collector/loza.yaml` from `0.2.6` to `0.2.7`.
2. Update collector changelog or release notes.
3. Push to `main`.
4. CI detects the collector version bump.
5. CI builds and publishes Docker images:
   - `astraive/loza:0.2.7`
   - `ghcr.io/astraive/loza:0.2.7`
6. CI creates `collector/v0.2.7`.

To release the JS SDK:

1. Change `sdks/js/loza-js.yaml` from `0.2.6` to `0.2.7`.
2. Ensure `sdks/js/package.json` and `sdks/js/bun.lock` also use `0.2.7`.
3. Push to `main`.
4. CI runs JS tests and builds the package.
5. CI publishes the npm package `@astraive/loza` via Bun.
6. CI creates `sdks/js/v0.2.7`.

To create a full repository release:

1. Change `loza.yaml` from `0.2.6` to `0.2.7`.
2. Push to `main`.
3. CI validates the release manifests and affected checks.
4. CI creates tag `v0.2.7`.
5. CI creates the umbrella GitHub Release.

To release the CLI:

1. Change `cli/loza-cli.yaml`.
2. Push to `main`.
3. CI runs CLI tests.
4. GoReleaser builds Linux, macOS, and Windows binaries for amd64 and arm64 where supported.
5. CI publishes a GitHub Release with checksums.
6. CI creates `cli/vX.Y.Z`.
7. CI verifies `go install github.com/astraive/loza/cli/cmd/loza@cli/vX.Y.Z`.

## Safety Rules

- Never publish on pull requests.
- Never publish from arbitrary branches unless manually dispatched.
- Never publish if tests fail.
- Never publish if the manifest version did not increase.
- Never overwrite existing tags.
- Never publish npm as `loza-js`; the package is `loza`.
- Never publish Docker `latest` from a non-main branch.
- Never publish all components just because one changed.
- A root `loza.yaml` bump creates only the umbrella GitHub Release.
- If spec changes, test dependents but publish only spec unless dependents have their own version bumps.
