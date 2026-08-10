# CI Workflows

This document describes all GitHub Actions workflows in the Loza repository.

## CI Workflows (test on push/PR)

These workflows run automatically on pushes and pull requests.

### collector-ci.yml

**Triggers:** push/PR to `main` when `collector/`, `spec/`, `proto/`, or the workflow file changes.

**Jobs:**
- **proto-sync** — Regenerates protobuf code and checks for drift
- **unit** — Runs `go test ./...` in `collector/`
- **integration** — Runs integration tests with real Kafka and Redis service containers

### sdks-go.yml

**Triggers:** push/PR when `sdks/go/` or `spec/` changes; manual dispatch.

**Jobs:**
- **build-test** — Matrix build (ubuntu + windows): build, vet, lint, tests, conformance, govulncheck, stress tests, benchmarks
- **required-checks** — Enforces all jobs passed

### sdks-js.yml

**Triggers:** push/PR when `sdks/js/` or `spec/` changes; manual dispatch.

**Jobs:**
- **test** — Install with bun, typecheck, build, unit tests (excluding e2e-collector), spec conformance

### sdks-py.yml

**Triggers:** push/PR when `sdks/py/` or `spec/` changes; manual dispatch.

**Jobs:**
- **lint-test-conformance** — Ruff lint/format, pytest unit tests, spec conformance, build wheel

### sdks-rs.yml

**Triggers:** push/PR when `sdks/rs/` or `spec/` changes; manual dispatch.

**Jobs:**
- **lint** — `cargo fmt --check` and `cargo clippy`
- **build-test** — Build and run all tests (excluding collector e2e)
- **conformance** — Spec conformance runner + contract tests
- **required-checks** — Enforces all jobs passed

### lql-ci.yml

**Triggers:** push/PR when `lql/` changes.

**Jobs:**
- **build** — Build, test, clippy, format check
- **wasm** — Build WASM target with wasm-pack

### spec-ci.yml

**Triggers:** push/PR to `main` when `spec/` changes.

**Jobs:**
- **generation-check** — Verifies generated spec artifacts are up-to-date
- **mirror-check** — Verifies mirror folders are in sync
- **conformance-check** — Runs grouped SDK conformance for all SDKs
- **release-protection** — Protects `releases/` folder from accidental changes

### spec-validate.yml

**Triggers:** push/PR to `main`.

**Jobs:**
- **validate** — Checks generated artifacts, mirrors, conformance, and runs conformance runner

Note: `spec-ci.yml` covers the same ground with more depth. This workflow may be removed in the future.

### release-detect.yml

**Triggers:** push/PR to `main`; manual dispatch.

**Jobs:**
- **detect** — Validates release manifests and detects which components have version bumps
- **collector** — Runs collector tests if collector changed
- **cortex** — Runs cortex tests if cortex changed
- **cli** — Runs CLI tests if CLI changed
- **spec-and-sdks** — Runs spec validation and SDK conformance if spec or SDKs changed
- **lql** — Runs LQL tests if LQL changed

This workflow runs focused tests only for components that actually changed.

### loza-spec-rollout.yml

**Triggers:** push/PR to `main` when spec rollout files, docker-compose, or spec changes.

**Jobs:**
- **verify** — Runs stager/Kafka adapter tests, renders Helm templates, builds schema-service and stager Docker images

### verify-go-modules.yml

**Triggers:** `workflow_call` only (called by release-publish).

**Jobs:**
- **verify** — Tests Go module resolution, runs conformance for all SDKs, verifies `go install` works

## Benchmark Workflows

### benchmarks.yml

**Triggers:** Weekly schedule (Sunday midnight); manual dispatch.

**Jobs:**
- **duckdb-bench** — Runs full Go benchmark suite in `collector/`, uploads results as artifact

### duckdb-bench.yml

**Triggers:** push/PR to `main`.

**Jobs:**
- **bench** — Runs DuckDB-specific benchmarks, uploads results

## Code Generation Workflows

### proto-gen.yml

**Triggers:** push/PR when `proto/loza/core/*.proto` changes; manual dispatch.

**Jobs:**
- **generate** — Regenerates Go protobuf code and verifies it matches what's checked in

### openapi-gen.yml

**Triggers:** manual dispatch only.

**Jobs:**
- **generate** — Runs OpenAPI generator script, uploads generated clients

## Release Workflows

### release-publish.yml

**Triggers:** manual dispatch only.

**Inputs:**
- `components` — Comma-separated list (e.g., `collector,cortex`)
- `version` — Version to publish (e.g., `0.2.6`)
- `dry_run` — Validate and build without publishing (default: `true`)

**Jobs:**
- **plan** — Reads manifests, builds publish matrix
- **publish-docker** — Builds/pushes Docker images (collector, cortex)
- **publish-cli** — Publishes CLI via GoReleaser
- **publish-js** — Publishes JS SDK to npm
- **publish-py** — Publishes Python SDK to PyPI
- **publish-rs** — Publishes Rust SDK and LQL crates
- **publish-github-release** — Creates umbrella GitHub Release
- **verify-go-modules** — Verifies Go module tags

### publish-docker.yml

Reusable workflow. Builds Docker images for `collector` or `cortex`, pushes to Docker Hub and GHCR, creates component tag.

### publish-cli.yml

Reusable workflow. Runs GoReleaser for the `cli` component, creates tag, verifies `go install` works.

### publish-js.yml

Reusable workflow. Publishes `sdk-js` to npm with provenance, creates tag.

### publish-py.yml

Reusable workflow. Publishes `sdk-py` to PyPI using trusted publishing, creates tag.

### publish-rs.yml

Reusable workflow. Publishes `sdk-rs` to crates.io and `lql` crate (if enabled), creates tags.

### publish-github-release.yml

Reusable workflow. Creates umbrella GitHub Release for the `loza` component.

### publish-contract.yml

**Triggers:** manual dispatch; GitHub Release published.

**Jobs:**
- **publish-contract** — Generates and publishes the LOZA contract to S3/CloudFront

Requires AWS secrets: `LOZA_CONTRACT_BUCKET`, `LOZA_CONTRACT_PREFIX`, `CLOUDFRONT_DISTRIBUTION_ID`, `AWS_REGION`.

## Workflow Dependency Graph

```
release-publish.yml
├── plan (reads manifests, builds matrix)
├── publish-docker.yml ──────────── (collector, cortex)
├── publish-cli.yml ─────────────── (cli)
├── publish-js.yml ──────────────── (sdk-js)
├── publish-py.yml ──────────────── (sdk-py)
├── publish-rs.yml ──────────────── (sdk-rs, lql)
├── publish-github-release.yml ──── (loza umbrella)
└── verify-go-modules.yml ───────── (spec, sdk-go)

release-detect.yml
├── detect (finds changed components)
├── collector tests (if collector changed)
├── cortex tests (if cortex changed)
├── cli tests (if cli changed)
├── spec-and-sdks tests (if spec/SDKs changed)
└── lql tests (if lql changed)
```

## Removed Workflows

| Workflow | Why removed |
|----------|-------------|
| `sdks-py-release.yml` | Old tag-based (`py-v*`); replaced by `release-publish.yml → publish-py.yml` |
| `sdks-rs-release.yml` | Old tag-based (`rs-v*`); replaced by `release-publish.yml → publish-rs.yml` |
