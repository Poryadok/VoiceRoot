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

if [ -n "${STAGING_STAFF_TOKEN:-}" ]; then
  echo "Smoke: GET ${BASE}/api/v1/analytics/dashboard/product (staff)"
  dash_code="$(curl -sS -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer ${STAGING_STAFF_TOKEN}" \
    "${BASE}/api/v1/analytics/dashboard/product" || echo "000")"
  if [ "${dash_code}" != "200" ]; then
    echo "analytics dashboard failed: HTTP ${dash_code}"
    exit 1
  fi

  echo "Smoke: GET ${BASE}/api/v1/analytics/export?format=csv (staff)"
  export_code="$(curl -sS -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer ${STAGING_STAFF_TOKEN}" \
    "${BASE}/api/v1/analytics/export?format=csv" || echo "000")"
  if [ "${export_code}" != "200" ]; then
    echo "analytics export failed: HTTP ${export_code}"
    exit 1
  fi
else
  echo "Smoke: skipping analytics checks (STAGING_STAFF_TOKEN not set)"
fi

if [ -n "${VOICE_DEVELOPER_PORTAL_INGRESS_HOST:-}" ]; then
  PORTAL_URL="https://${VOICE_DEVELOPER_PORTAL_INGRESS_HOST}"
  PORTAL_URL="${PORTAL_URL%/}"
  echo "Smoke: GET ${PORTAL_URL}/ (developer portal)"
  portal_code="$(curl -sS -o /dev/null -w "%{http_code}" "${PORTAL_URL}/" || echo "000")"
  if [ "${portal_code}" != "200" ]; then
    echo "developer portal failed: HTTP ${portal_code}"
    exit 1
  fi
else
  echo "Smoke: skipping developer portal checks (VOICE_DEVELOPER_PORTAL_INGRESS_HOST not set)"
fi

if [ -n "${VOICE_ADMIN_INGRESS_HOST:-}" ]; then
  ADMIN_URL="https://${VOICE_ADMIN_INGRESS_HOST}"
  ADMIN_URL="${ADMIN_URL%/}"
  echo "Smoke: GET ${ADMIN_URL}/health (admin)"
  admin_health_tmp="$(mktemp)"
  admin_health_ok=false
  for attempt in 1 2 3 4 5 6; do
    admin_health_code="$(curl -sS -o "${admin_health_tmp}" -w "%{http_code}" "${ADMIN_URL}/health" || echo "000")"
    admin_health_body="$(tr -d '\r' < "${admin_health_tmp}")"
    if [ "${admin_health_code}" = "200" ] && [ "${admin_health_body}" = "ok" ]; then
      admin_health_ok=true
      break
    fi
    echo "admin health attempt ${attempt}/6: HTTP ${admin_health_code} body=${admin_health_body}"
    sleep 5
  done
  rm -f "${admin_health_tmp}"
  if [ "${admin_health_ok}" != "true" ]; then
    echo "admin health failed: expected HTTP 200 body ok; check Ingress and DNS for ${VOICE_ADMIN_INGRESS_HOST}"
    exit 1
  fi
else
  echo "Smoke: skipping admin checks (VOICE_ADMIN_INGRESS_HOST not set)"
fi

if [ -n "${VOICE_WEB_INGRESS_HOST:-}" ]; then
  WEB_URL="https://${VOICE_WEB_INGRESS_HOST}"
  WEB_URL="${WEB_URL%/}"

  echo "Smoke: GET ${WEB_URL}/health (Flutter web)"
  web_health_tmp="$(mktemp)"
  web_health_ok=false
  for attempt in 1 2 3 4 5 6; do
    web_health_code="$(curl -sS -o "${web_health_tmp}" -w "%{http_code}" "${WEB_URL}/health" || echo "000")"
    web_health_body="$(tr -d '\r' < "${web_health_tmp}")"
    if [ "${web_health_code}" = "200" ] && [ "${web_health_body}" = "ok" ]; then
      web_health_ok=true
      break
    fi
    echo "web health attempt ${attempt}/6: HTTP ${web_health_code} body=${web_health_body}"
    sleep 5
  done
  rm -f "${web_health_tmp}"
  if [ "${web_health_ok}" != "true" ]; then
    echo "Flutter web health failed: expected HTTP 200 body ok; check Ingress and DNS for ${VOICE_WEB_INGRESS_HOST}"
    exit 1
  fi

  echo "Smoke: GET ${WEB_URL}/ (Flutter web)"
  web_root_tmp="$(mktemp)"
  web_root_code="$(curl -sS -o "${web_root_tmp}" -w "%{http_code}" "${WEB_URL}/" || echo "000")"
  web_root_body="$(tr -d '\r' < "${web_root_tmp}")"
  rm -f "${web_root_tmp}"
  if [ "${web_root_code}" != "200" ]; then
    echo "Flutter web root failed: HTTP ${web_root_code}"
    exit 1
  fi
  if ! echo "${web_root_body}" | grep -qF '<!DOCTYPE html' && ! echo "${web_root_body}" | grep -qF 'flutter_bootstrap'; then
    echo "Flutter web root failed: body missing <!DOCTYPE html or flutter_bootstrap"
    exit 1
  fi
else
  echo "Smoke: skipping Flutter web checks (VOICE_WEB_INGRESS_HOST not set)"
fi

livekit_signaling_ok() {
  case "$1" in
    101|200|400) return 0 ;;
    *) return 1 ;;
  esac
}

if [ -n "${VOICE_LIVEKIT_INGRESS_HOST:-}" ]; then
  LIVEKIT_WS="wss://${VOICE_LIVEKIT_INGRESS_HOST}"
  echo "Smoke: LiveKit signaling probe ${LIVEKIT_WS}"
  if command -v websocat >/dev/null 2>&1; then
    if ! timeout 10 websocat -k -n1 "${LIVEKIT_WS}" </dev/null 2>/dev/null; then
      echo "LiveKit WebSocket probe failed for ${LIVEKIT_WS}"
      exit 1
    fi
  else
    # Ingress is Traefik `web` (HTTP). DNS-only livekit (grey cloud) also exposes origin TLS
    # with a self-signed cert — skip verification for the HTTPS probe.
    lk_ws_headers=(
      -H "Connection: Upgrade"
      -H "Upgrade: websocket"
      -H "Sec-WebSocket-Version: 13"
      -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ=="
    )
    lk_code="$(curl -sS -o /dev/null -w "%{http_code}" \
      "${lk_ws_headers[@]}" \
      "http://${VOICE_LIVEKIT_INGRESS_HOST}/" 2>/dev/null || echo "000")"
    if ! livekit_signaling_ok "${lk_code}"; then
      lk_code="$(curl -sk -o /dev/null -w "%{http_code}" \
        "${lk_ws_headers[@]}" \
        "https://${VOICE_LIVEKIT_INGRESS_HOST}/" 2>/dev/null || echo "000")"
    fi
    if ! livekit_signaling_ok "${lk_code}"; then
      echo "LiveKit signaling probe failed: HTTP ${lk_code} (expected 101/200/400)"
      exit 1
    fi
  fi
else
  echo "Smoke: skipping LiveKit checks (VOICE_LIVEKIT_INGRESS_HOST not set)"
fi

echo "Staging smoke passed."
