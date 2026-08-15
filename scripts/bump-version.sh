#!/usr/bin/env bash
#
# bump-version.sh — update the version string across all Loza components.
#
# Usage:
#   ./scripts/bump-version.sh <new-version> [old-version] [--dry-run] [--check]
#
# Flags:
#   --dry-run   Show what would change without modifying files.
#   --check     Verify all tracked files contain the same version. Exit 1 if not.
#
# The script auto-detects the old version from the first file that contains one,
# unless you pass it explicitly as the second positional argument.

set -euo pipefail

# ---------------------------------------------------------------------------
# Resolve the repo root (parent of scripts/)
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ---------------------------------------------------------------------------
# Files to update — relative to REPO_ROOT
# Each entry is: <path>|<sed-regex>
#
# The sed regex must match the *entire version-containing line fragment* so we
# can replace just the version number while preserving surrounding syntax.
# ---------------------------------------------------------------------------
FILES=(
  # Collector fallback version — used when loza.yaml cannot be found.
  "collector/internal/version/version.go"

  # CLI — version.Version is set via ldflags ("dev" default), so we skip it.
  # If it ever gets a hardcoded default, add it here.

  # Python SDK — pyproject.toml version = "X.Y.Z"
  "sdks/py/pyproject.toml"

  # Rust SDK — Cargo.toml version = "X.Y.Z"
  "sdks/rs/Cargo.toml"

  # JS SDK — package.json "version": "X.Y.Z"
  "sdks/js/package.json"
  "sdks/js/package-lock.json"


  # Cortex match — Cargo.toml version = "X.Y.Z"
  "cortex/crates/cortex-match/Cargo.toml"

  # Helm charts — version: X.Y.Z  +  appVersion: "X.Y.Z"
  "cortex/deploy/helm/cortex/Chart.yaml"
  "collector/deploy/helm/loza/Chart.yaml"


  # Umbrella release metadata
  "loza.yaml"
  # Project metadata YAML
  "collector/loza.yaml"
  "cortex/loza-cortex.yaml"
  "cli/loza-cli.yaml"
  "sdks/go/loza-go.yaml"
  "sdks/py/loza-py.yaml"
  "sdks/rs/loza-rs.yaml"
  "spec/docs/sdk-parity-manifest.json"
  "sdks/js/loza-js.yaml"
  "spec/loza-spec.yaml"

  # Goreleaser
  "collector/deploy/goreleaser.yml"


  # Docker image tags (astraive/loza-<component>:X.Y.Z)
  "cortex/configs/docker-compose.yml"

  # K8s deployment image tags
  "cortex/configs/cortex-deployment.yaml"
  "cortex/configs/k8s.yaml"
  "collector/deploy/k8s/collector-deployment.yaml"

  # Chart default image tags
  "collector/deploy/helm/loza/values.yaml"
  "cortex/deploy/helm/cortex/values.yaml"
)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
die()  { echo "ERROR: $*" >&2; exit 1; }
info() { echo "  $*"; }

usage() {
  cat <<EOF
Usage: $(basename "$0") [OPTIONS] <new-version> [old-version]

Update the version string across all Loza components.

Arguments:
  new-version    New semver version (e.g. 0.3.0)
  old-version    Current version to replace (auto-detected if omitted)

Options:
  --dry-run      Show what would change without modifying files
  --check        Verify all files have the same version; exit 1 if mismatched
  -h, --help     Show this help message
EOF
  exit 0
}

