"""LOXA Python SDK sampler benchmark.

Measures sampler decision overhead with different sampler types.

Usage:
    cd sdks/py
    python bench/sampler_bench.py
"""

from __future__ import annotations

import json
import statistics
import sys
import time

sys.path.insert(0, sys.path[0] + "/.." if sys.path[0].endswith("bench") else ".")

import loxa


def _run_sampler_bench(
    name: str,
    sampler,
    iterations: int = 10000,
) -> dict:
    """Run a single sampler benchmark."""
    sink = loxa.MemorySink()
    logger = loxa.new(loxa.test("bench").with_sink(sink).with_sampler(sampler))

    latencies: list[float] = []

    for _ in range(iterations):
        t0 = time.perf_counter()
        ctx = logger.start_event(loxa.Params(event="bench.sampler"))
        loxa.finish(ctx, "success")
        loxa.emit(ctx)
        latencies.append((time.perf_counter() - t0) * 1_000_000)

    latencies.sort()
    return {
        "benchmark": name,
        "iterations": iterations,
        "avg_microseconds": round(statistics.mean(latencies), 2),
        "p50_microseconds": round(statistics.median(latencies), 2),
        "p95_microseconds": round(latencies[int(len(latencies) * 0.95)], 2),
        "p99_microseconds": round(latencies[int(len(latencies) * 0.99)], 2),
    }


def run_sampler_benchmarks(iterations: int = 10000) -> dict:
    """Run all sampler benchmarks."""
    results = {}

    samplers = [
        ("sampler_all", loxa.SampleAll()),
        ("sampler_none", loxa.SampleNone()),
        ("sampler_random_05", loxa.SampleRandom(0.5)),
        ("sampler_random_01", loxa.SampleRandom(0.1)),
        ("sampler_errors", loxa.SampleErrors()),
    ]

    for name, sampler in samplers:
        result = _run_sampler_bench(name, sampler, iterations)
        results[name] = result
        print(f"  {name}: {result['avg_microseconds']} us/op")

    return results


if __name__ == "__main__":
    print("Running LOXA Python SDK sampler benchmarks...")
    print()
    results = run_sampler_benchmarks()
    print()
    print(json.dumps(results, indent=2))
