#!/usr/bin/env bash
# Fail closed before any staging apply or rollout if Auth would use NoopMailSender.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
CHECK="${ROOT}/scripts/staging/check-resend-key.py"

validate_manifest() {
  if [ "${STAGING_SECRET_OFFLINE_PARSE:-}" = 1 ]; then
    python3 "${CHECK}" "${NS}" --yaml
  else
    kubectl create --dry-run=client --validate=false -f - -o json 2>/dev/null | python3 "${CHECK}" "${NS}"
  fi
}

failure='invalid Secret document'
if [ -n "${STAGING_APP_SECRETS_YAML_B64:-}" ]; then
  if ! failure="$(printf '%s' "${STAGING_APP_SECRETS_YAML_B64}" | base64 -d 2>/dev/null | validate_manifest)"; then
    echo "ERROR: staging voice-app-secrets preflight failed: ${failure}" >&2
    exit 1
  fi
elif [ -f "${ROOT}/deploy/staging/secret.yaml" ]; then
  if ! failure="$(validate_manifest <"${ROOT}/deploy/staging/secret.yaml")"; then
    echo "ERROR: staging voice-app-secrets preflight failed: ${failure}" >&2
    exit 1
  fi
elif [ -n "${STAGING_APP_SECRETS_YAML:-}" ]; then
  if [ ! -f "${STAGING_APP_SECRETS_YAML}" ] ||
     ! failure="$(validate_manifest <"${STAGING_APP_SECRETS_YAML}")"; then
    echo "ERROR: staging voice-app-secrets preflight failed: ${failure}" >&2
    exit 1
  fi
else
  if ! failure="$(kubectl get secret voice-app-secrets -n "${NS}" -o json 2>/dev/null |
     python3 "${CHECK}" "${NS}")"; then
    echo "ERROR: staging voice-app-secrets preflight failed: ${failure}" >&2
    exit 1
  fi
fi

echo 'Staging app Secret preflight passed.'
