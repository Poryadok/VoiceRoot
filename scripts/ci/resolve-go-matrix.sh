#!/usr/bin/env bash
# Resolve backend Go CI matrix from paths-filter JSON (FILTER_JSON) and FORCE_FULL.
# Writes go_services (JSON array), run_pkg, run_go to GITHUB_OUTPUT.
set -euo pipefail

GO_SERVICES=(
  analytics bot chat federation file gateway matchmaking messaging moderation
  notification realtime role search social space story subscription user voice
)

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

services=()
run_pkg=false

if truthy "${FORCE_FULL:-}" || filter_val global || filter_val protos || filter_val pkg; then
  services=("${GO_SERVICES[@]}")
  run_pkg=true
else
  for svc in "${GO_SERVICES[@]}"; do
    if filter_val "svc_${svc}"; then
      services+=("$svc")
    fi
  done
  if ((${#services[@]} > 0)) || filter_val pkg; then
    run_pkg=true
  fi
  if filter_val pkg && ((${#services[@]} == 0)); then
    services=("${GO_SERVICES[@]}")
  fi
fi

if ((${#services[@]} == 0)); then
  go_json='[]'
  run_go=false
else
  go_json="$(printf '%s\n' "${services[@]}" | jq -R . | jq -s -c .)"
  run_go=true
fi

{
  echo "go_services=${go_json}"
  echo "run_pkg=${run_pkg}"
  echo "run_go=${run_go}"
} >>"${GITHUB_OUTPUT:-/dev/stdout}"

echo "resolve-go-matrix: services=${go_json} run_pkg=${run_pkg} run_go=${run_go}"
