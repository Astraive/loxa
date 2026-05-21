# Benchmarks

Central benchmark orchestration for the LOXA monorepo. All benchmark results are saved as JSON in `results/`.

## Quick Start

```bash
./bench/run-all.sh                    # Run all benchmarks
./bench/run-sdk.sh --sdk go           # Run Go SDK benchmarks only
./bench/run-collector.sh              # Run collector benchmarks only
./bench/run-cortex.sh                 # Run cortex PCE benchmarks only
```

## Benchmark Suites

| Suite | Component | Language | What is Measured | Target |
|-------|-----------|----------|------------------|--------|
| SDK Emit | sdks/go, sdks/py, sdks/rs, sdks/js | Go/Py/Rs/JS | Full event lifecycle (StartEvent/Finish/Emit) | <10us/op |
| SDK Encoder | sdks/go, sdks/py, sdks/rs, sdks/js | Go/Py/Rs/JS | JSON encoding with enrichment | <5us/op |
| SDK Sampler | sdks/go, sdks/py, sdks/rs, sdks/js | Go/Py/Rs/JS | Sampling decision overhead | <1us/op |
| SDK Middleware | sdks/go, sdks/py, sdks/rs, sdks/js | Go/Py/Rs/JS | HTTP middleware end-to-end | <50us/op |
| DuckDB Sink | collector | Go | Single/batch INSERT, point SELECT | <10us/op |
| Projection | collector | Go | JSON decode + path extraction | <1us/op |
| Load Test | collector | Go | Sustained throughput (5000 events/5s) | >500 events/sec |
| Cortex PCE | cortex (via sdks/py) | Python | Cold start, throughput, reconstruction latency | p95 <2s |

## JSON Output Format

Results are saved to `results/<suite>-<timestamp>.json`:

```json
{
  "suite": "sdk-go",
  "component": "sdks/go",
  "timestamp": "2026-05-22T10:30:00Z",
  "results": [
    {
      "name": "BenchmarkEmit",
      "iterations": 1000000,
      "ns_per_op": 842,
      "bytes_per_op": 384,
      "allocs_per_op": 5,
      "pass": true
    }
  ],
  "summary": { "total": 4, "passed": 4, "failed": 0 }
}
```

## Component Benchmarks

Each component has its own benchmark directory:
- `sdks/go/bench/` -- Go SDK benchmarks (4 suites)
- `sdks/py/bench/` -- Python SDK benchmarks (5 suites)
- `sdks/rs/bench/` -- Rust SDK benchmarks (4 suites)
- `sdks/js/bench/` -- JavaScript SDK benchmarks (4 suites)
- `collector/internal/sinks/duckdb/` -- DuckDB benchmarks
- `collector/internal/sinks/internal/projection/` -- Projection benchmarks
- `sdks/py/bench/cortex_bench.py` -- Cortex PCE benchmarks
