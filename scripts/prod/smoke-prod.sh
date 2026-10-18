#!/usr/bin/env bash
# Post-deploy production auth-boundary probes plus shared health/API smoke.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/prod/load-prod-domains.sh
source "${ROOT}/scripts/prod/load-prod-domains.sh"
# shellcheck source=scripts/prod/map-prod-env.sh
source "${ROOT}/scripts/prod/map-prod-env.sh"

export VOICE_STAGING_URL="https://${VOICE_GATEWAY_INGRESS_HOST}"
export VOICE_K8S_NAMESPACE="${VOICE_K8S_NAMESPACE:-voice-prod}"

# Check the public production auth boundary without accounts or mutations.
# Neither a working backend nor a configured staff token may turn these into
# successful requests. Do not follow redirects or weaken certificate validation.
for route in \
  '/api/v1/spaces/00000000-0000-0000-0000-000000000001' \
  '/api/v1/analytics/dashboard/product' \
  '/api/v1/admin/moderation/reports?status=pending&queue=content'; do
  for credential in missing invalid; do
    headers=()
    if [ "${credential}" = invalid ]; then
      headers=(-H 'Authorization: Bearer voice-production-smoke-invalid')
    fi
    # -q disables user curlrc credentials/options for these negative probes.
    if ! code="$(curl -q -sS --connect-timeout 5 --max-time 15 \
      -o /dev/null -w '%{http_code}' "${headers[@]}" "${VOICE_STAGING_URL}${route}")"; then
      echo "Production auth probe transport failed: ${route} (${credential} credential)" >&2
      exit 1
    fi
    if [ "${code}" != 401 ]; then
      echo "Production auth probe failed: ${route} (${credential} credential): HTTP ${code}; expected 401" >&2
      exit 1
    fi
  done
done

bash "${ROOT}/scripts/staging/smoke-staging.sh"
echo 'Production smoke passed.'
