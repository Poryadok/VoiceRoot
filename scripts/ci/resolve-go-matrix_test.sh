#!/usr/bin/env bash
# Tests for resolve-go-matrix.sh (S2S one-hop expansion).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT="${ROOT}/scripts/ci/resolve-go-matrix.sh"

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

assert_exact_services() {
  local json="$1"
  local expected="$2"
  local count unique
  count="$(echo "${json}" | jq 'length')"
  unique="$(echo "${json}" | jq 'unique | length')"
  [[ "${count}" -eq "${unique}" ]] || fail "expected no duplicate services in ${json}"
  [[ "$(echo "${json}" | jq -cS 'sort')" == "$(echo "${expected}" | jq -cS 'sort')" ]] \
    || fail "expected exactly ${expected}, got ${json}"
}

run_matrix() {
  local out
  out="$(mktemp)"
  GITHUB_OUTPUT="${out}" FILTER_JSON="${FILTER_JSON:-}" \
    FORCE_FULL="${FORCE_FULL:-}" bash "${SCRIPT}" >/dev/null
  go_services="$(grep '^go_services=' "${out}" | head -1 | cut -d= -f2-)"
  integration_go_services="$(grep '^integration_go_services=' "${out}" | head -1 | cut -d= -f2-)"
  run_go="$(grep '^run_go=' "${out}" | head -1 | cut -d= -f2-)"
  rm -f "${out}"
}

echo "== chat change pulls file, messaging, and gateway only (one-hop) =="
FILTER_JSON='{"code":"true","svc_chat":"true"}' run_matrix
assert_contains "${go_services}" chat
assert_contains "${go_services}" file
assert_contains "${go_services}" messaging
assert_contains "${go_services}" gateway
assert_exact_services "${go_services}" '["chat","file","messaging","gateway"]'
[[ "${run_go}" == "true" ]] || fail "expected run_go=true for chat"

echo "== file change pulls story (one-hop) =="
FILTER_JSON='{"code":"true","svc_file":"true"}' run_matrix
assert_contains "${go_services}" file
assert_contains "${go_services}" messaging
assert_contains "${go_services}" story
assert_contains "${go_services}" gateway
[[ "${run_go}" == "true" ]] || fail "expected run_go=true"

echo "== messaging change does not pull story =="
FILTER_JSON='{"code":"true","svc_messaging":"true"}' run_matrix
assert_contains "${go_services}" messaging
assert_not_contains "${go_services}" story

echo "== controlledgame change selects its test-only module =="
FILTER_JSON='{"code":"true","svc_controlledgame":"true"}' run_matrix
assert_exact_services "${go_services}" '["controlledgame"]'
[[ "${run_go}" == "true" ]] || fail "expected run_go=true for controlledgame"

echo "== gameintegration change selects the existing GIS module =="
FILTER_JSON='{"code":"true","svc_gameintegration":"true"}' run_matrix
assert_exact_services "${go_services}" '["gameintegration","gateway"]'
[[ "${run_go}" == "true" ]] || fail "expected run_go=true for gameintegration"

echo "== ci_global keeps broad backend tests and limits PR integration to changed services =="
FILTER_JSON='{"code":"true","ci_global":"true","svc_controlledgame":"true","svc_gameintegration":"true"}' run_matrix
assert_contains "${go_services}" analytics
assert_contains "${go_services}" controlledgame
assert_contains "${go_services}" gameintegration
assert_exact_services "${integration_go_services}" '["controlledgame","gameintegration"]'

echo "== global (scripts/staging|prod) runs full Go matrix =="
FILTER_JSON='{"code":"true","global":"true"}' run_matrix
[[ "${run_go}" == "true" ]] || fail "expected run_go=true for global"
count="$(echo "${go_services}" | jq 'length')"
unique="$(echo "${go_services}" | jq 'unique | length')"
[[ "${count}" -eq "${unique}" ]] || fail "expected no duplicate services in ${go_services}"
assert_contains "${go_services}" controlledgame
assert_contains "${go_services}" gameintegration
assert_exact_services "${integration_go_services}" '[]'

echo "All resolve-go-matrix tests passed."
