#!/usr/bin/env bash
# run-collector.sh -- Run collector sink conformance suite.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$RESULTS_DIR"

if ! command -v go &>/dev/null; then
    echo "ERROR: go is required for collector conformance tests"
    exit 1
fi

echo "=== Collector Sink Conformance ==="
echo "Timestamp: $TIMESTAMP"
echo ""

cd "$REPO_ROOT/collector"
OUT="$RESULTS_DIR/collector-conformance-${TIMESTAMP}.json"

go test ./internal/sinks/conformance/... -json -count=1 -v 2>/dev/null | \
    python3 -c "
import sys, json
results = []
passed = 0
failed = 0
for line in sys.stdin:
    try:
        obj = json.loads(line)
        if obj.get('Action') == 'pass' and obj.get('Test'):
            results.append({'name': obj['Test'], 'pass': True})
            passed += 1
        elif obj.get('Action') == 'fail' and obj.get('Test'):
            results.append({'name': obj['Test'], 'pass': False})
            failed += 1
    except: pass
json.dump({
    'suite': 'collector-conformance',
    'component': 'collector',
    'timestamp': '$TIMESTAMP',
    'results': results,
    'summary': {'total': len(results), 'passed': passed, 'failed': failed}
}, sys.stdout, indent=2)
" > "$OUT"

echo "PASS: collector-conformance" || echo "FAIL: collector-conformance"

cd "$REPO_ROOT"
echo "Results: $OUT"
