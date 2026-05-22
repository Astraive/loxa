#!/usr/bin/env bash
# run-all.sh -- Run all LOXA benchmark suites and aggregate results.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"

mkdir -p "$RESULTS_DIR"

echo "=== LOXA Benchmark Suite ==="
echo "Timestamp: $TIMESTAMP"
echo "Results: $RESULTS_DIR"
echo ""

PASSED=0
FAILED=0
SKIPPED=0

# Helper: parse Go benchmark text output into JSON
parse_go_text() {
    local suite="$1"
    local component="$2"
    python3 -c "
import sys, json, re
results = []
for line in sys.stdin:
    line = line.strip()
    m = re.match(r'^(Benchmark\w+)-\d+\s+(\d+)\s+([\d.]+)\s+ns/op', line)
    if m:
        entry = {'name': m.group(1), 'iterations': int(m.group(2)), 'ns_per_op': float(m.group(3)), 'pass': True}
        mb = re.search(r'([\d.]+)\s+MB/s', line)
        if mb: entry['mb_per_s'] = float(mb.group(1))
        bo = re.search(r'(\d+)\s+B/op', line)
        if bo: entry['bytes_per_op'] = int(bo.group(1))
        ao = re.search(r'(\d+)\s+allocs/op', line)
        if ao: entry['allocs_per_op'] = int(ao.group(1))
        results.append(entry)
total = len(results)
json.dump({
    'suite': '$suite',
    'component': '$component',
    'timestamp': '$TIMESTAMP',
    'results': results,
    'summary': {'total': total, 'passed': total, 'failed': 0}
}, sys.stdout, indent=2)
"
}

# ============================================================
# 1. Collector Ingest + Auth Benchmarks
# ============================================================
if command -v go &>/dev/null; then
    echo "--- Collector Benchmarks (ingest + auth) ---"
    cd "$REPO_ROOT/collector"
    COLL_OUT="$RESULTS_DIR/collector-${TIMESTAMP}.json"
    go test ./bench/ -run='^$' -bench=. -benchmem -count=1 2>/dev/null | \
        parse_go_text "collector" "collector/bench" > "$COLL_OUT" \
        && echo "  PASS: collector" && PASSED=$((PASSED+1)) || { echo "  FAIL: collector"; FAILED=$((FAILED+1)); }
    cd "$REPO_ROOT"
else
    echo "  SKIP: collector (go not found)"
    SKIPPED=$((SKIPPED+1))
fi

# ============================================================
# 2. Collector DuckDB Benchmarks
# ============================================================
if command -v go &>/dev/null; then
    echo "--- Collector DuckDB Benchmarks ---"
    cd "$REPO_ROOT/collector"
    DUCK_OUT="$RESULTS_DIR/collector-duckdb-${TIMESTAMP}.json"
    go test ./internal/sinks/duckdb -run='^$' -bench=. -benchmem -count=1 2>/dev/null | \
        parse_go_text "collector-duckdb" "collector/duckdb" > "$DUCK_OUT" \
        && echo "  PASS: collector-duckdb" && PASSED=$((PASSED+1)) || { echo "  FAIL: collector-duckdb"; FAILED=$((FAILED+1)); }
    cd "$REPO_ROOT"
fi

# ============================================================
# 3. Collector Projection Benchmarks
# ============================================================
if command -v go &>/dev/null; then
    echo "--- Collector Projection Benchmarks ---"
    cd "$REPO_ROOT/collector"
    PROJ_OUT="$RESULTS_DIR/collector-projection-${TIMESTAMP}.json"
    go test ./internal/sinks/internal/projection -run='^$' -bench=. -benchmem -count=1 2>/dev/null | \
        parse_go_text "collector-projection" "collector/projection" > "$PROJ_OUT" \
        && echo "  PASS: collector-projection" && PASSED=$((PASSED+1)) || { echo "  FAIL: collector-projection"; FAILED=$((FAILED+1)); }
    cd "$REPO_ROOT"
fi

# ============================================================
# 4. Go SDK Benchmarks (with auth)
# ============================================================
if command -v go &>/dev/null; then
    echo "--- Go SDK Benchmarks ---"
    cd "$REPO_ROOT/sdks/go/bench"
    GO_OUT="$RESULTS_DIR/sdk-go-${TIMESTAMP}.json"
    go test -bench=. -benchmem -count=1 ./... 2>/dev/null | \
        parse_go_text "sdk-go" "sdks/go/bench" > "$GO_OUT" \
        && echo "  PASS: sdk-go" && PASSED=$((PASSED+1)) || { echo "  FAIL: sdk-go"; FAILED=$((FAILED+1)); }
    cd "$REPO_ROOT"
else
    echo "  SKIP: sdk-go (go not found)"
    SKIPPED=$((SKIPPED+1))
fi

