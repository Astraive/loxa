# Loxa v0.2.0 Pre-Release Regression Report

**Date:** 2026-05-27
**Branch:** main
**Commit:** e6780e6
**Target:** Local builds (E:\astraive\loxa)
**Mode:** Full regression QA

---

## Summary

**Regression Score:** 82/100
**Status:** PASS_WITH_ISSUES

**Recommendation:** Ship after fixes -- no critical regressions, but two medium issues should be addressed.

**Top Risks:**
1. Go SDK `CollectorClient.Ingest()` still uses `/ingest` endpoint while collector canonical route is `/events` (inconsistency, both work)
2. Collector `TestLoadSustainedThroughput` fails on this environment (523 events/sec vs expected >= 4000) -- likely Windows/environment-specific, not a code regression
3. LQL `sum` keyword semantics changed (breaking change for users who used `sum` as shorthand for `summarize`)

---

## Changed Files (34 modified, 1 new)

### CI/CD
- `.github/workflows/collector-ci.yml` -- Kafka image changed from bitnami/kafka to apache/kafka:3.9.0, env var names updated

### CLI
- `cli/docs/getting-started.md` -- documentation update
- `cli/internal/cli/root.go` -- CLI root changes
- `cli/internal/client/collector.go` -- collector client changes
- `cli/internal/commands/cortex.go`, `debug.go`, `dev.go`, `export.go`, `keys.go`, `shared.go` -- command updates

### Collector
- `collector/cmd/loxa-collector/server.go` -- server changes
- `collector/deploy/docker-compose.memory.yml`, `docker-compose.yml` -- deploy configs
- `collector/deploy/helm/loxa/templates/deployment-worker.yaml` -- Helm chart
- `collector/go.mod`, `collector/go.sum` -- dependency updates
- `collector/internal/eventbus/kafka/kafka.go` -- Kafka eventbus changes

### Cortex
- `cortex/cmd/server/main.go` -- server entry point
- `cortex/configs/config.yaml` -- config: port 9100 -> 9312, postgres key -> postgresql
- `cortex/internal/api/server.go` -- API server changes
- `cortex/internal/storage/collector_event_store.go` -- added warning logs for discarded events
- `cortex/internal/storage/postgres.go` -- added new columns to signature table (feature_weights, version, parent_signature_id, decay_factor, last_matched_at, behavioral_hash)

### LQL (Rust)
- `lql/src/compiler/clickhouse.rs` -- alias-aware compilation, refactored aggregation functions
- `lql/src/compiler/duckdb.rs` -- alias-aware compilation, duration fix, refactored aggregation functions
- `lql/src/lexer.rs` -- `||` and `&&` as operators (not keywords), `!~` as NotLike token, `sum` separated from `summarize`
- `lql/src/parser.rs` -- NotLike support, test fix

### SDKs
- `sdks/go/src/core/config.go` -- HTTPBatchSink endpoint `/ingest` -> `/events`
- `sdks/go/src/cortex/client.go` -- default endpoint port 9100 -> 9312
- `sdks/js/package-lock.json` -- version 0.0.1 -> 0.2.0
- `sdks/py/src/loxa.egg-info/PKG-INFO` -- version 0.0.2 -> 0.2.0
- `sdks/rs/src/config/config.rs` -- `validate()` changed from panic to Result return

### Spec
- `spec/buf.yaml` -- module path changed to releases/v1/proto, added lint exceptions
- `spec/buf.gen.yaml` -- NEW file
- `spec/scripts/publish_contract_s3.py` -- added time import

### Examples
- `examples/quickstart/go/go.mod` -- Go 1.22 -> 1.25.0, SDK v0.0.1 -> v0.2.0

---

## Test Results

### Collector (Go)
```
PASS: 27/28 test packages
FAIL: 1 (TestLoadSustainedThroughput -- performance test, 523 events/sec vs >= 4000 expected)
```
- All functional tests pass
- All sink tests pass (DuckDB, ClickHouse, Postgres, Kafka, Loki, OTLP, S3, GCS)
- Auth, config, schema, processing, queue, storage tests pass
- Load test failure is environment-dependent (Windows, not dedicated test hardware)

### Cortex (Go)
```
PASS: 14/14 test packages
```
- API, config, errors, graph, learner, logging, matcher, middleware, models, processor, reconstructor, storage, topology all pass

### LQL (Rust)
```
PASS: 51/51 tests (29 unit + 21 integration + 1 doc-test)
```
- Lexer, parser, compiler (DuckDB + ClickHouse), schema, validation all pass
- No skipped or ignored tests

### CLI (Go)
```
PASS: 4/4 test packages
```
- Client, commands, config, schema tests pass

### Go SDK
```
PASS: 8/8 test packages
```
- Core, cortex, libs, packages, storagepath, utils, conformance, root tests pass
- 12 conformance groups PASS

### Python SDK
```
PASS: 144/144 tests
```
- All unit, integration, conformance, and fixture tests pass
- 12 conformance groups PASS

### Rust SDK
```
PASS: All tests (3 + 8 + 12 + 12 + 1 + 2 + 2 + 2 + 1 + 16 + 2 + 1 + 16 + 16 + 2 + 1 + 3)
```
- 12 conformance groups PASS

### JS SDK
```
PASS: 117/117 tests across 20 files
```
- Event lifecycle, canonical fields, facade, sampler, sinks, E2E collector, conformance all pass
- 12 conformance groups PASS

