#!/usr/bin/env bash
# Runs the Go test suite with race detection and fails when total statement
# coverage drops below THRESHOLD (default 80). Used by `make test` and CI.
set -euo pipefail

THRESHOLD="${COVERAGE_THRESHOLD:-80}"
PROFILE="${COVERAGE_PROFILE:-coverage.out}"

cd "$(dirname "$0")/.."

go test -race -covermode=atomic -coverprofile="${PROFILE}" ./...

total=$(go tool cover -func="${PROFILE}" | awk '/^total:/ {gsub("%", "", $3); print $3}')
echo "total statement coverage: ${total}% (threshold ${THRESHOLD}%)"

if awk -v t="${total}" -v min="${THRESHOLD}" 'BEGIN { exit !(t+0 < min+0) }'; then
  echo "coverage gate FAILED" >&2
  exit 1
fi
echo "coverage gate passed"
