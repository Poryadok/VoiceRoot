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

enable_dedicated_build() {
  local name="$1"
  add_unique "${name}"
  case "${name}" in
    auth) run_auth=true ;;
    web)
      run_web=true
      run_flutter_tier2=true
      ;;
    admin) run_admin=true ;;
    developer-portal) run_developer_portal=true ;;
  esac
}

registry_lc() {
  echo "${VOICE_IMAGE_REGISTRY}" | tr '[:upper:]' '[:lower:]'
}

promote_manifest_exists() {
  local name="$1"
  local tag="$2"
  docker manifest inspect "$(registry_lc)/${name}:${tag}" >/dev/null 2>&1
}

all_promote_manifests_exist() {
  local tag="$1"
  local name
  for name in "${promote[@]:-}"; do
    promote_manifest_exists "${name}" "${tag}" || return 1
  done
  return 0
}

# Skip SHAs where CI did not publish images (e.g. docs-only pushes).
find_promote_base_sha() {
  local sha="$1"
  local max_depth="${PROMOTE_BASE_MAX_DEPTH:-20}"
  local depth=0
  local candidate="${sha}"

  if ((${#promote[@]} == 0)); then
    echo "${sha}"
    return 0
  fi

  while [[ -n "${candidate}" && ${depth} -lt ${max_depth} ]]; do
    if all_promote_manifests_exist "${candidate}"; then
      if [[ "${candidate}" != "${sha}" ]]; then
        echo "resolve-staging-matrix: promote base ${sha} -> ${candidate} (manifest walk-back)" >&2
      fi
      echo "${candidate}"
      return 0
    fi
    candidate="$(git -C "${ROOT}" rev-parse "${candidate}^" 2>/dev/null || true)"
    depth=$((depth + 1))
  done

  echo "${sha}"
  return 1
}

# Promote only works when BASE_TAG already exists in GHCR; otherwise schedule a build.
adjust_promote_for_missing_manifests() {
  local name lang src new_promote=()
  for name in "${promote[@]:-}"; do
    src="$(registry_lc)/${name}:${base_sha}"
    if promote_manifest_exists "${name}" "${base_sha}"; then
      new_promote+=("${name}")
      continue
    fi
    echo "adjust-promote: ${name} missing at ${src}; scheduling build" >&2
    lang="$(jq -r --arg n "${name}" '.images[] | select(.name == $n) | .language' "${CATALOG}")"
    if [[ "${lang}" == "go" ]]; then
      add_unique "${name}"
    else
      enable_dedicated_build "${name}"
    fi
  done
  if ((${#new_promote[@]} > 0)); then
    promote=("${new_promote[@]}")
  else
    promote=()
  fi
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

promote=()
for name in "${ALL_NAMES[@]}"; do
  if ! contains "${name}" "${build[@]:-}"; then
    promote+=("${name}")
  fi
done

base_sha="${BASE_SHA:-}"
if [[ -z "${base_sha}" || "${base_sha}" == "0000000000000000000000000000000000000000" ]]; then
  base_sha="$(git -C "${ROOT}" rev-parse HEAD^ 2>/dev/null || true)"
fi

if truthy "${MANIFEST_CHECK:-}" && [[ -n "${VOICE_IMAGE_REGISTRY:-}" && -n "${base_sha}" ]]; then
  if ((${#promote[@]} > 0)); then
    base_sha="$(find_promote_base_sha "${base_sha}")"
  fi
  adjust_promote_for_missing_manifests
fi

# No code changes -> empty build (caller should skip image jobs).
if ! truthy "${FORCE_FULL:-}" && [[ "${FILTER_CODE:-true}" == "false" ]]; then
  build=()
  promote=()
fi

# apply-app-manifests rewrites every deployment image tag on each code deploy.
if [[ "${FILTER_CODE:-true}" == "true" ]]; then
  needs_user_space_rollout=true
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
