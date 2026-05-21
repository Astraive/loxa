#!/usr/bin/env python3
"""summary.py -- Read benchmark JSON results and produce a markdown summary table."""

import argparse
import json
import os
import sys
from datetime import datetime
from pathlib import Path


def load_results(results_dir: str) -> list[dict]:
    """Load all JSON benchmark result files from the results directory."""
    results = []
    results_path = Path(results_dir)
    if not results_path.exists():
        return results
    for f in sorted(results_path.glob("*.json")):
        if f.name.startswith("summary"):
            continue
        try:
            with open(f) as fh:
                data = json.load(fh)
            if "suite" in data:
                results.append(data)
        except (json.JSONDecodeError, OSError):
            continue
    return results


def render_markdown(results: list[dict]) -> str:
    """Render benchmark results as a markdown summary table."""
    lines = [
        "# Benchmark Summary",
        "",
        f"Generated: {datetime.utcnow().strftime('%Y-%m-%d %H:%M:%S UTC')}",
        "",
    ]

    if not results:
        lines.append("No benchmark results found.")
        return "\n".join(lines)

    # Overall summary
    total_suites = len(results)
    total_benchmarks = sum(r.get("summary", {}).get("total", 0) for r in results)
    total_passed = sum(r.get("summary", {}).get("passed", 0) for r in results)
    total_failed = sum(r.get("summary", {}).get("failed", 0) for r in results)

    lines.append("## Overall")
    lines.append("")
    lines.append(f"| Metric | Value |")
    lines.append(f"|--------|-------|")
    lines.append(f"| Suites | {total_suites} |")
    lines.append(f"| Benchmarks | {total_benchmarks} |")
    lines.append(f"| Passed | {total_passed} |")
    lines.append(f"| Failed | {total_failed} |")
    lines.append("")

    # Per-suite detail
    lines.append("## Per-Suite Results")
    lines.append("")
    lines.append("| Suite | Component | Timestamp | Total | Passed | Failed | Status |")
    lines.append("|-------|-----------|-----------|-------|--------|--------|--------|")

    for r in results:
        suite = r.get("suite", "?")
        component = r.get("component", "?")
        ts = r.get("timestamp", "?")
        s = r.get("summary", {})
        t = s.get("total", 0)
        p = s.get("passed", 0)
        f = s.get("failed", 0)
        status = "PASS" if f == 0 else "FAIL"
        lines.append(f"| {suite} | {component} | {ts} | {t} | {p} | {f} | {status} |")

    lines.append("")

    # Individual benchmark details
    lines.append("## Benchmark Details")
    lines.append("")
    lines.append("| Suite | Benchmark | Iterations | ns/op | Status |")
    lines.append("|-------|-----------|------------|-------|--------|")

    for r in results:
        suite = r.get("suite", "?")
        for b in r.get("results", []):
            name = b.get("name", "?")
            iters = b.get("iterations", 0)
            ns = b.get("ns_per_op", 0)
            status = "PASS" if b.get("pass") else "FAIL"
            lines.append(f"| {suite} | {name} | {iters:,} | {ns:,.0f} | {status} |")

    lines.append("")
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description="Generate benchmark summary from JSON results")
    parser.add_argument("--results-dir", default="results", help="Directory containing benchmark JSON files")
    parser.add_argument("--output", "-o", help="Output file (default: stdout)")
    args = parser.parse_args()

    results = load_results(args.results_dir)
    markdown = render_markdown(results)

    if args.output:
        with open(args.output, "w") as f:
            f.write(markdown)
        print(f"Summary written to {args.output}", file=sys.stderr)
    else:
        print(markdown)


if __name__ == "__main__":
    main()
