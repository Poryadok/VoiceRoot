#!/usr/bin/env bash
# Tests for resolve-staging-matrix.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT="${ROOT}/scripts/ci/resolve-staging-matrix.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_contains() {
  local json="$1"
  local name="$2"
  echo "${json}" | jq -e --arg n "${name}" 'index($n) != null' >/dev/null || fail "expected ${name} in ${json}"
}

assert_not_contains() {
  local json="$1"
  local name="$2"
  if echo "${json}" | jq -e --arg n "${name}" 'index($n) != null' >/dev/null; then
    fail "did not expect ${name} in ${json}"
  fi
}

run_matrix() {
  local out
  out="$(mktemp)"
  GITHUB_OUTPUT="${out}" FILTER_JSON="${FILTER_JSON:-}" GO_SERVICES_JSON="${GO_SERVICES_JSON:-[]}" \
    FORCE_FULL="${FORCE_FULL:-}" FILTER_CODE="${FILTER_CODE:-true}" \
    BASE_SHA="${BASE_SHA:-deadbeef}" HEAD_SHA="${HEAD_SHA:-cafebabe}" \
    STAGING_FORCE_FULL_ROLLOUT="${STAGING_FORCE_FULL_ROLLOUT:-}" \
    bash "${SCRIPT}" >/dev/null
  build_services="$(grep '^build_services=' "${out}" | head -1 | cut -d= -f2-)"
  promote_services="$(grep '^promote_services=' "${out}" | head -1 | cut -d= -f2-)"
  needs_full_rollout="$(grep '^needs_full_rollout=' "${out}" | head -1 | cut -d= -f2-)"
  needs_user_space_rollout="$(grep '^needs_user_space_rollout=' "${out}" | head -1 | cut -d= -f2-)"
  rm -f "${out}"
}

echo "== docs-only (code=false) =="
FILTER_JSON='{"code":"false"}' GO_SERVICES_JSON='[]' FILTER_CODE=false run_matrix
[[ "${build_services}" == "[]" ]] || fail "build should be empty for docs-only"
[[ "${promote_services}" == "[]" ]] || fail "promote should be empty for docs-only"

echo "== messaging change expands to chat + gateway =="
FILTER_JSON='{"code":"true","svc_messaging":"true"}' GO_SERVICES_JSON='["messaging","chat"]' run_matrix
assert_contains "${build_services}" messaging
assert_contains "${build_services}" chat
assert_contains "${build_services}" gateway
assert_not_contains "${build_services}" auth

echo "== user change sets needs_user_space_rollout =="
FILTER_JSON='{"code":"true","svc_user":"true"}' GO_SERVICES_JSON='["user","social","space"]' run_matrix
assert_contains "${build_services}" user
[[ "${needs_user_space_rollout}" == "true" ]] || fail "expected needs_user_space_rollout"

echo "== auth path only =="
FILTER_JSON='{"code":"true","auth":"true"}' GO_SERVICES_JSON='[]' run_matrix
assert_contains "${build_services}" auth
assert_not_contains "${build_services}" gateway

echo "== FORCE_FULL builds all =="
FORCE_FULL=true GO_SERVICES_JSON='[]' run_matrix
count="$(echo "${build_services}" | jq 'length')"
[[ "${count}" -eq 23 ]] || fail "expected 23 build services, got ${count}"

echo "== staging_infra sets needs_full_rollout =="
FILTER_JSON='{"code":"true","staging_infra":"true"}' GO_SERVICES_JSON='[]' run_matrix
[[ "${needs_full_rollout}" == "true" ]] || fail "expected needs_full_rollout for staging_infra"

echo "All resolve-staging-matrix tests passed."
