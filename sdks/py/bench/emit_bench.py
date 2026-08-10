"""LOZA Python SDK emit cycle benchmark.

Measures the full emit cycle: start_event, finish, encode, deliver to MemorySink.

Usage:
    cd sdks/py
    python bench/emit_bench.py
"""

from __future__ import annotations

import json
import statistics
import sys
import time
from dataclasses import dataclass

sys.path.insert(0, sys.path[0] + "/.." if sys.path[0].endswith("bench") else ".")

import loza


@dataclass
class BenchmarkResult:
    benchmark: str
    iterations: int
    total_seconds: float
    ops_per_second: float
    avg_microseconds: float
    p50_microseconds: float
    p95_microseconds: float
    p99_microseconds: float

    def to_dict(self) -> dict:
        return {
            "benchmark": self.benchmark,
            "iterations": self.iterations,
            "total_seconds": round(self.total_seconds, 4),
            "ops_per_second": round(self.ops_per_second, 1),
            "avg_microseconds": round(self.avg_microseconds, 2),
            "p50_microseconds": round(self.p50_microseconds, 2),
            "p95_microseconds": round(self.p95_microseconds, 2),
            "p99_microseconds": round(self.p99_microseconds, 2),
        }


def run_emit_benchmark(iterations: int = 10000) -> BenchmarkResult:
    """Run the emit cycle benchmark."""
    sink = loza.MemorySink()
    logger = loza.new(loza.test("bench").with_sink(sink))

    latencies: list[float] = []

    for _ in range(iterations):
        t0 = time.perf_counter()
        ctx = logger.start_event(loza.Params(event="bench.emit"))
        loza.finish(ctx, "success")
        loza.emit(ctx)
        latencies.append((time.perf_counter() - t0) * 1_000_000)

    latencies.sort()
    total = sum(latencies) / 1_000_000

    return BenchmarkResult(
        benchmark="emit_cycle",
        iterations=iterations,
        total_seconds=total,
        ops_per_second=iterations / total if total > 0 else 0,
        avg_microseconds=statistics.mean(latencies),
        p50_microseconds=statistics.median(latencies),
        p95_microseconds=latencies[int(len(latencies) * 0.95)],
        p99_microseconds=latencies[int(len(latencies) * 0.99)],
    )


def run_emit_enriched_benchmark(iterations: int = 10000) -> BenchmarkResult:
    """Run the emit cycle benchmark with enriched attributes."""
    sink = loza.MemorySink()
    logger = loza.new(loza.test("bench").with_sink(sink))

    latencies: list[float] = []

    for _ in range(iterations):
        t0 = time.perf_counter()
        ctx = logger.start_event(loza.Params(event="bench.emit.enriched"))
        loza.enrich(ctx,
            loza.String("user.id", "u-abc123"),
            loza.Int("status_code", 200),
            loza.Float64("duration_ms", 42.5),
            loza.Bool("cache_hit", True),
        )
        loza.finish(ctx, "success")
        loza.emit(ctx)
        latencies.append((time.perf_counter() - t0) * 1_000_000)

    latencies.sort()
    total = sum(latencies) / 1_000_000

    return BenchmarkResult(
        benchmark="emit_enriched",
        iterations=iterations,
        total_seconds=total,
        ops_per_second=iterations / total if total > 0 else 0,
        avg_microseconds=statistics.mean(latencies),
        p50_microseconds=statistics.median(latencies),
        p95_microseconds=latencies[int(len(latencies) * 0.95)],
        p99_microseconds=latencies[int(len(latencies) * 0.99)],
    )


if __name__ == "__main__":
    print("Running LOZA Python SDK benchmarks...")
    print()

    results = []

    r1 = run_emit_benchmark()
    results.append(r1)
    print(f"  {r1.benchmark}: {r1.avg_microseconds:.2f} us/op, "
          f"{r1.ops_per_second:.0f} ops/sec")

    r2 = run_emit_enriched_benchmark()
    results.append(r2)
    print(f"  {r2.benchmark}: {r2.avg_microseconds:.2f} us/op, "
          f"{r2.ops_per_second:.0f} ops/sec")

    print()
    print(json.dumps([r.to_dict() for r in results], indent=2))
