#!/usr/bin/env bash
# Offline tests for render-taskdef.sh. No AWS calls: TASKDEF_FIXTURE replaces them.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
script="$here/../render-taskdef.sh"
fixture="$here/fixtures/live-taskdef.json"
out="$(mktemp -d)"
trap 'rm -rf "$out"' EXIT

pass=0 failed=0
check() {
  local name="$1"; shift
  if "$@"; then pass=$((pass + 1)); echo "ok   - $name"; else failed=$((failed + 1)); echo "FAIL - $name"; fi
}

TASKDEF_FIXTURE="$fixture" "$script" --container items-api --port 8080 \
  --image 111111111111.dkr.ecr.us-east-1.amazonaws.com/items-api:abc123 \
  --version abc123 --out "$out" >/dev/null

check "image is replaced on the app container" \
  bash -c "jq -e '.containerDefinitions[] | select(.name==\"items-api\") | .image | endswith(\":abc123\")' '$out/taskdef.json' >/dev/null"
check "SERVICE_VERSION is set exactly once" \
  bash -c "test \"\$(jq '[.containerDefinitions[0].environment[] | select(.name==\"SERVICE_VERSION\")] | length' '$out/taskdef.json')\" = 1"
check "SERVICE_VERSION carries the release SHA" \
  bash -c "jq -e '.containerDefinitions[0].environment[] | select(.name==\"SERVICE_VERSION\") | .value==\"abc123\"' '$out/taskdef.json' >/dev/null"
check "other environment variables are preserved" \
  bash -c "jq -e '.containerDefinitions[0].environment[] | select(.name==\"APP_ENV\") | .value==\"staging\"' '$out/taskdef.json' >/dev/null"
check "secrets wiring is preserved" \
  bash -c "jq -e '.containerDefinitions[0].secrets | length == 1' '$out/taskdef.json' >/dev/null"
check "sidecar container is untouched" \
  bash -c "jq -e '.containerDefinitions[1].image | endswith(\"sidecar:1\")' '$out/taskdef.json' >/dev/null"
check "read-only fields are removed" \
  bash -c "jq -e 'has(\"taskDefinitionArn\") | not' '$out/taskdef.json' >/dev/null && jq -e 'has(\"revision\") | not' '$out/taskdef.json' >/dev/null"
check "appspec placeholders are rendered" \
  bash -c "grep -q 'ContainerName: \"items-api\"' '$out/appspec.yaml' && grep -q 'ContainerPort: 8080' '$out/appspec.yaml' && grep -q '<TASK_DEFINITION>' '$out/appspec.yaml'"

if TASKDEF_FIXTURE="$fixture" "$script" --container missing --port 8080 --image a:b --version x --out "$out/neg" >/dev/null 2>&1; then
  failed=$((failed + 1)); echo "FAIL - unknown container is rejected"
else
  pass=$((pass + 1)); echo "ok   - unknown container is rejected"
fi
if "$script" --container items-api --port 8080 --image untagged --version x --out "$out/neg2" >/dev/null 2>&1; then
  failed=$((failed + 1)); echo "FAIL - untagged image is rejected"
else
  pass=$((pass + 1)); echo "ok   - untagged image is rejected"
fi

echo "passed=$pass failed=$failed"
[[ $failed -eq 0 ]]
