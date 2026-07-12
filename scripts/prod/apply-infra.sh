#!/usr/bin/env bash
# Apply production infra: namespace, config, secrets, data plane, schema, migrations.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MANIFEST_DIR="${PROD_MANIFEST_DIR:-${ROOT}/deploy/prod}"
# shellcheck source=scripts/prod/load-prod-domains.sh
source "${ROOT}/scripts/prod/load-prod-domains.sh"
REGISTRY="${VOICE_IMAGE_REGISTRY:-ghcr.io/voiceroot/voiceroot}"
TAG="${VOICE_IMAGE_TAG:?VOICE_IMAGE_TAG required for production}"
NS="${VOICE_K8S_NAMESPACE:-voice-prod}"

render() {
  sed -e "s|__IMAGE_REGISTRY__|${REGISTRY}|g" \
      -e "s|__IMAGE_TAG__|${TAG}|g" \
      -e "s|IMAGE_PLACEHOLDER|${REGISTRY}/gateway:${TAG}|g" \
      "$1"
}

kubectl apply -f "${MANIFEST_DIR}/namespace.yaml"
sed -e "s|__GATEWAY_INGRESS_HOST__|${VOICE_GATEWAY_INGRESS_HOST}|g" \
    -e "s|__DEVELOPER_PORTAL_INGRESS_HOST__|${VOICE_DEVELOPER_PORTAL_INGRESS_HOST}|g" \
    -e "s|__ADMIN_INGRESS_HOST__|${VOICE_ADMIN_INGRESS_HOST}|g" \
    -e "s|__LIVEKIT_INGRESS_HOST__|${VOICE_LIVEKIT_INGRESS_HOST}|g" \
  "${MANIFEST_DIR}/configmap-app.yaml" | kubectl apply -f -

if [ -n "${PROD_APP_SECRETS_YAML_B64:-}" ]; then
  export STAGING_APP_SECRETS_YAML_B64="${PROD_APP_SECRETS_YAML_B64}"
fi
if [ -n "${PROD_APP_SECRETS_YAML:-}" ]; then
  export STAGING_APP_SECRETS_YAML="${PROD_APP_SECRETS_YAML}"
fi

if [ -n "${PROD_APP_SECRETS_YAML_B64:-}" ] || [ ! -f "${MANIFEST_DIR}/secret.yaml" ]; then
  bash "${ROOT}/scripts/staging/ensure-app-secrets.sh"
elif [ -f "${MANIFEST_DIR}/secret.yaml" ]; then
  kubectl apply -f "${MANIFEST_DIR}/secret.yaml"
fi

if ! kubectl get secret voice-app-secrets -n "${NS}" >/dev/null 2>&1; then
  echo "ERROR: secret voice-app-secrets missing in ${NS}" >&2
  exit 1
fi

bash "${ROOT}/scripts/staging/patch-app-secrets-database-urls.sh"
if [ -n "${PROD_STAFF_TOKEN:-}" ]; then
  export STAGING_STAFF_TOKEN="${PROD_STAFF_TOKEN}"
  bash "${ROOT}/scripts/staging/patch-gateway-staff-token.sh"
fi

LIVEKIT_API_KEY="$(kubectl get secret voice-app-secrets -n "${NS}" -o jsonpath='{.data.LIVEKIT_API_KEY}' 2>/dev/null | base64 -d 2>/dev/null || true)"
LIVEKIT_API_SECRET="$(kubectl get secret voice-app-secrets -n "${NS}" -o jsonpath='{.data.LIVEKIT_API_SECRET}' 2>/dev/null | base64 -d 2>/dev/null || true)"
if [ -z "${LIVEKIT_API_KEY}" ] || [ -z "${LIVEKIT_API_SECRET}" ]; then
  echo "ERROR: LIVEKIT_API_KEY and LIVEKIT_API_SECRET must be set in voice-app-secrets" >&2
  exit 1
fi

render "${MANIFEST_DIR}/infra.yaml" | \
  sed -e "s|__LIVEKIT_API_KEY__|${LIVEKIT_API_KEY}|g" \
      -e "s|__LIVEKIT_API_SECRET__|${LIVEKIT_API_SECRET}|g" | \
  kubectl apply -f -

kubectl wait --for=condition=ready pod/voice-postgres-0 -n "${NS}" --timeout=120s
bash "${ROOT}/scripts/staging/init-postgres-databases.sh"
bash "${ROOT}/scripts/staging/ensure-gateway-schema.sh"

if ! kubectl rollout status statefulset/voice-clickhouse -n "${NS}" --timeout=180s; then
  echo "ERROR: voice-clickhouse rollout failed" >&2
  exit 1
fi
bash "${ROOT}/scripts/staging/apply-clickhouse-init.sh"
bash "${ROOT}/scripts/staging/apply-migrate-jobs.sh"

echo "Production infra apply complete."
