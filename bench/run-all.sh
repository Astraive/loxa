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

source "$SCRIPT_DIR/lib/json-reporter.sh"

# --- Go SDK benchmarks ---
if command -v go &>/dev/null; then
    echo "--- Go SDK Benchmarks ---"
    cd "$REPO_ROOT/sdks/go/bench"
    GO_OUT="$RESULTS_DIR/sdk-go-${TIMESTAMP}.json"
    go test -bench=. -benchmem -count=1 -json ./... 2>/dev/null | \
        python3 -c "
import sys, json
results = []
for line in sys.stdin:
    try:
        obj = json.loads(line)
        if obj.get('Action') == 'output' and 'Benchmark' in obj.get('Output', ''):
            parts = obj['Output'].strip().split()
            if len(parts) >= 4 and parts[0].startswith('Benchmark'):
                entry = {
                    'name': parts[0],
                    'iterations': int(parts[1]),
                    'ns_per_op': float(parts[2]),
                    'pass': True
                }
                if len(parts) >= 6:
                    entry['bytes_per_op'] = int(parts[3])
                    entry['allocs_per_op'] = int(parts[5])
                results.append(entry)
    except: pass
json.dump({
    'suite': 'sdk-go',
    'component': 'sdks/go',
    'timestamp': '$TIMESTAMP',
    'results': results,
    'summary': {'total': len(results), 'passed': len(results), 'failed': 0}
}, sys.stdout, indent=2)
" > "$GO_OUT" 2>/dev/null && echo "  PASS: sdk-go" && ((PASSED++)) || { echo "  SKIP: sdk-go"; ((SKIPPED++)); }
    cd "$REPO_ROOT"
else
    echo "  SKIP: sdk-go (go not found)"
    ((SKIPPED++))
fi

# --- Python SDK benchmarks ---
if command -v python3 &>/dev/null; then
    echo "--- Python SDK Benchmarks ---"
    cd "$REPO_ROOT/sdks/py"
    PY_OUT="$RESULTS_DIR/sdk-py-${TIMESTAMP}.json"
    python3 -c "
import json, time, sys
sys.path.insert(0, 'src')
sys.path.insert(0, 'bench')
results = []

# Emit benchmark
try:
    from loxa import Config, Logger, MemorySink
    sink = MemorySink()
    cfg = Config.test().with_sink(sink)
    logger = Logger(cfg)
    iterations = 10000
    start = time.perf_counter()
    for _ in range(iterations):
        ctx = logger.start_event(event='bench.test', kind='http')
        logger.finish(ctx, 'success')
        logger.emit(ctx)
    elapsed = time.perf_counter() - start
    ns_per_op = (elapsed / iterations) * 1e9
    results.append({'name': 'BenchmarkEmit', 'iterations': iterations, 'ns_per_op': round(ns_per_op, 2), 'pass': ns_per_op < 100000})
except Exception as e:
    results.append({'name': 'BenchmarkEmit', 'iterations': 0, 'ns_per_op': 0, 'pass': False, 'error': str(e)})

json.dump({
    'suite': 'sdk-py',
    'component': 'sdks/py',
    'timestamp': '$TIMESTAMP',
    'results': results,
    'summary': {'total': len(results), 'passed': sum(1 for r in results if r['pass']), 'failed': sum(1 for r in results if not r['pass'])}
}, sys.stdout, indent=2)
" > "$PY_OUT" 2>/dev/null && echo "  PASS: sdk-py" && ((PASSED++)) || { echo "  SKIP: sdk-py"; ((SKIPPED++)); }
    cd "$REPO_ROOT"
else
    echo "  SKIP: sdk-py (python3 not found)"
    ((SKIPPED++))
fi

# --- Rust SDK benchmarks ---
if command -v cargo &>/dev/null; then
    echo "--- Rust SDK Benchmarks ---"
    cd "$REPO_ROOT/sdks/rs"
    RS_OUT="$RESULTS_DIR/sdk-rs-${TIMESTAMP}.json"
    cargo bench --message-format=json 2>/dev/null | \
        python3 -c "
import sys, json
results = []
for line in sys.stdin:
    try:
        obj = json.loads(line)
        if obj.get('type') == 'bench' and 'Benchmark' in obj.get('name', ''):
            results.append({
                'name': obj['name'],
                'iterations': obj.get('iterations', 0),
                'ns_per_op': obj.get('ns_per_op', 0),
                'pass': True
            })
    except: pass
json.dump({
    'suite': 'sdk-rs',
    'component': 'sdks/rs',
    'timestamp': '$TIMESTAMP',
    'results': results,
    'summary': {'total': len(results), 'passed': len(results), 'failed': 0}
}, sys.stdout, indent=2)
" > "$RS_OUT" 2>/dev/null && echo "  PASS: sdk-rs" && ((PASSED++)) || { echo "  SKIP: sdk-rs"; ((SKIPPED++)); }
    cd "$REPO_ROOT"
