#!/usr/bin/env sh
set -eu

profile="${COVERAGE_PROFILE:-$(mktemp "${TMPDIR:-/tmp}/lql-client-go-coverage.XXXXXX.out")}"
cleanup() {
  rm -f "$profile"
}
trap cleanup EXIT INT TERM

go test -covermode=atomic -coverprofile="$profile" ./...
summary="$(go tool cover -func="$profile" | awk '$1 == "total:" { print $3 }')"
if [ -z "$summary" ]; then
  echo "coverage summary was not produced" >&2
  exit 1
fi
coverage="${summary%%%}"
awk -v coverage="$coverage" 'BEGIN {
  if (coverage + 0 < 95) {
    printf "coverage %.1f%% is below required 95%%\n", coverage + 0 > "/dev/stderr"
    exit 1
  }
  printf "coverage %.1f%% (required >=95%%)\n", coverage + 0
}'
