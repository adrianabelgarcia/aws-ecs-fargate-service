#!/usr/bin/env bash
# Renders the task definition and AppSpec for one release.
#
# The live task definition is read from the ECS service, so the release keeps
# every setting that Terraform and earlier releases configured (secrets, roles,
# log options, sizing). Only three things change: the image, the SERVICE_VERSION
# environment variable, and nothing else.
#
# Usage:
#   deploy/render-taskdef.sh --cluster C --service S --container N \
#       --port 8080 --image IMAGE_URI --version SHA --out DIR
#
# Offline mode for tests: set TASKDEF_FIXTURE to a describe-task-definition
# JSON file and the script skips every AWS call.
set -euo pipefail

usage() {
  sed -n '2,13p' "$0" | sed 's/^# \{0,1\}//'
  exit 2
}

cluster="" service="" container="" port="" image="" version="" out=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --cluster)   cluster="$2";   shift 2 ;;
    --service)   service="$2";   shift 2 ;;
    --container) container="$2"; shift 2 ;;
    --port)      port="$2";      shift 2 ;;
    --image)     image="$2";     shift 2 ;;
    --version)   version="$2";   shift 2 ;;
    --out)       out="$2";       shift 2 ;;
    -h|--help)   usage ;;
    *) echo "unknown argument: $1" >&2; usage ;;
  esac
done

fail() { echo "render-taskdef: $*" >&2; exit 1; }

[[ -n "$container" ]] || fail "--container is required"
[[ -n "$port" && "$port" =~ ^[0-9]+$ ]] || fail "--port must be numeric"
[[ -n "$image" ]] || fail "--image is required"
[[ "$image" == *:* || "$image" == *@sha256:* ]] || fail "--image must include a tag or digest"
[[ -n "$version" ]] || fail "--version is required"
[[ -n "$out" ]] || fail "--out is required"
command -v jq >/dev/null || fail "jq is required"

mkdir -p "$out"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

if [[ -n "${TASKDEF_FIXTURE:-}" ]]; then
  cp "$TASKDEF_FIXTURE" "$work/live.json"
else
  [[ -n "$cluster" && -n "$service" ]] || fail "--cluster and --service are required"
  command -v aws >/dev/null || fail "aws CLI is required"
  live_arn="$(aws ecs describe-services --cluster "$cluster" --services "$service" \
    --query 'services[0].taskDefinition' --output text)"
  [[ "$live_arn" == arn:aws:ecs:* ]] || fail "could not resolve live task definition for $service"
  aws ecs describe-task-definition --task-definition "$live_arn" \
    --query 'taskDefinition' --output json > "$work/live.json"
fi

# Fields the RegisterTaskDefinition API rejects or that describe a revision
# rather than define one.
jq --arg name "$container" --arg image "$image" --arg version "$version" '
  del(.taskDefinitionArn, .revision, .status, .requiresAttributes,
      .compatibilities, .registeredAt, .registeredBy, .deregisteredAt)
  | .containerDefinitions |= map(
      if .name == $name then
        .image = $image
        | .environment = (((.environment // []) | map(select(.name != "SERVICE_VERSION")))
                          + [{"name": "SERVICE_VERSION", "value": $version}])
      else . end)
  | if ([.containerDefinitions[] | select(.name == $name)] | length) != 1
    then error("container \($name) not found exactly once in task definition") else . end
' "$work/live.json" > "$out/taskdef.json"

sed -e "s|__CONTAINER_NAME__|${container}|g" -e "s|__CONTAINER_PORT__|${port}|g" \
  "$(dirname "$0")/appspec.yaml.tpl" > "$out/appspec.yaml"

echo "rendered $out/taskdef.json (image=$image, SERVICE_VERSION=$version)"
echo "rendered $out/appspec.yaml (container=$container port=$port)"
