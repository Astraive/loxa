#!/usr/bin/env bash
# json-reporter.sh -- Shell functions for JSON benchmark output.
# Source this file from benchmark runner scripts.

# Global state for benchmark suite
_BENCH_SUITE=""
_BENCH_COMPONENT=""
_BENCH_RESULTS="[]"
_BENCH_START_TIME=""

# bench_begin <suite> <component>
# Start a new benchmark suite.
bench_begin() {
    _BENCH_SUITE="$1"
    _BENCH_COMPONENT="$2"
    _BENCH_RESULTS="[]"
    _BENCH_START_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}

# bench_result <name> <iterations> <ns_per_op> <pass> [bytes_per_op] [allocs_per_op]
# Add a single benchmark result to the current suite.
bench_result() {
    local name="$1"
    local iterations="$2"
    local ns_per_op="$3"
    local pass="$4"
    local bytes_per_op="${5:-0}"
    local allocs_per_op="${6:-0}"

    local entry
    entry=$(python3 -c "
import json
print(json.dumps({
    'name': '$name',
    'iterations': $iterations,
    'ns_per_op': $ns_per_op,
    'bytes_per_op': $bytes_per_op,
    'allocs_per_op': $allocs_per_op,
    'pass': $( [ "$pass" = "true" ] || [ "$pass" = "1" ] && echo 'True' || echo 'False' )
}))
" 2>/dev/null)

    _BENCH_RESULTS=$(python3 -c "
import json
results = json.loads('$_BENCH_RESULTS')
results.append($entry)
print(json.dumps(results))
" 2>/dev/null)
}

# bench_end
# Finalize the suite. Prints the JSON to stdout.
bench_end() {
    local total passed failed
    total=$(python3 -c "import json; r=json.loads('$_BENCH_RESULTS'); print(len(r))" 2>/dev/null)
    passed=$(python3 -c "import json; r=json.loads('$_BENCH_RESULTS'); print(sum(1 for x in r if x.get('pass')))" 2>/dev/null)
    failed=$(python3 -c "import json; r=json.loads('$_BENCH_RESULTS'); print(sum(1 for x in r if not x.get('pass')))" 2>/dev/null)

    python3 -c "
import json
print(json.dumps({
    'suite': '$_BENCH_SUITE',
    'component': '$_BENCH_COMPONENT',
    'timestamp': '$_BENCH_START_TIME',
    'results': json.loads('$_BENCH_RESULTS'),
    'summary': {'total': $total, 'passed': $passed, 'failed': $failed}
}, indent=2))
" 2>/dev/null
}

# bench_write_json <file>
# Finalize the suite and write JSON to the given file.
bench_write_json() {
    local file="$1"
    bench_end > "$file"
}
