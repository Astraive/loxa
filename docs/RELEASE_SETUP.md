# Release Setup

This document describes how to publish a Loza release.

## Overview

Loza uses a **manual-only** release workflow. Pushing to `main` does NOT trigger publishing. You must explicitly dispatch the `release-publish.yml` workflow with a component list, version, and dry-run flag.

The release system is manifest-driven. Each component has a YAML manifest (e.g., `collector/loza.yaml`, `sdks/js/package.json`) that declares its version. The release workflow reads these manifests and publishes only when the requested version matches the manifest version.

## Components

| Component | Manifest | Publishes to |
|-----------|----------|--------------|
| `collector` | `collector/loza.yaml` | Docker Hub + GHCR |
| `cortex` | `cortex/loza-cortex.yaml` | Docker Hub + GHCR |
| `cli` | `cli/loza-cli.yaml` | GitHub Releases (GoReleaser) |
| `sdk-go` | `sdks/go/loza-go.yaml` | Go module proxy (component tag) |
| `sdk-js` | `sdks/js/loza-js.yaml` | npm |
| `sdk-py` | `sdks/py/loza-py.yaml` | PyPI |
| `sdk-rs` | `sdks/rs/loza-rs.yaml` | crates.io |
| `spec` | `spec/loza-spec.yaml` | GitHub Releases |
| `loza` | `loza.yaml` | GitHub Release (umbrella) |

## How to Publish

### 1. Bump versions

Update the version in each component manifest you want to release. All manifests for a single release must have the same version.

```bash
# Example: bump collector to 0.2.6
# Edit collector/loza.yaml → version: "0.2.6"
```

### 2. Validate manifests locally

```bash
python scripts/release/validate-manifests.py
```

This checks that all manifests are parseable and consistent.

### 3. Dry-run the release

Go to **Actions → Release Publish → Run workflow**:

- **components**: `collector,cortex` (comma-separated)
- **version**: `0.2.6`
- **dry_run**: `true` (default)

This validates everything without actually publishing.

### 4. Publish for real

Run the same workflow with **dry_run**: `false`.

## Reusable Publish Workflows

The release controller calls these reusable workflows:

| Workflow | Trigger | What it does |
|----------|---------|--------------|
| `publish-docker.yml` | `workflow_call` | Builds and pushes collector/cortex images to Docker Hub and GHCR |
| `publish-cli.yml` | `workflow_call` | Runs GoReleaser for the CLI binary |
| `npm-publish.yml` | `workflow_call` | Publishes the JS SDK to npm with provenance |
| `pypip-publish.yml` | `workflow_call` | Publishes the Python SDK to PyPI with trusted publishing |
| `cargo-publish.yml` | `workflow_call` | Publishes the Rust SDK to crates.io with trusted publishing |
| `publish-go.yml` | `workflow_call` | Creates and verifies the Go SDK module tag |
| `publish-github-release.yml` | `workflow_call` | Creates the umbrella GitHub Release |
| `verify-go-modules.yml` | `workflow_call` | Verifies the spec Go module and all SDK conformance |

## Secrets Required

| Secret | Used by |
|--------|---------|
| `DOCKERHUB_USERNAME` | publish-docker |
| `DOCKERHUB_TOKEN` | publish-docker |
| `GITHUB_TOKEN` | all workflows that create tags/releases |

## Component Tags

Each component gets a Git tag after successful publishing:

| Component | Tag format |
|-----------|------------|
| collector | `collector/v0.2.6` |
| cortex | `cortex/v0.2.6` |
| cli | `cli/v0.2.6` |
| sdk-go | `sdks/go/v0.2.6` |
| sdk-js | `sdks/js/v0.2.6` |
| sdk-py | `sdks/py/v0.2.6` |
| sdk-rs | `sdks/rs/v0.2.6` |
| spec | `spec/v0.2.6` |
| loza (umbrella) | `v0.2.6` |

## Legacy Workflows (removed)

The following old tag-based release workflows have been removed:

- `sdks-py-release.yml` (triggered on `py-v*` tags) — replaced by `release-publish.yml → pypip-publish.yml`
- `sdks-rs-release.yml` (triggered on `rs-v*` tags) — replaced by `release-publish.yml → cargo-publish.yml`

## Troubleshooting

### "unknown component"

Check the component name against the `release.yaml` registry file. Valid names: `collector`, `cortex`, `cli`, `sdk-go`, `sdk-js`, `sdk-py`, `sdk-rs`, `spec`, `loza`.

### Dry run succeeded but real publish failed

Check that the Docker Hub secrets are configured when publishing `collector` or `cortex`. The SDK workflows use trusted publishing or Git tags and do not require registry API tokens.
