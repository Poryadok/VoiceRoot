#!/usr/bin/env bash
# PKCS#8 PEM validation for voice-app-secrets AUTH_JWT_PRIVATE_KEY.

jwt_pem_is_valid() {
  local pem="${1:-}"
  if [ -z "${pem}" ]; then
    return 1
  fi
  case "${pem}" in
    *REPLACE_WITH*|*CHANGE_ME*|*change-me*|*YOUR_* ) return 1 ;;
  esac
  if ! printf '%s' "${pem}" | grep -Eq 'BEGIN (RSA )?PRIVATE KEY'; then
    return 1
  fi
  local b64
  b64="$(
    printf '%s' "${pem}" \
      | sed -e 's/-----BEGIN [^-]*-----//g' -e 's/-----END [^-]*-----//g' \
      | tr -d '[:space:]'
  )"
  [ -n "${b64}" ] || return 1
  printf '%s' "${b64}" | base64 -d >/dev/null 2>&1
}
