# Sourced only by the private Compose smoke and its local shell contract checks.

VOICE_FCM_DIAG_UNKNOWN='compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'

voice_fcm_diag_identity_valid() {
  [[ "${1:-}" =~ ^[1-9][0-9]{0,8}$ && "${2:-}" =~ ^[1-9][0-9]{0,8}$ ]]
}

voice_fcm_diag_parse_log() {
  local data="${1-}" line count=0 line_count=0 found='' phase_index
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
      if [[ ! "${line}" =~ ^(compose_fcm_diag\ valid=(true|false)\ reason=(matched|unknown|ambiguous|overflow|expired|incomplete)\ admission=(none|invalid|window|tuple|identity|mixed|overflow)\ candidates=([0-9]{1,2})\ attempts=([0-9]{1,4})\ member_result=(ok|error|unknown)\ member_count=([0-9]{1,4})\ recipient_present=(true|false|unknown)\ inbox=(main|requests|unknown)\ base_push=(true|false|unknown)\ final_push=(true|false|unknown)\ presence=(online|offline|unknown)\ policy=(ok|error|unknown)\ token_rows=([0-9]{1,4})\ fcm_tokens=([0-9]{1,4})\ dispatcher_returns=([0-9]{1,4})\ route=(ack|nak|unknown)\ phase_evidence=(known|unknown)\ consumer_bound_before_expiry=(true|false|unknown)\ callback_entered_before_expiry=(true|false|unknown)\ decode_succeeded_before_expiry=(true|false|unknown)\ route_started_before_expiry=(true|false|unknown)\ route_returned_before_expiry=(true|false|unknown)\ consumer_bound_at_expiry=(true|false|unknown)\ callback_entered_at_expiry=(true|false|unknown)\ decode_succeeded_at_expiry=(true|false|unknown)\ route_started_at_expiry=(true|false|unknown)\ route_returned_at_expiry=(true|false|unknown))$ ]]; then
        printf '%s\n' "${VOICE_FCM_DIAG_UNKNOWN}"
        return 0
      fi
      if [[ "${BASH_REMATCH[2]}" == "true" && ( "${BASH_REMATCH[3]}" != "matched" || "${BASH_REMATCH[4]}" != "none" ) ]]; then
        printf '%s\n' "${VOICE_FCM_DIAG_UNKNOWN}"
        return 0
      fi
      if [[ "${BASH_REMATCH[2]}" == "false" && ( "${BASH_REMATCH[3]}" == "matched" || "${BASH_REMATCH[6]}" != "0" ) ]]; then
        printf '%s\n' "${VOICE_FCM_DIAG_UNKNOWN}"
        return 0
      fi
      if ((10#${BASH_REMATCH[5]} > 16)); then
        printf '%s\n' "${VOICE_FCM_DIAG_UNKNOWN}"
        return 0
      fi
      if [[ "${BASH_REMATCH[19]}" == "known" ]]; then
        for phase_index in {20..29}; do
          if [[ "${BASH_REMATCH[$phase_index]}" == "unknown" ]]; then
            printf '%s\n' "${VOICE_FCM_DIAG_UNKNOWN}"
            return 0
          fi
        done
      else
        for phase_index in {20..29}; do
          if [[ "${BASH_REMATCH[$phase_index]}" != "unknown" ]]; then
            printf '%s\n' "${VOICE_FCM_DIAG_UNKNOWN}"
            return 0
          fi
        done
      fi
      if [[ "${BASH_REMATCH[2]}" == "false" && ( "${BASH_REMATCH[7]}" != "unknown" || "${BASH_REMATCH[8]}" != "0" || "${BASH_REMATCH[9]}" != "unknown" || "${BASH_REMATCH[10]}" != "unknown" || "${BASH_REMATCH[11]}" != "unknown" || "${BASH_REMATCH[12]}" != "unknown" || "${BASH_REMATCH[13]}" != "unknown" || "${BASH_REMATCH[14]}" != "unknown" || "${BASH_REMATCH[15]}" != "0" || "${BASH_REMATCH[16]}" != "0" || "${BASH_REMATCH[17]}" != "0" || "${BASH_REMATCH[18]}" != "unknown" ) ]]; then
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
