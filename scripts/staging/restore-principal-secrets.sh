#!/usr/bin/env bash
# Restore the staging principal key/certificate set only when none of it exists.
set -euo pipefail

NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
if [ "${NS}" != voice-staging ]; then
  echo "ERROR: staging principal bundle cannot be applied to ${NS}" >&2
  exit 1
fi

secrets=(
  voice-social-principal-signing
  voice-file-principal-signing
  voice-search-principal-signing
  voice-principal-ca
  voice-social-principal-tls
  voice-user-principal-tls
  voice-space-principal-tls
  voice-file-principal-tls
  voice-user-file-principal-tls
  voice-search-principal-tls
)

present=0
for name in "${secrets[@]}"; do
  if kubectl get secret "${name}" -n "${NS}" >/dev/null 2>&1; then
    present=$((present + 1))
  fi
done

if [ "${present}" -eq "${#secrets[@]}" ]; then
  echo "Principal secrets already present in ${NS}"
  exit 0
fi
if [ "${present}" -ne 0 ]; then
  echo "ERROR: partial principal secret set in ${NS}; restore the missing Secrets from the staging secret manager" >&2
  exit 1
fi

: "${STAGING_PRINCIPAL_SECRETS_B64:?Set the staging environment secret STAGING_PRINCIPAL_SECRETS_B64}"
printf '%s' "${STAGING_PRINCIPAL_SECRETS_B64}" | base64 -d | gzip -d | kubectl create -n "${NS}" -f -
