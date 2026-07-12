#!/usr/bin/env bash
# Apply app manifests (services, gateway, frontends) with image tag substitution.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/load-staging-domains.sh
source "${ROOT}/scripts/staging/load-staging-domains.sh"
REGISTRY="${VOICE_IMAGE_REGISTRY:?VOICE_IMAGE_REGISTRY required}"
TAG="${VOICE_IMAGE_TAG:?VOICE_IMAGE_TAG required}"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"

render() {
  sed -e "s|__IMAGE_REGISTRY__|${REGISTRY}|g" \
      -e "s|__IMAGE_TAG__|${TAG}|g" \
      -e "s|IMAGE_PLACEHOLDER|${REGISTRY}/gateway:${TAG}|g" \
      "$1"
}

patch_image_pull_secrets() {
  local secret_name="${VOICE_IMAGE_PULL_SECRET:-}"
  if [ -z "${secret_name}" ]; then
    return 0
  fi
  for dep in $(kubectl get deployment -n "${NS}" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}'); do
    kubectl patch deployment "${dep}" -n "${NS}" --type=strategic \
      -p="{\"spec\":{\"template\":{\"spec\":{\"imagePullSecrets\":[{\"name\":\"${secret_name}\"}]}}}}" \
      2>/dev/null || true
  done
}

render "${ROOT}/deploy/staging/services.yaml" | kubectl apply -f -
render "${ROOT}/deploy/staging/gateway-deployment.yaml" | kubectl apply -f -

if kubectl get deployment voice-auth -n "${NS}" >/dev/null 2>&1; then
  kubectl scale deployment/voice-auth -n "${NS}" --replicas=0
  kubectl wait --for=delete pod -l app=voice-auth -n "${NS}" --timeout=120s 2>/dev/null || true
fi

patch_image_pull_secrets

if [ -f "${ROOT}/deploy/staging/developer-portal.yaml" ]; then
  render "${ROOT}/deploy/staging/developer-portal.yaml" | \
    sed -e "s|__DEVELOPER_PORTAL_INGRESS_HOST__|${VOICE_DEVELOPER_PORTAL_INGRESS_HOST}|g" | \
    kubectl apply -f -
fi
if [ -f "${ROOT}/deploy/staging/flutter-web.yaml" ]; then
  render "${ROOT}/deploy/staging/flutter-web.yaml" | \
    sed -e "s|__WEB_INGRESS_HOST__|${VOICE_WEB_INGRESS_HOST}|g" | \
    kubectl apply -f -
fi
if [ -f "${ROOT}/deploy/staging/admin.yaml" ]; then
  render "${ROOT}/deploy/staging/admin.yaml" | \
    sed -e "s|__ADMIN_INGRESS_HOST__|${VOICE_ADMIN_INGRESS_HOST}|g" | \
    kubectl apply -f -
fi

patch_image_pull_secrets
echo "Staging app manifests applied (tag ${TAG})."
