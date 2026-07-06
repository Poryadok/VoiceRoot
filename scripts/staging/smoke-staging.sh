#!/usr/bin/env bash
# Post-deploy smoke for staging (health + minimal API).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/load-staging-domains.sh
source "${ROOT}/scripts/staging/load-staging-domains.sh"

BASE="${VOICE_STAGING_URL:-https://${VOICE_GATEWAY_INGRESS_HOST}}"
BASE="${BASE%/}"

echo "Smoke: GET ${BASE}/health"
# Gateway /health is plain text "ok" (not JSON like backend microservices).
health_tmp="$(mktemp)"
health_ok=false
for attempt in 1 2 3 4 5 6; do
  health_code="$(curl -sS -o "${health_tmp}" -w "%{http_code}" "${BASE}/health" || echo "000")"
  health_body="$(tr -d '\r' < "${health_tmp}")"
  if [ "${health_code}" = "200" ] && [ "${health_body}" = "ok" ]; then
    health_ok=true
    break
  fi
  echo "health attempt ${attempt}/6: HTTP ${health_code} body=${health_body}"
  sleep 5
done
rm -f "${health_tmp}"
if [ "${health_ok}" != "true" ]; then
  echo "health failed: expected HTTP 200 body ok; check gateway Ingress (voice-gateway-http/https) and DNS for ${BASE#https://}"
  exit 1
fi

echo "Smoke: GET ${BASE}/api/v1/version?platform=windows&version=1.0.0"
version_tmp="$(mktemp)"
version_code="$(curl -sS -o "${version_tmp}" -w "%{http_code}" \
  "${BASE}/api/v1/version?platform=windows&version=1.0.0" || echo "000")"
version_body="$(tr -d '\r' < "${version_tmp}")"
rm -f "${version_tmp}"
if [ "${version_code}" != "200" ] || ! echo "${version_body}" | grep -q '"latest_version"'; then
  echo "version failed: HTTP ${version_code} body=${version_body}"
  exit 1
fi

# Auth register requires body — 400/422 without body is OK (proves route exists).
code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${BASE}/api/v1/auth/register" \
  -H "Content-Type: application/json" -d '{}')
if [ "$code" = "000" ]; then
  echo "auth route unreachable"
  exit 1
fi
echo "Smoke: POST /api/v1/auth/register -> HTTP ${code} (upstream wired)"

echo "Staging smoke passed."
