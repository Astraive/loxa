#!/usr/bin/env bash
# check-doc-links.sh -- Validate all markdown links resolve to existing files.
# Reports broken links with file:line and exits non-zero if any are found.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BROKEN=0
TOTAL=0

# Find all markdown files, excluding node_modules, .git, vendor
while IFS= read -r md_file; do
    dir_of_file="$(dirname "$md_file")"
    line_num=0

    while IFS= read -r line; do
        line_num=$((line_num + 1))

        # Extract markdown links: [text](path) -- handle multiple per line
        # Skip URLs (http/https), anchors-only, and mailto links
        echo "$line" | grep -oP '\[([^\]]*)\]\(([^)]+)\)' | while IFS= read -r match; do
            # Extract the path portion from inside parentheses
            link_path=$(echo "$match" | sed 's/\[[^]]*\](//' | sed 's/)$//' | sed 's/ .*//')

            # Skip external URLs, anchors, and mailto
            if [[ "$link_path" =~ ^https?:// ]] || [[ "$link_path" =~ ^# ]] || [[ "$link_path" =~ ^mailto: ]]; then
                continue
            fi

            # Strip anchor from path (e.g., file.md#section -> file.md)
            link_path="${link_path%%#*}"

            # Skip empty paths
            if [ -z "$link_path" ]; then
                continue
            fi

            # URL decode basic percent-encoded chars
            link_path=$(echo "$link_path" | sed 's/%20/ /g')

            # Resolve relative to the directory of the markdown file
            resolved="$dir_of_file/$link_path"
            resolved="$(cd "$(dirname "$resolved")" 2>/dev/null && echo "$(pwd)/$(basename "$resolved")" 2>/dev/null || echo "$resolved")"

            TOTAL=$((TOTAL + 1))

            if [ ! -e "$resolved" ]; then
                # Try from repo root
                from_root="$REPO_ROOT/$link_path"
                if [ ! -e "$from_root" ]; then
                    echo "BROKEN: ${md_file#${REPO_ROOT}/}:${line_num} -> $link_path"
                    BROKEN=$((BROKEN + 1))
                fi
            fi
        done
    done < "$md_file"
done < <(find "$REPO_ROOT" -name '*.md' -not -path '*/node_modules/*' -not -path '*/.git/*' -not -path '*/vendor/*' 2>/dev/null)

echo ""
echo "Checked $TOTAL links"
if [ "$BROKEN" -gt 0 ]; then
    echo "BROKEN: $BROKEN broken link(s) found"
    exit 1
else
    echo "OK: All links valid"
    exit 0
fi
