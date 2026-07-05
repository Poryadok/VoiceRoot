#!/usr/bin/env bash
# Source deploy/staging/domains.defaults; preserve caller env overrides.
set -euo pipefail

_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
_defaults="${_root}/deploy/staging/domains.defaults"

_saved_gw="${VOICE_GATEWAY_INGRESS_HOST:-}"
_saved_dp="${VOICE_DEVELOPER_PORTAL_INGRESS_HOST:-}"
_saved_base="${VOICE_STAGING_BASE_DOMAIN:-}"

if [ -f "${_defaults}" ]; then
  # shellcheck disable=SC1090
  set -a
  source "${_defaults}"
  set +a
fi

[ -n "${_saved_gw}" ] && VOICE_GATEWAY_INGRESS_HOST="${_saved_gw}"
[ -n "${_saved_dp}" ] && VOICE_DEVELOPER_PORTAL_INGRESS_HOST="${_saved_dp}"
[ -n "${_saved_base}" ] && VOICE_STAGING_BASE_DOMAIN="${_saved_base}"

export VOICE_STAGING_BASE_DOMAIN VOICE_GATEWAY_INGRESS_HOST VOICE_DEVELOPER_PORTAL_INGRESS_HOST
