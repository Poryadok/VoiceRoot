#!/usr/bin/env bash
# Apply deploy/gateway/ingress.yaml for staging (or any namespace/host from env).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/load-staging-domains.sh
source "${ROOT}/scripts/staging/load-staging-domains.sh"

INGRESS_HOST="${VOICE_GATEWAY_INGRESS_HOST:-}"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
TLS="${VOICE_GATEWAY_TLS_SECRET:-voice-gateway-tls}"

if [ -z "${INGRESS_HOST}" ]; then
  echo "VOICE_GATEWAY_INGRESS_HOST not set — skip gateway Ingress (set repo Variable or domains.defaults)."
  exit 0
fi

echo "Applying gateway Ingress: host=${INGRESS_HOST} namespace=${NS} tls=${TLS}"
sed -e "s|__K_NAMESPACE__|${NS}|g" \
    -e "s|__INGRESS_HOST__|${INGRESS_HOST}|g" \
    -e "s|__TLS_SECRET_NAME__|${TLS}|g" \
  "${ROOT}/deploy/gateway/ingress.yaml" | kubectl apply -f -
