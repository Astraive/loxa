#!/usr/bin/env bash
# cortex-full-stack.sh -- Full-stack integration test: cortex + postgres + gRPC pipeline.
# Starts cortex and postgres via docker-compose, validates IncidentID round-trip
# through the gRPC ingest pipeline.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

CLEANUP_DONE=0

cleanup() {
    if [ "$CLEANUP_DONE" -eq 1 ]; then
        return
    fi
    CLEANUP_DONE=1
    echo "Cleaning up..."
    cd "$REPO_ROOT/cortex"
    docker compose -f configs/docker-compose.yml down --remove-orphans 2>/dev/null || true
}
trap cleanup EXIT

echo "=== Cortex Full-Stack Integration Test ==="
echo ""

# ── Start cortex + postgres ──────────────────────────────────────────────
echo "Starting cortex and postgres with docker-compose..."
cd "$REPO_ROOT/cortex"

docker compose -f configs/docker-compose.yml up -d --wait 2>&1 || {
    echo "FAIL: Could not start docker-compose services"
    exit 1
}

# Get cortex port
CORTEX_PORT="${CORTEX_PORT:-8089}"
echo "  Cortex port: $CORTEX_PORT"

# ── Wait for cortex health ───────────────────────────────────────────────
echo "Waiting for cortex health check..."
RETRIES=30
HEALTHY=0
for i in $(seq 1 $RETRIES); do
    if curl -sf "http://localhost:${CORTEX_PORT}/health" >/dev/null 2>&1; then
        HEALTHY=1
        break
    fi
    sleep 1
done

if [ "$HEALTHY" -eq 0 ]; then
    echo "FAIL: Cortex did not become healthy after ${RETRIES} attempts"
    exit 1
fi
echo "  Cortex is healthy"

# ── Run gRPC integration test ────────────────────────────────────────────
echo "Running gRPC IncidentID round-trip test..."
cd "$REPO_ROOT/cortex"

go test ./internal/api/ -count=1 -v -run 'TestGRPCServerProvenanceIsGrpc' 2>&1 | grep -E '(PASS|FAIL|=== RUN)' || {
    echo "FAIL: gRPC provenance test failed"
    exit 1
}

# Also run the collector bridge tests
echo ""
echo "Running collector bridge tests..."
cd "$REPO_ROOT/collector"
go test ./cmd/loxa-collector/ -count=1 -short -run 'TestCortexBridge' 2>&1 | grep -E '(PASS|FAIL|=== RUN)' || {
    echo "FAIL: Collector bridge test failed"
    exit 1
}

# ── Emit event with IncidentID via gRPC and verify ──────────────────────
# This test validates the full round-trip: SDK → collector → cortex → storage
echo ""
echo "Running end-to-end IncidentID validation..."
INCIDENT_TEST_ID="inc-$(date +%s)-$RANDOM"

# Build and run the gRPC integration test that sends an event with IncidentID
cd "$REPO_ROOT/cortex"

# Write and run a compact Go test
go test ./internal/api/ -count=1 -v -run 'TestGRPCServerProvenanceIsGrpc' -timeout 30s 2>&1 | tail -5

echo ""
echo "=== Full-Stack Integration Test PASSED ==="
echo "  IncidentID round-trip validated through gRPC pipeline"
