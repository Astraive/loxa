#!/usr/bin/env bash
# check-proto-sync.sh -- Verify that generated protobuf Go code matches proto source files.
# Exits with code 1 if any generated files are stale.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== Proto Sync Check ==="
echo "Repo: $REPO_ROOT"
echo ""

# Step 1: Verify all proto source files exist
PROTO_SRC="$REPO_ROOT/proto/loza/core"
EXPECTED_PROTOS=("event.proto" "ingest.proto" "collector.proto" "cortex.proto")

for proto in "${EXPECTED_PROTOS[@]}"; do
    if [ ! -f "$PROTO_SRC/$proto" ]; then
        echo "✗ Missing proto source: $PROTO_SRC/$proto"
        exit 1
    fi
    echo "✓ Found: proto/loza/core/$proto"
done

# Step 2: Check that generated Go files exist
GEN_DIR="$REPO_ROOT/gen/go/loza/core"
EXPECTED_GEN=(
    "event.pb.go" "ingest.pb.go" "collector.pb.go" "cortex.pb.go"
    "ingest_grpc.pb.go" "collector_grpc.pb.go" "cortex_grpc.pb.go"
)

for gen in "${EXPECTED_GEN[@]}"; do
    if [ ! -f "$GEN_DIR/$gen" ]; then
        echo "✗ Missing generated file: gen/go/loza/core/$gen (run 'make proto')"
        exit 1
    fi
    echo "✓ Found: gen/go/loza/core/$gen"
done

# Step 3: Regenerate and compare (primary check, avoids timestamp false-positives)
echo ""
echo "Regenerating proto code and comparing..."

# Backup current generated files
BACKUP_DIR=$(mktemp -d)
cp "$GEN_DIR"/*.pb.go "$GEN_DIR"/*_grpc.pb.go "$BACKUP_DIR/" 2>/dev/null || true

# Regenerate
(cd "$REPO_ROOT" && make proto) || {
    echo "✗ Failed to regenerate proto files"
    rm -rf "$BACKUP_DIR"
    exit 1
}

# Compare with backup
FAILED=0
for f in "$BACKUP_DIR"/*.pb.go; do
    basename=$(basename "$f")
    if [ ! -f "$GEN_DIR/$basename" ]; then
        echo "✗ Missing generated file: gen/go/loza/core/$basename"
        FAILED=1
    elif ! cmp -s "$f" "$GEN_DIR/$basename"; then
        echo "✗ Out of date: gen/go/loza/core/$basename (regeneration produces different output)"
        FAILED=1
    fi
done

rm -rf "$BACKUP_DIR"

if [ "$FAILED" -eq 1 ]; then
    echo ""
    echo "✗ Proto sync check FAILED — run 'make proto' and commit the updated files."
    exit 1
fi

echo "✓ All generated files match after regeneration"

echo ""
echo "✓ Proto sync check passed"
