#!/usr/bin/env bash
# ci-conformance.sh -- CI entry point for conformance workflow.
# Runs all conformance tests and prepares results for artifact upload.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== CI Conformance Workflow ==="
echo "Repo: $REPO_ROOT"
echo ""

# Run all conformance tests
"$REPO_ROOT/conformance/run-all.sh"
EXIT_CODE=$?

# If in GitHub Actions, set output for artifact upload
if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "results_dir=$REPO_ROOT/conformance/results" >> "$GITHUB_OUTPUT"
    echo "exit_code=$EXIT_CODE" >> "$GITHUB_OUTPUT"
fi

exit $EXIT_CODE
