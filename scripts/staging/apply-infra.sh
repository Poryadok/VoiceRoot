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
MINIO_IMAGE="${VOICE_MINIO_IMAGE:-ghcr.io/poryadok/voiceroot/minio:86b2017f06d0d471e8b43abc78031e86756defe3@sha256:ab7687bc47a84c3aec0d9706dabd47b4719b081683cde745f8a1b84c6c7681e0}"
MINIO_MC_IMAGE="${VOICE_MINIO_MC_IMAGE:-ghcr.io/poryadok/voiceroot/minio-mc:86b2017f06d0d471e8b43abc78031e86756defe3@sha256:66a55c322fed37a3fefa0b815d195b01e7903d1bbffccd80d5a8af3cedf343f2}"
MINIO_STORAGE_CLASS="${VOICE_MINIO_STORAGE_CLASS:-local-path}"
MINIO_STORAGE_SIZE="${VOICE_MINIO_STORAGE_SIZE:-20Gi}"

clean_install_mode=false
clean_install_bootstrap=false
clean_install_mode_value=""
fresh_install="${VOICE_NATS_FRESH_INSTALL:-false}"
VOICE_NATS_PRESERVE_SERVICE_SELECTOR=false
case "${fresh_install}" in
  true|false) ;;
  *) echo 'ERROR: VOICE_NATS_FRESH_INSTALL must be true or false' >&2; exit 1 ;;
esac

if ! clean_install_state_json="$(kubectl get configmap voice-nats-clean-install-state -n "${NS}" --ignore-not-found=true -o json 2>/dev/null)"; then
  echo 'ERROR: unable to inspect NATS clean-install state; refusing staging infra apply' >&2
  exit 1
fi
if [ -n "${clean_install_state_json}" ] && [ "${clean_install_state_json}" != null ]; then
  if ! clean_install_mode_value="$(printf '%s' "${clean_install_state_json}" | jq -er '.data.mode // empty')" || [ "${clean_install_mode_value}" != clean-install ]; then
    echo 'ERROR: NATS clean-install marker is invalid; refusing staging infra apply' >&2
    exit 1
  fi
  clean_install_storage_class="$(printf '%s' "${clean_install_state_json}" | jq -er '.data.storageClass')" || {
    echo 'ERROR: NATS clean-install marker is missing its storage class' >&2
    exit 1
  }
  clean_install_storage_size="$(printf '%s' "${clean_install_state_json}" | jq -er '.data.storageSize')" || {
    echo 'ERROR: NATS clean-install marker is missing its storage size' >&2
    exit 1
  }
  [ "${clean_install_storage_class}" = "${NATS_STORAGE_CLASS}" ] || { echo 'ERROR: clean-install NATS storage class differs from its reset preflight' >&2; exit 1; }
  [ "${clean_install_storage_size}" = "${NATS_STORAGE_SIZE}" ] || { echo 'ERROR: clean-install NATS storage size differs from its reset preflight' >&2; exit 1; }
  if [ "${fresh_install}" = true ]; then
    [ "${NS}" = voice-staging ] || { echo 'ERROR: clean install is restricted to voice-staging' >&2; exit 1; }
    clean_install_mode=true
    clean_install_bootstrap=true
  else
    if ! kubectl get service voice-nats -n "${NS}" -o json 2>/dev/null | jq -e '
      .spec.selector as $selector |
      ($selector | type) == "object" and
      ($selector | keys == ["app"]) and
      ($selector.app == "voice-nats" or $selector.app == "voice-nats-pvc-candidate")
    ' >/dev/null 2>&1; then
      echo 'ERROR: cannot safely preserve the live voice-nats Service selector; refusing staging infra apply' >&2
      exit 1
    fi
    VOICE_NATS_PRESERVE_SERVICE_SELECTOR=true
  fi
elif [ "${fresh_install}" = true ]; then
  echo 'ERROR: clean-install opt-in requires the namespace reset marker' >&2
  exit 1
fi
export VOICE_NATS_PRESERVE_SERVICE_SELECTOR

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

if [ -z "${clean_install_mode_value}" ]; then
  NATS_MIGRATION_EVIDENCE="${VOICE_NATS_MIGRATION_EVIDENCE:-}" \
  NATS_SOURCE_CONTEXT="${NATS_SOURCE_CONTEXT:-}" \
  VOICE_NATS_STORAGE_CLASS="${NATS_STORAGE_CLASS}" \
  VOICE_NATS_STORAGE_SIZE="${NATS_STORAGE_SIZE}" \
    bash "${ROOT}/scripts/staging/guard-nats-pvc-migration.sh" --prepare
fi

render "${ROOT}/deploy/staging/infra.yaml" | \
  sed -e "s|__LIVEKIT_API_KEY__|${LIVEKIT_API_KEY}|g" \
      -e "s|__LIVEKIT_API_SECRET__|${LIVEKIT_API_SECRET}|g" | \
  bash "${ROOT}/scripts/staging/filter-staging-infra-source-nats.sh" | \
  kubectl apply -f -

run_nats_bootstrap_jobs() {
  for bootstrap in realtime notification search analytics-chat; do
    kubectl delete job "voice-nats-${bootstrap}-bootstrap" -n "${NS}" --ignore-not-found
    sed "s|__NAMESPACE__|${NS}|g" "${ROOT}/deploy/templates/nats-${bootstrap}-bootstrap.yaml" | kubectl apply -f -
    kubectl wait --for=condition=complete "job/voice-nats-${bootstrap}-bootstrap" -n "${NS}" --timeout=120s
  done
}

# Promote only when both the manual reset marker and explicit clean-install
# opt-in are present; stale-marker fallback omits the Service from infra apply.
if [ "${clean_install_mode}" = true ]; then
  kubectl rollout status deployment/voice-nats-pvc-candidate -n "${NS}" --timeout=300s
  kubectl patch service voice-nats -n "${NS}" --type=merge -p '{"spec":{"selector":{"app":"voice-nats-pvc-candidate"}}}'
  if [ "${clean_install_bootstrap}" = true ]; then
    run_nats_bootstrap_jobs
  fi
elif [ "${VOICE_NATS_BOOTSTRAP_AFTER_ACCEPTANCE:-false}" = true ]; then
  NATS_MIGRATION_EVIDENCE="${VOICE_NATS_MIGRATION_EVIDENCE:-}" \
  VOICE_NATS_STORAGE_CLASS="${NATS_STORAGE_CLASS}" \
  VOICE_NATS_STORAGE_SIZE="${NATS_STORAGE_SIZE}" \
    bash "${ROOT}/scripts/staging/guard-nats-pvc-migration.sh" --acceptance
  run_nats_bootstrap_jobs
else
  echo 'NATS bootstrap jobs deferred: candidate must be restored and accepted before bootstrap.'
fi

# Bucket Jobs have immutable pod templates. Replace only the known previous
# client image after a completed, semantically exact bucket-creation Job.
bash "${ROOT}/scripts/staging/replace-minio-bucket-jobs.sh" "${NS}" "${MINIO_MC_IMAGE}"

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
