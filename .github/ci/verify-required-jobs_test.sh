#!/usr/bin/env bash
# Unit tests for verify-required-jobs.sh (ci-gate path-filter contract).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT="${ROOT}/.github/ci/verify-required-jobs.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

run_gate() {
  local expect_rc="$1"
  shift
  local out
  out="$(mktemp)"
  set +e
  env "$@" bash "${SCRIPT}" pull_request true auto >"${out}" 2>&1
  local rc=$?
  set -e
  if [[ "${rc}" -ne "${expect_rc}" ]]; then
    cat "${out}" >&2
    rm -f "${out}"
    fail "expected exit ${expect_rc}, got ${rc} (env: $*)"
  fi
  rm -f "${out}"
}

all_jobs_success() {
  JOB_PROTOBUF=success
  JOB_COMPOSE_CONFIG=success
  JOB_CI_SCRIPT_TESTS=success
  JOB_FLUTTER=success
  JOB_FLUTTER_DEVICE_DRIVER=success
  JOB_WEB=success
  JOB_GOLANGCI=success
  JOB_BACKEND_GO_PKG=success
  JOB_BACKEND_GO=success
  JOB_BACKEND_GO_INTEGRATION_PR=success
  JOB_BACKEND_AUTH=success
  JOB_DEVELOPER_PORTAL=success
  JOB_ADMIN=success
}

echo "== docs-only PR skips gate =="
run_gate 0 CODE=false

echo "== non-PR event skips gate =="
set +e
bash "${SCRIPT}" push true auto >/dev/null 2>&1
rc=$?
set -e
[[ "${rc}" -eq 0 ]] || fail "expected exit 0 for non-PR"

echo "== global change requires full tier-1 including Go jobs =="
all_jobs_success
run_gate 0 \
  GLOBAL=true RUN_GO=true RUN_PKG=true \
  JOB_PROTOBUF="${JOB_PROTOBUF}" JOB_COMPOSE_CONFIG="${JOB_COMPOSE_CONFIG}" \
  JOB_CI_SCRIPT_TESTS="${JOB_CI_SCRIPT_TESTS}" \
  JOB_FLUTTER="${JOB_FLUTTER}" JOB_FLUTTER_DEVICE_DRIVER="${JOB_FLUTTER_DEVICE_DRIVER}" \
  JOB_WEB="${JOB_WEB}" JOB_GOLANGCI="${JOB_GOLANGCI}" \
  JOB_BACKEND_GO_PKG="${JOB_BACKEND_GO_PKG}" JOB_BACKEND_GO="${JOB_BACKEND_GO}" \
  JOB_BACKEND_GO_INTEGRATION_PR="${JOB_BACKEND_GO_INTEGRATION_PR}" \
  JOB_BACKEND_AUTH="${JOB_BACKEND_AUTH}" JOB_DEVELOPER_PORTAL="${JOB_DEVELOPER_PORTAL}" \
  JOB_ADMIN="${JOB_ADMIN}"

echo "== global with skipped ci-script-tests fails (regression guard) =="
run_gate 1 \
  GLOBAL=true \
  JOB_PROTOBUF=success JOB_COMPOSE_CONFIG=success JOB_CI_SCRIPT_TESTS=skipped

echo "== global with skipped golangci fails (regression guard) =="
all_jobs_success
run_gate 1 \
  GLOBAL=true RUN_GO=true RUN_PKG=true \
  JOB_PROTOBUF="${JOB_PROTOBUF}" JOB_COMPOSE_CONFIG="${JOB_COMPOSE_CONFIG}" \
  JOB_CI_SCRIPT_TESTS="${JOB_CI_SCRIPT_TESTS}" \
  JOB_FLUTTER="${JOB_FLUTTER}" JOB_FLUTTER_DEVICE_DRIVER="${JOB_FLUTTER_DEVICE_DRIVER}" \
  JOB_WEB="${JOB_WEB}" JOB_GOLANGCI=skipped \
  JOB_BACKEND_GO_PKG="${JOB_BACKEND_GO_PKG}" JOB_BACKEND_GO="${JOB_BACKEND_GO}" \
  JOB_BACKEND_GO_INTEGRATION_PR="${JOB_BACKEND_GO_INTEGRATION_PR}" \
  JOB_BACKEND_AUTH="${JOB_BACKEND_AUTH}" JOB_DEVELOPER_PORTAL="${JOB_DEVELOPER_PORTAL}" \
  JOB_ADMIN="${JOB_ADMIN}"

