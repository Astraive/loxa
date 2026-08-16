#!/usr/bin/env bash
# run-sdk.sh -- Run SDK benchmarks for one or all SDKs.
# Usage: ./bench/run-sdk.sh --sdk go|py|rs|js|all
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$RESULTS_DIR"

SDK="all"
while [[ $# -gt 0 ]]; do
    case "$1" in
        --sdk) SDK="$2"; shift 2 ;;
        *) echo "Unknown arg: $1"; exit 1 ;;
    esac
done

source "$SCRIPT_DIR/lib/json-reporter.sh"

run_go_sdk() {
    if ! command -v go &>/dev/null; then
        echo "SKIP: go not found"
        return 0
    fi
    echo "--- Go SDK Benchmarks ---"
    cd "$REPO_ROOT/sdks/go/bench"
    local out="$RESULTS_DIR/sdk-go-${TIMESTAMP}.json"
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
                entry = {'name': parts[0], 'iterations': int(parts[1]), 'ns_per_op': float(parts[2]), 'pass': True}
                if len(parts) >= 6:
                    entry['bytes_per_op'] = int(parts[3])
                    entry['allocs_per_op'] = int(parts[5])
                results.append(entry)
    except: pass
json.dump({'suite':'sdk-go','component':'sdks/go','timestamp':'$TIMESTAMP','results':results,'summary':{'total':len(results),'passed':len(results),'failed':0}}, sys.stdout, indent=2)
" > "$out" 2>/dev/null && echo "  PASS: sdk-go" || echo "  FAIL: sdk-go"
    cd "$REPO_ROOT"
}

run_py_sdk() {
    if ! command -v python3 &>/dev/null; then
        echo "SKIP: python3 not found"
        return 0
    fi
    echo "--- Python SDK Benchmarks ---"
    cd "$REPO_ROOT/sdks/py"
    local out="$RESULTS_DIR/sdk-py-${TIMESTAMP}.json"
    python3 bench/cortex_bench.py > "$out" 2>/dev/null && echo "  PASS: sdk-py" || echo "  FAIL: sdk-py"
    cd "$REPO_ROOT"
}

run_rs_sdk() {
    if ! command -v cargo &>/dev/null; then
        echo "SKIP: cargo not found"
        return 0
    fi
    echo "--- Rust SDK Benchmarks ---"
    cd "$REPO_ROOT/sdks/rs"
    local out="$RESULTS_DIR/sdk-rs-${TIMESTAMP}.json"
    cargo bench --message-format=json 2>/dev/null | \
        python3 -c "
import sys, json
results = []
for line in sys.stdin:
    try:
        obj = json.loads(line)
        if obj.get('type') == 'bench':
            results.append({'name':obj['name'],'iterations':obj.get('iterations',0),'ns_per_op':obj.get('ns_per_op',0),'pass':True})
    except: pass
json.dump({'suite':'sdk-rs','component':'sdks/rs','timestamp':'$TIMESTAMP','results':results,'summary':{'total':len(results),'passed':len(results),'failed':0}}, sys.stdout, indent=2)
" > "$out" 2>/dev/null && echo "  PASS: sdk-rs" || echo "  FAIL: sdk-rs"
    cd "$REPO_ROOT"
}

run_js_sdk() {
    if ! command -v bun &>/dev/null; then
        echo "SKIP: bun not found"
        return 0
    fi
    if [ ! -f "$REPO_ROOT/sdks/js/package.json" ]; then
        echo "SKIP: sdks/js/package.json not found"
        return 0
    fi
    echo "--- JavaScript SDK Benchmarks ---"
    cd "$REPO_ROOT/sdks/js"
    local out="$RESULTS_DIR/sdk-js-${TIMESTAMP}.json"
    local tmp="/tmp/vitest_bench_${TIMESTAMP}.txt"
    bun bench/index.js 2>&1 | tee "$tmp" || true
    python3 "$SCRIPT_DIR/lib/parse-vitest.py" "$tmp" "sdk-js" "$TIMESTAMP" "$out" 2>/dev/null && echo "  PASS: sdk-js" || echo "  FAIL: sdk-js"
    cd "$REPO_ROOT"
}

case "$SDK" in
    go)  run_go_sdk ;;
    py)  run_py_sdk ;;
    rs)  run_rs_sdk ;;
    js)  run_js_sdk ;;
    all)
        run_go_sdk
        run_py_sdk
        run_rs_sdk
        run_js_sdk
        ;;
    *)
        echo "Unknown SDK: $SDK (valid: go, py, rs, js, all)"
        exit 1
        ;;
esac

echo ""
echo "Results saved to: $RESULTS_DIR/"
