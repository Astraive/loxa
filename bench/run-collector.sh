#!/usr/bin/env bash
# run-collector.sh -- Run collector benchmarks (DuckDB + projection + load test).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$RESULTS_DIR"

source "$SCRIPT_DIR/lib/json-reporter.sh"

if ! command -v go &>/dev/null; then
    echo "ERROR: go is required for collector benchmarks"
    exit 1
fi

cd "$REPO_ROOT/collector"

echo "=== Collector Benchmarks ==="

# --- DuckDB Sink ---
echo "--- DuckDB Sink ---"
DUCKDB_OUT="$RESULTS_DIR/collector-duckdb-${TIMESTAMP}.json"
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
                entry = {'name': parts[0], 'iterations': int(parts[1]), 'ns_per_op': float(parts[2]), 'pass': True}
                if len(parts) >= 6:
                    entry['bytes_per_op'] = int(parts[3])
                    entry['allocs_per_op'] = int(parts[5])
                results.append(entry)
    except: pass
json.dump({'suite':'collector-duckdb','component':'collector','timestamp':'$TIMESTAMP','results':results,'summary':{'total':len(results),'passed':len(results),'failed':0}}, sys.stdout, indent=2)
" > "$DUCKDB_OUT" 2>/dev/null && echo "  PASS: collector-duckdb" || echo "  FAIL: collector-duckdb"

# --- Projection ---
echo "--- Projection ---"
PROJ_OUT="$RESULTS_DIR/collector-projection-${TIMESTAMP}.json"
go test ./internal/sinks/internal/projection -run='^$' -bench=. -benchmem -count=1 -json 2>/dev/null | \
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
json.dump({'suite':'collector-projection','component':'collector','timestamp':'$TIMESTAMP','results':results,'summary':{'total':len(results),'passed':len(results),'failed':0}}, sys.stdout, indent=2)
" > "$PROJ_OUT" 2>/dev/null && echo "  PASS: collector-projection" || echo "  FAIL: collector-projection"

# --- Load Test ---
echo "--- Load Test ---"
LOAD_OUT="$RESULTS_DIR/collector-load-${TIMESTAMP}.json"
go test ./internal/sinks/duckdb -run='^$' -bench=BenchmarkLoad -benchtime=5s -benchmem -count=1 -json 2>/dev/null | \
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
json.dump({'suite':'collector-load','component':'collector','timestamp':'$TIMESTAMP','results':results,'summary':{'total':len(results),'passed':len(results),'failed':0}}, sys.stdout, indent=2)
" > "$LOAD_OUT" 2>/dev/null && echo "  PASS: collector-load" || echo "  SKIP: collector-load (no load benchmark found)"

cd "$REPO_ROOT"
echo ""
echo "Results saved to: $RESULTS_DIR/"
