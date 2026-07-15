#!/usr/bin/env bash
# User/space ordered rollout (tiers 2-3) without full app-tier restart.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
CATALOG="${ROOT}/scripts/ci/staging-image-catalog.json"
# shellcheck source=scripts/staging/rollout-helpers.sh
source "${ROOT}/scripts/staging/rollout-helpers.sh"

changed_includes() {
  local name="$1"
  local changed="${CHANGED_SERVICES:-}"
  [ -z "${changed}" ] && return 1
  IFS=',' read -ra names <<<"${changed}"
  for n in "${names[@]}"; do
    n="$(echo "${n}" | tr -d '[:space:]')"
    [ "${n}" = "${name}" ] && return 0
  done
  return 1
}

set_changed_image() {
  local img_name="$1"
  changed_includes "${img_name}" || return 0
  local dep
  dep="$(jq -r --arg n "${img_name}" '.images[] | select(.name == $n) | .deployment' "${CATALOG}")"
  if [ -z "${dep}" ] || [ "${dep}" = "null" ]; then
    echo "WARN: unknown image name ${img_name} in catalog" >&2
    return 0
  fi
  if ! kubectl get deployment "${dep}" -n "${NS}" >/dev/null 2>&1; then
    echo "WARN: deployment ${dep} not found; skip image update" >&2
    return 0
  fi
  local ref="${VOICE_IMAGE_REGISTRY:?VOICE_IMAGE_REGISTRY required}/${img_name}:${VOICE_IMAGE_TAG:?VOICE_IMAGE_TAG required}"
  echo "Set image ${dep} -> ${ref} (ordered user/space tier)"
  kubectl set image "deployment/${dep}" -n "${NS}" \
    "$(kubectl get deployment "${dep}" -n "${NS}" -o jsonpath='{.spec.template.spec.containers[0].name}')=${ref}"
}

patch_user_skip_space() {
  kubectl set env deployment/voice-user -n "$NS" SPACE_GRPC_ADDR=""
}

restore_user_space_addr() {
  kubectl set env deployment/voice-user -n "$NS" SPACE_GRPC_ADDR- 2>/dev/null || true
}

echo "User/space tier rollout: update images then ordered restart"

bash "${ROOT}/scripts/staging/deploy-changed.sh"

echo "Tier 2: user before space (break user<->space dial deadlock)"
kubectl scale deployment/voice-space -n "$NS" --replicas=0
kubectl wait --for=delete pod -l app=voice-space -n "$NS" --timeout=180s 2>/dev/null || true
set_changed_image user
patch_user_skip_space
kubectl rollout restart deployment/voice-user -n "$NS"
wait_deploy voice-user

echo "Tier 3: space"
set_changed_image space
kubectl scale deployment/voice-space -n "$NS" --replicas=1
wait_deploy voice-space
restore_user_space_addr
kubectl rollout restart deployment/voice-user -n "$NS"
wait_deploy voice-user

echo "User/space tier rollout complete."
