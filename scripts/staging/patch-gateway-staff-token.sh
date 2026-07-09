#!/usr/bin/env bash
# Sync GATEWAY_STATIC_TOKENS_JSON in voice-app-secrets from STAGING_STAFF_TOKEN (CI smoke / staff analytics).
# Idempotent: skips when STAGING_STAFF_TOKEN is unset; updates when set.
set -euo pipefail

NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
SECRET_NAME="voice-app-secrets"
STAFF_USER_ID="${STAGING_STAFF_USER_ID:-00000000-0000-0000-0000-000000000099}"
STAFF_PROFILE_ID="${STAGING_STAFF_PROFILE_ID:-00000000-0000-0000-0000-000000000199}"

if [ -z "${STAGING_STAFF_TOKEN:-}" ]; then
  echo "STAGING_STAFF_TOKEN unset — skip GATEWAY_STATIC_TOKENS_JSON patch"
  exit 0
fi

if ! kubectl get secret "$SECRET_NAME" -n "$NS" >/dev/null 2>&1; then
  echo "WARN: ${SECRET_NAME} missing in ${NS}; skip staff token patch" >&2
  exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq required for patch-gateway-staff-token.sh" >&2
  exit 1
fi

static_json="$(jq -nc \
  --arg token "${STAGING_STAFF_TOKEN}" \
  --arg user_id "${STAFF_USER_ID}" \
  --arg profile_id "${STAFF_PROFILE_ID}" \
  '{($token): {user_id: $user_id, profile_id: $profile_id, roles: ["staff"]}}')"

current="$(kubectl get secret "$SECRET_NAME" -n "$NS" -o "jsonpath={.data.GATEWAY_STATIC_TOKENS_JSON}" 2>/dev/null | base64 -d 2>/dev/null || true)"
if [ "${current}" = "${static_json}" ]; then
  echo "${SECRET_NAME}: GATEWAY_STATIC_TOKENS_JSON already synced"
  exit 0
fi

echo "Patching ${SECRET_NAME}: GATEWAY_STATIC_TOKENS_JSON (staff smoke token)"
patch_payload="$(jq -nc --arg json "${static_json}" \
  '{stringData: {GATEWAY_STATIC_TOKENS_JSON: $json}}')"
kubectl patch secret "$SECRET_NAME" -n "$NS" --type=merge -p "${patch_payload}"

echo "Patched ${SECRET_NAME} staff token in ${NS}"
