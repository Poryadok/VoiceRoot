#!/usr/bin/env bash
# Source deploy/prod/domains.defaults; preserve caller env overrides.
set -euo pipefail

_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
_defaults="${_root}/deploy/prod/domains.defaults"

_saved_gw="${VOICE_GATEWAY_INGRESS_HOST:-}"
_saved_dp="${VOICE_DEVELOPER_PORTAL_INGRESS_HOST:-}"
_saved_web="${VOICE_WEB_INGRESS_HOST:-}"
_saved_admin="${VOICE_ADMIN_INGRESS_HOST:-}"
_saved_livekit="${VOICE_LIVEKIT_INGRESS_HOST:-}"
_saved_base="${VOICE_PROD_BASE_DOMAIN:-}"

if [ -f "${_defaults}" ]; then
  # shellcheck disable=SC1090
  set -a
  source "${_defaults}"
  set +a
fi

[ -n "${_saved_gw}" ] && VOICE_GATEWAY_INGRESS_HOST="${_saved_gw}"
[ -n "${_saved_dp}" ] && VOICE_DEVELOPER_PORTAL_INGRESS_HOST="${_saved_dp}"
[ -n "${_saved_web}" ] && VOICE_WEB_INGRESS_HOST="${_saved_web}"
[ -n "${_saved_admin}" ] && VOICE_ADMIN_INGRESS_HOST="${_saved_admin}"
[ -n "${_saved_livekit}" ] && VOICE_LIVEKIT_INGRESS_HOST="${_saved_livekit}"
[ -n "${_saved_base}" ] && VOICE_PROD_BASE_DOMAIN="${_saved_base}"

export VOICE_PROD_BASE_DOMAIN VOICE_GATEWAY_INGRESS_HOST VOICE_DEVELOPER_PORTAL_INGRESS_HOST
export VOICE_WEB_INGRESS_HOST VOICE_ADMIN_INGRESS_HOST VOICE_LIVEKIT_INGRESS_HOST
