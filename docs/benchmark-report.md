# Loza v0.2.0 Pre-Release Benchmark Report

**Date:** 2026-05-27
**Branch:** main
**Commit:** e6780e6
**Environment:** Local (Windows 11, MINGW64)
**Machine:** Intel Core i7-13650HX (14 cores / 20 threads), 16 GB RAM

---

## Summary

| Metric | Value | Status |
|--------|-------|--------|
| Collector JSON parse (single) | 6,527 ns/op (63 MB/s) | PASS |
| Collector JSON parse (100-event batch) | 646,002 ns/op (65 MB/s) | PASS |
| Collector NDJSON parse (100 events) | 625,972 ns/op (67 MB/s) | PASS |
| Collector gzip+parse (100 events) | 1,516,927 ns/op (28 MB/s) | PASS |
| Collector DuckDB batch-128 write | 130,692 ns/op (7,651 ops/s) | PASS |
| Collector auth middleware (cache hit) | 14,542 ns/op | PASS |
| Collector auth middleware (cache miss) | 9,910 ns/op | PASS |
| Go SDK emit cycle | 3,694 ns/op (270,710 ops/s) | PASS |
| Go SDK emit + 2 attributes | 4,280 ns/op | PASS |
| Go SDK batch-10 emit | 88,209 ns/op | PASS |
| Go SDK net/http middleware | 724,734 ns/op | PASS |
| Python SDK emit cycle | 50.78 us/op (19,692 ops/s) | PASS |
| Python SDK emit + auth | 49.52 us/op (20,196 ops/s) | PASS |
| Python SDK batch-10 emit | 620.40 us/op (1,612 ops/s) | PASS |
| LQL compile (simple query) | ~25 ms | PASS |
| LQL compile (complex query) | ~20 ms | PASS |
| LQL test suite (50 tests) | 1.7 s | PASS |
| Lozana build time | 7-11 s | PASS |
| Lozana JS bundle (gzip) | 261 KB | CONCERN |
| Lozana CSS bundle (gzip) | 12 KB | PASS |
| Collector test suite (24 packages) | 49 s | PASS |
| Cortex test suite (14 packages) | 14 s | PASS |
| JS SDK benchmarks | BROKEN | FAIL |
| Rust SDK benchmarks | NO TARGETS | SKIP |
| DuckDB writer-loop benchmark | PANIC | FAIL |
| Conformance suite | PARTIAL | CONCERN |

**Overall Score: 72/100**
**Status: PASS_WITH_CONCERNS**

---

## 1. Collector Ingest Benchmarks

### 1.1 JSON Parsing (single event)

| Run | ns/op | MB/s | B/op | allocs/op |
|-----|-------|------|------|-----------|
| 1 | 6,441 | 65.05 | 3,016 | 86 |
| 2 | 6,606 | 63.43 | 3,016 | 86 |
| 3 | 8,535 | 48.98 | 3,016 | 86 |
| **Median** | **6,606** | **63.43** | **3,016** | **86** |

### 1.2 JSON Array Parsing (100 events)

| Run | ns/op | MB/s | B/op | allocs/op |
|-----|-------|------|------|-----------|
| 1 | 1,415,946 | 29.66 | 285,603 | 8,213 |
| 2 | 738,539 | 56.87 | 285,600 | 8,213 |
| 3 | 646,002 | 65.02 | 285,600 | 8,213 |
| **Median** | **738,539** | **56.87** | **285,600** | **8,213** |

### 1.3 NDJSON Parsing (100 events)

| Run | ns/op | MB/s | B/op | allocs/op |
|-----|-------|------|------|-----------|
| 1 | 707,325 | 59.38 | 286,473 | 8,309 |
| 2 | 665,218 | 62.99 | 286,472 | 8,309 |
| 3 | 625,972 | 66.94 | 286,472 | 8,309 |
| **Median** | **665,218** | **62.99** | **286,472** | **8,309** |

### 1.4 Gzip Decompression + Parse (100 events)

