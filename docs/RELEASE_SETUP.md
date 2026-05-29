# Release Setup

This document describes how to publish a Loxa release.

## Overview

Loxa uses a **manual-only** release workflow. Pushing to `main` does NOT trigger publishing. You must explicitly dispatch the `release-publish.yml` workflow with a component list, version, and dry-run flag.

The release system is manifest-driven. Each component has a YAML manifest (e.g., `collector/loxa.yaml`, `sdks/js/package.json`) that declares its version. The release workflow reads these manifests and publishes only when the requested version matches the manifest version.

## Components

| Component | Manifest | Publishes to |
|-----------|----------|--------------|
| `collector` | `collector/loxa.yaml` | Docker Hub + GHCR |
| `cortex` | `cortex/loxa-cortex.yaml` | Docker Hub + GHCR |
| `cli` | `cli/loxa-cli.yaml` | GitHub Releases (GoReleaser) |
| `sdk-js` | `sdks/js/package.json` | npm |
| `sdk-py` | `sdks/py/pyproject.toml` | PyPI |
| `sdk-rs` | `sdks/rs/Cargo.toml` | crates.io |
| `lql` | `lql/lql.yaml` | crates.io |
| `spec` | `spec/loxa-spec.yaml` | GitHub Releases |
| `loxa` | `loxa.yaml` | GitHub Release (umbrella) |

## How to Publish

### 1. Bump versions

Update the version in each component manifest you want to release. All manifests for a single release must have the same version.

```bash
# Example: bump collector to 0.2.6
# Edit collector/loxa.yaml → version: "0.2.6"
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
| `publish-docker.yml` | `workflow_call` | Builds and pushes Docker images for collector/cortex |
| `publish-cli.yml` | `workflow_call` | Runs GoReleaser for the CLI binary |
| `publish-js.yml` | `workflow_call` | Publishes JS SDK to npm with provenance |
| `publish-py.yml` | `workflow_call` | Publishes Python SDK to PyPI |
| `publish-rs.yml` | `workflow_call` | Publishes Rust SDK and LQL crates |
| `publish-github-release.yml` | `workflow_call` | Creates umbrella GitHub Release |
| `verify-go-modules.yml` | `workflow_call` | Verifies Go module tags resolve correctly |

All reusable workflows accept `component`, `version`, and `dry_run` inputs. Each workflow checks if it's responsible for the given component and skips otherwise.

## Secrets Required

| Secret | Used by |
|--------|---------|
| `DOCKERHUB_USERNAME` | publish-docker |
| `DOCKERHUB_TOKEN` | publish-docker |
| `NPM_TOKEN` | publish-js |
| `PYPI_API_TOKEN` | publish-py |
| `CARGO_REGISTRY_TOKEN` | publish-rs |
| `GITHUB_TOKEN` | publish-cli, publish-github-release, publish-rs |

## Component Tags

Each component gets a Git tag after successful publishing:

| Component | Tag format |
|-----------|------------|
| collector | `collector/v0.2.6` |
| cortex | `cortex/v0.2.6` |
| cli | `cli/v0.2.6` |
| sdk-js | `sdk-js/v0.2.6` |
| sdk-py | `sdk-py/v0.2.6` |
| sdk-rs | `sdk-rs/v0.2.6` |
| lql | `lql/v0.2.6` |
| spec | `spec/v0.2.6` |
| loxa (umbrella) | `v0.2.6` |

## Legacy Workflows (removed)

The following old tag-based release workflows have been removed:

- `sdks-py-release.yml` (triggered on `py-v*` tags) — replaced by `release-publish.yml → publish-py.yml`
- `sdks-rs-release.yml` (triggered on `rs-v*` tags) — replaced by `release-publish.yml → publish-rs.yml`

## Troubleshooting

### "manifest version does not match requested version"

The version in the component's manifest file doesn't match the version you passed to the workflow. Update the manifest first.

### "unknown component"

Check the component name against the `release.yaml` registry file. Valid names: `collector`, `cortex`, `cli`, `sdk-js`, `sdk-py`, `sdk-rs`, `lql`, `spec`, `loxa`.

### Dry run succeeded but real publish failed

Check that the required secrets are configured in the repository settings. Each publish workflow needs its own secret (see table above).
