#!/usr/bin/env bash
# Rollout only changed deployments. CHANGED_SERVICES=comma-separated image names (gateway, chat, ...).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
REGISTRY="${VOICE_IMAGE_REGISTRY:?VOICE_IMAGE_REGISTRY required}"
TAG="${VOICE_IMAGE_TAG:?VOICE_IMAGE_TAG required}"
CHANGED="${CHANGED_SERVICES:-}"
CATALOG="${ROOT}/scripts/ci/staging-image-catalog.json"

if [ -z "${CHANGED}" ]; then
  echo "No CHANGED_SERVICES; skipping selective rollout"
  exit 0
fi

IFS=',' read -ra NAMES <<<"${CHANGED}"

deployment_for() {
  jq -r --arg n "$1" '.images[] | select(.name == $n) | .deployment' "${CATALOG}"
}

set_deployment_image() {
  local dep="$1"
  local img_name="$2"
  local ref="${REGISTRY}/${img_name}:${TAG}"
  echo "Set image ${dep} -> ${ref}"
  kubectl set image "deployment/${dep}" -n "${NS}" \
    "$(kubectl get deployment "${dep}" -n "${NS}" -o jsonpath='{.spec.template.spec.containers[0].name}')=${ref}"
}

rollout_one() {
  local dep="$1"
  local img_name="$2"
  set_deployment_image "${dep}" "${img_name}"
  kubectl rollout status "deployment/${dep}" -n "${NS}" --timeout=300s
}

defer_user_space_rollout() {
  local name="$1"
  [ "${NEEDS_USER_SPACE_ROLLOUT:-false}" = "true" ] || return 1
  [ "${name}" = "user" ] || [ "${name}" = "space" ] || return 1
  return 0
}

for name in "${NAMES[@]}"; do
  name="$(echo "${name}" | tr -d '[:space:]')"
  [ -z "${name}" ] && continue
  if defer_user_space_rollout "${name}"; then
    echo "Defer ${name} image rollout to user/space ordered tier"
    continue
  fi
  dep="$(deployment_for "${name}")"
  if [ -z "${dep}" ] || [ "${dep}" = "null" ]; then
    echo "WARN: unknown image name ${name} in catalog" >&2
    continue
  fi
  if ! kubectl get deployment "${dep}" -n "${NS}" >/dev/null 2>&1; then
    echo "WARN: deployment ${dep} not found; skip" >&2
    continue
  fi
  rollout_one "${dep}" "${name}"
done

echo "Selective rollout complete."
