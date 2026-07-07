#!/usr/bin/env bash
# Render production manifests with image registry/tag and apply to cluster.
# Full stack mirrors staging when deploy/prod/ is extended; skeleton applies bot + namespace.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REGISTRY="${VOICE_IMAGE_REGISTRY:-ghcr.io/voiceroot/voiceroot}"
TAG="${VOICE_IMAGE_TAG:?VOICE_IMAGE_TAG required for production}"
NS="${VOICE_K8S_NAMESPACE:-voice-prod}"

render() {
  sed -e "s|__IMAGE_REGISTRY__|${REGISTRY}|g" \
      -e "s|__IMAGE_TAG__|${TAG}|g" \
      "$1"
}

patch_image_pull_secrets() {
  local secret_name="${VOICE_IMAGE_PULL_SECRET:-}"
  if [ -z "${secret_name}" ]; then
    return 0
  fi
  for dep in $(kubectl get deployment -n "${NS}" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true); do
    kubectl patch deployment "${dep}" -n "${NS}" --type=json \
      -p="[{\"op\":\"add\",\"path\":\"/spec/template/spec/imagePullSecrets\",\"value\":[{\"name\":\"${secret_name}\"}]}]" \
      2>/dev/null || true
  done
}

echo "Applying Voice production stack: ${REGISTRY} tag ${TAG} namespace ${NS}"

kubectl apply -f "${ROOT}/deploy/prod/namespace.yaml"

if [ -f "${ROOT}/deploy/prod/configmap-app.yaml" ]; then
  kubectl apply -f "${ROOT}/deploy/prod/configmap-app.yaml"
fi

if [ -f "${ROOT}/deploy/prod/secret.yaml" ]; then
  kubectl apply -f "${ROOT}/deploy/prod/secret.yaml"
fi

if [ -f "${ROOT}/deploy/prod/infra.yaml" ]; then
  render "${ROOT}/deploy/prod/infra.yaml" | kubectl apply -f -
fi

if [ -f "${ROOT}/deploy/prod/services.yaml" ]; then
  render "${ROOT}/deploy/prod/services.yaml" | kubectl apply -f -
  patch_image_pull_secrets
fi

if [ -f "${ROOT}/deploy/prod/gateway-deployment.yaml" ]; then
  render "${ROOT}/deploy/prod/gateway-deployment.yaml" | kubectl apply -f -
  patch_image_pull_secrets
  kubectl rollout status "deployment/voice-gateway" -n "${NS}" --timeout=300s
fi

echo "Production apply complete. Image tag: ${TAG}"
