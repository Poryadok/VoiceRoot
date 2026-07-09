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

patch_image_pull_secrets() {
  local secret_name="${VOICE_IMAGE_PULL_SECRET:-}"
  if [ -z "${secret_name}" ]; then
    return 0
  fi
  echo "Patching imagePullSecrets=${secret_name} on app deployments"
  for dep in $(kubectl get deployment -n "${NS}" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}'); do
    kubectl patch deployment "${dep}" -n "${NS}" --type=strategic \
      -p="{\"spec\":{\"template\":{\"spec\":{\"imagePullSecrets\":[{\"name\":\"${secret_name}\"}]}}}}" \
      2>/dev/null || true
  done
}

echo "Applying Voice staging stack: ${REGISTRY} tag ${TAG} namespace ${NS}"

kubectl apply -f "${ROOT}/deploy/staging/namespace.yaml"
sed -e "s|__GATEWAY_INGRESS_HOST__|${VOICE_GATEWAY_INGRESS_HOST}|g" \
    -e "s|__DEVELOPER_PORTAL_INGRESS_HOST__|${VOICE_DEVELOPER_PORTAL_INGRESS_HOST}|g" \
  "${ROOT}/deploy/staging/configmap-app.yaml" | kubectl apply -f -

if [ -n "${STAGING_APP_SECRETS_YAML_B64:-}" ] || [ ! -f "${ROOT}/deploy/staging/secret.yaml" ]; then
  bash "${ROOT}/scripts/staging/ensure-app-secrets.sh"
elif [ -f "${ROOT}/deploy/staging/secret.yaml" ]; then
  kubectl apply -f "${ROOT}/deploy/staging/secret.yaml"
fi

if ! kubectl get secret voice-app-secrets -n "${NS}" >/dev/null 2>&1; then
  echo "ERROR: secret voice-app-secrets missing in ${NS}. Create deploy/staging/secret.yaml or set STAGING_APP_SECRETS_YAML in CI." >&2
  exit 1
fi

bash "${ROOT}/scripts/staging/patch-app-secrets-database-urls.sh"
bash "${ROOT}/scripts/staging/patch-gateway-staff-token.sh"

render "${ROOT}/deploy/staging/infra.yaml" | kubectl apply -f -
render "${ROOT}/deploy/staging/services.yaml" | kubectl apply -f -
render "${ROOT}/deploy/staging/gateway-deployment.yaml" | kubectl apply -f -

patch_image_pull_secrets

echo "Ensuring Postgres databases exist..."
kubectl wait --for=condition=ready pod/voice-postgres-0 -n "${NS}" --timeout=120s
bash "${ROOT}/scripts/staging/init-postgres-databases.sh"
bash "${ROOT}/scripts/staging/ensure-gateway-schema.sh"

echo "Ensuring ClickHouse schema..."
if ! kubectl rollout status statefulset/voice-clickhouse -n "${NS}" --timeout=180s; then
  echo "ERROR: voice-clickhouse rollout failed" >&2
  kubectl get pods -n "${NS}" -l app=voice-clickhouse -o wide >&2 || true
  kubectl describe pod voice-clickhouse-0 -n "${NS}" >&2 || true
  kubectl logs voice-clickhouse-0 -n "${NS}" --tail=80 >&2 || true
  exit 1
fi
bash "${ROOT}/scripts/staging/apply-clickhouse-init.sh"

bash "${ROOT}/scripts/staging/apply-migrate-jobs.sh"

bash "${ROOT}/scripts/staging/rollout-app-tier.sh"

if [ -f "${ROOT}/deploy/staging/developer-portal.yaml" ]; then
  render "${ROOT}/deploy/staging/developer-portal.yaml" | \
    sed -e "s|__DEVELOPER_PORTAL_INGRESS_HOST__|${VOICE_DEVELOPER_PORTAL_INGRESS_HOST}|g" | \
    kubectl apply -f -
  patch_image_pull_secrets
fi

echo "Waiting for gateway rollout..."
kubectl rollout status "deployment/voice-gateway" -n "${NS}" --timeout=300s

if kubectl get deployment voice-developer-portal -n "${NS}" >/dev/null 2>&1; then
  echo "Waiting for developer-portal rollout..."
  kubectl rollout status "deployment/voice-developer-portal" -n "${NS}" --timeout=300s
fi

bash "${ROOT}/scripts/staging/apply-gateway-ingress.sh"

if [ "${VOICE_APPLY_OBSERVABILITY:-}" = "true" ]; then
  echo "Applying observability stack (VOICE_APPLY_OBSERVABILITY=true)..."
  kubectl apply -f "${ROOT}/deploy/observability/" || echo "WARN: observability apply failed; check deploy/observability/README.md"
fi

echo "Staging apply complete. Image tag: ${TAG}"
