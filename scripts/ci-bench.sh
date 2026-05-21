#!/usr/bin/env bash
# ci-bench.sh -- CI entry point for benchmark workflow.
# Runs all benchmarks and prepares results for artifact upload.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== CI Benchmark Workflow ==="
echo "Repo: $REPO_ROOT"
echo ""

# Run all benchmarks
"$REPO_ROOT/bench/run-all.sh"
EXIT_CODE=$?

# If in GitHub Actions, set output for artifact upload
if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "results_dir=$REPO_ROOT/bench/results" >> "$GITHUB_OUTPUT"
    echo "exit_code=$EXIT_CODE" >> "$GITHUB_OUTPUT"
fi

exit $EXIT_CODE
