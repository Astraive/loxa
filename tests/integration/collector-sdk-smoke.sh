#!/usr/bin/env bash
# collector-sdk-smoke.sh -- Integration smoke test: collector + SDK event flow.
# Starts collector, emits a test event, queries it, and verifies storage.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

COLLECTOR_PORT="${COLLECTOR_PORT:-9090}"
COLLECTOR_URL="http://localhost:${COLLECTOR_PORT}"
COLLECTOR_PID=""
CLEANUP_DONE=0

cleanup() {
    if [ "$CLEANUP_DONE" -eq 1 ]; then
        return
    fi
    CLEANUP_DONE=1
    if [ -n "$COLLECTOR_PID" ] && kill -0 "$COLLECTOR_PID" 2>/dev/null; then
        echo "Stopping collector (PID=$COLLECTOR_PID)..."
        kill "$COLLECTOR_PID" 2>/dev/null || true
        wait "$COLLECTOR_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

echo "=== Collector SDK Smoke Test ==="
echo "Collector URL: $COLLECTOR_URL"
echo ""

# --- Start collector ---
echo "Starting collector..."
cd "$REPO_ROOT/collector"

# Try to build and run the collector
if command -v go &>/dev/null; then
    go build -o /tmp/loxa-collector-smoke ./cmd/collector 2>/dev/null || \
    go build -o /tmp/loxa-collector-smoke . 2>/dev/null || {
        echo "FAIL: Could not build collector"
        exit 1
    }
    /tmp/loxa-collector-smoke --addr ":${COLLECTOR_PORT}" &
    COLLECTOR_PID=$!
else
    echo "FAIL: go is required to build the collector"
    exit 1
fi

# --- Wait for health check ---
echo "Waiting for collector health check..."
RETRIES=30
HEALTHY=0
for i in $(seq 1 $RETRIES); do
    if curl -sf "${COLLECTOR_URL}/health" >/dev/null 2>&1; then
        HEALTHY=1
        break
    fi
    sleep 0.5
done

if [ "$HEALTHY" -eq 0 ]; then
    echo "FAIL: Collector did not become healthy after ${RETRIES} attempts"
    exit 1
fi
echo "  Collector is healthy (PID=$COLLECTOR_PID)"

# --- Emit test event ---
echo "Emitting test event..."
EVENT_MARKER="smoke-$(date +%s)"
PAYLOAD=$(cat <<EOF
{
    "service": "smoke-test",
    "event": "test.smoke",
    "kind": "http",
    "outcome": "success",
    "attrs": {
        "http.method": "POST",
        "http.path": "/api/v1/test",
        "http.status_code": 200,
        "test.marker": "${EVENT_MARKER}"
    }
}
EOF
)

EMIT_RESPONSE=$(curl -sf -X POST "${COLLECTOR_URL}/api/v1/ingest" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" 2>/dev/null) || {
    echo "FAIL: Could not emit event to collector"
    exit 1
}
echo "  Emit response: $EMIT_RESPONSE"

# --- Wait for storage ---
sleep 1

# --- Query and verify ---
echo "Querying stored events..."
QUERY_RESPONSE=$(curl -sf "${COLLECTOR_URL}/api/v1/query?service=smoke-test&limit=10" 2>/dev/null) || {
    echo "FAIL: Could not query events"
    exit 1
}

# Check if our marker is in the response
if echo "$QUERY_RESPONSE" | grep -q "$EVENT_MARKER"; then
    echo "  Found event with marker: $EVENT_MARKER"
else
    echo "FAIL: Event not found in query response"
    echo "  Response: $QUERY_RESPONSE"
    exit 1
fi

# --- Verify with version/status endpoints ---
echo "Checking version endpoint..."
curl -sf "${COLLECTOR_URL}/version" >/dev/null 2>&1 && echo "  /version OK" || echo "  WARN: /version not available"

echo "Checking status endpoint..."
curl -sf "${COLLECTOR_URL}/api/v1/status" >/dev/null 2>&1 && echo "  /api/v1/status OK" || echo "  WARN: /api/v1/status not available"

echo ""
echo "=== Smoke Test PASSED ==="
echo "  Collector PID: $COLLECTOR_PID"
echo "  Event marker: $EVENT_MARKER"