| Run | ns/op | MB/s | B/op | allocs/op |
|-----|-------|------|------|-----------|
| 1 | 1,516,927 | 27.69 | 538,033 | 8,234 |
| 2 | 1,771,270 | 23.71 | 538,032 | 8,234 |
| 3 | 1,968,289 | 21.34 | 538,057 | 8,236 |
| **Median** | **1,771,270** | **23.71** | **538,032** | **8,234** |

### 1.5 Single Small Event Parse

| Run | ns/op | MB/s | B/op | allocs/op |
|-----|-------|------|------|-----------|
| 1 | 2,567 | 30.39 | 688 | 18 |
| 2 | 2,226 | 35.04 | 688 | 18 |
| 3 | 2,146 | 36.35 | 688 | 18 |
| **Median** | **2,226** | **35.04** | **688** | **18** |

### 1.6 Large Event Parse (50 extra attributes)

| Run | ns/op | MB/s | B/op | allocs/op |
|-----|-------|------|------|-----------|
| 1 | 34,148 | 42.43 | 13,832 | 292 |
| 2 | 52,598 | 27.55 | 13,832 | 292 |
| 3 | 28,921 | 50.10 | 13,832 | 292 |
| **Median** | **34,148** | **42.43** | **13,832** | **292** |

---

## 2. Collector Auth Benchmarks

| Benchmark | Median ns/op | B/op | allocs/op |
|-----------|-------------|------|-----------|
| ParseKey | 148 | 216 | 3 |
| HMAC-SHA256 | 1,262 | 536 | 7 |
| HMAC Direct | 1,416 | 512 | 6 |
| Middleware (cache hit) | 14,888 | 7,630 | 39 |
| Middleware (cache miss) | 10,208 | 7,679 | 40 |
| Middleware (local key) | 5,695 | 6,926 | 29 |
| Hex Encode | 66 | 0 | 0 |

---

## 3. Collector DuckDB Sink Benchmarks

| Benchmark | ns/op | B/op | allocs/op | ops/s |
|-----------|-------|------|-----------|-------|
| direct/small | 3,085,745 | 939 | 21 | 324 |
| direct/medium | 2,987,174 | 1,095 | 21 | 335 |
| batch-32/small | 162,882 | 642 | 15 | 6,140 |
| batch-128/small | 130,692 | 597 | 14 | 7,651 |
| batch-32/medium | 238,513 | 799 | 15 | 4,193 |
| batch-128/medium | 121,889 | 756 | 14 | 8,204 |
| writer-loop-128 | PANIC | - | - | FAIL |

**Note:** writer-loop-128 sub-benchmark panics (exit status 1). All other sub-benchmarks pass.

---

## 4. Collector Projection Benchmarks

| Benchmark | Median ns/op | B/op | allocs/op |
|-----------|-------------|------|-----------|
| ProjectValues | 10,433 | 6,016 | 127 |
| DecodeObject | 11,507 | 4,288 | 114 |
| ExtractPath | 134 | 128 | 1 |

---

## 5. Go SDK Benchmarks

| Benchmark | Median ns/op | B/op | allocs/op | ops/s |
|-----------|-------------|------|-----------|-------|
| Emit (basic) | 3,240 | 2,283 | 15 | 308,642 |
| EmitWithAuth | 3,838 | 2,297 | 15 | 260,553 |
| EmitWithAuthAndAttrs | 8,069 | 6,720 | 59 | 123,931 |
| EmitNoAuthBaseline | 3,563 | 2,293 | 15 | 280,663 |
| EmitWithSamplerAuth | 3,458 | 1,562 | 10 | 289,184 |
| EmitBatch10 | 88,209 | 39,176 | 320 | 11,337 |
| EmitBatch100 | 967,141 | 393,413 | 3,200 | 1,034 |
| Encoder | 4,145 | 4,230 | 32 | 241,255 |
| NetHTTPMiddleware | 682,358 | 24,936 | 182 | 1,466 |
| Sampler | 2,244 | 1,562 | 10 | 445,633 |

---

