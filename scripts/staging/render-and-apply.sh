#!/usr/bin/env bash
# Render staging manifests with image registry/tag and apply to cluster.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REGISTRY="${VOICE_IMAGE_REGISTRY:-ghcr.io/voiceroot/voiceroot}"
TAG="${VOICE_IMAGE_TAG:-latest}"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"

render() {
  sed -e "s|__IMAGE_REGISTRY__|${REGISTRY}|g" \
      -e "s|__IMAGE_TAG__|${TAG}|g" \
      -e "s|IMAGE_PLACEHOLDER|${REGISTRY}/gateway:${TAG}|g" \
      "$1"
}

echo "Applying Voice staging stack: ${REGISTRY} tag ${TAG} namespace ${NS}"

kubectl apply -f "${ROOT}/deploy/staging/namespace.yaml"
kubectl apply -f "${ROOT}/deploy/staging/configmap-app.yaml"

if [ -f "${ROOT}/deploy/staging/secret.yaml" ]; then
  kubectl apply -f "${ROOT}/deploy/staging/secret.yaml"
else
  echo "WARN: deploy/staging/secret.yaml missing — copy from secret.example.yaml"
fi

render "${ROOT}/deploy/staging/infra.yaml" | kubectl apply -f -
render "${ROOT}/deploy/staging/services.yaml" | kubectl apply -f -
render "${ROOT}/deploy/staging/gateway-deployment.yaml" | kubectl apply -f -

echo "Waiting for gateway rollout..."
kubectl rollout status "deployment/voice-gateway" -n "${NS}" --timeout=300s

echo "Staging apply complete."
