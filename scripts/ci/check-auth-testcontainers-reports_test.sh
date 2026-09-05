#!/usr/bin/env bash
# Unit tests for the Auth Testcontainers Surefire report gate. No Docker required.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT="${ROOT}/scripts/ci/check-auth-testcontainers-reports.sh"
FIXTURES="${ROOT}/scripts/ci/testdata/auth-testcontainers-reports/valid"
MAKEFILE="${ROOT}/Makefile"
WORKFLOW="${ROOT}/.github/workflows/ci.yml"
PATH_FILTERS="${ROOT}/.github/ci/path-filters.yml"
AUTH_TEST_SOURCES="${ROOT}/src/backend/auth/src/test/java"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

copy_reports() {
  local destination="$1"
  cp -R "${FIXTURES}" "${destination}"
}

expect_pass() {
  if ! bash "${SCRIPT}" "$1"; then
    fail "expected report check to pass for $1"
  fi
}

expect_fail() {
  if bash "${SCRIPT}" "$1" >/dev/null 2>&1; then
    fail "expected report check to fail for $1"
  fi
}

echo "== valid reports pass =="
copy_reports "${TMP_DIR}/valid"
expect_pass "${TMP_DIR}/valid"

echo "== missing required suite fails =="
copy_reports "${TMP_DIR}/missing"
rm -f "${TMP_DIR}/missing/TEST-voice.backend.auth.AuthJdbcRedisIntegrationTest.xml"
expect_fail "${TMP_DIR}/missing"

echo "== skipped Testcontainers suite fails =="
copy_reports "${TMP_DIR}/skipped"
sed -i 's/skipped="0"/skipped="1"/' "${TMP_DIR}/skipped/TEST-voice.backend.auth.AuthJdbcRedisIntegrationTest.xml"
expect_fail "${TMP_DIR}/skipped"

echo "== failing Testcontainers suite fails =="
copy_reports "${TMP_DIR}/failed"
sed -i 's/failures="0"/failures="1"/' "${TMP_DIR}/failed/TEST-voice.backend.auth.AuthJdbcRedisIntegrationTest.xml"
expect_fail "${TMP_DIR}/failed"

echo "== erroring Testcontainers suite fails =="
copy_reports "${TMP_DIR}/errors"
sed -i 's/errors="0"/errors="1"/' "${TMP_DIR}/errors/TEST-voice.backend.auth.AuthJdbcRedisIntegrationTest.xml"
expect_fail "${TMP_DIR}/errors"

echo "== zero-test Testcontainers suite fails =="
copy_reports "${TMP_DIR}/zero-tests"
sed -i 's/tests="1"/tests="0"/' "${TMP_DIR}/zero-tests/TEST-voice.backend.auth.AuthJdbcRedisIntegrationTest.xml"
expect_fail "${TMP_DIR}/zero-tests"

echo "== canonical Auth CI wiring runs the report gate =="
auth_target="$(sed -n '/^auth-test-ci:/,/^auth-image-ci:/p' "${MAKEFILE}")"
[[ "${auth_target}" == *"mvn -B test"* ]] || fail "auth-test-ci must run Maven tests"
[[ "${auth_target}" == *"check-auth-testcontainers-reports.sh"* ]] || fail "auth-test-ci must check Surefire reports"
[[ "${auth_target#*mvn -B test}" == *"check-auth-testcontainers-reports.sh"* ]] || fail "auth-test-ci must check reports after Maven"
grep -A 35 '^  backend-auth:' "${WORKFLOW}" | grep -F 'run: make auth-test-ci' >/dev/null || fail "backend-auth must use canonical auth-test-ci"
grep -A 12 '^auth:' "${PATH_FILTERS}" | grep -Fx '  - src/backend/migrations/auth_db/**' >/dev/null || fail "Auth path filter must include auth_db migrations"
ci_script_target="$(sed -n '/^ci-script-tests:/,/^generate-staging-services:/p' "${MAKEFILE}")"
[[ "${ci_script_target}" == *"check-auth-testcontainers-reports_test.sh"* ]] || fail "ci-script-tests must run the Auth report gate fixture test"

echo "== every disabledWithoutDocker suite has a required report and fixture =="
source_suites="$(while IFS= read -r -d '' source; do
  if tr '\n' ' ' <"${source}" | grep -Eq '@Testcontainers[[:space:]]*\([[:space:]]*disabledWithoutDocker[[:space:]]*=[[:space:]]*true'; then
    relative="${source#"${AUTH_TEST_SOURCES}"/}"
    printf '%s\n' "${relative%.java}" | tr '/' '.'
  fi
done < <(find "${AUTH_TEST_SOURCES}" -name '*.java' -print0) | LC_ALL=C sort)"
required_suites="$(sed -n '/^required_suites=(/,/^)/p' "${SCRIPT}" \
  | sed -nE 's/^[[:space:]]*"([^"]+)"$/\1/p' \
  | LC_ALL=C sort)"
[[ "${source_suites}" == "${required_suites}" ]] || fail "required Testcontainers suites drift from disabledWithoutDocker inventory"
while IFS= read -r suite; do
  [[ -f "${FIXTURES}/TEST-${suite}.xml" ]] || fail "missing fixture report for ${suite}"
done <<<"${source_suites}"

echo "All Auth Testcontainers report gate tests passed."