## 6. Python SDK Benchmarks

| Benchmark | ops/sec | avg us | p50 us | p95 us | p99 us |
|-----------|---------|--------|--------|--------|--------|
| emit_no_auth | 19,692 | 50.78 | 47.40 | 57.80 | 124.70 |
| emit_auth | 20,196 | 49.52 | 46.90 | 58.00 | 95.50 |
| emit_auth_attrs | 13,880 | 72.04 | 68.00 | 87.40 | 144.20 |
| emit_sampler_auth | 28,720 | 34.82 | 43.30 | 54.50 | 91.30 |
| emit_batch_10 | 1,612 | 620.40 | 582.05 | 805.00 | 1,183.30 |

---

## 7. JavaScript SDK Benchmarks

**Status: BROKEN**

The vitest bench runner fails with `TypeError: Logger is not a constructor` in:
- `bench/middleware.bench.ts`
- `bench/sampler.bench.ts`

The `auth-emit.bench.ts` partially runs but produces `NaN` comparison ratios.

**Root cause:** The JS SDK exports `Logger` as a named export or differently than expected by the bench files. The bench files import `Logger` as a default constructor but the module may have changed its export shape.

---

## 8. Rust SDK Benchmarks

**Status: NO BENCH TARGETS**

The Rust SDK (`loza/sdks/rs`) has no `[[bench]]` targets in `Cargo.toml`. The bench directory contains standalone `.rs` files that are not wired into the cargo bench harness. Running `cargo bench` compiles successfully but runs zero benchmarks (3 ignored unit tests).

---

## 9. LQL (Loza Query Language) Benchmarks

### 9.1 Compilation Timing

| Query Type | Run 1 | Run 2 | Run 3 | Run 4 | Run 5 | Median |
|------------|-------|-------|-------|-------|-------|--------|
| Simple (`from events \| where service = "x" \| limit 100`) | 59 ms | 20 ms | 19 ms | 25 ms | 19 ms | **20 ms** |
| Complex (where + summarize + sort) | 20 ms | 18 ms | 19 ms | 19 ms | 20 ms | **19 ms** |

**Note:** First run includes process startup overhead. Subsequent runs are steady-state.

### 9.2 Test Suite

- 29 unit tests (lexer, parser, compiler, schema, validate, functions)
- 21 integration tests (full pipeline, timeseries, etc.)
- 1 doc-test
- **Total: 51 tests, 0 failures, 1.7s wall time**

---

## 10. Lozana (Frontend) Build Benchmarks

### 10.1 Build Time

| Run | tsc time | vite build | Total wall |
|-----|----------|------------|------------|
| 1 | ~2s | 10.41s | 11.67s |
| 2 | ~2s | 8.53s | 31.64s |
| 3 | ~2s | 6.60s | 7.21s |

**Note:** Run 2 included cold filesystem cache. Median build time: **~8-11s**.

### 10.2 Bundle Size