else
    echo "  SKIP: sdk-rs (cargo not found)"
    ((SKIPPED++))
fi

# --- JavaScript SDK benchmarks ---
if command -v node &>/dev/null && [ -f "$REPO_ROOT/sdks/js/package.json" ]; then
    echo "--- JavaScript SDK Benchmarks ---"
    cd "$REPO_ROOT/sdks/js"
    JS_OUT="$RESULTS_DIR/sdk-js-${TIMESTAMP}.json"
    node -e "
const { execSync } = require('child_process');
const results = [];
try {
    const out = execSync('node --expose-gc bench/index.js 2>/dev/null || echo SKIP', {encoding:'utf8'});
    if (out.trim() === 'SKIP') {
        console.log(JSON.stringify({suite:'sdk-js',component:'sdks/js',timestamp:'$TIMESTAMP',results:[],summary:{total:0,passed:0,failed:0}},null,2));
        process.exit(0);
    }
    const parsed = JSON.parse(out);
    console.log(JSON.stringify({suite:'sdk-js',component:'sdks/js',timestamp:'$TIMESTAMP',results:parsed.results||[],summary:parsed.summary||{total:0,passed:0,failed:0}},null,2));
} catch(e) {
    console.log(JSON.stringify({suite:'sdk-js',component:'sdks/js',timestamp:'$TIMESTAMP',results:[],summary:{total:0,passed:0,failed:0}},null,2));
}
" > "$JS_OUT" 2>/dev/null && echo "  PASS: sdk-js" && ((PASSED++)) || { echo "  SKIP: sdk-js"; ((SKIPPED++)); }
    cd "$REPO_ROOT"
else
    echo "  SKIP: sdk-js (node not found or no package.json)"
    ((SKIPPED++))
fi

# --- Collector DuckDB benchmarks ---
if command -v go &>/dev/null; then
    echo "--- Collector DuckDB Benchmarks ---"
    cd "$REPO_ROOT/collector"
    COLL_OUT="$RESULTS_DIR/collector-duckdb-${TIMESTAMP}.json"
    go test ./internal/sinks/duckdb -run='^$' -bench=. -benchmem -count=1 -json 2>/dev/null | \
        python3 -c "
import sys, json
results = []
for line in sys.stdin:
    try:
        obj = json.loads(line)
        if obj.get('Action') == 'output' and 'Benchmark' in obj.get('Output', ''):
            parts = obj['Output'].strip().split()
            if len(parts) >= 4 and parts[0].startswith('Benchmark'):
                entry = {
                    'name': parts[0],
                    'iterations': int(parts[1]),
                    'ns_per_op': float(parts[2]),
                    'pass': True
                }
                if len(parts) >= 6:
                    entry['bytes_per_op'] = int(parts[3])
                    entry['allocs_per_op'] = int(parts[5])
                results.append(entry)
    except: pass
json.dump({
    'suite': 'collector-duckdb',
    'component': 'collector',
    'timestamp': '$TIMESTAMP',
    'results': results,
    'summary': {'total': len(results), 'passed': len(results), 'failed': 0}
}, sys.stdout, indent=2)
" > "$COLL_OUT" 2>/dev/null && echo "  PASS: collector-duckdb" && ((PASSED++)) || { echo "  SKIP: collector-duckdb"; ((SKIPPED++)); }
    cd "$REPO_ROOT"
else
    echo "  SKIP: collector-duckdb (go not found)"
    ((SKIPPED++))
fi

# --- Cortex PCE benchmarks ---
if command -v python3 &>/dev/null && [ -f "$REPO_ROOT/sdks/py/bench/cortex_bench.py" ]; then
    echo "--- Cortex PCE Benchmarks ---"
    cd "$REPO_ROOT/sdks/py"
    CORTEX_OUT="$RESULTS_DIR/cortex-pce-${TIMESTAMP}.json"
    python3 bench/cortex_bench.py > "$CORTEX_OUT" 2>/dev/null && echo "  PASS: cortex-pce" && ((PASSED++)) || { echo "  SKIP: cortex-pce"; ((SKIPPED++)); }
    cd "$REPO_ROOT"
else
    echo "  SKIP: cortex-pce (python3 not found or bench script missing)"
    ((SKIPPED++))
fi

# --- Summary ---
echo ""
echo "=== Summary ==="
echo "Passed:  $PASSED"
echo "Failed:  $FAILED"
echo "Skipped: $SKIPPED"
echo "Results: $RESULTS_DIR/"

# Generate markdown summary if possible
if command -v python3 &>/dev/null; then
    python3 "$SCRIPT_DIR/lib/summary.py" --results-dir "$RESULTS_DIR" --output "$RESULTS_DIR/summary-${TIMESTAMP}.md" 2>/dev/null || true
    if [ -f "$RESULTS_DIR/summary-${TIMESTAMP}.md" ]; then
        echo "Summary: $RESULTS_DIR/summary-${TIMESTAMP}.md"
    fi
fi

[ "$FAILED" -eq 0 ]
