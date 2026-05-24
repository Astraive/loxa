"""LOXA Python SDK auth emit benchmark.

Measures emit cycle with API key authentication configured.

Usage:
    cd sdks/py
    python bench/auth_emit_bench.py
"""

from __future__ import annotations

import json
import statistics
import sys
import time
from dataclasses import dataclass

sys.path.insert(0, sys.path[0] + "/.." if sys.path[0].endswith("bench") else ".")

import loxa


# Suppress MemorySink stdout output during benchmarks
class NullSink:
    """A sink that discards all events silently."""
    def write(self, encoded: str) -> None: pass
    def flush(self) -> None: pass
    def close(self) -> None: pass


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


def _run_bench(name: str, iterations: int, setup_fn, run_fn) -> BenchmarkResult:
    logger = setup_fn()
    latencies: list[float] = []

    for _ in range(iterations):
        t0 = time.perf_counter()
        run_fn(logger)
        latencies.append((time.perf_counter() - t0) * 1_000_000)

    latencies.sort()
    total = sum(latencies) / 1_000_000

    return BenchmarkResult(
        benchmark=name,
        iterations=iterations,
        total_seconds=total,
        ops_per_second=iterations / total if total > 0 else 0,
        avg_microseconds=statistics.mean(latencies),
        p50_microseconds=statistics.median(latencies),
        p95_microseconds=latencies[int(len(latencies) * 0.95)],
        p99_microseconds=latencies[int(len(latencies) * 0.99)],
    )


def bench_emit_auth(iterations: int = 10000) -> BenchmarkResult:
    """Emit with API key configured."""
    def setup():
        sink = NullSink()
        return loxa.new(loxa.test("bench").with_sink(sink).with_api_key("lx_sec_live_kBenchKey_secret"))

    def run(logger):
        ctx = logger.start_event(loxa.Params(event="bench.auth.emit"))
        logger.finish(ctx, "success")
        logger.emit(ctx)

    return _run_bench("emit_auth", iterations, setup, run)


def bench_emit_auth_attrs(iterations: int = 10000) -> BenchmarkResult:
    """Emit with API key + enriched attributes."""
    def setup():
        sink = NullSink()
        return loxa.new(loxa.test("bench").with_sink(sink).with_api_key("lx_sec_live_kBenchKey_secret"))

    def run(logger):
        ctx = logger.start_event(loxa.Params(event="bench.auth.attrs"))
        logger.enrich(ctx,
            loxa.String("http.method", "POST"),
            loxa.String("http.path", "/api/payments"),
            loxa.Int("http.status", 200),
            loxa.Float64("payment.amount", 99.99),
            loxa.Bool("payment.success", True),
        )
        logger.finish(ctx, "success")
        logger.emit(ctx)

    return _run_bench("emit_auth_attrs", iterations, setup, run)


def bench_emit_no_auth(iterations: int = 10000) -> BenchmarkResult:
    """Emit without API key (baseline)."""
    def setup():
        sink = NullSink()
        return loxa.new(loxa.test("bench").with_sink(sink))

    def run(logger):
        ctx = logger.start_event(loxa.Params(event="bench.baseline"))
        logger.finish(ctx, "success")
        logger.emit(ctx)

    return _run_bench("emit_no_auth", iterations, setup, run)


def bench_emit_sampler_auth(iterations: int = 10000) -> BenchmarkResult:
    """Emit with sampler + API key."""
    def setup():
        sink = NullSink()
        return loxa.new(loxa.test("bench").with_sink(sink)
            .with_api_key("lx_sec_live_kBenchKey_secret")
            .with_sampler(loxa.SampleRandom(0.5)))

    def run(logger):
        ctx = logger.start_event(loxa.Params(event="bench.sampler.auth"))
        logger.finish(ctx, "success")
        logger.emit(ctx)

    return _run_bench("emit_sampler_auth", iterations, setup, run)


def bench_emit_batch_10(iterations: int = 1000) -> BenchmarkResult:
    """Emit 10 events per iteration."""
    def setup():
        sink = NullSink()
        return loxa.new(loxa.test("bench").with_sink(sink).with_api_key("lx_sec_live_kBenchKey_secret"))

    def run(logger):
        for j in range(10):
            ctx = logger.start_event(loxa.Params(event="bench.batch"))
            logger.enrich(ctx, loxa.Int("batch.index", j))
            logger.finish(ctx, "success")
            logger.emit(ctx)

    return _run_bench("emit_batch_10", iterations, setup, run)


if __name__ == "__main__":
    import sys
    print("LOXA Python SDK Auth Benchmarks", file=sys.stderr)
    print("=" * 50, file=sys.stderr)

    results = []
    for bench_fn in [bench_emit_no_auth, bench_emit_auth, bench_emit_auth_attrs,
                     bench_emit_sampler_auth, bench_emit_batch_10]:
        r = bench_fn()
        results.append(r)
        print(f"  {r.benchmark:25s} {r.avg_microseconds:10.2f} us/op  "
              f"{r.ops_per_second:12.0f} ops/sec  "
              f"p95={r.p95_microseconds:.2f}us", file=sys.stderr)

    # Output JSON to stdout for orchestrator
    print(json.dumps([r.to_dict() for r in results], indent=2))