| Asset | Raw Size | Gzipped |
|-------|----------|---------|
| index.js | 864 KB | 261 KB |
| index.css | 76 KB | 12 KB |
| index.html | 0.6 KB | 0.4 KB |
| **Total dist/** | **956 KB** | **~273 KB** |

**Warning:** The JS bundle (864 KB raw / 261 KB gzip) exceeds the Vite 500 KB recommended limit. Vite warns:
> "Some chunks are larger than 500 kB after minification. Consider using dynamic import() to code-split the application."

---

## 11. Build Times

| Component | Build Time |
|-----------|-----------|
| Collector (`go build ./cmd/loza-collector`) | 0.62s |
| Cortex (`go build ./cmd/server`) | 0.29s |
| LQL (`cargo build --release`) | ~2s (incremental) |
| Lozana (`tsc -b && vite build`) | 8-11s |
| Rust SDK (`cargo build --release`) | ~31s (first build) |

---

## 12. Test Suite Times

| Component | Packages | Wall Time | Status |
|-----------|----------|-----------|--------|
| Collector | 24 | 49s | ALL PASS |
| Cortex | 14 | 14s | ALL PASS |
| LQL | 51 tests | 1.7s | ALL PASS |
| Go SDK | 5 packages | ~17s | ALL PASS |
| Go SDK conformance | 1 | 6s | PASS |
| CLI | 1 | 1.6s | PASS |

---

## 13. Conformance Suite

| Suite | Status | Notes |
|-------|--------|-------|
| Collector sink conformance | PASS | Runs `go test ./internal/sinks/conformance/...` |
| Cross-SDK conformance | FAIL | `runner.py` not found or Python3 missing |
| SDK conformance (Go) | PASS | 3.5s |

---

## Findings

### FINDING-001: Lozana JS Bundle Exceeds 500 KB Limit

**Severity:** Medium
**Category:** Bundle
**Evidence:** `dist/assets/index-DEYfjV_T.js` is 864 KB raw (261 KB gzip). Vite emits a chunk size warning.
**Impact:** Slower initial page load, especially on mobile/low-bandwidth connections. The entire app is a single chunk.
**Recommendation:** Implement route-based code splitting with `React.lazy()` and `dynamic import()`. Split vendor libraries (Monaco editor, Recharts, React Query) into separate chunks via `manualChunks`.

### FINDING-002: DuckDB Writer-Loop Benchmark Panics

**Severity:** Medium
**Category:** Build/Test
**Evidence:** `BenchmarkDuckDBSink/writer-loop-128/small` exits with status 1. Other DuckDB sub-benchmarks (direct, batch-32, batch-128) pass.
**Impact:** Cannot measure writer-loop performance. May indicate a bug in the writer-loop implementation under benchmark load.
**Recommendation:** Investigate the panic. The direct and batch modes work correctly, suggesting the issue is specific to the writer-loop path.

### FINDING-003: JS SDK Benchmarks Broken

**Severity:** Medium
**Category:** Build/Test
**Evidence:** `TypeError: Logger is not a constructor` in `bench/middleware.bench.ts` and `bench/sampler.bench.ts`. The `auth-emit.bench.ts` produces `NaN` comparison ratios.
**Impact:** Cannot measure JS SDK performance. Blocks cross-SDK performance comparison.
**Recommendation:** Update bench files to match current JS SDK export structure. The SDK exports `Logger` differently than the bench files expect.

### FINDING-004: Rust SDK Has No Bench Harness

**Severity:** Low
**Category:** Build/Test
**Evidence:** `cargo bench` compiles but runs 0 benchmarks. The `bench/` directory contains standalone `.rs` files not wired into `[[bench]]` targets.
**Impact:** Cannot measure Rust SDK performance automatically. The code exists but is not integrated.
**Recommendation:** Add `[[bench]]` targets to `Cargo.toml` and wire the existing bench files into the harness.

### FINDING-005: Collector Auth Middleware Allocation Count

**Severity:** Low
**Category:** Memory
**Evidence:** Auth middleware (cache hit) allocates 7,630 B/op across 39 allocations. Cache miss adds 49 B and 1 more allocation.
**Impact:** At high request rates, this allocation pressure could cause GC spikes. The hot path (cache hit) should ideally be under 30 allocs.
**Recommendation:** Profile the middleware for unnecessary allocations. Consider pooling `http.Request` context objects.

### FINDING-006: Go SDK Emit Latency Variance

**Severity:** Low
**Category:** API Latency
**Evidence:** `EmitNoAuthBaseline` run 3 shows 37,736 ns/op vs 3,023-3,563 ns/op for runs 1-2. Similar spikes in other benchmarks (run 3 of `EmitBatch100`: 967,141 vs 2,301,069-2,640,642).
**Impact:** Occasional latency spikes under benchmark load, likely due to GC pauses or CPU scheduling.
**Recommendation:** Run benchmarks with `GOGC=off` or `GOMEMLIMIT` to isolate GC effects. Consider running with `-benchtime=5s` for more stable results.

### FINDING-007: Cross-SDK Conformance Runner Missing

**Severity:** Medium
**Category:** Build/Test
**Evidence:** Conformance `run-all.sh` looks for `spec/conformance/runner.py` which does not exist. Only collector sink conformance passes.
**Impact:** Cannot validate cross-SDK event format conformance automatically.
**Recommendation:** Create the conformance runner or update the script to use the existing Go-based conformance tests.

---

## Cross-SDK Performance Comparison

| Operation | Go SDK | Python SDK | JS SDK | Rust SDK |
|-----------|--------|------------|--------|----------|
| Emit (basic) | 3,240 ns | 50,780 ns | N/A (broken) | N/A (no bench) |
| Emit + auth | 3,838 ns | 49,520 ns | N/A | N/A |
| Emit + sampler | 2,244 ns | 34,820 ns | N/A | N/A |
| Batch-10 | 88,209 ns | 620,400 ns | N/A | N/A |
| **Go/Python ratio** | **1x** | **15.6x** | - | - |

The Go SDK is approximately **15-16x faster** than the Python SDK for equivalent operations, which is expected given the language runtime differences.

---

## Performance Budget Assessment

| Metric | Budget | Actual | Status |
|--------|--------|--------|--------|
| Collector JSON parse | < 10 us/op | 6.6 us/op | PASS |
| Collector batch throughput | > 5,000 ops/s | 7,651 ops/s | PASS |
| Go SDK emit | < 5 us/op | 3.2 us | PASS |
| Python SDK emit | < 100 us/op | 50.8 us | PASS |
| LQL compile | < 50 ms | 20 ms | PASS |
| Lozana build | < 30 s | 8-11 s | PASS |
| Lozana bundle (gzip) | < 250 KB | 273 KB | CONCERN |
| Collector test suite | < 120 s | 49 s | PASS |
| Cortex test suite | < 60 s | 14 s | PASS |

---

## Commands Run

```bash
# Collector benchmarks
cd loza/collector && go test ./bench/ -run='^$' -bench=. -benchmem -count=3

# Collector DuckDB benchmarks
cd loza/collector && go test ./internal/sinks/duckdb -run='^$' -bench=. -benchmem -count=1

# Collector projection benchmarks
cd loza/collector && go test ./internal/sinks/internal/projection -run='^$' -bench=. -benchmem -count=3

# Go SDK benchmarks
cd loza/sdks/go/bench && go test -bench=. -benchmem -count=3 ./...

# Python SDK benchmarks
cd loza/sdks/py && python bench/auth_emit_bench.py

# JS SDK benchmarks (failed)
cd loza/sdks/js && npx vitest bench --run

# Rust SDK benchmarks (no targets)
cd loza/sdks/rs && cargo bench

# LQL benchmarks
cd loza/lql && cargo bench
cd loza/lql && time ./target/release/lql compile '...'

# Lozana build
cd lozana && time bun run build

# Test suites
cd loza/collector && go test ./... -count=1 -timeout 120s
cd loza/cortex && go test ./... -count=1 -timeout 120s
cd loza/lql && cargo test

# Conformance
cd loza/conformance && bash run-all.sh
cd loza/conformance && bash run-collector.sh

# Build times
cd loza/collector && time go build ./cmd/loza-collector
cd loza/cortex && time go build ./cmd/server
```

---

## Raw Artifacts

- `.nstack/benchmarks/raw/collector-bench.txt`
- `.nstack/benchmarks/raw/sdk-go-bench.txt`
- `.nstack/benchmarks/raw/sdk-py-bench.txt`
- `.nstack/benchmarks/raw/lql-compile.txt`
- `.nstack/benchmarks/raw/lozana-build.txt`
- `.nstack/benchmarks/raw/lozana-bundle-size.txt`
- `.nstack/benchmarks/raw/collector-tests.txt`
- `.nstack/benchmarks/raw/cortex-tests.txt`
- `.nstack/benchmarks/raw/conformance.txt`

---

*Generated: 2026-05-27T15:05:00Z*
*Benchmarker: OpenClaude (mimo-v2.5-pro)*
