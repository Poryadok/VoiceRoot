#!/usr/bin/env bash
# Apply staging infra: namespace, config, secrets, data plane, schema, migrations.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/load-staging-domains.sh
source "${ROOT}/scripts/staging/load-staging-domains.sh"
REGISTRY="${VOICE_IMAGE_REGISTRY:-ghcr.io/voiceroot/voiceroot}"
TAG="${VOICE_IMAGE_TAG:?VOICE_IMAGE_TAG required}"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
NATS_STORAGE_CLASS="${VOICE_NATS_STORAGE_CLASS:?VOICE_NATS_STORAGE_CLASS must be set from the reviewed staging preflight}"
NATS_STORAGE_SIZE="${VOICE_NATS_STORAGE_SIZE:?VOICE_NATS_STORAGE_SIZE must be set from the reviewed staging capacity evidence}"
MINIO_IMAGE="${VOICE_MINIO_IMAGE:-quay.io/minio/minio:RELEASE.2024-12-18T13-15-44Z@sha256:1dce27c494a16bae114774f1cec295493f3613142713130c2d22dd5696be6ad3}"
MINIO_MC_IMAGE="${VOICE_MINIO_MC_IMAGE:-quay.io/minio/mc:RELEASE.2025-08-13T08-35-41Z@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727}"
MINIO_STORAGE_CLASS="${VOICE_MINIO_STORAGE_CLASS:-local-path}"
MINIO_STORAGE_SIZE="${VOICE_MINIO_STORAGE_SIZE:-20Gi}"

render() {
  sed -e "s|__IMAGE_REGISTRY__|${REGISTRY}|g" \
      -e "s|__IMAGE_TAG__|${TAG}|g" \
      -e "s|IMAGE_PLACEHOLDER|${REGISTRY}/gateway:${TAG}|g" \
      -e "s|__NATS_STORAGE_CLASS__|${NATS_STORAGE_CLASS}|g" \
      -e "s|__NATS_STORAGE_SIZE__|${NATS_STORAGE_SIZE}|g" \
      "$1"
}

kubectl apply -f "${ROOT}/deploy/staging/namespace.yaml"
sed -e "s|__GATEWAY_INGRESS_HOST__|${VOICE_GATEWAY_INGRESS_HOST}|g" \
    -e "s|__DEVELOPER_PORTAL_INGRESS_HOST__|${VOICE_DEVELOPER_PORTAL_INGRESS_HOST}|g" \
    -e "s|__ADMIN_INGRESS_HOST__|${VOICE_ADMIN_INGRESS_HOST}|g" \
    -e "s|__LIVEKIT_INGRESS_HOST__|${VOICE_LIVEKIT_INGRESS_HOST}|g" \
  "${ROOT}/deploy/staging/configmap-app.yaml" | kubectl apply -f -

if [ -n "${STAGING_APP_SECRETS_YAML_B64:-}" ] || [ ! -f "${ROOT}/deploy/staging/secret.yaml" ]; then
  bash "${ROOT}/scripts/staging/ensure-app-secrets.sh"
elif [ -f "${ROOT}/deploy/staging/secret.yaml" ]; then
  kubectl apply -f "${ROOT}/deploy/staging/secret.yaml"
fi

if ! kubectl get secret voice-app-secrets -n "${NS}" >/dev/null 2>&1; then
  echo "ERROR: secret voice-app-secrets missing in ${NS}" >&2
  exit 1
fi

if ! kubectl get secret voice-minio-credentials -n "${NS}" >/dev/null 2>&1; then
  echo "ERROR: secret voice-minio-credentials missing in ${NS}" >&2
  exit 1
fi

for secret in voice-nats-operator voice-nats-hub-tls voice-nats-bootstrap-credentials voice-nats-service-credentials; do
  if ! kubectl get secret "${secret}" -n "${NS}" >/dev/null 2>&1; then
    echo "ERROR: NATS activation secret ${secret} missing in ${NS}" >&2
    exit 1
  fi
done
for key in operator.jwt account.jwt system-account.jwt account.public system-account.public; do
  if ! kubectl get secret voice-nats-operator -n "${NS}" -o "jsonpath={.data.${key//./\\.}}" | grep -q .; then
    echo "ERROR: voice-nats-operator missing required ${key} in ${NS}" >&2
    exit 1
  fi
done
for key in tls.crt tls.key ca.crt; do
  if ! kubectl get secret voice-nats-hub-tls -n "${NS}" -o "jsonpath={.data.${key//./\\.}}" | grep -q .; then
    echo "ERROR: voice-nats-hub-tls missing required ${key} in ${NS}" >&2
    exit 1
  fi
done
for key in bootstrap.creds; do
  if ! kubectl get secret voice-nats-bootstrap-credentials -n "${NS}" -o "jsonpath={.data.${key//./\\.}}" | grep -q .; then
    echo "ERROR: voice-nats-bootstrap-credentials missing required ${key} in ${NS}" >&2
    exit 1
  fi
done
for service in auth analytics bot chat file matchmaking messaging moderation notification realtime role search social space story subscription user voice; do
  key="${service}.creds"
  if ! kubectl get secret voice-nats-service-credentials -n "${NS}" -o "jsonpath={.data.${key//./\\.}}" | grep -q .; then
    echo "ERROR: voice-nats-service-credentials missing required ${key} in ${NS}" >&2
    exit 1
  fi
done

