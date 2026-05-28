# Code Review: Loxa v0.2.4 Final Release Audit

**Reviewer**: Code Reviewer Agent
**Date**: 2026-05-28
**Scope**: Full project -- collector, cortex, cli, lql, all SDKs (go, py, js, rs), loxana, eventbus, spec, examples, shipping infrastructure, documentation

---

## Summary

This is a comprehensive code review of the Loxa v0.2.4 monorepo. The project spans 10+ Go modules, 4 SDK languages, a Rust/WASM LQL compiler, a Vite+React dashboard, Helm charts, Dockerfiles, and 18 CI workflows. The codebase shows strong architectural discipline: consistent config layering, defense-in-depth security, comprehensive test coverage, and well-documented port allocation.

However, the version bump from 0.2.3 to 0.2.4 was **incomplete** -- the bump script did not cover all files that contain version strings. Additionally, all production Dockerfiles have **wrong EXPOSE ports** that contradict the canonical port map.

---

## Findings

### CRITICAL

#### [CRITICAL] All SDK ingest envelopes report "0.2.0" instead of "0.2.4"

Files:
- `E:/astraive/loxa/loxa/sdks/py/src/loxa/core/http_client.py:127` -- `sdk_version: str = "0.2.0"`
- `E:/astraive/loxa/loxa/sdks/py/src/loxa/core/pipeline.py:317` -- `"version": "0.2.0"`
- `E:/astraive/loxa/loxa/sdks/py/src/loxa/sinks/httpbatch/httpbatch.py:38` -- `sdk_version: str = "0.2.0"`
- `E:/astraive/loxa/loxa/sdks/js/src/sinks/standard-sinks.ts:197` -- `this.sdkVersion = opts.sdkVersion || '0.2.0'`
- `E:/astraive/loxa/loxa/sdks/js/src/collector/client.ts:69` -- `buildIngestEnvelope('loxa-js', '0.2.0', ...)`
- `E:/astraive/loxa/loxa/sdks/rs/src/core/client.rs:175` -- `sdk_version: "0.2.0".to_string()`

Issue:
Every event emitted by the Python, JavaScript, and Rust SDKs reports `source.version = "0.2.0"` in the ingest envelope sent to the collector. This misidentifies the SDK version in production telemetry, making it impossible to correlate event provenance with actual deployed SDK versions.

Why existing guards do not catch it:
The bump script (`scripts/bump-version.sh`) updates `pyproject.toml`, `Cargo.toml`, and `package.json` version fields, but does not update the hardcoded `sdk_version` string defaults in the SDK source code. The Go SDK was correctly bumped at `sdks/go/src/core/standard_sinks.go:302` (`cfg.SDKVersion = "0.2.4"`), but the other three SDKs were missed.

Fix:
Replace the hardcoded `"0.2.0"` defaults in all three SDKs with `"0.2.4"`, or better, read the version from the package manifest at runtime.

---

#### [CRITICAL] All collector Dockerfiles EXPOSE wrong ports (9090/9092 instead of 9308/9309)

Files:
- `E:/astraive/loxa/loxa/collector/Dockerfile:41` -- `EXPOSE 9090 9092`
- `E:/astraive/loxa/loxa/collector/deploy/docker/Dockerfile.collector:40` -- `EXPOSE 9090 9092`
- `E:/astraive/loxa/loxa/collector/deploy/Dockerfile.collector:40` -- `EXPOSE 9090 9092`

Issue:
The canonical port map (`docs/ports.md`) defines collector HTTP on port 9308 and collector gRPC on 9309. The defaults YAML (`collector/loxa-collector.defaults.yaml`) confirms `addr: ":9308"`. But all three collector Dockerfiles `EXPOSE 9090 9092`, which are the old pre-remap ports. While EXPOSE is informational (the app binds to 9308 regardless), this misleads container orchestrators, Docker Compose examples, and developers reading the Dockerfile.

Why existing guards do not catch it:
The bump script does not track Dockerfile EXPOSE directives. CI builds succeed because the app binds correctly regardless of EXPOSE.

Fix:
Change `EXPOSE 9090 9092` to `EXPOSE 9308 9309` in all three collector Dockerfiles.

