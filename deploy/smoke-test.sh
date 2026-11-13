#!/usr/bin/env bash
# Post-deploy smoke test against the public endpoint.
#
# Checks what a release must never break: liveness and readiness, the public
# read path, that write endpoints reject anonymous callers, and that a
# security header is present. Exits non-zero on the first failure so the
# pipeline stops before a bad release is announced.
#
# Usage: deploy/smoke-test.sh https://items.staging.acme-demo.dev
set -euo pipefail

base="${1:?usage: smoke-test.sh BASE_URL}"
base="${base%/}"
curl_opts=(--silent --show-error --max-time 10 --retry 5 --retry-delay 3 --retry-connrefused)

expect_status() {
  local want="$1" path="$2" method="${3:-GET}" body="${4:-}"
  local got
  if [[ -n "$body" ]]; then
    got="$(curl "${curl_opts[@]}" -o /dev/null -w '%{http_code}' -X "$method" \
      -H 'Content-Type: application/json' --data "$body" "$base$path")"
  else
    got="$(curl "${curl_opts[@]}" -o /dev/null -w '%{http_code}' -X "$method" "$base$path")"
  fi
  if [[ "$got" != "$want" ]]; then
    echo "smoke: $method $path returned $got, want $want" >&2
    exit 1
  fi
  echo "smoke: $method $path -> $got"
}

expect_status 200 /healthz
expect_status 200 /readyz
expect_status 200 "/api/v1/items?limit=5"
expect_status 401 /api/v1/items POST '{"name":"smoke"}'

headers="$(curl "${curl_opts[@]}" -D - -o /dev/null "$base/healthz")"
if ! grep -qi '^strict-transport-security:' <<<"$headers"; then
  echo "smoke: Strict-Transport-Security header missing" >&2
  exit 1
fi
echo "smoke: security headers present"
echo "smoke: all checks passed for $base"
