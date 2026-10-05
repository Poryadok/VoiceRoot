#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/compose-fcm-diagnostic-lib.sh"

phase_known='phase_evidence=known consumer_bound_before_expiry=false callback_entered_before_expiry=false decode_succeeded_before_expiry=false route_started_before_expiry=false route_returned_before_expiry=false consumer_bound_at_expiry=true callback_entered_at_expiry=true decode_succeeded_at_expiry=true route_started_at_expiry=true route_returned_at_expiry=true'
phase_unknown='phase_evidence=unknown consumer_bound_before_expiry=unknown callback_entered_before_expiry=unknown decode_succeeded_before_expiry=unknown route_started_before_expiry=unknown route_returned_before_expiry=unknown consumer_bound_at_expiry=unknown callback_entered_at_expiry=unknown decode_succeeded_at_expiry=unknown route_started_at_expiry=unknown route_returned_at_expiry=unknown'
valid="compose_fcm_diag valid=true reason=matched admission=none candidates=1 attempts=2 member_result=ok member_count=2 recipient_present=true inbox=main base_push=true final_push=true presence=offline policy=ok token_rows=1 fcm_tokens=1 dispatcher_returns=1 route=ack ${phase_known}"
unknown="${VOICE_FCM_DIAG_UNKNOWN}"
unknownPhase="${valid% phase_evidence=*} ${phase_unknown}"
[[ "$(voice_fcm_diag_parse_log "${valid}")" == "${valid}" ]]
parsed=''
voice_fcm_diag_status_valid 0 $'pre=ok\npost=ok\n'
! voice_fcm_diag_status_valid 1 $'pre=ok\npost=ok\n'
! voice_fcm_diag_status_valid 0 $'pre=ok\npost=unknown\n'
! voice_fcm_diag_status_valid 0 $'pre=ok\npost=ok\npost=ok\n'
lifecycle=$'compose_fcm_lifecycle control_sample=invalid\ncompose_fcm_lifecycle control_sample=valid\ncompose_fcm_lifecycle window_expiry=completed'
voice_fcm_diag_lifecycle_valid "${lifecycle}"
voice_fcm_diag_lifecycle_valid $'compose_fcm_lifecycle control_sample=valid\ncompose_fcm_lifecycle window_expiry=completed'
! voice_fcm_diag_lifecycle_valid $'compose_fcm_lifecycle control_sample=valid\ncompose_fcm_lifecycle control_sample=valid\ncompose_fcm_lifecycle window_expiry=completed'
! voice_fcm_diag_lifecycle_valid $'compose_fcm_lifecycle control_sample=valid\ncompose_fcm_lifecycle window_expiry=completed\ncompose_fcm_lifecycle private=unknown'
! voice_fcm_diag_lifecycle_valid $'compose_fcm_lifecycle control_sample=valid'
voice_fcm_diag_parse_collected_log 0 "${valid}" parsed
[[ "${VOICE_FCM_DIAG_PARSE_RESULT}" == accepted && "${parsed}" == "${valid}" ]]
[[ "$(voice_fcm_diag_parse_log "${unknownPhase}")" == "${unknownPhase}" ]]
unknownTarget="compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown ${phase_known}"
[[ "$(voice_fcm_diag_parse_log "${unknownTarget}")" == "${unknownTarget}" ]]
for reason in expired incomplete; do
  emitted="${unknownTarget/reason=unknown/reason=${reason}}"
  [[ "$(voice_fcm_diag_parse_log "${emitted}")" == "${emitted}" ]]
