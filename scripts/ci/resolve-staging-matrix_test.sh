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
  promote_from_sha="$(grep '^promote_from_sha=' "${out}" | head -1 | cut -d= -f2-)"
  rm -f "${out}"
}

echo "== docs-only (code=false) =="
FILTER_JSON='{"code":"false"}' GO_SERVICES_JSON='[]' FILTER_CODE=false run_matrix
[[ "${build_services}" == "[]" ]] || fail "build should be empty for docs-only"
[[ "${promote_services}" == "[]" ]] || fail "promote should be empty for docs-only"
[[ "${needs_user_space_rollout}" == "false" ]] || fail "docs-only should not set needs_user_space_rollout"

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

echo "== messaging-only change also sets needs_user_space_rollout =="
FILTER_JSON='{"code":"true","svc_messaging":"true"}' GO_SERVICES_JSON='["messaging","chat"]' run_matrix
assert_contains "${build_services}" messaging
[[ "${needs_user_space_rollout}" == "true" ]] || fail "expected needs_user_space_rollout for any code deploy"

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

echo "== BASE_SHA zero uses HEAD^ not HEAD_SHA =="
FILTER_JSON='{"code":"true","svc_chat":"true"}' GO_SERVICES_JSON='["chat","messaging"]' \
  BASE_SHA=0000000000000000000000000000000000000000 HEAD_SHA=deadbeefcafebabe run_matrix
parent="$(git -C "${ROOT}" rev-parse HEAD^)"
[[ "${promote_from_sha}" == "${parent}" ]] || fail "expected promote_from_sha=${parent}, got ${promote_from_sha}"

echo "== compose only (no global) sets needs_full_rollout, empty build =="
FILTER_JSON='{"code":"true","compose":"true"}' GO_SERVICES_JSON='[]' run_matrix
[[ "${needs_full_rollout}" == "true" ]] || fail "expected needs_full_rollout for compose-only"
[[ "${build_services}" == "[]" ]] || fail "expected empty build for compose-only without go services"

echo "== manifest check moves missing non-go promote targets to build =="
promote_out="$(mktemp)"
FILTER_JSON='{"code":"true","svc_chat":"true"}' GO_SERVICES_JSON='["chat","messaging"]' \
  MANIFEST_CHECK=true VOICE_IMAGE_REGISTRY=ghcr.io/example/voice BASE_SHA=deadbeef \
  GITHUB_OUTPUT="${promote_out}" bash "${SCRIPT}" >/dev/null
promote_services="$(grep '^promote_services=' "${promote_out}" | head -1 | cut -d= -f2-)"
build_services="$(grep '^build_services=' "${promote_out}" | head -1 | cut -d= -f2-)"
run_admin="$(grep '^run_admin=' "${promote_out}" | head -1 | cut -d= -f2-)"
assert_not_contains "${promote_services}" admin
assert_contains "${build_services}" admin
[[ "${run_admin}" == "true" ]] || fail "expected run_admin after missing manifest"
rm -f "${promote_out}"

echo "== manifest check moves missing go promote targets to build =="
promote_out="$(mktemp)"
FILTER_JSON='{"code":"true","compose":"true"}' GO_SERVICES_JSON='[]' \
  MANIFEST_CHECK=true VOICE_IMAGE_REGISTRY=ghcr.io/example/voice BASE_SHA=deadbeef \
  GITHUB_OUTPUT="${promote_out}" bash "${SCRIPT}" >/dev/null
promote_services="$(grep '^promote_services=' "${promote_out}" | head -1 | cut -d= -f2-)"
build_services="$(grep '^build_services=' "${promote_out}" | head -1 | cut -d= -f2-)"
assert_not_contains "${promote_services}" gateway
assert_contains "${build_services}" gateway
rm -f "${promote_out}"

echo "All resolve-staging-matrix tests passed."
