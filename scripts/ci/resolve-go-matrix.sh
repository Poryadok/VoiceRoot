#!/usr/bin/env bash
# Resolve backend Go CI matrix from paths-filter JSON (FILTER_JSON) and FORCE_FULL.
# Writes go_services (JSON array), run_pkg, run_go to GITHUB_OUTPUT.
#
# S2S dependency map (blast radius): when a service changes, also test dependents that
# call it over gRPC — messaging<->chat/file/realtime, user<->social/space/notification,
# space<->user/role, voice<->realtime; any Go service change also runs gateway tests.
# Trade-off: wider CI than minimal path filter; narrower than global.
set -euo pipefail
command -v jq >/dev/null || { echo 'resolve-go-matrix: jq is required' >&2; exit 1; }
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

GO_SERVICES=(
  analytics bot chat controlledgame federation file gameintegration gateway
  matchmaking messaging moderation notification realtime role search social
  space story subscription user voice
)

filter_val() {
  local key="$1"
  if [[ -n "${FILTER_JSON:-}" ]]; then
    local value
    value="$(printf '%s' "${FILTER_JSON}" | jq -r --arg k "${key}" '.[$k] // "false"')" || {
      echo "resolve-go-matrix: cannot read filter ${key}" >&2
      exit 1
    }
    value="${value%$'\r'}"
    case "$value" in
      true) return 0 ;;
      false) return 1 ;;
      *) echo "resolve-go-matrix: invalid filter ${key}" >&2; exit 1 ;;
    esac
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

add_integration_unique() {
  local svc="$1"
  local existing
  for existing in "${integration_services[@]:-}"; do
    [[ "${existing}" == "${svc}" ]] && return 0
  done
  integration_services+=("${svc}")
}

expand_s2s_deps() {
  local seed=("$@")
  local svc
  local had_deployable_seed=false
  for svc in "${seed[@]}"; do
    add_unique "$svc"
    [[ "$svc" == "controlledgame" ]] || had_deployable_seed=true
    case "$svc" in
      messaging) add_unique chat; add_unique file; add_unique realtime ;;
      chat) add_unique messaging; add_unique file ;;
      file) add_unique messaging; add_unique story ;;
      realtime) add_unique messaging; add_unique voice ;;
      voice) add_unique realtime ;;
      notification) add_unique user ;;
      user) add_unique social; add_unique space; add_unique notification ;;
      social) add_unique user ;;
      space) add_unique user; add_unique role ;;
      role) add_unique space ;;
    esac
  done
  if [[ "${had_deployable_seed}" == true ]]; then
    add_unique gateway
  fi
}

services=()
integration_services=()
run_pkg=false

# Shared source, generated contracts and global wiring can affect any service;
# keep full integration coverage until a narrower consumer inventory is proven.
if truthy "${FORCE_FULL:-}" || filter_val global || filter_val ci_global || filter_val protos || filter_val pkg || filter_val go_build_shared || filter_val auth; then
  integration_services=("${GO_SERVICES[@]}")
else
  for svc in "${GO_SERVICES[@]}"; do
    if filter_val "svc_${svc}"; then
      add_integration_unique "${svc}"
      # The callback receiver contract is consumed by Game Integration.
      if [[ "${svc}" == "controlledgame" ]]; then
        add_integration_unique gameintegration
      fi
      # Event consumers and dedicated invalidations: docs/CONTRACT_MATRIX.md.
      case "${svc}" in
        user) consumers=(analytics social notification federation realtime) ;;
        social) consumers=(analytics notification chat federation) ;;
        role) consumers=(analytics notification federation realtime voice) ;;
        messaging) consumers=(analytics notification search moderation realtime) ;;
        chat) consumers=(analytics notification realtime search) ;;
        space) consumers=(analytics notification realtime search voice) ;;
        voice) consumers=(analytics notification realtime) ;;
        moderation) consumers=(analytics notification user) ;;
        subscription) consumers=(analytics user space voice file notification) ;;
        file) consumers=(analytics messaging) ;;
        matchmaking) consumers=(analytics notification voice chat) ;;
        story) consumers=(analytics notification matchmaking) ;;
        federation) consumers=(analytics role moderation) ;;
        bot) consumers=(analytics messaging) ;;
        notification|search|gateway) consumers=(analytics) ;;
        *) consumers=() ;;
      esac
      for consumer in "${consumers[@]}"; do add_integration_unique "${consumer}"; done
    fi
  done
fi

if truthy "${FORCE_FULL:-}" || filter_val global || filter_val ci_global || filter_val protos || filter_val pkg || filter_val go_build_shared; then
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

# Docker source consumers are independent of S2S test edges. Include their
# ordinary checks too, so every selected build has a backend-go matrix entry.
docker_json="$(python3 "${ROOT}/scripts/ci/resolve-go-docker-matrix.py")"
docker_services="$(printf '%s' "${docker_json}" | jq -r '.[]')"
while IFS= read -r svc; do
  svc="${svc%$'\r'}"
  [[ -n "${svc}" ]] && add_unique "${svc}"
done <<<"${docker_services}"

# Preserve existing RPC-neighbour and source-consumer coverage for full tests.
for svc in "${services[@]}"; do
  add_integration_unique "${svc}"
done

if ((${#services[@]} == 0)); then
  go_json='[]'
  run_go=false
else
  go_json="$(printf '%s\n' "${services[@]}" | jq -R . | jq -s -c .)"
  run_go=true
fi

if ((${#integration_services[@]} == 0)); then
  integration_json='[]'
else
  integration_json="$(printf '%s\n' "${integration_services[@]}" | jq -R . | jq -s -c .)"
fi
integration_analytics=false
for svc in "${integration_services[@]}"; do
  [[ "${svc}" == analytics ]] && integration_analytics=true
done

{
  echo "go_services=${go_json}"
  echo "docker_go_services=${docker_json}"
  echo "integration_go_services=${integration_json}"
  echo "integration_analytics=${integration_analytics}"
  echo "run_pkg=${run_pkg}"
  echo "run_go=${run_go}"
} >>"${GITHUB_OUTPUT:-/dev/stdout}"

echo "resolve-go-matrix: services=${go_json} integration=${integration_json} run_pkg=${run_pkg} run_go=${run_go}"
