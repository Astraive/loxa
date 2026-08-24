#!/usr/bin/env bash
set -euo pipefail
threshold=95
go_candidate=$(command -v go 2>/dev/null || true)
if [[ "$go_candidate" == *.exe ]]; then
  go_cmd=go.exe
elif [[ -n "$go_candidate" ]]; then
  go_cmd=go
elif command -v go.exe >/dev/null 2>&1; then
  go_cmd=go.exe
else
  echo "Go toolchain not found (expected go or go.exe)" >&2
  exit 1
fi
hash -r
go_run() {
  if [[ "$go_cmd" == "go.exe" ]]; then
    cmd.exe /c go.exe "$@"
  else
    "$go_cmd" "$@"
  fi
}
root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/loza-go-coverage.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT

check_module() {
  local dir=$1
  shift
  local name pkg profile summary pct
  name=$(basename "$dir")
  echo "== $dir =="
  pushd "$dir" >/dev/null
  for pkg in "$@"; do
    profile="$work_dir/${name}-$(echo "$pkg" | tr '/.' '__').out"
    go_run test -covermode=atomic -coverprofile="$profile" "$pkg"
    summary=$(go_run tool cover -func="$profile" | awk '/^total:/ {print $NF}')
    pct=${summary%%%}
    if [[ -z "$pct" ]]; then
      echo "coverage unavailable for $pkg" >&2
      exit 1
    fi
    printf '%s: %s%%\n' "$pkg" "$pct"
    awk -v pct="$pct" -v threshold="$threshold" 'BEGIN { exit !(pct + 0 >= threshold) }' || {
      echo "coverage gate failed: $dir $pkg is ${pct}% (required >=${threshold}%)" >&2
      exit 1
    }
  done
  popd >/dev/null
}

mapfile -t source_packages < <(cd "$root_dir" && {
  go_run list ./src/...
} | sed 's#^github.com/astraive/loza/sdks/go#.#')
check_module "$root_dir" "${source_packages[@]}"

mapfile -t middleware_packages < <(cd "$root_dir/src/middleware" && go_run list ./... | sed 's#^github.com/astraive/loza/sdks/go/src/middleware#.#')
check_module "$root_dir/src/middleware" "${middleware_packages[@]}"

mapfile -t integration_packages < <(cd "$root_dir/src/integrations" && go_run list ./... | sed 's#^github.com/astraive/loza/sdks/go/src/integrations#.#')
check_module "$root_dir/src/integrations" "${integration_packages[@]}"
