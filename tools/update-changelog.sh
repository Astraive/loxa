#!/usr/bin/env bash
# update-changelog.sh -- Parse git log since last tag and generate changelog entries.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

# Find the last tag
LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
HEAD=$(git rev-parse --short HEAD)
DATE=$(date +%Y-%m-%d)

if [ -z "$LAST_TAG" ]; then
    echo "No tags found. Generating changelog from all commits."
    RANGE="$HEAD"
    SINCE_TEXT="all history"
else
    RANGE="${LAST_TAG}..HEAD"
    SINCE_TEXT="since $LAST_TAG"
    COMMIT_COUNT=$(git rev-list "$RANGE" --count 2>/dev/null || echo "0")
    if [ "$COMMIT_COUNT" = "0" ]; then
        echo "No new commits $SINCE_TEXT. Nothing to update."
        exit 0
    fi
fi

echo "# Changelog"
echo ""
echo "## [Unreleased] -- $DATE"
echo ""
echo "Changes $SINCE_TEXT:"
echo ""

# Categorize commits by conventional commit prefix
declare -A CATEGORIES
CATEGORIES=(
    ["feat"]="Features"
    ["fix"]="Bug Fixes"
    ["perf"]="Performance"
    ["refactor"]="Refactoring"
    ["docs"]="Documentation"
    ["test"]="Tests"
    ["chore"]="Chores"
    ["ci"]="CI/CD"
    ["build"]="Build"
)

# Collect commits into categories
declare -A CAT_COMMITS

while IFS= read -r line; do
    if [ -z "$line" ]; then
        continue
    fi

    matched=0
    for prefix in "${!CATEGORIES[@]}"; do
        if [[ "$line" =~ ^[a-f0-9]+\ ${prefix}: ]]; then
            msg="${line#* }"
            key="${prefix}"
            if [ -z "${CAT_COMMITS[$key]+x}" ]; then
                CAT_COMMITS[$key]=""
            fi
            CAT_COMMITS[$key]="${CAT_COMMITS[$key]}${msg}"$'\n'
            matched=1
            break
        fi
    done

    if [ "$matched" -eq 0 ]; then
        if [ -z "${CAT_COMMITS[other]+x}" ]; then
            CAT_COMMITS[other]=""
        fi
        CAT_COMMITS[other]="${CAT_COMMITS[other]}${line#* }"$'\n'
    fi
done < <(git log "$RANGE" --pretty=format:"%h %s" --no-merges 2>/dev/null)

# Output in order
for prefix in feat fix perf refactor docs test ci build chore other; do
    if [ -n "${CAT_COMMITS[$prefix]+x}" ] && [ -n "${CAT_COMMITS[$prefix]}" ]; then
        label="${CATEGORIES[$prefix]:-Other}"
        echo "### $label"
        echo ""
        while IFS= read -r commit; do
            if [ -n "$commit" ]; then
                # Strip the prefix from the message for cleaner output
                clean="${commit#${prefix}: }"
                clean="${clean#${prefix}:}"
                echo "- $clean"
            fi
        done <<< "${CAT_COMMITS[$prefix]}"
        echo ""
    fi
done

echo "---"
echo "Generated from git log $SINCE_TEXT ($HEAD)"
