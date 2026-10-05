# Sourced only by the private Compose smoke and its local shell contract checks.

VOICE_FCM_DIAG_UNKNOWN='compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'

voice_fcm_diag_emit() {
  local text="${1-}" output_var="${2-}"
  if [[ -n "${output_var}" ]]; then
    printf -v "${output_var}" '%s' "${text}"
  else
    printf '%s\n' "${text}"
  fi
}

voice_fcm_diag_identity_valid() {
  [[ "${1:-}" =~ ^[1-9][0-9]{0,8}$ && "${2:-}" =~ ^[1-9][0-9]{0,8}$ ]]
}

voice_fcm_diag_parse_log() {
  local data="${1-}" output_var="${2-}" line count=0 line_count=0 found='' phase_index malformed=false
  local LC_ALL=C
  VOICE_FCM_DIAG_PARSE_RESULT=unknown
  if (( ${#data} > 65536 )); then
    VOICE_FCM_DIAG_PARSE_RESULT=unknown
    voice_fcm_diag_emit "${VOICE_FCM_DIAG_UNKNOWN}" "${output_var}"
    return 0
  fi
  while IFS= read -r line; do
    ((line_count+=1))
    if ((line_count > 100)); then
      VOICE_FCM_DIAG_PARSE_RESULT=unknown
      voice_fcm_diag_emit "${VOICE_FCM_DIAG_UNKNOWN}" "${output_var}"
      return 0
    fi
    if [[ "${line}" == *compose_fcm_diag* ]]; then
      ((count+=1))
      if ((count > 1)); then
        VOICE_FCM_DIAG_PARSE_RESULT=duplicate
        continue
      fi
      if [[ ! "${line}" =~ ^(compose_fcm_diag\ valid=(true|false)\ reason=(matched|unknown|ambiguous|overflow|expired|incomplete)\ admission=(none|invalid|window|tuple|identity|mixed|overflow)\ candidates=([0-9]{1,2})\ attempts=([0-9]{1,4})\ member_result=(ok|error|unknown)\ member_count=([0-9]{1,4})\ recipient_present=(true|false|unknown)\ inbox=(main|requests|unknown)\ base_push=(true|false|unknown)\ final_push=(true|false|unknown)\ presence=(online|offline|unknown)\ policy=(ok|error|unknown)\ token_rows=([0-9]{1,4})\ fcm_tokens=([0-9]{1,4})\ dispatcher_returns=([0-9]{1,4})\ route=(ack|nak|unknown)\ phase_evidence=(known|unknown)\ consumer_bound_before_expiry=(true|false|unknown)\ callback_entered_before_expiry=(true|false|unknown)\ decode_succeeded_before_expiry=(true|false|unknown)\ route_started_before_expiry=(true|false|unknown)\ route_returned_before_expiry=(true|false|unknown)\ consumer_bound_at_expiry=(true|false|unknown)\ callback_entered_at_expiry=(true|false|unknown)\ decode_succeeded_at_expiry=(true|false|unknown)\ route_started_at_expiry=(true|false|unknown)\ route_returned_at_expiry=(true|false|unknown))$ ]]; then
        malformed=true
        continue
      fi
      if [[ "${BASH_REMATCH[2]}" == "true" && ( "${BASH_REMATCH[3]}" != "matched" || "${BASH_REMATCH[4]}" != "none" ) ]]; then
        malformed=true
        continue
      fi
      if [[ "${BASH_REMATCH[2]}" == "false" && ( "${BASH_REMATCH[3]}" == "matched" || "${BASH_REMATCH[6]}" != "0" ) ]]; then
        malformed=true
        continue
      fi
      if ((10#${BASH_REMATCH[5]} > 16)); then
        malformed=true
        continue
      fi
      if [[ "${BASH_REMATCH[19]}" == "known" ]]; then
        for phase_index in {20..29}; do
          if [[ "${BASH_REMATCH[$phase_index]}" == "unknown" ]]; then
            malformed=true
            break
          fi
        done
      else
        for phase_index in {20..29}; do
          if [[ "${BASH_REMATCH[$phase_index]}" != "unknown" ]]; then
            malformed=true
            break
          fi
        done
      fi
      if [[ "${malformed}" == true ]]; then continue; fi
      if [[ "${BASH_REMATCH[2]}" == "false" && ( "${BASH_REMATCH[7]}" != "unknown" || "${BASH_REMATCH[8]}" != "0" || "${BASH_REMATCH[9]}" != "unknown" || "${BASH_REMATCH[10]}" != "unknown" || "${BASH_REMATCH[11]}" != "unknown" || "${BASH_REMATCH[12]}" != "unknown" || "${BASH_REMATCH[13]}" != "unknown" || "${BASH_REMATCH[14]}" != "unknown" || "${BASH_REMATCH[15]}" != "0" || "${BASH_REMATCH[16]}" != "0" || "${BASH_REMATCH[17]}" != "0" || "${BASH_REMATCH[18]}" != "unknown" ) ]]; then
        malformed=true
        continue
      fi
      found="${BASH_REMATCH[1]}"
    fi
  done <<<"${data}"
  if ((count == 1)); then
    if [[ "${malformed}" == true ]]; then
      VOICE_FCM_DIAG_PARSE_RESULT=malformed
      voice_fcm_diag_emit "${VOICE_FCM_DIAG_UNKNOWN}" "${output_var}"
    else
      VOICE_FCM_DIAG_PARSE_RESULT=accepted
      voice_fcm_diag_emit "${found}" "${output_var}"
    fi
  elif ((count > 1)); then
    VOICE_FCM_DIAG_PARSE_RESULT=duplicate
    voice_fcm_diag_emit "${VOICE_FCM_DIAG_UNKNOWN}" "${output_var}"
  else
    VOICE_FCM_DIAG_PARSE_RESULT=missing
    voice_fcm_diag_emit "${VOICE_FCM_DIAG_UNKNOWN}" "${output_var}"
  fi
}

voice_fcm_diag_parse_collected_log() {
  local read_status="${1-}" data="${2-}" output_var="${3-}"
  if [[ "${read_status}" != 0 ]]; then
    VOICE_FCM_DIAG_PARSE_RESULT=unknown
    voice_fcm_diag_emit "${VOICE_FCM_DIAG_UNKNOWN}" "${output_var}"
    return 0
  fi
  voice_fcm_diag_parse_log "${data}" "${output_var}"
}

voice_fcm_diag_status_valid() {
  local data="${2-}" pre post
  local status_re=$'^pre=(ok|failed|unknown)\npost=(ok|failed|unknown)\n$'
  VOICE_FCM_DIAG_STATUS_SUMMARY=''
  [[ "${1-}" == 0 && "${data}" =~ ${status_re} ]] || return 1
  pre="${BASH_REMATCH[1]}"
  post="${BASH_REMATCH[2]}"
  case "${pre}/${post}" in
    unknown/unknown|failed/unknown|ok/unknown|ok/ok|ok/failed) ;;
    *) return 1 ;;
  esac
  VOICE_FCM_DIAG_STATUS_SUMMARY="compose_fcm_status pre=${pre} post=${post}"
}

voice_fcm_diag_lifecycle_valid() {
  local data="${1-}" line invalid=0 valid=0 expired=0
  VOICE_FCM_DIAG_LIFECYCLE_SUMMARY=''
  while IFS= read -r line; do
    case "${line}" in
      'compose_fcm_lifecycle control_sample=invalid') ((invalid+=1)) ;;
      'compose_fcm_lifecycle control_sample=valid') ((valid+=1)) ;;
      'compose_fcm_lifecycle window_expiry=completed') ((expired+=1)) ;;
      *compose_fcm_lifecycle*|*compose_fcm_lifecycl*) return 1 ;;
    esac
  done <<<"${data}"
  ((invalid <= 1 && valid <= 1 && expired <= 1)) || return 1
  VOICE_FCM_DIAG_LIFECYCLE_SUMMARY="compose_fcm_lifecycle_status sample_invalid=$([[ ${invalid} == 1 ]] && printf observed || printf missing) sample_valid=$([[ ${valid} == 1 ]] && printf observed || printf missing) window_expiry=$([[ ${expired} == 1 ]] && printf observed || printf missing)"
}

