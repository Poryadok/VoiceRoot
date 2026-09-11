#!/usr/bin/env bash
# Apply the TLS-only production LiveKit signaling ingress.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/prod/load-prod-domains.sh
source "${ROOT}/scripts/prod/load-prod-domains.sh"

INGRESS_HOST="${VOICE_LIVEKIT_INGRESS_HOST:-}"
NS="${VOICE_K8S_NAMESPACE:-voice-prod}"

if [ -z "${INGRESS_HOST}" ]; then
  echo "VOICE_LIVEKIT_INGRESS_HOST not set — skip LiveKit Ingress."
  exit 0
fi

echo "Applying LiveKit Ingress: host=${INGRESS_HOST} namespace=${NS}"
sed -e "s|namespace: voice-prod|namespace: ${NS}|g" \
    -e "s|__LIVEKIT_INGRESS_HOST__|${INGRESS_HOST}|g" \
  "${ROOT}/deploy/prod/livekit-ingress.yaml" | kubectl apply -f -
