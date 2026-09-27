#!/usr/bin/env bash
# Fail closed before any staging apply or rollout if Auth would use NoopMailSender.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
CHECK="${ROOT}/scripts/staging/check-resend-key.py"

validate_manifest() {
  kubectl create --dry-run=client --validate=false -f - -o json | python3 "${CHECK}" "${NS}"
}

if [ -n "${STAGING_APP_SECRETS_YAML_B64:-}" ]; then
  if ! printf '%s' "${STAGING_APP_SECRETS_YAML_B64}" | base64 -d | validate_manifest >/dev/null 2>&1; then
    echo 'ERROR: staging voice-app-secrets must contain nonblank AUTH_RESEND_API_KEY' >&2
    exit 1
  fi
elif [ -f "${ROOT}/deploy/staging/secret.yaml" ]; then
  if ! validate_manifest <"${ROOT}/deploy/staging/secret.yaml" >/dev/null 2>&1; then
    echo 'ERROR: staging voice-app-secrets must contain nonblank AUTH_RESEND_API_KEY' >&2
    exit 1
  fi
elif [ -n "${STAGING_APP_SECRETS_YAML:-}" ]; then
  if [ ! -f "${STAGING_APP_SECRETS_YAML}" ] ||
     ! validate_manifest <"${STAGING_APP_SECRETS_YAML}" >/dev/null 2>&1; then
    echo 'ERROR: staging voice-app-secrets must contain nonblank AUTH_RESEND_API_KEY' >&2
    exit 1
  fi
else
  if ! kubectl get secret voice-app-secrets -n "${NS}" -o json 2>/dev/null |
     python3 "${CHECK}" "${NS}" >/dev/null 2>&1; then
    echo 'ERROR: staging voice-app-secrets must contain nonblank AUTH_RESEND_API_KEY' >&2
    exit 1
  fi
fi

echo 'Staging Resend credential preflight passed.'
