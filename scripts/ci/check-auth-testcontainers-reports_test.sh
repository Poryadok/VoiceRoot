#!/usr/bin/env bash
# Unit tests for the Auth Testcontainers Surefire report gate. No Docker required.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT="${ROOT}/scripts/ci/check-auth-testcontainers-reports.sh"
FIXTURES="${ROOT}/scripts/ci/testdata/auth-testcontainers-reports/valid"
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

echo "All Auth Testcontainers report gate tests passed."
