#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/compose-fcm-diagnostic-lib.sh"

valid='compose_fcm_diag valid=true reason=matched candidates=1 attempts=2 member_result=ok member_count=2 recipient_present=true inbox=main base_push=true final_push=true presence=offline policy=ok token_rows=1 fcm_tokens=1 dispatcher_returns=1 route=ack'
unknown="${VOICE_FCM_DIAG_UNKNOWN}"
[[ "$(voice_fcm_diag_parse_log "${valid}")" == "${valid}" ]]
[[ "$(voice_fcm_diag_parse_log '')" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid}"$'\n'"${valid}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log 'password=private-sentinel')" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "untrusted-prefix ${valid}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid}"$'\n'"untrusted-prefix ${valid}")" == "${unknown}" ]]
[[ "$(voice_fcm_diag_parse_log "${valid/ candidates=1 / candidates=17 }")" == "${unknown}" ]]
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