done
[[ "$(voice_fcm_diag_parse_log "${unknownTarget/recipient_present=unknown/recipient_present=true}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${unknownTarget/attempts=0/attempts=1}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${unknownTarget/reason=unknown/reason=matched}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${unknownTarget/valid=false/valid=true}")" == "${unknown}" ]]
incompleteValid="${unknownTarget/valid=false/valid=true}"
incompleteValid="${incompleteValid/reason=unknown/reason=incomplete}"
[[ "$(voice_fcm_diag_parse_log "${incompleteValid}")" == "${unknown}" ]]
incompleteAttempts="${unknownTarget/reason=unknown/reason=incomplete}"
incompleteAttempts="${incompleteAttempts/attempts=0/attempts=1}"
[[ "$(voice_fcm_diag_parse_log "${incompleteAttempts}")" == "${unknown}" ]]
expiredRoute="${unknownTarget/reason=unknown/reason=expired}"
expiredRoute="${expiredRoute/route=unknown/route=ack}"
[[ "$(voice_fcm_diag_parse_log "${expiredRoute}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid% phase_evidence=*}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid/callback_entered_before_expiry=false/callback_entered_before_expiry=unknown}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid/phase_evidence=known/phase_evidence=unknown}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid} callback_entered_at_expiry=true")" == "${unknown}" ]]
for admission in invalid window tuple identity mixed overflow; do
  candidate="${valid/admission=none/admission=${admission}}"
  [[ "$(voice_fcm_diag_parse_log "${candidate}")" == "${unknown}" ]]
done
[[ "$(voice_fcm_diag_parse_log '')" == "${unknown}" ]]
voice_fcm_diag_parse_collected_log 0 '' parsed
[[ "${VOICE_FCM_DIAG_PARSE_RESULT}" == missing && "${parsed}" == "${unknown}" ]]
voice_fcm_diag_parse_collected_log 0 "${valid/admission=none/admission=untrusted}" parsed
[[ "${VOICE_FCM_DIAG_PARSE_RESULT}" == malformed && "${parsed}" == "${unknown}" ]]
voice_fcm_diag_parse_collected_log 0 "${valid}"$'\n'"${valid}" parsed
[[ "${VOICE_FCM_DIAG_PARSE_RESULT}" == duplicate && "${parsed}" == "${unknown}" ]]
voice_fcm_diag_parse_collected_log 124 "${valid}" parsed
[[ "${VOICE_FCM_DIAG_PARSE_RESULT}" == unknown && "${parsed}" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid}"$'\n'"${valid}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log 'password=private-sentinel')" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "untrusted-prefix ${valid}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid}"$'\n'"untrusted-prefix ${valid}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid/ candidates=1 / candidates=17 }")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid/admission=none/admission=untrusted}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid/admission=none/admission=private}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid/ member_count=2 / member_count=10000 }")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid%route=ack}route=unk")" == "${unknown}" ]]
oversized="$(printf '%65537s' x)"
voice_fcm_diag_parse_log "${oversized}" parsed
[[ "${parsed}" == "${unknown}" ]]
[[ "${VOICE_FCM_DIAG_PARSE_RESULT}" == unknown ]]
many_lines=''
for _ in {1..101}; do many_lines+=$'\n'; done
voice_fcm_diag_parse_log "${many_lines}" parsed
[[ "${parsed}" == "${unknown}" ]]
[[ "${VOICE_FCM_DIAG_PARSE_RESULT}" == unknown ]]
! voice_fcm_diag_identity_valid '' 4
! voice_fcm_diag_identity_valid 0 4
! voice_fcm_diag_identity_valid 4 0
! voice_fcm_diag_identity_valid root 4
voice_fcm_diag_identity_valid 1000 1000

tmp="$(mktemp -d)"
file="${tmp}/control"
status="${tmp}/status"
: >"${file}"
: >"${status}"
voice_fcm_diag_cleanup "${file}" "${tmp}" "${status}"
[[ ! -e "${file}" && ! -e "${status}" && ! -e "${tmp}" ]]

tmp="$(mktemp -d)"
file="${tmp}/control"
: >"${file}"
mkdir "${tmp}/not-empty"
if voice_fcm_diag_cleanup "${file}" "${tmp}"; then
  exit 1
fi
[[ ! -e "${file}" && -d "${tmp}" ]]
rmdir "${tmp}/not-empty"
rmdir "${tmp}"

if voice_fcm_diag_preserve_status 7; then
  exit 1
else
  [[ "$?" == 7 ]]
fi

printf '%s\n' 'compose_fcm_diag_contract=pass'
