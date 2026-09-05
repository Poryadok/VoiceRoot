#!/usr/bin/env bash
# Tests for e2e-manifest.sh section parsing.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MANIFEST="${ROOT}/.github/ci/e2e-features.yml"
SCRIPT="${ROOT}/scripts/ci/e2e-manifest.sh"
GO_DOWNLOAD_HELPER="${ROOT}/src/backend/scripts/docker-go-mod-download.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

count_gateway="$(bash "${SCRIPT}" "${MANIFEST}" smoke_gateway | wc -l | tr -d '[:space:]')"
count_flutter="$(bash "${SCRIPT}" "${MANIFEST}" smoke_flutter | wc -l | tr -d '[:space:]')"
count_full_flutter="$(bash "${SCRIPT}" "${MANIFEST}" full_flutter | wc -l | tr -d '[:space:]')"

[[ "${count_gateway}" -ge 10 ]] || fail "expected >=10 smoke_gateway entries, got ${count_gateway}"
[[ "${count_flutter}" -ge 10 ]] || fail "expected >=10 smoke_flutter entries, got ${count_flutter}"
[[ "${count_full_flutter}" -ge 20 ]] || fail "expected >=20 full_flutter entries, got ${count_full_flutter}"
[[ "${count_full_flutter}" -ge "${count_flutter}" ]] || fail "full_flutter (${count_full_flutter}) should cover smoke_flutter (${count_flutter})"

if bash "${SCRIPT}" "${MANIFEST}" smoke_gateway | grep 'TestComposeAuthLifecycle_live' >/dev/null; then
  :
else
  fail "expected TestComposeAuthLifecycle_live in smoke_gateway"
fi

if bash "${SCRIPT}" "${MANIFEST}" full_flutter | grep 'test/gateway_dm_ws_live_integration_test.dart' >/dev/null; then
  :
else
  fail "expected gateway_dm_ws_live_integration_test.dart in full_flutter"
fi

if bash "${SCRIPT}" "${MANIFEST}" restart_proof_gateway | grep -x 'TestComposeFileAttachmentRestartProof_live' >/dev/null; then
  :
else
  fail "expected TestComposeFileAttachmentRestartProof_live in restart_proof_gateway"
fi

[[ -f "${GO_DOWNLOAD_HELPER}" ]] || fail "Docker Go module download helper is missing: ${GO_DOWNLOAD_HELPER}"

if LC_ALL=C grep -q $'\r' "${GO_DOWNLOAD_HELPER}"; then
  fail "Docker Go module download helper contains CRLF bytes"
fi

helper_eol="$(git -C "${ROOT}" check-attr eol -- "src/backend/scripts/docker-go-mod-download.sh" | awk -F': ' '{print $3}')"
[[ "${helper_eol}" == "lf" ]] || fail "Docker Go module download helper must have eol=lf, got ${helper_eol:-unset}"

echo "== A1 CI reachability contract =="
bash "$ROOT/scripts/ci/compose-a1-ci-reachability_test.sh"

echo "== A1 isolated gateway section is exact and ordered =="
a1_expected=(
  TestComposeA1TwoAccountsFoundation_live
  TestComposeA1DailyMessagingREST_live
  TestComposeA1GroupReadIsolation_live
  TestComposeA1ChannelReadIsolation_live
  TestComposeA1BlockDMDenyBothDirections_live
)
a1_output_file="$(mktemp)"
a1_error_file="$(mktemp)"
set +e
bash "${SCRIPT}" "${MANIFEST}" a1_multi_account_gateway >"${a1_output_file}" 2>"${a1_error_file}"
a1_parser_rc=$?
set -e
if [[ "${a1_parser_rc}" -ne 0 ]]; then
  cat "${a1_error_file}" >&2 || true
  rm -f "${a1_output_file}" "${a1_error_file}"
  fail "e2e manifest parser must support a1_multi_account_gateway before exact-content checks"
fi
mapfile -t a1_actual < "${a1_output_file}"
rm -f "${a1_output_file}" "${a1_error_file}"
[[ "${#a1_actual[@]}" -eq "${#a1_expected[@]}" ]] || \
  fail "a1_multi_account_gateway must contain exactly five tests, got ${#a1_actual[@]}"
for i in "${!a1_expected[@]}"; do
  [[ "${a1_actual[$i]}" == "${a1_expected[$i]}" ]] || \
    fail "a1_multi_account_gateway order drift at index ${i}: expected ${a1_expected[$i]}, got ${a1_actual[$i]}"
done

echo "== T055/T106 Flutter profile-handoff section is exact =="
profile_handoff_expected=(
  'test/t055_profile_switch_reconnect_inbox_e2e_live_test.dart'
  'test/t106_account_soft_delete_e2e_live_test.dart'
)
t055_output_file="$(mktemp)"
t055_error_file="$(mktemp)"
set +e
bash "${SCRIPT}" "${MANIFEST}" a1_flutter_profile_handoff >"${t055_output_file}" 2>"${t055_error_file}"
t055_parser_rc=$?
set -e
if [[ "${t055_parser_rc}" -ne 0 ]]; then
  cat "${t055_error_file}" >&2 || true
  rm -f "${t055_output_file}" "${t055_error_file}"
  fail "e2e manifest parser must support a1_flutter_profile_handoff before exact-content checks"
fi
mapfile -t profile_handoff_actual < "${t055_output_file}"
rm -f "${t055_output_file}" "${t055_error_file}"
[[ "${#profile_handoff_actual[@]}" -eq "${#profile_handoff_expected[@]}" ]] || \
  fail "a1_flutter_profile_handoff must contain exactly two tests, got ${#profile_handoff_actual[@]}"
for i in "${!profile_handoff_expected[@]}"; do
  [[ "${profile_handoff_actual[$i]}" == "${profile_handoff_expected[$i]}" ]] || \
    fail "a1_flutter_profile_handoff order drift at index ${i}: expected ${profile_handoff_expected[$i]}, got ${profile_handoff_actual[$i]:-<empty>}"
done

if bash "${SCRIPT}" "${MANIFEST}" missing_section >/dev/null 2>&1; then
  fail "expected failure for missing section"
fi

echo "All e2e-manifest tests passed."
