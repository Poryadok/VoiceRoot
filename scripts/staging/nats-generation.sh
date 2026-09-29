#!/usr/bin/env bash
# Select the active staging NATS identity generation for all ordinary deploys.
set -euo pipefail

nats_generation_valid() {
  [[ "${1:-}" =~ ^r[0-9]{8}[a-z0-9]{0,8}$ ]]
}

nats_generation_load() {
  local ns="${VOICE_K8S_NAMESPACE:-voice-staging}" marker
  [[ "$ns" == voice-staging ]] || { echo 'ERROR: NATS generation is staging-only' >&2; return 1; }
  if ! marker="$(kubectl get configmap voice-nats-generation -n "$ns" -o json 2>&1)"; then
    if [[ "$marker" == *'configmaps "voice-nats-generation" not found'* ]]; then
      marker=''
    else
      echo 'ERROR: cannot read staging NATS generation' >&2
      return 1
    fi
  fi
  NATS_GENERATION=legacy
  NATS_PREVIOUS_GENERATION=''
  NATS_MARKER_PRESENT=false
  if [[ -n "$marker" ]]; then
    local phase generation previous
    phase="$(jq -er '.data.phase // empty' <<<"$marker" 2>/dev/null)" || return 1
    generation="$(jq -er '.data.generation // empty' <<<"$marker" 2>/dev/null)" || return 1
    previous="$(jq -er '.data.previousGeneration // empty' <<<"$marker" 2>/dev/null)" || return 1
    [[ "$phase" == active ]] || { echo 'ERROR: NATS generation rotation is in progress' >&2; return 1; }
    [[ "$generation" == legacy ]] || nats_generation_valid "$generation" || {
      echo 'ERROR: invalid active NATS generation' >&2; return 1;
    }
    [[ "$previous" == legacy ]] || nats_generation_valid "$previous" || {
      echo 'ERROR: invalid previous NATS generation' >&2; return 1;
    }
    [[ "$generation" != "$previous" ]] || { echo 'ERROR: duplicate NATS generation' >&2; return 1; }
    NATS_GENERATION="$generation"
    NATS_PREVIOUS_GENERATION="$previous"
    NATS_MARKER_PRESENT=true
  fi
  if [[ "$NATS_GENERATION" == legacy ]]; then
    NATS_OPERATOR_SECRET=voice-nats-operator
    NATS_TLS_SECRET=voice-nats-hub-tls
    NATS_BOOTSTRAP_SECRET=voice-nats-bootstrap-credentials
    NATS_SERVICE_SECRET=voice-nats-service-credentials
    NATS_PVC=voice-nats-jsdata
  else
    NATS_OPERATOR_SECRET="voice-nats-operator-${NATS_GENERATION}"
    NATS_TLS_SECRET="voice-nats-hub-tls-${NATS_GENERATION}"
    NATS_BOOTSTRAP_SECRET="voice-nats-bootstrap-credentials-${NATS_GENERATION}"
    NATS_SERVICE_SECRET="voice-nats-service-credentials-${NATS_GENERATION}"
    NATS_PVC="voice-nats-jsdata-${NATS_GENERATION}"
  fi
  export NATS_GENERATION NATS_PREVIOUS_GENERATION NATS_MARKER_PRESENT NATS_OPERATOR_SECRET NATS_TLS_SECRET
  export NATS_BOOTSTRAP_SECRET NATS_SERVICE_SECRET NATS_PVC
}

nats_generation_render() {
  local source="${1:?manifest file required}"
  [[ -f "$source" ]] || { echo 'ERROR: NATS manifest missing' >&2; return 1; }
  if [[ "$NATS_GENERATION" == legacy ]]; then
    cat "$source"
  else
    sed -e "s|voice-nats-operator|${NATS_OPERATOR_SECRET}|g" \
        -e "s|voice-nats-hub-tls|${NATS_TLS_SECRET}|g" \
        -e "s|voice-nats-bootstrap-credentials|${NATS_BOOTSTRAP_SECRET}|g" \
        -e "s|voice-nats-service-credentials|${NATS_SERVICE_SECRET}|g" \
        -e "s|voice-nats-jsdata|${NATS_PVC}|g" "$source"
  fi
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  case "${1:-}" in
    --check)
      [[ $# -eq 1 ]] || exit 2
      nats_generation_load
      ;;
    --render)
      [[ $# -eq 2 ]] || exit 2
      nats_generation_load
      nats_generation_render "$2"
      ;;
    *) echo 'usage: nats-generation.sh --check | --render FILE' >&2; exit 2 ;;
  esac
fi
