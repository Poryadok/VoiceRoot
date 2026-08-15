#!/usr/bin/env bash
# Map PROD_* operator env vars to STAGING_* names used by shared staging ops scripts.
# Source from prod render/apply wrappers (do not execute directly).
set -euo pipefail

if [ -n "${PROD_APP_SECRETS_YAML_B64:-}" ]; then
  export STAGING_APP_SECRETS_YAML_B64="${PROD_APP_SECRETS_YAML_B64}"
fi
if [ -n "${PROD_APP_SECRETS_YAML:-}" ]; then
  export STAGING_APP_SECRETS_YAML="${PROD_APP_SECRETS_YAML}"
fi
if [ -n "${PROD_STAFF_TOKEN:-}" ]; then
  export STAGING_STAFF_TOKEN="${PROD_STAFF_TOKEN}"
fi
if [ -n "${VOICE_PROD_K8S_NAMESPACE:-}" ] && [ -z "${VOICE_K8S_NAMESPACE:-}" ]; then
  export VOICE_K8S_NAMESPACE="${VOICE_PROD_K8S_NAMESPACE}"
fi
if [ -n "${VOICE_PROD_IMAGE_PULL_SECRET:-}" ] && [ -z "${VOICE_IMAGE_PULL_SECRET:-}" ]; then
  export VOICE_IMAGE_PULL_SECRET="${VOICE_PROD_IMAGE_PULL_SECRET}"
fi
if [ "${VOICE_PROD_APPLY_OBSERVABILITY:-}" = "true" ] && [ -z "${VOICE_APPLY_OBSERVABILITY:-}" ]; then
  export VOICE_APPLY_OBSERVABILITY=true
fi
