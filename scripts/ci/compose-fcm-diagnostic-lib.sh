# Sourced only by the private Compose smoke and its local shell contract checks.

VOICE_FCM_DIAG_UNKNOWN='compose_fcm_diag valid=false reason=unknown candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'

voice_fcm_diag_identity_valid() {
  [[ "${1:-}" =~ ^[1-9][0-9]{0,8}$ && "${2:-}" =~ ^[1-9][0-9]{0,8}$ ]]
}

voice_fcm_diag_parse_log() {
  local data="${1-}" line count=0 line_count=0 found=''
  local LC_ALL=C
  if (( ${#data} > 65536 )); then
    printf '%s\n' "${VOICE_FCM_DIAG_UNKNOWN}"
    return 0
  fi
  while IFS= read -r line; do
    ((line_count+=1))
    if ((line_count > 100)); then
      printf '%s\n' "${VOICE_FCM_DIAG_UNKNOWN}"
      return 0
    fi
    if [[ "${line}" == *compose_fcm_diag* ]]; then
      if [[ ! "${line}" =~ ^(compose_fcm_diag\ valid=(true|false)\ reason=(matched|unknown|ambiguous|overflow)\ candidates=([0-9]{1,2})\ attempts=([0-9]{1,4})\ member_result=(ok|error|unknown)\ member_count=([0-9]{1,4})\ recipient_present=(true|false|unknown)\ inbox=(main|requests|unknown)\ base_push=(true|false|unknown)\ final_push=(true|false|unknown)\ presence=(online|offline|unknown)\ policy=(ok|error|unknown)\ token_rows=([0-9]{1,4})\ fcm_tokens=([0-9]{1,4})\ dispatcher_returns=([0-9]{1,4})\ route=(ack|nak|unknown))$ ]]; then
        printf '%s\n' "${VOICE_FCM_DIAG_UNKNOWN}"
        return 0
      fi
      if ((10#${BASH_REMATCH[4]} > 16)); then
        printf '%s\n' "${VOICE_FCM_DIAG_UNKNOWN}"
        return 0
      fi
      found="${BASH_REMATCH[1]}"
      ((count+=1))
    fi
  done <<<"${data}"
  if ((count == 1)); then
    printf '%s\n' "${found}"
  else
    printf '%s\n' "${VOICE_FCM_DIAG_UNKNOWN}"
  fi
}

voice_fcm_diag_cleanup() {
  local file="${1-}" dir="${2-}"
  [[ -n "${file}" ]] && rm -f -- "${file}" || return 1
  [[ -n "${dir}" ]] && rmdir -- "${dir}" 2>/dev/null || return 1
}

voice_fcm_diag_preserve_status() {
  return "${1:-1}"
}
