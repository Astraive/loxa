#!/usr/bin/env bash
# release-all.sh -- Orchestrates release for all LOXA components.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v0.0.1"
    exit 1
fi

# Validate version format
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "ERROR: Version must match vX.Y.Z format (got: $VERSION)"
    exit 1
fi

echo "=== LOXA Release: $VERSION ==="
echo ""

# Ensure clean working tree
if [ -n "$(git status --porcelain)" ]; then
    echo "ERROR: Working tree is not clean. Commit or stash changes first."
    exit 1
fi

# Run lint checks
echo "--- Running doc lint ---"
"$SCRIPT_DIR/lint-docs.sh" || { echo "WARN: Doc lint failed, continuing..."; }

# Run conformance tests
echo "--- Running conformance ---"
"$REPO_ROOT/conformance/run-all.sh" || { echo "WARN: Conformance had failures"; }

# Tag the release
echo "--- Tagging $VERSION ---"
git tag -a "$VERSION" -m "Release $VERSION"
echo "Tagged: $VERSION"

echo ""
echo "=== Release $VERSION prepared ==="
echo "Push the tag with: git push origin $VERSION"
