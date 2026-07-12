#!/usr/bin/env bash
# Apply deploy/livekit/ingress.yaml for staging LiveKit signaling (wss via Cloudflare).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/load-staging-domains.sh
source "${ROOT}/scripts/staging/load-staging-domains.sh"

INGRESS_HOST="${VOICE_LIVEKIT_INGRESS_HOST:-}"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"

if [ -z "${INGRESS_HOST}" ]; then
  echo "VOICE_LIVEKIT_INGRESS_HOST not set — skip LiveKit Ingress."
  exit 0
fi

echo "Applying LiveKit Ingress: host=${INGRESS_HOST} namespace=${NS}"
sed -e "s|__K_NAMESPACE__|${NS}|g" \
    -e "s|__INGRESS_HOST__|${INGRESS_HOST}|g" \
  "${ROOT}/deploy/livekit/ingress.yaml" | kubectl apply -f -
