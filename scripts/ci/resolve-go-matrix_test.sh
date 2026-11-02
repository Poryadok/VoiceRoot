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
  docker_go_services="$(grep '^docker_go_services=' "${out}" | head -1 | cut -d= -f2-)"
  integration_go_services="$(grep '^integration_go_services=' "${out}" | head -1 | cut -d= -f2-)"
  integration_analytics="$(grep '^integration_analytics=' "${out}" | head -1 | cut -d= -f2-)"
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
assert_exact_services "${docker_go_services}" '["chat","messaging"]'
[[ "${run_go}" == "true" ]] || fail "expected run_go=true for chat"

echo "== file change pulls story (one-hop) =="
FILTER_JSON='{"code":"true","svc_file":"true"}' run_matrix
assert_exact_services "${docker_go_services}" '["file"]'
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

echo "== ci_global retains all backend tests and integrations =="
FILTER_JSON='{"code":"true","ci_global":"true","svc_controlledgame":"true","svc_gameintegration":"true"}' run_matrix
assert_contains "${go_services}" analytics
assert_contains "${go_services}" controlledgame
assert_contains "${go_services}" gameintegration
assert_exact_services "${integration_go_services}" "${go_services}"

echo "== controlledgame callback changes also validate the Game Integration consumer =="
FILTER_JSON='{"code":"true","svc_controlledgame":"true"}' run_matrix
assert_exact_services "${integration_go_services}" '["controlledgame","gameintegration"]'

echo "== global (scripts/staging|prod) runs full Go matrix =="
FILTER_JSON='{"code":"true","global":"true"}' run_matrix
[[ "${run_go}" == "true" ]] || fail "expected run_go=true for global"
count="$(echo "${go_services}" | jq 'length')"
unique="$(echo "${go_services}" | jq 'unique | length')"
[[ "${count}" -eq "${unique}" ]] || fail "expected no duplicate services in ${go_services}"
assert_contains "${go_services}" controlledgame
assert_contains "${go_services}" gameintegration
assert_exact_services "${integration_go_services}" "${go_services}"

echo "== shared build inputs retain all images and service checks =="
FILTER_JSON='{"code":"true","go_build_shared":"true"}' run_matrix
assert_exact_services "${docker_go_services}" "${go_services}"
assert_contains "${docker_go_services}" gateway
assert_contains "${docker_go_services}" voice

echo "== source consumers receive a service job even without an S2S edge =="
FILTER_JSON='{"code":"true","svc_federation":"true"}' run_matrix
assert_contains "${go_services}" voice
assert_exact_services "${docker_go_services}" '["federation","voice"]'

echo "== docs-only selects no service checks or images =="
FILTER_JSON='{"code":"false"}' run_matrix
assert_exact_services "${go_services}" '[]'
assert_exact_services "${docker_go_services}" '[]'
[[ "${integration_analytics}" == "false" ]] || fail "docs-only must not require ClickHouse integration"

echo "== shared inputs must not omit Go integration consumers =="
for shared in pkg protos go_build_shared global ci_global; do
  FILTER_JSON="{\"code\":\"true\",\"${shared}\":\"true\"}" run_matrix
  assert_exact_services "${integration_go_services}" "${go_services}"
  assert_contains "${integration_go_services}" chat
  assert_contains "${integration_go_services}" voice
done

echo "== service consumers and authentication boundaries retain integration checks =="
FILTER_JSON='{"code":"true","svc_chat":"true"}' run_matrix
[[ "${integration_analytics}" == "true" ]] || fail "Chat events affect Analytics integration"
assert_contains "${integration_go_services}" chat
assert_contains "${integration_go_services}" messaging
assert_contains "${integration_go_services}" file
assert_contains "${integration_go_services}" gateway
assert_contains "${integration_go_services}" analytics
assert_contains "${integration_go_services}" notification
assert_contains "${integration_go_services}" realtime
assert_contains "${integration_go_services}" search
assert_not_contains "${integration_go_services}" story
FILTER_JSON='{"code":"true","svc_messaging":"true"}' run_matrix
assert_contains "${integration_go_services}" search
assert_contains "${integration_go_services}" moderation
assert_contains "${integration_go_services}" notification
FILTER_JSON='{"code":"true","svc_role":"true"}' run_matrix
assert_contains "${integration_go_services}" voice
FILTER_JSON='{"code":"true","svc_user":"true"}' run_matrix
assert_contains "${integration_go_services}" realtime
FILTER_JSON='{"code":"true","svc_voice":"true"}' run_matrix
assert_contains "${integration_go_services}" realtime
FILTER_JSON='{"code":"true","svc_search":"true"}' run_matrix
assert_contains "${integration_go_services}" analytics
FILTER_JSON='{"code":"true","auth":"true"}' run_matrix
assert_exact_services "${integration_go_services}" '["analytics","bot","chat","controlledgame","federation","file","gameintegration","gateway","matchmaking","messaging","moderation","notification","realtime","role","search","social","space","story","subscription","user","voice"]'

echo "== documented event and dedicated invalidation consumers are selected =="
while read -r publisher consumers; do
  FILTER_JSON="{\"code\":\"true\",\"svc_${publisher}\":\"true\"}" run_matrix
  IFS=',' read -ra expected_consumers <<< "${consumers}"
  for consumer in "${expected_consumers[@]}"; do
    assert_contains "${integration_go_services}" "${consumer}"
  done
done <<'EVENTS'
user analytics,social,notification,federation,realtime
social analytics,notification,chat,federation
role analytics,notification,federation,realtime,voice
messaging analytics,notification,search,moderation,realtime
chat analytics,notification,realtime,search
space analytics,notification,realtime,search,voice
voice analytics,notification,realtime
moderation analytics,notification,user
subscription analytics,user,space,voice,file,notification
file analytics,messaging
matchmaking analytics,notification,voice,chat
story analytics,notification,matchmaking
federation analytics,role,moderation
bot analytics,messaging
notification analytics
search analytics
gateway analytics
EVENTS

echo "All resolve-go-matrix tests passed."
