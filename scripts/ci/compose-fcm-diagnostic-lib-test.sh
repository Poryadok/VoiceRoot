#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/compose-fcm-diagnostic-lib.sh"

phase_known='phase_evidence=known consumer_bound_before_expiry=false callback_entered_before_expiry=false decode_succeeded_before_expiry=false route_started_before_expiry=false route_returned_before_expiry=false consumer_bound_at_expiry=true callback_entered_at_expiry=true decode_succeeded_at_expiry=true route_started_at_expiry=true route_returned_at_expiry=true'
phase_unknown='phase_evidence=unknown consumer_bound_before_expiry=unknown callback_entered_before_expiry=unknown decode_succeeded_before_expiry=unknown route_started_before_expiry=unknown route_returned_before_expiry=unknown consumer_bound_at_expiry=unknown callback_entered_at_expiry=unknown decode_succeeded_at_expiry=unknown route_started_at_expiry=unknown route_returned_at_expiry=unknown'
valid="compose_fcm_diag valid=true reason=matched admission=none candidates=1 attempts=2 member_result=ok member_count=2 recipient_present=true inbox=main base_push=true final_push=true presence=offline policy=ok token_rows=1 fcm_tokens=1 dispatcher_returns=1 route=ack ${phase_known}"
unknown="${VOICE_FCM_DIAG_UNKNOWN}"
unknownPhase="${valid% phase_evidence=*} ${phase_unknown}"
[[ "$(voice_fcm_diag_parse_log "${valid}")" == "${valid}" ]]
[[ "$(voice_fcm_diag_parse_log "${unknownPhase}")" == "${unknownPhase}" ]]
unknownTarget="compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown ${phase_known}"
[[ "$(voice_fcm_diag_parse_log "${unknownTarget}")" == "${unknownTarget}" ]]
[[ "$(voice_fcm_diag_parse_log "${unknownTarget/recipient_present=unknown/recipient_present=true}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid% phase_evidence=*}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid/callback_entered_before_expiry=false/callback_entered_before_expiry=unknown}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid/phase_evidence=known/phase_evidence=unknown}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid} callback_entered_at_expiry=true")" == "${unknown}" ]]
for admission in invalid window tuple identity mixed overflow; do
  candidate="${valid/admission=none/admission=${admission}}"
  [[ "$(voice_fcm_diag_parse_log "${candidate}")" == "${unknown}" ]]
done
[[ "$(voice_fcm_diag_parse_log '')" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid}"$'\n'"${valid}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log 'password=private-sentinel')" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "untrusted-prefix ${valid}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid}"$'\n'"untrusted-prefix ${valid}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid/ candidates=1 / candidates=17 }")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid/admission=none/admission=untrusted}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid/admission=none/admission=private}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid/ member_count=2 / member_count=10000 }")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid%route=ack}route=unk")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "$(printf '%65537s' x)")" == "${unknown}" ]]
many_lines=''
for _ in {1..101}; do many_lines+=$'\n'; done
[[ "$(voice_fcm_diag_parse_log "${many_lines}")" == "${unknown}" ]]
! voice_fcm_diag_identity_valid '' 4
! voice_fcm_diag_identity_valid 0 4
! voice_fcm_diag_identity_valid 4 0
! voice_fcm_diag_identity_valid root 4
voice_fcm_diag_identity_valid 1000 1000

tmp="$(mktemp -d)"
file="${tmp}/control"
: >"${file}"
voice_fcm_diag_cleanup "${file}" "${tmp}"
[[ ! -e "${file}" && ! -e "${tmp}" ]]

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
