#!/usr/bin/env bash
# run-sdk.sh -- Run SDK conformance tests for one or all SDKs.
# Usage: ./conformance/run-sdk.sh --sdk go|py|rs|js|all
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

RUNNER="$REPO_ROOT/spec/conformance/runner.py"
if ! command -v python3 &>/dev/null; then
    echo "ERROR: python3 is required for conformance testing"
    exit 1
fi
if [ ! -f "$RUNNER" ]; then
    echo "ERROR: $RUNNER not found"
    exit 1
fi

echo "=== SDK Conformance ==="
echo "SDK: $SDK"
echo "Timestamp: $TIMESTAMP"
echo ""

case "$SDK" in
    go|py|rs|js)
        OUT="$RESULTS_DIR/sdk-${SDK}-conformance-${TIMESTAMP}.json"
        python3 "$RUNNER" --sdk "$SDK" --verbose --output "$OUT"
        echo "Results: $OUT"
        ;;
    all)
        OUT="$RESULTS_DIR/sdk-conformance-${TIMESTAMP}.json"
        python3 "$RUNNER" --sdk all --verbose --output "$OUT"
        echo "Results: $OUT"
        ;;
    *)
        echo "Unknown SDK: $SDK (valid: go, py, rs, js, all)"
        exit 1
        ;;
esac
