#!/usr/bin/env bash
# check-kebab-case.sh -- Check that doc filenames use kebab-case.
# Exempts: CHANGELOG.md, README.md, RELEASE.md, LICENSE
# Reports violations and exits non-zero if any are found.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

VIOLATIONS=0

# Exempt filenames (case-insensitive check)
is_exempt() {
    local basename="$1"
    case "${basename}" in
        CHANGELOG.md|README.md|RELEASE.md|LICENSE|LICENSE.md) return 0 ;;
        *) return 1 ;;
    esac
}

# Check if filename is valid kebab-case: lowercase letters, digits, hyphens only
is_kebab_case() {
    local basename="$1"
    # Must match: lowercase letter or digit, followed by lowercase letters, digits, or hyphens, ending with .md
    if [[ "$basename" =~ ^[a-z][a-z0-9-]*\.md$ ]]; then
        return 0
    fi
    return 1
}

while IFS= read -r md_file; do
    filename="$(basename "$md_file")"

    # Skip exempt files
    if is_exempt "$filename"; then
        continue
    fi

    # Skip files in node_modules, .git, vendor
    if [[ "$md_file" =~ /node_modules/ ]] || [[ "$md_file" =~ /\.git/ ]] || [[ "$md_file" =~ /vendor/ ]]; then
        continue
    fi

    if ! is_kebab_case "$filename"; then
        echo "VIOLATION: ${md_file#${REPO_ROOT}/} (expected kebab-case: lowercase with hyphens)"
        VIOLATIONS=$((VIOLATIONS + 1))
    fi
done < <(find "$REPO_ROOT" -name '*.md' -not -path '*/node_modules/*' -not -path '*/.git/*' -not -path '*/vendor/*' 2>/dev/null)

echo ""
if [ "$VIOLATIONS" -gt 0 ]; then
    echo "VIOLATIONS: $VIOLATIONS file(s) not using kebab-case"
    exit 1
else
    echo "OK: All markdown filenames are kebab-case"
    exit 0
fi
