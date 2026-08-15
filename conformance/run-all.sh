#!/usr/bin/env bash
# run-all.sh -- Run all LOZA conformance tests and aggregate results.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$RESULTS_DIR"

PYTHON_BIN="${PYTHON_BIN:-python3}"
if ! command -v "$PYTHON_BIN" >/dev/null 2>&1; then
    PYTHON_BIN="python"
fi

echo "=== LOZA Conformance Suite ==="
echo "Timestamp: $TIMESTAMP"
echo "Results: $RESULTS_DIR"
echo ""

PASSED=0
FAILED=0
SKIPPED=0

# --- Cross-SDK Conformance via spec runner ---
echo "--- Cross-SDK Conformance ---"
RUNNER="$REPO_ROOT/spec/conformance/runner.py"
if command -v "$PYTHON_BIN" >/dev/null 2>&1 && [ -f "$RUNNER" ]; then
    SDK_CONFORMANCE_OUT="$RESULTS_DIR/sdk-conformance-${TIMESTAMP}.json"
    if "$PYTHON_BIN" "$RUNNER" --sdk all --verbose --json > "$SDK_CONFORMANCE_OUT" 2>/dev/null; then
        echo "  PASS: sdk-conformance"
        PASSED=$((PASSED + 1))
    else
        echo "  FAIL: sdk-conformance"
        FAILED=$((FAILED + 1))
    fi
else
    echo "  SKIP: sdk-conformance (runner not found or python3 missing)"
    SKIPPED=$((SKIPPED + 1))
fi

# --- Collector Sink Conformance ---
echo "--- Collector Sink Conformance ---"
if command -v go &>/dev/null; then
    COLL_CONFORMANCE_OUT="$RESULTS_DIR/collector-conformance-${TIMESTAMP}.json"
    cd "$REPO_ROOT/collector"
    if go test ./internal/sinks/conformance/... -json -count=1 2>/dev/null | \
        "$PYTHON_BIN" -c "
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
            results.append({'name': obj['Test'], 'pass': False, 'error': obj.get('Output','')})
            failed += 1
    except: pass
json.dump({
    'suite': 'collector-conformance',
    'component': 'collector',
    'timestamp': '$TIMESTAMP',
    'results': results,
    'summary': {'total': len(results), 'passed': passed, 'failed': failed}
}, sys.stdout, indent=2)
" > "$COLL_CONFORMANCE_OUT" 2>/dev/null; then
        echo "  PASS: collector-conformance"
        PASSED=$((PASSED + 1))
    else
        echo "  FAIL: collector-conformance"
        FAILED=$((FAILED + 1))
    fi
    cd "$REPO_ROOT"
else
    echo "  SKIP: collector-conformance (go not found)"
    SKIPPED=$((SKIPPED + 1))
fi

# --- Python SDK Verifier (105 subchecks) ---
echo "--- Python SDK Verifier ---"
VERIFY="$REPO_ROOT/spec/conformance/verify.py"
if command -v "$PYTHON_BIN" >/dev/null 2>&1 && [ -f "$VERIFY" ]; then
    PY_VERIFY_OUT="$RESULTS_DIR/py-verify-${TIMESTAMP}.json"
    cd "$REPO_ROOT"
    if "$PYTHON_BIN" "$VERIFY" --json > "$PY_VERIFY_OUT" 2>/dev/null; then
        echo "  PASS: py-verify"
        PASSED=$((PASSED + 1))
    else
        echo "  FAIL: py-verify"
        FAILED=$((FAILED + 1))
    fi
    cd "$REPO_ROOT"
else
    echo "  SKIP: py-verify (verify.py not found or python3 missing)"
    SKIPPED=$((SKIPPED + 1))
fi

# --- Summary ---
echo ""
echo "=== Summary ==="
echo "Passed:  $PASSED"
echo "Failed:  $FAILED"
echo "Skipped: $SKIPPED"
echo "Results: $RESULTS_DIR/"

[ "$FAILED" -eq 0 ]
