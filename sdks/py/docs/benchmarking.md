# Benchmarking

How to run and interpret benchmarks for the LOXA Python SDK.

## Running Benchmarks

### SDK Benchmarks

From the repository root:

```bash
cd sdks/py
python bench/emit_bench.py
python bench/encoder_bench.py
python bench/sampler_bench.py
python bench/middleware_bench.py
```

### Cortex Benchmark

The Cortex benchmark measures the Persistent Context Engine performance:

```bash
cd sdks/py
python bench/cortex_bench.py
```

This benchmark requires a running cortex server at `http://localhost:9100`. It measures:
- Cold start time (target: <=60s)
- Ingestion throughput (target: >=1000 events/sec)
- Fast reconstruction p95 (target: <=2s)
- Deep reconstruction p95 (target: <=6s)

## What Is Measured

| Benchmark | File | Measures |
|-----------|------|----------|
| Emit cycle | `emit_bench.py` | Full emit cycle: start_event, finish, encode, deliver to MemorySink. |
| Encoder | `encoder_bench.py` | Emit cycle with enrichment fields. Measures JSON encoding overhead. |
| Sampler | `sampler_bench.py` | Emit cycle with a sampler attached. Measures sampler decision overhead. |
| Middleware | `middleware_bench.py` | ASGI middleware lifecycle. Measures request capture, event creation, finish, emit. |
| Cortex PCE | `cortex_bench.py` | Cold start, ingestion throughput, fast/deep reconstruction latency. |

## Expected Results

SDK benchmarks output JSON results to stdout. Typical results on modern hardware:

| Benchmark | Time (ms/op) | Notes |
|-----------|-------------|-------|
| Emit cycle | 0.05-0.15 | MemorySink, no network. |
| Encoder | 0.08-0.20 | With 2 enriched attributes. |
| Sampler | 0.04-0.12 | SampleRandom(0.5). |
| Middleware | 0.10-0.30 | ASGI overhead per request. |

## Output Format

SDK benchmarks produce JSON output:

```json
{
  "benchmark": "emit_cycle",
  "iterations": 10000,
  "total_seconds": 0.85,
  "ops_per_second": 11764.7,
  "avg_microseconds": 85.0,
  "p50_microseconds": 80.0,
  "p95_microseconds": 120.0,
  "p99_microseconds": 200.0
}
```

## Adding New Benchmarks

Create a new file `sdks/py/bench/<name>_bench.py`:

```python
"""My feature benchmark."""

import time
import json
import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import loxa


def run_benchmark(iterations: int = 10000) -> dict:
    sink = loxa.MemorySink()
    logger = loxa.new(loxa.test("bench").with_sink(sink))

    latencies = []
    for _ in range(iterations):
        t0 = time.perf_counter()
        ctx = loxa.start_event(loxa.Params(event="bench.test"))
        loxa.finish(ctx, "success")
        loxa.emit(ctx)
        latencies.append((time.perf_counter() - t0) * 1_000_000)

    latencies.sort()
    return {
        "benchmark": "my_feature",
        "iterations": iterations,
        "avg_microseconds": sum(latencies) / len(latencies),
        "p50_microseconds": latencies[len(latencies) // 2],
        "p95_microseconds": latencies[int(len(latencies) * 0.95)],
        "p99_microseconds": latencies[int(len(latencies) * 0.99)],
    }


if __name__ == "__main__":
    result = run_benchmark()
    print(json.dumps(result, indent=2))
```
