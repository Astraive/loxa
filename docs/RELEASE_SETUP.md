# LOXA Release Setup

LOXA publishes each component only when that component's YAML manifest version increases. The central registry is `release.yaml`; every workflow reads component paths from that file. Registry owners and publisher accounts are `astraive`.

## Repository Settings

Go to:

```txt
GitHub repo -> Settings -> Actions -> General
```

Set:

- Allow GitHub Actions.
- Workflow permissions: Read and write permissions.
- Allow GitHub Actions to create and approve pull requests if maintainers use automation for release PRs.
- Enable access to packages/GHCR.

The publishing workflows request:

```yaml
permissions:
  contents: write
  packages: write
  id-token: write
  actions: read
```

## Required Secrets

Add these in `GitHub repo -> Settings -> Secrets and variables -> Actions`:

```txt
DOCKERHUB_USERNAME
DOCKERHUB_TOKEN
NPM_TOKEN
PYPI_API_TOKEN
CARGO_REGISTRY_TOKEN
```

`NPM_TOKEN` is optional when npm trusted publishing is configured. `PYPI_API_TOKEN` is optional when PyPI trusted publishing is configured. `CARGO_REGISTRY_TOKEN` is required for crates.io publishing. Docker Hub secrets are required for collector and cortex image publishing.

## Docker Hub

1. Create Docker Hub repositories:
   - `astraive/loxa`
   - `astraive/loxa-cortex`
2. Create a Docker Hub access token.
3. Add GitHub Actions secrets:
   - `DOCKERHUB_USERNAME`
   - `DOCKERHUB_TOKEN`

Collector images publish as:

```txt
astraive/loxa:X.Y.Z
```

Cortex images publish as:

```txt
astraive/loxa-cortex:X.Y.Z
```

`latest` is published only from `main` for stable manifest versions.

## GHCR

GHCR uses `GITHUB_TOKEN`; no extra registry token is required.

After the first publish, change package visibility to public if needed. Images are:

```txt
ghcr.io/astraive/loxa
ghcr.io/astraive/loxa-cortex
```

## npm

Package name must be:

```txt
loxa
```

Do not publish `loxa-js`.

Users install the JavaScript SDK independently:

```txt
npm install loxa
```

Preferred option: configure npm trusted publishing for the `loxa` package and this repository workflow.

Fallback option: create an npm automation token and add it as:

```txt
NPM_TOKEN
```

## PyPI

Package name must be:

```txt
loxa
```

Users install the Python SDK independently:

```txt
pip install loxa
```

Preferred option: configure PyPI trusted publishing for this repository workflow.

Fallback option: create a PyPI API token scoped to `loxa` and add it as:

```txt
PYPI_API_TOKEN
```

## crates.io

1. Create a crates.io API token.
2. Add it as:

```txt
CARGO_REGISTRY_TOKEN
```

The Rust SDK package name is:

```txt
loxa
```

Users install the Rust SDK independently:

```txt
cargo add loxa
```

The LQL crate package is:

```txt
loxa-lql
```

LQL publishes to crates.io only when `lql/lql.yaml` contains `publish.crates`.

## Go Modules

Go publishing happens through Git tags. There is no Go registry token.

Required module tags:

```txt
spec/vX.Y.Z
cli/vX.Y.Z
sdks/go/vX.Y.Z
```

The Go verification workflow creates or verifies component tags, then checks module resolution with `go get` or `go install`.

Users install the Go SDK with:

```txt
go get github.com/astraive/loxa/sdks/go
```

## Umbrella GitHub Release

The root `loxa.yaml` manifest controls full repository releases. When only `loxa.yaml` has a version bump, CI creates a GitHub Release tagged:

```txt
vX.Y.Z
```

This is a GitHub-only umbrella release. It does not publish every component to Docker Hub, npm, PyPI, or crates.io.