---

#### [CRITICAL] Cortex Dockerfile EXPOSEs wrong ports and sets wrong default env vars

File: `E:/astraive/loxa/loxa/cortex/configs/Dockerfile:47-50`

Issue:
The Cortex Dockerfile exposes `8080 9090` and sets `CORTEX_SERVER_PORT=8080`. The canonical port map defines Cortex HTTP on 9312 and gRPC on 9313. The `cortex/configs/loxa-cortex.defaults.yaml` confirms `port: 9312`. This means container deployments using this Dockerfile will bind cortex to port 8080 instead of 9312, breaking the entire port allocation scheme.

Why existing guards do not catch it:
CI does not run the Cortex Docker image with port validation.

Fix:
Change `EXPOSE 8080 9090` to `EXPOSE 9312 9313` and `CORTEX_SERVER_PORT=8080` to `CORTEX_SERVER_PORT=9312`.

---

### HIGH

#### [HIGH] Loxana package.json version stuck at 0.2.3

File: `E:/astraive/loxa/loxana/package.json:3` -- `"version": "0.2.3"`

Issue:
The `loxana/loxana.yaml` metadata file correctly shows `version: 0.2.4`, but `package.json` (which is the source of truth for npm and build tooling) still says `0.2.3`. The bump script does not include `loxana/package.json` because Loxana is a separate repo (`loxana/` is not at the monorepo root).

Why existing guards do not catch it:
The bump script's `FILES` array does not include `loxana/package.json`. Loxana's `loxana.yaml` was manually bumped but `package.json` was missed.

Fix:
Either add `loxana/package.json` to the bump script or manually bump it to `0.2.4`.

---

#### [HIGH] Collector and Cortex Dockerfiles have no HEALTHCHECK directives

Files:
- `E:/astraive/loxa/loxa/collector/Dockerfile` -- no HEALTHCHECK
- `E:/astraive/loxa/loxa/collector/deploy/docker/Dockerfile.collector` -- no HEALTHCHECK
- `E:/astraive/loxa/loxa/collector/deploy/docker/Dockerfile.worker` -- no HEALTHCHECK
- `E:/astraive/loxa/loxa/cortex/configs/Dockerfile` -- no HEALTHCHECK

Issue:
The Loxana and spec service Dockerfiles have proper HEALTHCHECK directives, but all collector and cortex Dockerfiles lack them. Docker/containerd health checks are critical for orchestrators to detect unresponsive containers and trigger restarts. Without them, a hung collector or cortex process will receive traffic indefinitely.

Why existing guards do not catch it:
Kubernetes deployments use liveness/readiness probes (which are present in the K8s manifests), but bare Docker and Docker Compose deployments rely on Dockerfile HEALTHCHECK.

Fix:
Add `HEALTHCHECK` directives using `/health` (collector) and `/healthz` (cortex) endpoints.

---

#### [HIGH] Root CHANGELOG.md has no entries for v0.2.1 through v0.2.4

File: `E:/astraive/loxa/loxa/CHANGELOG.md`

Issue:
The root CHANGELOG.md only contains a single `[0.2.0]` entry. The collector CHANGELOG.md has entries through v0.2.3. The root changelog should aggregate all releases. Users checking `CHANGELOG.md` at the repo root will see only the initial release, missing 4 releases of security fixes and features.

Fix:
Add `[0.2.1]`, `[0.2.2]`, `[0.2.3]`, and `[0.2.4]` entries to the root CHANGELOG.

---

#### [HIGH] CLI CHANGELOG.md stuck at v0.0.2

File: `E:/astraive/loxa/loxa/cli/CHANGELOG.md`

Issue:
The CLI CHANGELOG only has a `[0.0.2]` entry from 2026-05-20. All subsequent releases (0.2.1 through 0.2.4) are missing. The CLI has had significant changes (version bump, new commands, security fixes).

Fix:
Add entries for all releases since 0.0.2.

---

#### [HIGH] Cortex SECURITY.md references stale ":0.2.3" image tags

File: `E:/astraive/loxa/loxa/cortex/SECURITY.md:45`