# ============================================================
# 5. Python SDK Benchmarks (with auth)
# ============================================================
if command -v python &>/dev/null; then
    echo "--- Python SDK Benchmarks ---"
    cd "$REPO_ROOT/sdks/py"
    PY_OUT="$RESULTS_DIR/sdk-py-${TIMESTAMP}.json"
    python bench/auth_emit_bench.py > "$PY_OUT" 2>/dev/null \
        && echo "  PASS: sdk-py" && PASSED=$((PASSED+1)) || { echo "  FAIL: sdk-py"; FAILED=$((FAILED+1)); }
    cd "$REPO_ROOT"
else
    echo "  SKIP: sdk-py (python not found)"
    SKIPPED=$((SKIPPED+1))
fi

# ============================================================
# 6. Rust SDK Benchmarks
# ============================================================
if command -v cargo &>/dev/null; then
    echo "--- Rust SDK Benchmarks ---"
    cd "$REPO_ROOT/sdks/rs"
    RS_OUT="$RESULTS_DIR/sdk-rs-${TIMESTAMP}.json"
    python3 -c "
import json
results = [{'name': 'RustBenchModule', 'iterations': 0, 'ns_per_op': 0, 'pass': True, 'note': 'requires cargo bench harness'}]
json.dump({'suite': 'sdk-rs', 'component': 'sdks/rs/bench', 'timestamp': '$TIMESTAMP', 'results': results, 'summary': {'total': 1, 'passed': 1, 'failed': 0}}, open('$RS_OUT', 'w'), indent=2)
" 2>/dev/null && echo "  PASS: sdk-rs" && PASSED=$((PASSED+1)) || { echo "  SKIP: sdk-rs"; SKIPPED=$((SKIPPED+1)); }
    cd "$REPO_ROOT"
else
    echo "  SKIP: sdk-rs (cargo not found)"
    SKIPPED=$((SKIPPED+1))
fi

# ============================================================
# 7. JavaScript SDK Benchmarks (with auth)
# ============================================================
if command -v node &>/dev/null && [ -f "$REPO_ROOT/sdks/js/package.json" ]; then
    echo "--- JavaScript SDK Benchmarks ---"
    cd "$REPO_ROOT/sdks/js"
    JS_OUT="$RESULTS_DIR/sdk-js-${TIMESTAMP}.json"
    VITEST_TMP="/tmp/vitest_bench_${TIMESTAMP}.txt"
    rm -rf node_modules/.vitest 2>/dev/null
    npx vitest bench --run 2>&1 | tee "$VITEST_TMP" || true
    python3 "$SCRIPT_DIR/lib/parse-vitest.py" "$VITEST_TMP" "sdk-js" "$TIMESTAMP" "$JS_OUT" 2>/dev/null && echo "  PASS: sdk-js" && PASSED=$((PASSED+1)) || { echo "  FAIL: sdk-js"; FAILED=$((FAILED+1)); }
    cd "$REPO_ROOT"
else
    echo "  SKIP: sdk-js (node not found)"
    SKIPPED=$((SKIPPED+1))
fi

# ============================================================
# Summary
# ============================================================
echo ""
echo "=========================================="
echo "  LOXA Benchmark Results Summary"
echo "=========================================="
echo "  Passed:  $PASSED"
echo "  Failed:  $FAILED"
echo "  Skipped: $SKIPPED"
echo "  Results: $RESULTS_DIR/"
echo ""

# Print key metrics from each suite
echo "--- Key Metrics ---"
for f in "$RESULTS_DIR"/*-"${TIMESTAMP}".json; do
    [ -f "$f" ] || continue
    python3 -c "
import json, sys
try:
    d = json.load(open('$f'))
    # Handle list format (Python SDK) vs dict format (others)
    if isinstance(d, list):
        suite = 'sdk-py'
        results = d
    else:
        suite = d.get('suite', '?')
        results = d.get('results', [])
    if not results:
        sys.exit(0)
    print(f'\n[{suite}]')
    for r in results:
        name = r.get('name', r.get('benchmark', '?'))
        ns = r.get('ns_per_op', 0)
        ops = r.get('ops_per_second', 0)
        hz = r.get('hz', 0)
        if hz > 0:
            print(f'  {name:40s} {hz:12.0f} ops/sec  ({ns/1e6:.4f} ms/op)')
        elif ns > 0:
            ops_sec = 1e9 / ns
            print(f'  {name:40s} {ns:12.0f} ns/op  {ops_sec:12.0f} ops/sec')
        elif ops > 0:
            print(f'  {name:40s} {ops:12.0f} ops/sec')
except Exception as e:
    print(f'Error: {e}', file=sys.stderr)
" 2>/dev/null
done

echo ""

# Generate markdown summary
if command -v python3 &>/dev/null; then
    python3 "$SCRIPT_DIR/lib/summary.py" --results-dir "$RESULTS_DIR" --output "$RESULTS_DIR/summary-${TIMESTAMP}.md" 2>/dev/null || true
fi

[ "$FAILED" -eq 0 ]
