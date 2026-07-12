#!/usr/bin/env bash
# Resolve backend Go CI matrix from paths-filter JSON (FILTER_JSON) and FORCE_FULL.
# Writes go_services (JSON array), run_pkg, run_go to GITHUB_OUTPUT.
#
# S2S dependency map (blast radius): when a service changes, also test dependents that
# call it over gRPC — messaging<->chat/file/realtime, user<->social/space/notification,
# space<->user/role, voice<->realtime; any Go service change also runs gateway tests.
# Trade-off: wider CI than minimal path filter; narrower than global.
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

add_unique() {
  local svc="$1"
  local s
  for s in "${services[@]:-}"; do
    [[ "$s" == "$svc" ]] && return 0
  done
  services+=("$svc")
}

expand_s2s_deps() {
  local seed=("$@")
  local svc
  local had_seed=false
  for svc in "${seed[@]}"; do
    had_seed=true
    add_unique "$svc"
    case "$svc" in
      messaging) add_unique chat; add_unique file; add_unique realtime ;;
      chat) add_unique messaging ;;
      file) add_unique messaging ;;
      realtime) add_unique messaging; add_unique voice ;;
      voice) add_unique realtime ;;
      notification) add_unique user ;;
      user) add_unique social; add_unique space; add_unique notification ;;
      social) add_unique user ;;
      space) add_unique user; add_unique role ;;
      role) add_unique space ;;
    esac
  done
  if [[ "${had_seed}" == true ]]; then
    add_unique gateway
  fi
}

services=()
run_pkg=false

if truthy "${FORCE_FULL:-}" || filter_val global || filter_val protos || filter_val pkg; then
  services=("${GO_SERVICES[@]}")
  run_pkg=true
else
  seed=()
  for svc in "${GO_SERVICES[@]}"; do
    if filter_val "svc_${svc}"; then
      seed+=("$svc")
    fi
  done
  if ((${#seed[@]} > 0)); then
    expand_s2s_deps "${seed[@]}"
  fi
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