verify_nats_hub_tls() (
  umask 077
  cert_file="$(mktemp)"
  ca_file="$(mktemp)"
  trap 'rm -f "${cert_file}" "${ca_file}"' EXIT
  kubectl get secret voice-nats-hub-tls -n "${NS}" -o jsonpath='{.data.tls\.crt}' | base64 -d >"${cert_file}" 2>/dev/null
  kubectl get secret voice-nats-hub-tls -n "${NS}" -o jsonpath='{.data.ca\.crt}' | base64 -d >"${ca_file}" 2>/dev/null
  openssl verify -CAfile "${ca_file}" "${cert_file}" >/dev/null 2>&1 && \
    openssl x509 -in "${cert_file}" -noout -checkhost voice-nats >/dev/null 2>&1 || {
      echo "ERROR: voice-nats-hub-tls must form a trusted chain and include DNS SAN voice-nats" >&2
      exit 1
    }
)
verify_nats_hub_tls

bash "${ROOT}/scripts/staging/patch-app-secrets-database-urls.sh"
bash "${ROOT}/scripts/staging/patch-gateway-staff-token.sh"

# Select the hub before it is created or restarted. An empty selector is safe;
# applying this after the rollout would leave a direct-hub bypass window.
sed "s|__NAMESPACE__|${NS}|g" "${ROOT}/deploy/templates/network-policy-nats-hub.yaml" | kubectl apply -f -

LIVEKIT_API_KEY="$(kubectl get secret voice-app-secrets -n "${NS}" -o jsonpath='{.data.LIVEKIT_API_KEY}' 2>/dev/null | base64 -d 2>/dev/null || true)"
LIVEKIT_API_SECRET="$(kubectl get secret voice-app-secrets -n "${NS}" -o jsonpath='{.data.LIVEKIT_API_SECRET}' 2>/dev/null | base64 -d 2>/dev/null || true)"
if [ -z "${LIVEKIT_API_KEY}" ] || [ -z "${LIVEKIT_API_SECRET}" ]; then
  echo "ERROR: LIVEKIT_API_KEY and LIVEKIT_API_SECRET must be set in voice-app-secrets" >&2
  exit 1
fi

NATS_MIGRATION_EVIDENCE="${VOICE_NATS_MIGRATION_EVIDENCE:-}" \
NATS_SOURCE_CONTEXT="${NATS_SOURCE_CONTEXT:-}" \
VOICE_NATS_STORAGE_CLASS="${NATS_STORAGE_CLASS}" \
VOICE_NATS_STORAGE_SIZE="${NATS_STORAGE_SIZE}" \
  bash "${ROOT}/scripts/staging/guard-nats-pvc-migration.sh" --prepare

render "${ROOT}/deploy/staging/infra.yaml" | \
  sed -e "s|__LIVEKIT_API_KEY__|${LIVEKIT_API_KEY}|g" \
      -e "s|__LIVEKIT_API_SECRET__|${LIVEKIT_API_SECRET}|g" | \
  bash "${ROOT}/scripts/staging/filter-staging-infra-source-nats.sh" | \
  kubectl apply -f -

# Do not bootstrap or mutate NATS streams/consumers during candidate creation.
# The old voice-nats Service continues to select the emptyDir source. The PVC
# candidate is isolated behind voice-nats-pvc-candidate until a separate,
# accepted migration command performs the fenced selector cutover.
if [ "${VOICE_NATS_BOOTSTRAP_AFTER_ACCEPTANCE:-false}" = true ]; then
  NATS_MIGRATION_EVIDENCE="${VOICE_NATS_MIGRATION_EVIDENCE:-}" \
  VOICE_NATS_STORAGE_CLASS="${NATS_STORAGE_CLASS}" \
  VOICE_NATS_STORAGE_SIZE="${NATS_STORAGE_SIZE}" \
    bash "${ROOT}/scripts/staging/guard-nats-pvc-migration.sh" --acceptance
  for item in realtime notification search analytics-chat; do
    kubectl delete job "voice-nats-${item}-bootstrap" -n "${NS}" --ignore-not-found
    sed "s|__NAMESPACE__|${NS}|g" "${ROOT}/deploy/templates/nats-${item}-bootstrap.yaml" | kubectl apply -f -
    kubectl wait --for=condition=complete "job/voice-nats-${item}-bootstrap" -n "${NS}" --timeout=120s
  done
else
  echo 'NATS bootstrap jobs deferred: candidate must be restored and accepted before bootstrap.'
fi

sed -e "s|__VOICE_MINIO_IMAGE__|${MINIO_IMAGE}|g" \
    -e "s|__VOICE_MINIO_MC_IMAGE__|${MINIO_MC_IMAGE}|g" \
    -e "s|__VOICE_MINIO_STORAGE_CLASS__|${MINIO_STORAGE_CLASS}|g" \
    -e "s|__VOICE_MINIO_STORAGE_SIZE__|${MINIO_STORAGE_SIZE}|g" \
  "${ROOT}/deploy/staging/minio.yaml" | kubectl apply -f -

kubectl wait --for=condition=ready pod/voice-postgres-0 -n "${NS}" --timeout=120s
bash "${ROOT}/scripts/staging/init-postgres-databases.sh"
bash "${ROOT}/scripts/staging/sync-postgres-password.sh"
bash "${ROOT}/scripts/staging/ensure-gateway-schema.sh"

if ! kubectl rollout status statefulset/voice-clickhouse -n "${NS}" --timeout=180s; then
  echo "ERROR: voice-clickhouse rollout failed" >&2
  exit 1
fi
bash "${ROOT}/scripts/staging/apply-clickhouse-init.sh"
bash "${ROOT}/scripts/staging/apply-migrate-jobs.sh"

echo "Staging infra apply complete."
