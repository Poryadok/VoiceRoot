#!/usr/bin/env bash
# Regression tests for the canonical Auth proto copy CI guard. No Docker required.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SOURCE_SCRIPT="${ROOT}/scripts/ci/check-auth-proto-sync.sh"
MAKEFILE="${ROOT}/Makefile"
WORKFLOW="${ROOT}/.github/workflows/ci.yml"
PATH_FILTERS="${ROOT}/.github/ci/path-filters.yml"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

fixture_root="${TMP_DIR}/fixture"
fixture_script="${fixture_root}/scripts/ci/check-auth-proto-sync.sh"
canon="${fixture_root}/protos/voice/auth/v1/auth.proto"
copy="${fixture_root}/src/backend/auth/src/main/proto/voice/auth/v1/auth.proto"
mkdir -p "$(dirname "${fixture_script}")" "$(dirname "${canon}")" "$(dirname "${copy}")"
cp "${SOURCE_SCRIPT}" "${fixture_script}"

write_matching_proto() {
  cat >"${canon}" <<'PROTO'
syntax = "proto3";
package voice.auth.v1;
message Session { string account_id = 1; }
PROTO
  cp "${canon}" "${copy}"
}

expect_pass() {
  if ! bash "${fixture_script}"; then
    fail "expected auth proto sync guard to pass: $1"
  fi
}

write_matching_proto
echo "== matching proto copies pass =="
expect_pass "matching copies"

echo "== comments and whitespace are ignored =="
printf '%s\n' '// Java source comment' 'syntax = "proto3";' 'package voice.auth.v1;' 'message Session { string account_id = 1; }' >"${copy}"
expect_pass "comment-only difference"

echo "== wire mismatch fails with canonical and copy labels =="
sed -i 's/account_id = 1/account_id = 2/' "${copy}"
output="${TMP_DIR}/mismatch.out"
if bash "${fixture_script}" >"${output}" 2>&1; then
  fail "expected wire mismatch to fail"
fi
grep -Fq 'Auth proto copy out of sync with protos/voice/auth/v1/auth.proto' "${output}" \
  || fail "mismatch diagnostic must name the canonical proto"
grep -Fxq -- '--- protos/voice/auth/v1/auth.proto' "${output}" \
  || fail "mismatch diagnostic must label canonical normalized diff"
grep -Fxq -- '+++ src/backend/auth/src/main/proto/voice/auth/v1/auth.proto' "${output}" \
  || fail "mismatch diagnostic must label Auth copy normalized diff"

echo "== missing Auth copy fails clearly =="
rm "${copy}"
if bash "${fixture_script}" >"${output}" 2>&1; then
  fail "expected missing Auth copy to fail"
fi
grep -Fq 'missing Auth copy proto:' "${output}" \
  || fail "missing-copy diagnostic must identify the missing file"

echo "== canonical CI wiring covers either auth proto path =="
make_target="$(sed -n '/^check-auth-proto-sync:/,/^[[:alnum:]_-]*:/p' "${MAKEFILE}")"
printf '%s\n' "${make_target}" | grep -Fq 'scripts/ci/check-auth-proto-sync.sh' \
  || fail "check-auth-proto-sync Make target must invoke the guard"
ci_script_target="$(sed -n '/^ci-script-tests:/,/^[[:alnum:]_-]*:/p' "${MAKEFILE}")"
printf '%s\n' "${ci_script_target}" | grep -Fq 'check-auth-proto-sync_test.sh' \
  || fail "ci-script-tests must run this regression test"
protobuf_job="$(sed -n '/^  protobuf:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
printf '%s\n' "${protobuf_job}" | grep -Fq 'name: Check Auth proto copy sync' \
  || fail "protobuf CI job must name the Auth proto sync check"
printf '%s\n' "${protobuf_job}" | grep -Fq 'run: bash scripts/ci/check-auth-proto-sync.sh' \
  || fail "protobuf CI job must execute the Auth proto sync check"
protos_filter="$(sed -n '/^protos:$/,/^[[:alnum:]_ -]*:$/p' "${PATH_FILTERS}")"
printf '%s\n' "${protos_filter}" | grep -Fxq '  - protos/**' \
  || fail "protobuf CI path filter must include canonical protos"
printf '%s\n' "${protos_filter}" | grep -Fxq '  - src/backend/auth/src/main/proto/voice/auth/v1/auth.proto' \
  || fail "protobuf CI path filter must include Auth proto copy"

echo "Auth proto sync guard regression tests passed."