Issue:
Line 45 states "All deployment manifests pin images to `:0.2.3` instead of `:latest`". The actual deployment manifests (e.g., `cortex/deploy/helm/cortex/values.yaml:12`) correctly use `tag: "0.2.4"`. The SECURITY.md documentation is stale and could mislead security auditors.

Fix:
Update to `:0.2.4`.

---

#### [HIGH] Cortex README says "v0.2.3 STABLE"

File: `E:/astraive/loxa/loxa/cortex/README.md:3`

Issue:
Line 3 states `**Status: v0.2.3 STABLE**` but the actual version is 0.2.4. This is the first thing users see when reading the Cortex documentation.

Fix:
Update to `v0.2.4 STABLE`.

---

### MEDIUM

#### [MEDIUM] Helm collector values.yaml route inconsistency with defaults.yaml

Files:
- `E:/astraive/loxa/loxa/collector/deploy/helm/loxa/values.yaml:85` -- `ingest: /ingest`
- `E:/astraive/loxa/loxa/collector/loxa-collector.defaults.yaml:46` -- `ingest: /events`
- `E:/astraive/loxa/loxa/collector/deploy/helm/loxa/values.yaml:86` -- `health: /healthz`
- `E:/astraive/loxa/loxa/collector/loxa-collector.defaults.yaml:47` -- `health: /health`

Issue:
The Helm values file uses `/ingest` for the ingest route and `/healthz` for health, while the defaults YAML uses `/events` and `/health`. The code registers both `/events` and `/ingest` (and both `/health` and `/healthz`), so this works functionally. But the inconsistency between the canonical defaults and the Helm deployment config could confuse operators.

Fix:
Align Helm values with defaults.yaml: `ingest: /events`, `health: /health`.

---

#### [MEDIUM] RELEASE.md references v0.2.0 everywhere

File: `E:/astraive/loxa/loxa/RELEASE.md`

Issue:
The release process document still says "All components share version 0.2.0" and provides tag examples like `collector-v0.2.0`, `py-v0.2.0`, etc. This is stale by 4 releases.

Fix:
Update to reference 0.2.4 or use a variable/template approach.

---

#### [MEDIUM] docs/release-notes.md has stale install commands

File: `E:/astraive/loxa/loxa/docs/release-notes.md:151-165`

Issue:
Install instructions reference `docker pull astraive/loxa:0.2.0`, `helm install loxa loxa/loxa --version 0.2.0`, `loxa==0.2.0` (PyPI), `loxa = "0.2.0"` (Crates.io). All should reference 0.2.4.

Fix:
Update all version references to 0.2.4.

---

#### [MEDIUM] docs/getting-started.md has stale badge

File: `E:/astraive/loxa/loxa/docs/getting-started.md:3`

Issue:
Line 3 shows `![Version](https://img.shields.io/badge/version-0.2.0-blue)`. Should be 0.2.4.

Fix:
Update badge to 0.2.4.

---

#### [MEDIUM] Multiple SDK test fixtures reference "0.2.0"

Files (non-exhaustive):
- `E:/astraive/loxa/loxa/sdks/go/src/core/ingest_envelope_fixture_test.go:52,91`
- `E:/astraive/loxa/loxa/sdks/go/src/core/config_file_test.go:189,210,211`
- `E:/astraive/loxa/loxa/sdks/py/tests/test_ingest_envelope_fixture.py:37`
- `E:/astraive/loxa/loxa/sdks/py/tests/test_non_mutating_normalization.py:37`
- `E:/astraive/loxa/loxa/sdks/js/tests/conformance.test.ts:41,46`
- `E:/astraive/loxa/loxa/sdks/rs/tests/ingest_envelope_fixture.rs:86`
- `E:/astraive/loxa/loxa/sdks/rs/tests/domain_helpers_test.rs:346`

Issue:
Test fixtures that verify ingest envelope contents expect `version: "0.2.0"`. If the SDK defaults are fixed to "0.2.4", these tests will fail. However, the test fixtures are testing the wire contract, not the SDK version, so some of these may be intentionally using "0.2.0" as test data. The fixture tests at `ingest_envelope_fixture_test.go` use `SDKVersion: "0.2.0"` as input and verify the output matches -- this is a fixture that should be updated when the SDK version changes.

