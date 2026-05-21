#!/usr/bin/env bash
# lint-docs.sh -- Run doc link and kebab-case checks.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== Documentation Lint ==="
echo ""

FAIL=0

echo "--- Checking doc links ---"
"$REPO_ROOT/tools/check-doc-links.sh" || FAIL=1
echo ""

echo "--- Checking kebab-case filenames ---"
"$REPO_ROOT/tools/check-kebab-case.sh" || FAIL=1
echo ""

if [ "$FAIL" -ne 0 ]; then
    echo "FAIL: Documentation lint errors found"
    exit 1
else
    echo "OK: All documentation lint checks passed"
    exit 0
fi
