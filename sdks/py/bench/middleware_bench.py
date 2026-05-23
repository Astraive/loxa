"""LOXA Python SDK ASGI middleware benchmark.

Measures the overhead of ASGI middleware per request.

Usage:
    cd sdks/py
    python bench/middleware_bench.py
"""

from __future__ import annotations

import json
import statistics
import sys
import time

sys.path.insert(0, sys.path[0] + "/.." if sys.path[0].endswith("bench") else ".")

import loxa


def run_middleware_simulation_benchmark(iterations: int = 10000) -> dict:
    """Simulate what ASGI middleware does per request and measure overhead."""
    sink = loxa.MemorySink()
    loxa.new(loxa.test("bench").with_sink(sink))

    latencies: list[float] = []

    for _ in range(iterations):
        t0 = time.perf_counter()

        # 1. Start HTTP event (what middleware does on request start)
        ctx = loxa.start_http_event(loxa.Params(
            event="GET /api/users",
            kind="http",
        ))

        # 2. Enrich with HTTP metadata
        loxa.enrich(ctx,
            loxa.String("http.user_agent", "Mozilla/5.0"),
            loxa.String("http.remote_ip", "127.0.0.1"),
        )

        # 3. Finish with response metadata (what middleware does on response end)
        loxa.finish(ctx, "success",
            loxa.Int("status_code", 200),
            loxa.Int("duration_ms", 15),
        )

        # 4. Emit
        loxa.emit(ctx)

        latencies.append((time.perf_counter() - t0) * 1_000_000)

    latencies.sort()
    total = sum(latencies) / 1_000_000

    return {
        "benchmark": "asgi_middleware_simulation",
        "iterations": iterations,
        "total_seconds": round(total, 4),
        "ops_per_second": round(iterations / total if total > 0 else 0, 1),
        "avg_microseconds": round(statistics.mean(latencies), 2),
        "p50_microseconds": round(statistics.median(latencies), 2),
        "p95_microseconds": round(latencies[int(len(latencies) * 0.95)], 2),
        "p99_microseconds": round(latencies[int(len(latencies) * 0.99)], 2),
    }


def run_plain_emit_benchmark(iterations: int = 10000) -> dict:
    """Baseline: plain emit without middleware overhead."""
    sink = loxa.MemorySink()
    logger = loxa.new(loxa.test("bench").with_sink(sink))

    latencies: list[float] = []

    for _ in range(iterations):
        t0 = time.perf_counter()
        ctx = logger.start_event(loxa.Params(event="bench.plain"))
        loxa.finish(ctx, "success")
        loxa.emit(ctx)
        latencies.append((time.perf_counter() - t0) * 1_000_000)

    latencies.sort()
    total = sum(latencies) / 1_000_000

    return {
        "benchmark": "plain_emit",
        "iterations": iterations,
        "total_seconds": round(total, 4),
        "ops_per_second": round(iterations / total if total > 0 else 0, 1),
        "avg_microseconds": round(statistics.mean(latencies), 2),
        "p50_microseconds": round(statistics.median(latencies), 2),
        "p95_microseconds": round(latencies[int(len(latencies) * 0.95)], 2),
        "p99_microseconds": round(latencies[int(len(latencies) * 0.99)], 2),
    }


if __name__ == "__main__":
    print("Running LOXA Python SDK middleware benchmarks...")
    print()

    r1 = run_plain_emit_benchmark()
    print(f"  plain_emit: {r1['avg_microseconds']} us/op, {r1['ops_per_second']} ops/sec")

    r2 = run_middleware_simulation_benchmark()
    print(f"  asgi_middleware: {r2['avg_microseconds']} us/op, {r2['ops_per_second']} ops/sec")

    overhead = r2["avg_microseconds"] - r1["avg_microseconds"]
    print(f"  middleware overhead: {overhead:.2f} us/request")

    print()
    print(json.dumps([r1, r2], indent=2))