Fix:
Update test fixtures that verify SDK version strings to use "0.2.4".

---

#### [MEDIUM] docs/configuration.md references v0.2.0

File: `E:/astraive/loxa/loxa/docs/configuration.md:275`

Issue:
Example shows `service_version: 0.2.0`. Should be updated.

Fix:
Update to 0.2.4.

---

#### [MEDIUM] docs/specification-and-release-index.md references v0.2.0

File: `E:/astraive/loxa/loxa/docs/specification-and-release-index.md:5`

Issue:
States `**Version**: 0.2.0` and references v0.2.0 throughout. This is the main spec index document.

Fix:
Update to 0.2.4 or clarify that the spec version is independent of the release version.

---

### LOW

#### [LOW] Collector deploy/loxa-collector.yaml route inconsistency

File: `E:/astraive/loxa/loxa/collector/deploy/loxa-collector.yaml:19-22`

Issue:
This standalone config file uses `/ingest` and `/healthz` instead of `/events` and `/health` (the canonical defaults). Since the code registers all aliases, this works, but it's inconsistent.

Fix:
Align with canonical defaults.

---

#### [LOW] Go SDK test fixture uses AppVersion("0.2.0") in conformance test

File: `E:/astraive/loxa/loxa/sdks/go/tests/conformance/api_surface_test.go:98`

Issue:
Conformance test uses `loxa.AppVersion("0.2.0")` which sets the service version (not SDK version). This is test data, not a version tracking issue.

Fix:
Low priority; could update for consistency.

---

#### [LOW] RELEASE.md install instructions reference v0.0.1

File: `E:/astraive/loxa/loxa/docs/release-notes.md:145-146`

Issue:
Go install commands reference `v0.0.1` tags.

Fix:
Update to current version.

---

## Review Summary

| Severity | Count | Status |
|---|---:|---|
| CRITICAL | 3 | BLOCK |
| HIGH | 5 | BLOCK |
| MEDIUM | 7 | FAIL |
| LOW | 3 | WARN |

**Verdict: BLOCK**

---

## Positive Findings

1. **Consistent version in loxa*.yaml metadata files**: All 8 component metadata files (`collector/loxa.metadata.yaml`, `cortex/loxa-cortex.metadata.yaml`, `cli/loxa-cli.yaml`, `sdks/go/loxa-go.yaml`, `sdks/py/loxa-py.yaml`, `sdks/rs/loxa-rs.yaml`, `sdks/js/loxa-js.yaml`, `loxana/loxana.yaml`) correctly show `version: 0.2.4`.

2. **Consistent version in Go binaries**: All Go binaries (`collector/cmd/loxa-collector/main.go`, `collector/cmd/loxa-worker/main.go`, `collector/cmd/loxa-loadgen/main.go`, `cortex/cmd/server/main.go`, `cli/cmd/loxa/main.go`) correctly show `var version = "0.2.4"`.

3. **Consistent version in Helm charts**: Both `collector/deploy/helm/loxa/Chart.yaml` and `cortex/deploy/helm/cortex/Chart.yaml` correctly use `version: 0.2.4` and `appVersion: "0.2.4"`. Values.yaml image tags are also correct.

4. **Consistent version in Cargo.toml/pyproject.toml/package.json**: `sdks/rs/Cargo.toml`, `sdks/py/pyproject.toml`, `sdks/js/package.json`, `lql/Cargo.toml`, and `cortex/crates/cortex-match/Cargo.toml` all show `version = "0.2.4"`.

5. **K8s deployment manifests correct**: Both `collector/deploy/k8s/collector-deployment.yaml` and `cortex/configs/cortex-deployment.yaml` use `image: ghcr.io/astraive/*:0.2.4` and correct container ports (9308, 9312/9313).

6. **DSN parsing implemented in all 4 SDKs**: Go, Python, JavaScript, and Rust all have `loxa://` DSN parsing with tests. The Go SDK has comprehensive DSN tests covering edge cases.

7. **Defense-in-depth security**: WebSocket origin validation uses exact match (not prefix), `SetReadLimit(1MB)` on WebSocket connections, `enable_external_access=false` on DuckDB connections, security headers middleware, body size limits, and role-based access control.

