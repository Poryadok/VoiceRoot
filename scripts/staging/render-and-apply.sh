#!/usr/bin/env bash
# Render staging manifests with image registry/tag and apply to cluster.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/load-staging-domains.sh
source "${ROOT}/scripts/staging/load-staging-domains.sh"
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
sed -e "s|__GATEWAY_INGRESS_HOST__|${VOICE_GATEWAY_INGRESS_HOST}|g" \
    -e "s|__DEVELOPER_PORTAL_INGRESS_HOST__|${VOICE_DEVELOPER_PORTAL_INGRESS_HOST}|g" \
  "${ROOT}/deploy/staging/configmap-app.yaml" | kubectl apply -f -

if [ -f "${ROOT}/deploy/staging/secret.yaml" ]; then
  kubectl apply -f "${ROOT}/deploy/staging/secret.yaml"
else
  bash "${ROOT}/scripts/staging/ensure-app-secrets.sh"
fi

if ! kubectl get secret voice-app-secrets -n "${NS}" >/dev/null 2>&1; then
  echo "ERROR: secret voice-app-secrets missing in ${NS}. Create deploy/staging/secret.yaml or set STAGING_APP_SECRETS_YAML in CI." >&2
  exit 1
fi

render "${ROOT}/deploy/staging/infra.yaml" | kubectl apply -f -
render "${ROOT}/deploy/staging/services.yaml" | kubectl apply -f -
render "${ROOT}/deploy/staging/gateway-deployment.yaml" | kubectl apply -f -

echo "Ensuring Postgres databases exist..."
kubectl wait --for=condition=ready pod/voice-postgres-0 -n "${NS}" --timeout=120s
bash "${ROOT}/scripts/staging/init-postgres-databases.sh"
bash "${ROOT}/scripts/staging/ensure-gateway-schema.sh"

bash "${ROOT}/scripts/staging/rollout-app-tier.sh"

if [ -f "${ROOT}/deploy/staging/developer-portal.yaml" ]; then
  render "${ROOT}/deploy/staging/developer-portal.yaml" | \
    sed -e "s|__DEVELOPER_PORTAL_INGRESS_HOST__|${VOICE_DEVELOPER_PORTAL_INGRESS_HOST}|g" | \
    kubectl apply -f -
fi

echo "Waiting for gateway rollout..."
kubectl rollout status "deployment/voice-gateway" -n "${NS}" --timeout=300s || true

bash "${ROOT}/scripts/staging/apply-gateway-ingress.sh"

echo "Staging apply complete."