voice_fcm_diag_since_valid() {
  [[ "${1-}" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{9}Z$ ]]
}

voice_fcm_diag_timestamp_in_window() {
  local event_time="${1-}" since="${2-}"
  local LC_ALL=C
  voice_fcm_diag_since_valid "${event_time}" && \
    voice_fcm_diag_since_valid "${since}" && [[ "${event_time}" > "${since}" || "${event_time}" == "${since}" ]]
}

voice_fcm_diag_normalize_collected() {
  local status_read="${1-}" status_data="${2-}" log_status="${3-}"
  local log_data="${4-}" parse_result="${5-}" output_var="${6-}" summary_text=''
  VOICE_FCM_DIAG_STATUS_SUMMARY=''
  VOICE_FCM_DIAG_LIFECYCLE_SUMMARY=''
  if ! voice_fcm_diag_status_valid "${status_read}" "${status_data}" || \
      [[ "${log_status}" != 0 ]] || \
      ! voice_fcm_diag_lifecycle_valid "${log_data}" || \
      [[ "${parse_result}" != accepted && "${parse_result}" != missing ]]; then
    VOICE_FCM_DIAG_STATUS_SUMMARY=''
    VOICE_FCM_DIAG_LIFECYCLE_SUMMARY=''
    voice_fcm_diag_emit '' "${output_var}"
    return 1
  fi
  summary_text="${VOICE_FCM_DIAG_STATUS_SUMMARY}"$'\n'"${VOICE_FCM_DIAG_LIFECYCLE_SUMMARY}"
  voice_fcm_diag_emit "${summary_text}" "${output_var}"
}

voice_fcm_diag_cleanup() {
  local file="${1-}" dir="${2-}" status="${3-}" failed=false
  if [[ -n "${status}" ]] && ! rm -f -- "${status}"; then failed=true; fi
  if [[ -n "${file}" ]] && ! rm -f -- "${file}"; then failed=true; fi
  if [[ -n "${dir}" ]] && ! rmdir -- "${dir}" 2>/dev/null; then failed=true; fi
  [[ "${failed}" == false ]]
}

voice_fcm_diag_preserve_status() {
  return "${1:-1}"
}