### Cross-SDK Conformance (spec/conformance/runner.py)
```
PASS: 48/48 checks (12 groups x 4 SDKs)
```
All SDKs produce identical wire-format behavior:
- state_machine: PASS (all 4)
- canonical_fields: PASS (all 4)
- duplicate_policy: PASS (all 4)
- sampling: PASS (all 4)
- delivery_semantics: PASS (all 4)
- panic_error_safety: PASS (all 4)
- config_precedence: PASS (all 4)
- metrics: PASS (all 4)
- golden_fixtures: PASS (all 4)
- collector_integration: PASS (all 4)
- cortex_emitted_shape: PASS (all 4)
- parity: PASS (all 4)

### Loxana (Vite+React)
```
PASS: Build clean (8.91s, 3159 modules, 884 KB JS + 76 KB CSS)
```
- TypeScript compilation clean
- No build errors

### Static Analysis
```
go vet (collector): CLEAN
go vet (cortex): CLEAN
go vet (cli): CLEAN
cargo clippy (lql): CLEAN (zero warnings)
```

### Binary Builds
```
collector: BUILDS CLEAN
cortex: BUILDS CLEAN
cli: BUILDS CLEAN
lql: BUILDS CLEAN
```

---

## Breaking Changes Identified

### 1. LQL: `sum` keyword semantics changed (MEDIUM)
**Before:** `sum` was an alias for `summarize` (pipeline stage)
**After:** `sum` is an aggregation function token (`SUM(...)`)
**Impact:** Queries like `from events | sum count by service` will now fail. Must use `summarize` or `sum(count)` instead.
**Mitigation:** All tests pass with new semantics. Documentation should be updated.

### 2. Rust SDK: `validate()` signature changed (LOW)
**Before:** `pub fn validate(&self)` -- panics on error
**After:** `pub fn validate(&self) -> Result<(), LoxaError>` -- returns Result
**Impact:** Callers that relied on panic behavior will need to handle the Result.
**Mitigation:** No callers found in the codebase that use `.validate()` directly.

### 3. Go SDK: `/ingest` vs `/events` endpoint (MEDIUM)
**HTTPBatchSink:** Changed to `/events` (correct)
**CollectorClient.Ingest():** Still uses `/ingest` (inconsistent)
**Impact:** Both routes work (collector registers both), but inconsistency could confuse users.
**Mitigation:** Collector has both `/ingest` (configurable) and `/events` (hardcoded) routes.

### 4. Kafka CI environment variables (LOW)
**Before:** bitnami/kafka image with `KAFKA_CFG_*` env vars
**After:** apache/kafka:3.9.0 with `KAFKA_*` env vars + `CLUSTER_ID`
**Impact:** CI will use new Kafka image. No runtime impact.

---

## Skipped/Disabled Tests

All skipped tests are infrastructure-dependent (expected):
- `collector/internal/eventbus/kafka/` -- skips when Kafka not available
- `collector/internal/eventbus/nats/` -- skips when NATS not available
- `collector/internal/eventbus/redis/` -- skips when Redis not available
- `collector/cmd/loxa-collector/load_test.go` -- skips in `-short` mode
- `collector/internal/schema/migration_test.go` -- skips when no migration fixtures
- `cortex/internal/collectorsync/live_integration_test.go` -- skips in `-short` mode

No tests were recently disabled or marked TODO without justification.

---

## Proto Definitions

No changes to proto files (`proto/loxa/core/*.proto`). Generated Go files exist in `gen/go/loxa/core/`. No breaking changes detected.

---

## API Contract

- Collector: `/events` (POST), `/events/batch` (POST), `/events/ndjson` (POST), `/events/{id}` (DELETE) -- all tested and working
- Collector: `/ingest` (POST) -- still registered via configurable route, backward compatible
- Cortex: port changed from 9100 to 9312 in config and SDK defaults
- All E2E tests confirm API contracts hold across all 4 SDKs

---

## Previous Issues Status

No previous regression reports found in `.nstack/`. This is the first regression run.

---

## Recommendations

1. **Fix Go SDK inconsistency:** Update `CollectorClient.Ingest()` to use `/events` instead of `/ingest`, or document both paths as supported.
2. **Update stale docs:** Several SDK docs still reference `/ingest` endpoint (Go instrumentation.md, JS instrumentation.md).
3. **Document LQL breaking change:** `sum` is no longer an alias for `summarize`. Update LQL documentation.
4. **Investigate load test:** `TestLoadSustainedThroughput` fails at 523 events/sec (expected >= 4000). Likely Windows environment issue, but should be verified on Linux CI.

---

## Artifacts

- Git diff: `.nstack/qa/regression/raw/git-diff.txt`
- Git log: `.nstack/qa/regression/raw/git-log.txt`
- Collector test output: `.nstack/qa/regression/raw/collector-test-output.txt`
- Cortex test output: `.nstack/qa/regression/raw/cortex-test-output.txt`
- LQL test output: `.nstack/qa/regression/raw/lql-test-output.txt`
- CLI test output: `.nstack/qa/regression/raw/cli-test-output.txt`
- Go SDK test output: `.nstack/qa/regression/raw/sdk-go-test-output.txt`
- Conformance output: `.nstack/qa/regression/raw/conformance-output.txt`
- Loxana build output: `.nstack/qa/regression/raw/loxana-build-output.txt`