validate_semver() {
  local v="$1"
  if ! [[ "$v" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    die "Invalid semver: '$v' — expected X.Y.Z (e.g. 0.3.0)"
  fi
}

# Auto-detect the current version from the first file that has one.
detect_old_version() {
  for rel in "${FILES[@]}"; do
    local f="$REPO_ROOT/$rel"
    if [[ -f "$f" ]]; then
      local v
      v=$(grep -oE '[0-9]+\.[0-9]+\.[0-9]+' "$f" | head -1) || true
      if [[ -n "$v" ]]; then
        echo "$v"
        return 0
      fi
    fi
  done
  die "Could not auto-detect current version from any tracked file"
}

# Replace OLD_VER with NEW_VER in a file using sed.
# Prints the file path and a diff-like summary.
bump_file() {
  local file="$1"
  local old="$2"
  local new="$3"
  local dry_run="$4"

  if [[ ! -f "$file" ]]; then
    echo "  SKIP (file not found): ${file#"$REPO_ROOT/"}"
    return 0
  fi

  if ! grep -q "$old" "$file"; then
    echo "  SKIP (version '$old' not found): ${file#"$REPO_ROOT/"}"
    return 0
  fi

  if [[ "$dry_run" == "true" ]]; then
    echo "  WOULD UPDATE: ${file#"$REPO_ROOT/"}  ($old -> $new)"
  else
    # Use | as sed delimiter to avoid clashes with dots in version strings.
    sed -i "s|${old}|${new}|g" "$file"
    echo "  UPDATED: ${file#"$REPO_ROOT/"}  ($old -> $new)"
  fi
}

# --check mode: verify all files share the same version.
check_versions() {
  local -A seen  # version -> file list
  local all_ok=true

  echo "Checking version consistency across all tracked files..."
  echo ""

  check_one() {
    local file="$1"
    if [[ ! -f "$file" ]]; then
      echo "  MISSING: ${file#"$REPO_ROOT/"}"
      all_ok=false
      return
    fi
    local v
    v=$(grep -oE '[0-9]+\.[0-9]+\.[0-9]+' "$file" | head -1) || true
    if [[ -z "$v" ]]; then
      echo "  NO VERSION: ${file#"$REPO_ROOT/"}"
      all_ok=false
    else
      printf "  %-55s %s\n" "${file#"$REPO_ROOT/"}" "$v"
      seen["$v"]+="${file#"$REPO_ROOT/"} "
    fi
  }

  for rel in "${FILES[@]}"; do
    check_one "$REPO_ROOT/$rel"
  done

  echo ""
  local count=${#seen[@]}
  if [[ "$all_ok" == "true" && "$count" -eq 1 ]]; then
    local v="${!seen[*]}"
    echo "OK — all files report version $v"
    exit 0
  else
    echo "MISMATCH — found $count distinct versions:"
    for v in "${!seen[@]}"; do
      echo "  $v : ${seen[$v]}"
    done
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# Parse arguments
# ---------------------------------------------------------------------------
DRY_RUN=false
CHECK=false
NEW_VER=""
OLD_VER=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)  DRY_RUN=true; shift ;;
    --check)    CHECK=true;   shift ;;
    -h|--help)  usage ;;
    -*)         die "Unknown flag: $1" ;;
    *)
      if [[ -z "$NEW_VER" ]]; then
        NEW_VER="$1"
      elif [[ -z "$OLD_VER" ]]; then
        OLD_VER="$1"
      else
        die "Unexpected argument: $1"
      fi
      shift
      ;;
  esac
done

# --check needs no arguments
if [[ "$CHECK" == "true" ]]; then
  check_versions
fi

# Normal mode requires at least new-version
[[ -n "$NEW_VER" ]] || die "Missing required argument: <new-version>"
validate_semver "$NEW_VER"

# Detect or use provided old version
if [[ -z "$OLD_VER" ]]; then
  OLD_VER=$(detect_old_version)
  echo "Auto-detected current version: $OLD_VER"
else
  validate_semver "$OLD_VER"
fi

if [[ "$OLD_VER" == "$NEW_VER" ]]; then
  echo "Old and new versions are the same ($OLD_VER). Nothing to do."
  exit 0
fi

# ---------------------------------------------------------------------------
# Run
# ---------------------------------------------------------------------------
echo ""
if [[ "$DRY_RUN" == "true" ]]; then
  echo "DRY RUN — no files will be modified"
  echo ""
fi

echo "Bumping $OLD_VER -> $NEW_VER"
echo "---"

for rel in "${FILES[@]}"; do
  bump_file "$REPO_ROOT/$rel" "$OLD_VER" "$NEW_VER" "$DRY_RUN"
done

echo "---"

if [[ "$DRY_RUN" == "true" ]]; then
  echo "Dry run complete. No files were modified."
else
  echo "Done. Version bumped to $NEW_VER across all components."
  echo ""
  echo "Next steps:"
  echo "  1. Verify:  ./scripts/bump-version.sh --check"
  echo "  2. Build:   make build  (or equivalent)"
  echo "  3. Test:    make test"
  echo "  4. Commit:  git add -A && git commit -m \"chore: bump version to $NEW_VER\""
fi
