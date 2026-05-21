#!/usr/bin/env bash
# run-cortex.sh -- Run cortex PCE benchmarks via Python SDK.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$RESULTS_DIR"

if ! command -v python3 &>/dev/null; then
    echo "ERROR: python3 is required for cortex PCE benchmarks"
    exit 1
fi

CORTEX_BENCH="$REPO_ROOT/sdks/py/bench/cortex_bench.py"
if [ ! -f "$CORTEX_BENCH" ]; then
    echo "ERROR: $CORTEX_BENCH not found"
    exit 1
fi

echo "=== Cortex PCE Benchmarks ==="
echo "Timestamp: $TIMESTAMP"

cd "$REPO_ROOT/sdks/py"
OUT="$RESULTS_DIR/cortex-pce-${TIMESTAMP}.json"

if python3 bench/cortex_bench.py > "$OUT" 2>/dev/null; then
    echo "PASS: cortex-pce"
    # Show summary from JSON
    python3 -c "
import json, sys
with open('$OUT') as f:
    data = json.load(f)
s = data.get('summary', {})
print(f\"  Total: {s.get('total',0)}  Passed: {s.get('passed',0)}  Failed: {s.get('failed',0)}\")
for r in data.get('results', []):
    status = 'PASS' if r.get('pass') else 'FAIL'
    print(f\"  {status}: {r['name']} ({r.get('ns_per_op',0):.0f} ns/op)\")
" 2>/dev/null || true
else
    echo "FAIL: cortex-pce"
    exit 1
fi

cd "$REPO_ROOT"
echo ""
echo "Results: $OUT"