echo "== GLOBAL without RUN_GO still requires Go jobs (defense in depth) =="
all_jobs_success
run_gate 0 \
  GLOBAL=true RUN_GO=false RUN_PKG=false \
  JOB_PROTOBUF="${JOB_PROTOBUF}" JOB_COMPOSE_CONFIG="${JOB_COMPOSE_CONFIG}" \
  JOB_CI_SCRIPT_TESTS="${JOB_CI_SCRIPT_TESTS}" \
  JOB_FLUTTER="${JOB_FLUTTER}" JOB_FLUTTER_DEVICE_DRIVER="${JOB_FLUTTER_DEVICE_DRIVER}" \
  JOB_WEB="${JOB_WEB}" JOB_GOLANGCI="${JOB_GOLANGCI}" \
  JOB_BACKEND_GO_PKG="${JOB_BACKEND_GO_PKG}" JOB_BACKEND_GO="${JOB_BACKEND_GO}" \
  JOB_BACKEND_GO_INTEGRATION_PR="${JOB_BACKEND_GO_INTEGRATION_PR}" \
  JOB_BACKEND_AUTH="${JOB_BACKEND_AUTH}" JOB_DEVELOPER_PORTAL="${JOB_DEVELOPER_PORTAL}" \
  JOB_ADMIN="${JOB_ADMIN}"

echo "== messaging-only requires backend-go, not admin =="
all_jobs_success
run_gate 0 \
  RUN_GO=true RUN_PKG=true \
  JOB_PROTOBUF=skipped JOB_COMPOSE_CONFIG=skipped \
  JOB_FLUTTER=skipped JOB_FLUTTER_DEVICE_DRIVER=skipped \
  JOB_WEB=skipped JOB_GOLANGCI="${JOB_GOLANGCI}" \
  JOB_BACKEND_GO_PKG="${JOB_BACKEND_GO_PKG}" JOB_BACKEND_GO="${JOB_BACKEND_GO}" \
  JOB_BACKEND_GO_INTEGRATION_PR="${JOB_BACKEND_GO_INTEGRATION_PR}" \
  JOB_BACKEND_AUTH=skipped JOB_DEVELOPER_PORTAL=skipped JOB_ADMIN=skipped

echo "== auth filter requires backend-auth =="
all_jobs_success
run_gate 0 \
  FILTER_AUTH=true \
  JOB_PROTOBUF=skipped JOB_COMPOSE_CONFIG=skipped \
  JOB_FLUTTER=skipped JOB_FLUTTER_DEVICE_DRIVER=skipped \
  JOB_WEB=skipped JOB_GOLANGCI=skipped JOB_BACKEND_GO_PKG=skipped \
  JOB_BACKEND_GO=skipped JOB_BACKEND_GO_INTEGRATION_PR=skipped \
  JOB_BACKEND_AUTH="${JOB_BACKEND_AUTH}" JOB_DEVELOPER_PORTAL=skipped JOB_ADMIN=skipped

echo "== compose filter requires compose-config =="
all_jobs_success
run_gate 0 \
  COMPOSE=true \
  JOB_PROTOBUF=skipped JOB_COMPOSE_CONFIG="${JOB_COMPOSE_CONFIG}" \
  JOB_FLUTTER=skipped JOB_FLUTTER_DEVICE_DRIVER=skipped \
  JOB_WEB=skipped JOB_GOLANGCI=skipped JOB_BACKEND_GO_PKG=skipped \
  JOB_BACKEND_GO=skipped JOB_BACKEND_GO_INTEGRATION_PR=skipped \
  JOB_BACKEND_AUTH=skipped JOB_DEVELOPER_PORTAL=skipped JOB_ADMIN=skipped

echo "All verify-required-jobs tests passed."
