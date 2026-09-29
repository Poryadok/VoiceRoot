#!/usr/bin/env bash
# Apply deploy/gateway/ingress.yaml for staging (or any namespace/host from env).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/load-staging-domains.sh
source "${ROOT}/scripts/staging/load-staging-domains.sh"

INGRESS_HOST="${VOICE_GATEWAY_INGRESS_HOST:-}"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
TLS="${VOICE_GATEWAY_TLS_SECRET:-voice-gateway-tls}"

bucket_from_secret() {
  local encoded
  encoded="$(kubectl get secret voice-app-secrets -n "${NS}" -o "jsonpath={.data.$1}")"
  [ -n "${encoded}" ] || { echo "missing storage bucket in voice-app-secrets" >&2; exit 1; }
  printf '%s' "${encoded}" | base64 -d
}

if [ -z "${INGRESS_HOST}" ]; then
  echo "VOICE_GATEWAY_INGRESS_HOST not set — skip gateway Ingress (set repo Variable or domains.defaults)."
  exit 0
fi

FILE_BUCKET="$(bucket_from_secret FILE_R2_BUCKET)"
AVATAR_BUCKET="$(bucket_from_secret USER_R2_BUCKET)"
for bucket in "${FILE_BUCKET}" "${AVATAR_BUCKET}"; do
  [[ "${bucket}" =~ ^[a-z0-9][a-z0-9.-]*[a-z0-9]$ ]] || { echo "invalid storage bucket in voice-app-secrets" >&2; exit 1; }
done

if [ -n "${VOICE_STORAGE_INGRESS_HOST:-}" ]; then
  storage_tls="${VOICE_STORAGE_TLS_SECRET:-voice-storage-tls}"
  [ "$(kubectl get secret "${storage_tls}" -n "${NS}" -o 'jsonpath={.type}')" = 'kubernetes.io/tls' ] || {
    echo "storage TLS Secret missing or invalid type" >&2
    exit 1
  }
  sed -e "s|__K_NAMESPACE__|${NS}|g" \
      -e "s|__STORAGE_INGRESS_HOST__|${VOICE_STORAGE_INGRESS_HOST}|g" \
      -e "s|__STORAGE_TLS_SECRET__|${storage_tls}|g" \
      -e "s|__FILE_BUCKET__|${FILE_BUCKET}|g" \
      -e "s|__AVATAR_BUCKET__|${AVATAR_BUCKET}|g" \
    "${ROOT}/deploy/storage/ingress.yaml" | kubectl apply -f -
fi

echo "Applying gateway Ingress: host=${INGRESS_HOST} namespace=${NS} tls=${TLS}"
sed -e "s|__K_NAMESPACE__|${NS}|g" \
    -e "s|__INGRESS_HOST__|${INGRESS_HOST}|g" \
    -e "s|__TLS_SECRET_NAME__|${TLS}|g" \
    -e "s|__FILE_BUCKET__|${FILE_BUCKET}|g" \
    -e "s|__AVATAR_BUCKET__|${AVATAR_BUCKET}|g" \
  "${ROOT}/deploy/gateway/ingress.yaml" | kubectl apply -f -
