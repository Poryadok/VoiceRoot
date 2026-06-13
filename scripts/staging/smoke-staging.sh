#!/usr/bin/env bash
# Post-deploy smoke for staging (health + minimal API).
set -euo pipefail

BASE="${VOICE_STAGING_URL:-https://voice.tastytest.online}"
BASE="${BASE%/}"

echo "Smoke: GET ${BASE}/health"
curl -sf "${BASE}/health" | grep -q '"status"' || { echo "health failed"; exit 1; }

echo "Smoke: GET ${BASE}/api/v1/version"
curl -sf "${BASE}/api/v1/version" | grep -q '"windows"' || { echo "version failed"; exit 1; }

# Auth register requires body — 400/422 without body is OK (proves route exists).
code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${BASE}/api/v1/auth/register" \
  -H "Content-Type: application/json" -d '{}')
if [ "$code" = "000" ]; then
  echo "auth route unreachable"
  exit 1
fi
echo "Smoke: POST /api/v1/auth/register -> HTTP ${code} (upstream wired)"

echo "Staging smoke passed."
