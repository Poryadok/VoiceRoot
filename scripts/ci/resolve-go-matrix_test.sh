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

run_matrix() {
  local out
  out="$(mktemp)"
  GITHUB_OUTPUT="${out}" FILTER_JSON="${FILTER_JSON:-}" \
    FORCE_FULL="${FORCE_FULL:-}" bash "${SCRIPT}" >/dev/null
  go_services="$(grep '^go_services=' "${out}" | head -1 | cut -d= -f2-)"
  run_go="$(grep '^run_go=' "${out}" | head -1 | cut -d= -f2-)"
  rm -f "${out}"
}

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

echo "All resolve-go-matrix tests passed."