8. **Health/ready endpoints properly implemented**: Collector has `/health`, `/healthz`, `/ready`, `/readyz`. Cortex has `/healthz`, `/readyz`. All are excluded from auth middleware.

9. **Comprehensive CI coverage**: 18 CI workflows covering collector, cortex, LQL, all SDKs, Loxana, spec validation, conformance, and benchmarks.

10. **Bump script is well-designed**: `scripts/bump-version.sh` supports `--check` mode, `--dry-run`, auto-detection, and covers 19 files. The issue is it needs to cover more files (SDK source code defaults, Dockerfiles, docs).

11. **Port map documentation is excellent**: `docs/ports.md` clearly defines all ports, explains the 4-layer config system, and documents the `loxa://` DSN format.

12. **Consistent non-root containers**: All Dockerfiles create a `loxa` (or `loxana`) user and run as non-root.

13. **Collector Dockerfiles correctly handle config via entrypoint**: `docker-entrypoint.sh` supports `LOXA_CONFIG` (inline YAML) and `LOXA_CONFIG_FILE` (path) env vars.

14. **Security audit already conducted**: `docs/api-security-v0.2.4.md` documents 15 findings with remediation priorities.

---

## Recommendations (Priority Order)

1. **P0 -- Fix SDK version defaults**: Update `sdk_version` defaults in Python, JS, and Rust SDKs from "0.2.0" to "0.2.4". Update corresponding test fixtures.

2. **P0 -- Fix Dockerfile EXPOSE ports**: Update all 4 Dockerfiles (3 collector + 1 cortex) to use canonical ports 9308/9309 and 9312/9313.

3. **P1 -- Fix Loxana package.json version**: Bump to 0.2.4.

4. **P1 -- Update CHANGELOGs**: Add entries for 0.2.1-0.2.4 to root and CLI changelogs.

5. **P1 -- Update documentation**: Fix stale version references in RELEASE.md, release-notes.md, getting-started.md, configuration.md, cortex README, cortex SECURITY.md.

6. **P2 -- Add HEALTHCHECK to Go Dockerfiles**: Add health check directives to collector and cortex Dockerfiles.

7. **P2 -- Align Helm routes with defaults**: Use `/events` and `/health` in Helm values.

8. **P2 -- Expand bump script coverage**: Add SDK source code defaults, Dockerfile EXPOSE, Loxana package.json, and documentation files to the bump script.

---

## Component-by-Component Assessment

| Component | Version | Build | Tests | Security | Ports | Score |
|-----------|---------|-------|-------|----------|-------|-------|
| Collector | 0.2.4 | PASS | PASS | PASS | FAIL (Dockerfile) | 85/100 |
| Cortex | 0.2.4 | PASS | PASS | PASS | FAIL (Dockerfile) | 85/100 |
| CLI | 0.2.4 | PASS | PASS | PASS | PASS | 95/100 |
| LQL | 0.2.4 | PASS | PASS | PASS | N/A | 95/100 |
| Go SDK | 0.2.4 | PASS | PASS | PASS | PASS | 95/100 |
| Python SDK | 0.2.4* | PASS | PASS | PASS | PASS | 80/100 |
| JS SDK | 0.2.4* | PASS | PASS | PASS | PASS | 80/100 |
| Rust SDK | 0.2.4* | PASS | PASS | PASS | PASS | 80/100 |
| Loxana | 0.2.3/0.2.4 | PASS | PASS | PASS | PASS | 75/100 |
| Spec | N/A | PASS | PASS | PASS | N/A | 95/100 |
| Docs | mixed | N/A | N/A | N/A | N/A | 60/100 |
| Shipping | mixed | PASS | N/A | N/A | FAIL | 65/100 |

*SDK package versions are 0.2.4 but runtime sdk_version defaults are 0.2.0

---

## Overall Score: 72/100

The project has excellent architecture, security, and test coverage. The score is penalized by the incomplete version bump (3 CRITICAL + 5 HIGH findings), all of which are straightforward to fix. Once the version references and Dockerfile ports are corrected, this would score 90+.
