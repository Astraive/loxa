"""Persistent Context Engine benchmark runner.

Measures:
- Cold start time (target: <=60s)
- Ingestion throughput (target: >=1000 events/sec)
- Fast reconstruction p95 (target: <=2s)
- Deep reconstruction p95 (target: <=6s)
- Signature morphing accuracy
"""

from __future__ import annotations

import statistics
import time
import urllib.request
import json
from urllib.error import URLError

import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from bench.dataset import generate_dataset
from loxa.cortex.engine import Engine


def run_benchmark(cortex_url: str = "http://localhost:9100", seed: int = 42) -> dict:
    """Run the full benchmark suite.

    Args:
        cortex_url: URL of the cortex server
        seed: Random seed for dataset generation

    Returns:
        dict with benchmark results
    """
    print("=" * 60)
    print("LOXA Persistent Context Engine Benchmark")
    print("=" * 60)

    # Wait for cortex to be ready
    print("\nWaiting for cortex...")
    _wait_for_cortex(cortex_url)

    # Generate dataset
    print("Generating dataset...")
    dataset = generate_dataset(seed=seed)
    train = dataset["train_events"]
    eval_events = dataset["eval_events"]
    signals = dataset["eval_signals"]
    print(f"  Train: {len(train)} events")
    print(f"  Eval: {len(eval_events)} events")
    print(f"  Signals: {len(signals)}")

    results = {}

    # 1. Cold start
    print("\n--- Cold Start ---")
    engine = Engine(cortex_url)
    t0 = time.monotonic()
    engine.ingest(train)
    cold_start_s = time.monotonic() - t0
    results["cold_start_seconds"] = cold_start_s
    print(f"  Cold start: {cold_start_s:.1f}s (target: <=60s)")
    print(f"  {'PASS' if cold_start_s <= 60 else 'FAIL'}")

    # 2. Ingestion throughput
    print("\n--- Ingestion Throughput ---")
    batch_size = 1000
    n_batches = 10
    times = []
    for _ in range(n_batches):
        batch = eval_events[:batch_size]
        t0 = time.monotonic()
        engine.ingest(batch)
        elapsed = time.monotonic() - t0
        times.append(elapsed)

    avg_time = statistics.mean(times)
    throughput = batch_size / avg_time
    results["ingestion_throughput_eps"] = throughput
    results["ingestion_avg_ms"] = avg_time * 1000
    print(f"  Throughput: {throughput:.0f} events/sec (target: >=1000)")
    print(f"  Avg batch time: {avg_time*1000:.1f}ms")
    print(f"  {'PASS' if throughput >= 1000 else 'FAIL'}")

    # 3. Fast reconstruction latency
    print("\n--- Fast Reconstruction ---")
    fast_latencies = []
    for signal in signals[:10]:
        t0 = time.monotonic()
        ctx = engine.reconstruct_context(signal, mode="fast")
        elapsed = time.monotonic() - t0
        fast_latencies.append(elapsed)

    fast_p50 = statistics.median(fast_latencies)
    fast_p95 = sorted(fast_latencies)[int(len(fast_latencies) * 0.95)]
    fast_p99 = sorted(fast_latencies)[int(len(fast_latencies) * 0.99)]
    results["fast_p50_ms"] = fast_p50 * 1000
    results["fast_p95_ms"] = fast_p95 * 1000
    results["fast_p99_ms"] = fast_p99 * 1000
    print(f"  p50: {fast_p50*1000:.0f}ms")
    print(f"  p95: {fast_p95*1000:.0f}ms (target: <=2000ms)")
    print(f"  p99: {fast_p99*1000:.0f}ms")
    print(f"  {'PASS' if fast_p95 <= 2.0 else 'FAIL'}")

    # 4. Deep reconstruction latency
    print("\n--- Deep Reconstruction ---")
    deep_latencies = []
    for signal in signals[:5]:
        t0 = time.monotonic()
        ctx = engine.reconstruct_context(signal, mode="deep")
        elapsed = time.monotonic() - t0
        deep_latencies.append(elapsed)

    deep_p50 = statistics.median(deep_latencies)
    deep_p95 = sorted(deep_latencies)[int(len(deep_latencies) * 0.95)]
    results["deep_p50_ms"] = deep_p50 * 1000
    results["deep_p95_ms"] = deep_p95 * 1000
    print(f"  p50: {deep_p50*1000:.0f}ms")
    print(f"  p95: {deep_p95*1000:.0f}ms (target: <=6000ms)")
    print(f"  {'PASS' if deep_p95 <= 6.0 else 'FAIL'}")

    # 5. Context quality
    print("\n--- Context Quality ---")
    ctx = engine.reconstruct_context(signals[0], mode="fast")
    results["context_has_causal_chain"] = len(ctx.causal_chain) > 0
    results["context_has_similar"] = len(ctx.similar_past_incidents) > 0
    results["context_has_remediations"] = len(ctx.suggested_remediations) > 0
    results["context_confidence"] = ctx.confidence
    results["context_explain_length"] = len(ctx.explain)
    print(f"  Causal chain: {len(ctx.causal_chain)} edges")
    print(f"  Similar incidents: {len(ctx.similar_past_incidents)}")
    print(f"  Suggested remediations: {len(ctx.suggested_remediations)}")
    print(f"  Confidence: {ctx.confidence:.2f}")
    print(f"  Explain: {len(ctx.explain)} chars")

    # Summary
    print("\n" + "=" * 60)
    print("SUMMARY")
    print("=" * 60)
    all_pass = (
        cold_start_s <= 60
        and throughput >= 1000
        and fast_p95 <= 2.0
        and deep_p95 <= 6.0
    )
    print(f"  Overall: {'ALL PASS' if all_pass else 'SOME FAILURES'}")

    return results


def _wait_for_cortex(url: str, timeout: float = 60.0) -> None:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            req = urllib.request.Request(f"{url}/healthz", method="GET")
            with urllib.request.urlopen(req, timeout=2.0) as resp:
                if resp.status == 200:
                    return
        except (URLError, OSError):
            pass
        time.sleep(0.5)
    raise TimeoutError(f"Cortex not ready at {url} within {timeout}s")


if __name__ == "__main__":
    run_benchmark()
