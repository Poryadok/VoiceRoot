#!/usr/bin/env bash
# Resolve staging CI build/promote matrix from path filters and Go service matrix.
# Writes build_services, promote_services, all_services, rollout flags, frontend flags.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CATALOG="${ROOT}/scripts/ci/staging-image-catalog.json"

filter_val() {
  local key="$1"
  if [[ -n "${FILTER_JSON:-}" ]]; then
    [[ "$(echo "${FILTER_JSON}" | jq -r --arg k "${key}" '.[$k] // "false"')" == "true" ]]
  else
    local env_key="FILTER_${key}"
    [[ "${!env_key:-}" == "true" ]]
  fi
}

truthy() {
  [[ "${1:-}" == "true" ]]
}

json_array() {
  if (("$#" == 0)); then
    echo '[]'
  else
    printf '%s\n' "$@" | jq -R . | jq -s -c .
  fi
}

contains() {
  local needle="$1"
  shift
  local x
  for x in "$@"; do
    [[ "${x}" == "${needle}" ]] && return 0
  done
  return 1
}

add_unique() {
  local name="$1"
  local x
  for x in "${build[@]:-}"; do
    [[ "${x}" == "${name}" ]] && return 0
  done
  build+=("${name}")
}

mapfile -t ALL_NAMES < <(jq -r '.images[].name' "${CATALOG}")
mapfile -t GO_NAMES < <(jq -r '.images[] | select(.language == "go") | .name' "${CATALOG}")

# Parse go_services from resolve-go-matrix (JSON array string).
go_services=()
if [[ -n "${GO_SERVICES_JSON:-}" && "${GO_SERVICES_JSON}" != "[]" ]]; then
  mapfile -t go_services < <(echo "${GO_SERVICES_JSON}" | jq -r '.[]')
fi

build=()
run_auth=false
run_web=false
run_admin=false
run_developer_portal=false
run_flutter_tier2=false
needs_full_rollout=false
needs_user_space_rollout=false

if truthy "${STAGING_FORCE_FULL_ROLLOUT:-}"; then
  needs_full_rollout=true
fi

if truthy "${FORCE_FULL:-}" || filter_val global || filter_val staging_infra || filter_val compose; then
  needs_full_rollout=true
fi

if filter_val staging_infra; then
  needs_full_rollout=true
fi

# Frontend path flags (no master bypass here).
if truthy "${FORCE_FULL:-}" || filter_val auth; then
  run_auth=true
fi
if truthy "${FORCE_FULL:-}" || filter_val frontend; then
  run_web=true
  run_flutter_tier2=true
fi
if truthy "${FORCE_FULL:-}" || filter_val admin; then
  run_admin=true
fi
if truthy "${FORCE_FULL:-}" || filter_val developer-portal; then
  run_developer_portal=true
fi
if truthy "${FORCE_FULL:-}" || filter_val frontend || filter_val protos; then
  run_flutter_tier2=true
fi

if truthy "${FORCE_FULL:-}"; then
  build=("${ALL_NAMES[@]}")
else
  # Go services from path-filtered matrix.
  for svc in "${go_services[@]}"; do
    add_unique "${svc}"
  done

  # Gateway when any go service builds or gateway path changed.
  if filter_val svc_gateway; then
    add_unique gateway
  elif ((${#go_services[@]} > 0)); then
    add_unique gateway
  fi

  if [[ "${run_auth}" == true ]]; then
    add_unique auth
  fi
  if [[ "${run_web}" == true ]]; then
    add_unique web
  fi
  if [[ "${run_developer_portal}" == true ]]; then
    add_unique developer-portal
  fi
  if [[ "${run_admin}" == true ]]; then
    add_unique admin
  fi
fi

# User/space rollout when those images change.
for dep in user space; do
  if contains "${dep}" "${build[@]:-}"; then
    needs_user_space_rollout=true
    break
  fi
done

promote=()
for name in "${ALL_NAMES[@]}"; do
  if ! contains "${name}" "${build[@]:-}"; then
    promote+=("${name}")
  fi
done

# No code changes -> empty build (caller should skip image jobs).
if ! truthy "${FORCE_FULL:-}" && [[ "${FILTER_CODE:-true}" == "false" ]]; then
  build=()
  promote=()
fi

build_json="$(json_array ${build[@]+"${build[@]}"})"
promote_json="$(json_array ${promote[@]+"${promote[@]}"})"
all_json="$(json_array ${ALL_NAMES[@]+"${ALL_NAMES[@]}"})"

build_go=()
for name in "${build[@]:-}"; do
  for go in "${GO_NAMES[@]}"; do
    if [[ "${name}" == "${go}" ]]; then
      build_go+=("${name}")
      break
    fi
  done
done
build_go_json="$(json_array ${build_go[@]+"${build_go[@]}"})"

base_sha="${BASE_SHA:-}"
if [[ -z "${base_sha}" || "${base_sha}" == "0000000000000000000000000000000000000000" ]]; then
  base_sha="${HEAD_SHA:-}"
fi

{
  echo "build_services=${build_json}"
  echo "build_go_services=${build_go_json}"
  echo "promote_services=${promote_json}"
  echo "all_services=${all_json}"
  echo "needs_full_rollout=${needs_full_rollout}"
  echo "needs_user_space_rollout=${needs_user_space_rollout}"
  echo "run_auth=${run_auth}"
  echo "run_web=${run_web}"
  echo "run_admin=${run_admin}"
  echo "run_developer_portal=${run_developer_portal}"
  echo "run_flutter_tier2=${run_flutter_tier2}"
  echo "promote_from_sha=${base_sha}"
} >>"${GITHUB_OUTPUT:-/dev/stdout}"

echo "resolve-staging-matrix: build=${build_json} promote=${promote_json} full_rollout=${needs_full_rollout} user_space=${needs_user_space_rollout}"
