"""LOZA Python SDK encoder benchmark.

Measures JSON encoding performance with various attribute counts.

Usage:
    cd sdks/py
    python bench/encoder_bench.py
"""

from __future__ import annotations

import json
import statistics
import sys
import time

sys.path.insert(0, sys.path[0] + "/.." if sys.path[0].endswith("bench") else ".")

import loza


def run_encoder_benchmark(iterations: int = 10000) -> dict:
    """Run the encoder benchmark with varying attribute counts."""
    sink = loza.MemorySink()
    logger = loza.new(loza.test("bench").with_sink(sink))

    results = {}

    for attr_count in [0, 4, 12]:
        latencies: list[float] = []

        for _ in range(iterations):
            ctx = logger.start_event(loza.Params(event="bench.encoder"))

            if attr_count >= 4:
                loza.enrich(ctx,
                    loza.String("user.id", "u-abc123"),
                    loza.Int("status_code", 200),
                    loza.Float64("duration_ms", 42.5),
                    loza.Bool("cache_hit", True),
                )
            if attr_count >= 12:
                loza.enrich(ctx,
                    loza.String("tenant.id", "tenant-acme"),
                    loza.String("session.id", "sess-xyz"),
                    loza.String("request.id", "req-789"),
                    loza.String("trace.id", "trace-000"),
                    loza.String("span.id", "span-111"),
                    loza.String("http.method", "GET"),
                    loza.String("http.path", "/api/users"),
                    loza.String("http.user_agent", "Mozilla/5.0"),
                )

            loza.finish(ctx, "success")

            t0 = time.perf_counter()
            loza.emit(ctx)
            latencies.append((time.perf_counter() - t0) * 1_000_000)

        latencies.sort()
        key = f"encoder_{attr_count}_attrs"
        results[key] = {
            "benchmark": key,
            "iterations": iterations,
            "attr_count": attr_count,
            "avg_microseconds": round(statistics.mean(latencies), 2),
            "p50_microseconds": round(statistics.median(latencies), 2),
            "p95_microseconds": round(latencies[int(len(latencies) * 0.95)], 2),
            "p99_microseconds": round(latencies[int(len(latencies) * 0.99)], 2),
        }
        print(f"  {key}: {results[key]['avg_microseconds']} us/op")

    return results


if __name__ == "__main__":
    print("Running LOZA Python SDK encoder benchmarks...")
    print()
    results = run_encoder_benchmark()
    print()
    print(json.dumps(results, indent=2))
