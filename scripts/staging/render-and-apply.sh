#!/usr/bin/env bash
# Render staging manifests with image registry/tag and apply to cluster.
# DEPLOY_MODE: full (default) | app-only | images-only
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/load-staging-domains.sh
source "${ROOT}/scripts/staging/load-staging-domains.sh"
REGISTRY="${VOICE_IMAGE_REGISTRY:-ghcr.io/voiceroot/voiceroot}"
TAG="${VOICE_IMAGE_TAG:?VOICE_IMAGE_TAG required}"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
MODE="${DEPLOY_MODE:-full}"

export VOICE_IMAGE_REGISTRY="${REGISTRY}"
export VOICE_IMAGE_TAG="${TAG}"
export VOICE_K8S_NAMESPACE="${NS}"
export DEPLOY_MODE="${MODE}"

echo "Applying Voice staging: ${REGISTRY} tag ${TAG} namespace ${NS} mode=${MODE}"

case "${MODE}" in
  images-only)
    bash "${ROOT}/scripts/staging/rollout-subset.sh"
    bash "${ROOT}/scripts/staging/apply-gateway-ingress.sh"
    bash "${ROOT}/scripts/staging/apply-livekit-ingress.sh"
    ;;
  app-only)
    bash "${ROOT}/scripts/staging/apply-migrate-jobs.sh"
    bash "${ROOT}/scripts/staging/apply-app-manifests.sh"
    bash "${ROOT}/scripts/staging/rollout-subset.sh"
    bash "${ROOT}/scripts/staging/apply-gateway-ingress.sh"
    bash "${ROOT}/scripts/staging/apply-livekit-ingress.sh"
    kubectl rollout status "deployment/voice-gateway" -n "${NS}" --timeout=300s
    ;;
  full|*)
    bash "${ROOT}/scripts/staging/apply-infra.sh"
    bash "${ROOT}/scripts/staging/apply-app-manifests.sh"
    bash "${ROOT}/scripts/staging/rollout-app-tier.sh"
    bash "${ROOT}/scripts/staging/apply-gateway-ingress.sh"
    bash "${ROOT}/scripts/staging/apply-livekit-ingress.sh"
    kubectl rollout status "deployment/voice-gateway" -n "${NS}" --timeout=300s
    for dep in voice-developer-portal voice-web voice-admin; do
      if kubectl get deployment "${dep}" -n "${NS}" >/dev/null 2>&1; then
        kubectl rollout status "deployment/${dep}" -n "${NS}" --timeout=300s
      fi
    done
    ;;
esac

if [ "${VOICE_APPLY_OBSERVABILITY:-}" = "true" ]; then
  GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-}" \
  NOTIFICATIONS_ENABLED="${NOTIFICATIONS_ENABLED:-false}" \
  bash "${ROOT}/scripts/staging/apply-observability.sh"
fi

echo "Staging apply complete. Image tag: ${TAG}"
