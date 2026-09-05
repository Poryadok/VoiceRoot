#!/usr/bin/env bash
# Tests for e2e-manifest.sh section parsing.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MANIFEST="${ROOT}/.github/ci/e2e-features.yml"
SCRIPT="${ROOT}/scripts/ci/e2e-manifest.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

echo "== A1 CI reachability contract =="
bash "$ROOT/scripts/ci/compose-a1-ci-reachability_test.sh"

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

if bash "${SCRIPT}" "${MANIFEST}" missing_section >/dev/null 2>&1; then
  fail "expected failure for missing section"
fi

echo "All e2e-manifest tests passed."
