#!/usr/bin/env bash
# Restore the dedicated User→Search cursor signing key without rotating it.
set -euo pipefail

NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
SECRET=voice-user-search-projection
KEY=cursor-hmac-key

if kubectl get secret "${SECRET}" -n "${NS}" >/dev/null 2>&1; then
  if [ "$(kubectl get secret "${SECRET}" -n "${NS}" \
    -o go-template='{{if index .data "cursor-hmac-key"}}ok{{end}}')" != ok ]; then
    echo "ERROR: ${SECRET} exists but lacks ${KEY} in ${NS}" >&2
    exit 1
  fi
  echo "User search cursor key already present in ${NS}"
  exit 0
fi

: "${STAGING_USER_SEARCH_CURSOR_HMAC_KEY:?Set the staging environment secret STAGING_USER_SEARCH_CURSOR_HMAC_KEY}"
if [ "${#STAGING_USER_SEARCH_CURSOR_HMAC_KEY}" -lt 32 ]; then
  echo "ERROR: staging User search cursor key must have at least 32 bytes" >&2
  exit 1
fi

key_b64="$(printf '%s' "${STAGING_USER_SEARCH_CURSOR_HMAC_KEY}" | base64 | tr -d '\r\n')"
printf '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"%s"},"type":"Opaque","data":{"%s":"%s"}}' \
  "${SECRET}" "${KEY}" "${key_b64}" | kubectl create -n "${NS}" -f -
