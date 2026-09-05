#!/usr/bin/env bash
# Tests for the isolated A1 multi-account compose runner.
# Command shims keep this test offline: no real Docker, Go, curl, or sleeps.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SCRIPT="${ROOT}/scripts/ci/compose-a1-multi-account-proof.sh"
T055_REGEX='^TestComposeA1(TwoAccountsFoundation|DailyMessagingREST|GroupReadIsolation|ChannelReadIsolation|BlockDMDenyBothDirections)_live$'

fail() { echo "FAIL: $*" >&2; exit 1; }
assert_file() { [[ -f "$1" ]] || fail "missing file: $1"; }
assert_contains() {
  local file="$1" pattern="$2"
  grep -Eq -- "$pattern" "$file" || { cat "$file" >&2 || true; fail "expected ${pattern}"; }
}
assert_not_contains() {
  local file="$1" pattern="$2"
  if grep -Eq -- "$pattern" "$file"; then cat "$file" >&2 || true; fail "did not expect ${pattern}"; fi
}
assert_eq() { [[ "$1" == "$2" ]] || fail "expected '$2', got '$1'"; }
assert_abs() {
  [[ "$1" == /* || "$1" =~ ^[A-Za-z]:[/\\] ]] || fail "not absolute: $1"
}
assert_full_port_env() {
  local file="$1" base="${2:-25000}" offset=0 var
  local vars=(
    POSTGRES_PORT REDIS_PORT NATS_PORT NATS_HTTP_PORT CLICKHOUSE_HTTP_PORT
    LIVEKIT_PORT LIVEKIT_RTC_TCP_PORT LIVEKIT_RTC_UDP_PORT CLAMAV_PORT
    MINIO_PORT MINIO_CONSOLE_PORT NOTIFICATION_DEBUG_PORT GATEWAY_PORT WEB_PORT
    DEVELOPER_PORTAL_PORT ADMIN_PORT VERIFICATION_STUB_PORT
  )
  for var in "${vars[@]}"; do
    assert_contains "$file" "^env_${var}=$((base + offset))$"
    offset=$((offset + 1))
  done
}
assert_scrubbed_or_rejected() {
  local case_dir="$1" hostile="$2"
  local rc
  rc="$(cat "${case_dir}/rc")"
  if [[ "$rc" == 2 && ! -s "${case_dir}/commands.log" ]]; then
    return 0
  fi
  assert_eq "$rc" 0
  assert_not_contains "${case_dir}/commands.log" "$hostile"
  assert_contains "${case_dir}/commands.log" 'env_COMPOSE_PROJECT_NAME=voice-a1-multi-'
  assert_contains "${case_dir}/commands.log" '^env_COMPOSE_PROFILES=$'
  assert_contains "${case_dir}/commands.log" '^env_COMPOSE_FILE=$'
  assert_full_port_env "${case_dir}/commands.log"
}
assert_isolated_make_target() {
  local output_file="${TEST_TMP}/make-a1-target.out" rc runner_count
  set +e
  make -n compose-a1-multi-account-proof >"$output_file" 2>&1
  rc=$?
  set -e
  [[ "$rc" == 0 ]] || { cat "$output_file" >&2; fail 'make target compose-a1-multi-account-proof is missing or failed'; }
  runner_count="$(grep -Fc 'scripts/ci/compose-a1-multi-account-proof.sh' "$output_file" || true)"
  assert_eq "$runner_count" 1
  if grep -Eq 'compose-e2e|compose-file-attachment|docker compose' "$output_file"; then
    cat "$output_file" >&2
    fail 'A1 target must not delegate to generic/shared compose targets'
  fi
}

make_fake_tools() {
  local bin="$1"
  mkdir -p "$bin"
  cat >"${bin}/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log="${FAKE_LOG:?FAKE_LOG required}"
printf 'docker' >>"${log}"
for arg in "$@"; do printf ' <%s>' "$arg" >>"${log}"; done
printf '\n' >>"${log}"
for var in \
  POSTGRES_PORT REDIS_PORT NATS_PORT NATS_HTTP_PORT CLICKHOUSE_HTTP_PORT \
  LIVEKIT_PORT LIVEKIT_RTC_TCP_PORT LIVEKIT_RTC_UDP_PORT CLAMAV_PORT \
  MINIO_PORT MINIO_CONSOLE_PORT NOTIFICATION_DEBUG_PORT GATEWAY_PORT WEB_PORT \
  DEVELOPER_PORTAL_PORT ADMIN_PORT VERIFICATION_STUB_PORT; do
  printf 'env_%s=%s\n' "$var" "${!var:-}" >>"${log}"
done
printf 'env_COMPOSE_PROJECT_NAME=%s\n' "${COMPOSE_PROJECT_NAME:-}" >>"${log}"
printf 'env_COMPOSE_PROFILES=%s\n' "${COMPOSE_PROFILES:-}" >>"${log}"
printf 'env_COMPOSE_FILE=%s\n' "${COMPOSE_FILE:-}" >>"${log}"
if [[ "${1:-}" == ps ]]; then
  has_all=0
  has_quiet=0
  for arg in "${@:2}"; do
    case "$arg" in
      -a|--all) has_all=1 ;;
      -q|--quiet) has_quiet=1 ;;
      -aq|-qa) has_all=1; has_quiet=1 ;;
    esac
  done
  if [[ "${FAKE_DOCKER_MODE:-}" == collision && "$has_all" == 1 && "$has_quiet" == 1 ]]; then
    echo fake-existing-container
  fi
  if [[ "${FAKE_DOCKER_MODE:-}" == occupied && "${2:-}" == --format ]]; then echo '0.0.0.0:25003->5432/tcp'; fi
  exit 0
fi
if [[ "${1:-}" == inspect ]]; then echo healthy; exit 0; fi
if [[ "${1:-}" == compose ]]; then
  line="$*"
  previous=''
  for arg in "$@"; do
    if [[ "$previous" == --env-file ]]; then
      printf 'env_size=%s\n' "$(wc -c <"$arg")" >>"${log}"
    fi
    previous="$arg"
  done
  [[ "$line" == *' config '* ]] && exit 0
  [[ "$line" == *' up '* ]] && exit 0
  [[ "$line" == *' ps -q '* ]] && { echo fake-container; exit 0; }
  [[ "$line" == *' logs '* ]] && { echo fake diagnostic log; exit "${FAKE_LOGS_RC:-0}"; }
  [[ "$line" == *' ps '* ]] && { echo fake compose ps; exit 0; }
  [[ "$line" == *' down '* ]] && exit "${FAKE_DOWN_RC:-0}"
  exit 0
fi
exit 0
EOF
  cat >"${bin}/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log="${FAKE_LOG:?FAKE_LOG required}"
printf 'go cwd=%s' "$PWD" >>"${log}"
for arg in "$@"; do printf ' <%s>' "$arg" >>"${log}"; done
printf '\n' >>"${log}"
exit "${FAKE_GO_RC:-0}"
EOF
  cat >"${bin}/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log="${FAKE_LOG:?FAKE_LOG required}"
printf 'curl' >>"${log}"
for arg in "$@"; do printf ' <%s>' "$arg" >>"${log}"; done
printf '\n' >>"${log}"
exit "${FAKE_CURL_RC:-0}"
EOF
  cat >"${bin}/sleep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "${bin}/docker" "${bin}/go" "${bin}/curl" "${bin}/sleep"
}

new_case() {
  local work
  work="$(mktemp -d "${TEST_TMP}/a1-runner-$1.XXXXXXXX")"
  mkdir -p "${work}/tmp with spaces"
  make_fake_tools "${work}/bin"
  printf '%s\n' "$work"
}
run_runner() {
  local work="$1"; shift
  : >"${work}/commands.log"
  set +e
  (
    cd "${work}/tmp with spaces"
    env PATH="${work}/bin:${PATH}" FAKE_LOG="${work}/commands.log" \
      FAKE_DOCKER_MODE="${FAKE_DOCKER_MODE:-}" FAKE_GO_RC="${FAKE_GO_RC:-0}" \
      TMPDIR="${work}/tmp with spaces" VOICE_A1_MULTI_ACCOUNT_PORT_BASE=25000 \
      VOICE_RUN_LIVE_COMPOSE=true "$@" bash "$SCRIPT" >"${work}/stdout" 2>"${work}/stderr"
  )
  local rc=$?
  set -e
  printf '%s\n' "$rc" >"${work}/rc"
  return 0
}

TEST_TMP="$(mktemp -d "${TMPDIR:-/tmp}/a1-runner-tests.XXXXXXXX")"
trap 'rm -rf -- "$TEST_TMP"' EXIT
assert_file "$SCRIPT"

echo '== T055 regex matches exactly the five current tests =='
for test_name in \
  TestComposeA1TwoAccountsFoundation_live \
  TestComposeA1DailyMessagingREST_live \
  TestComposeA1GroupReadIsolation_live \
  TestComposeA1ChannelReadIsolation_live \
  TestComposeA1BlockDMDenyBothDirections_live; do
  printf '%s\n' "$test_name" | grep -Eq -- "$T055_REGEX" || fail "regex did not match ${test_name}"
done
if printf '%s\n' TestComposeAuthLifecycle_live | grep -Eq -- "$T055_REGEX"; then
  fail 'T055 regex matched an unrelated live test'
fi
if printf '%s\n' TestComposeA1GroupReadIsolation_liveExtra | grep -Eq -- "$T055_REGEX"; then
  fail 'T055 regex matched a near-miss test name'
fi
if printf '%s\n' TestComposeA1BlockDMDenyBothDirections_liveExtra | grep -Eq -- "$T055_REGEX"; then
  fail 'T055 regex matched the BlockDM near-miss test name'
fi

echo '== ambient project is rejected before Docker =='
case_dir="$(new_case ambient)"
run_runner "$case_dir" COMPOSE_PROJECT_NAME=ambient-project
assert_scrubbed_or_rejected "$case_dir" 'env_COMPOSE_PROJECT_NAME=ambient-project'

echo '== hostile Compose profile/file and ambient ports are rejected or scrubbed =='
case_dir="$(new_case hostile-compose-env)"
run_runner "$case_dir" \
  COMPOSE_PROFILES=hostile-profile COMPOSE_FILE=hostile-compose.yml \
  POSTGRES_PORT=1 REDIS_PORT=2 NATS_PORT=3 NATS_HTTP_PORT=4 \
  CLICKHOUSE_HTTP_PORT=5 LIVEKIT_PORT=6 LIVEKIT_RTC_TCP_PORT=7 \
  LIVEKIT_RTC_UDP_PORT=8 CLAMAV_PORT=9 MINIO_PORT=10 MINIO_CONSOLE_PORT=11 \
  NOTIFICATION_DEBUG_PORT=12 GATEWAY_PORT=13 WEB_PORT=14 \
  DEVELOPER_PORTAL_PORT=15 ADMIN_PORT=16 VERIFICATION_STUB_PORT=17
assert_scrubbed_or_rejected "$case_dir" 'env_COMPOSE_PROFILES=hostile-profile'
assert_not_contains "${case_dir}/commands.log" 'env_COMPOSE_FILE=hostile-compose.yml'

echo '== generated project collision is rejected =='
case_dir="$(new_case collision)"
FAKE_DOCKER_MODE=collision run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 2
assert_contains "${case_dir}/commands.log" 'label=com.docker.compose.project=voice-a1-multi-'
assert_not_contains "${case_dir}/commands.log" '<config>'
assert_not_contains "${case_dir}/commands.log" '<up>'

echo '== occupied and invalid port ranges are rejected =='
case_dir="$(new_case occupied)"
FAKE_DOCKER_MODE=occupied run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 2
assert_contains "${case_dir}/stderr" 'occupied Docker host port: 25003'
for invalid in 19999 65001 65520 nope; do
  case_dir="$(new_case "invalid-${invalid}")"
  run_runner "$case_dir" VOICE_A1_MULTI_ACCOUNT_PORT_BASE="${invalid}"
  assert_eq "$(cat "${case_dir}/rc")" 2
  assert_not_contains "${case_dir}/commands.log" '<config>'
done

echo '== config/up/health and exact Go regex =='
case_dir="$(new_case happy)"
run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 0
assert_contains "${case_dir}/commands.log" 'compose.*<--profile> <app> <config> <--quiet>'
assert_contains "${case_dir}/commands.log" 'compose.*<--profile> <app> <up> <-d> <--build>'
assert_contains "${case_dir}/commands.log" 'compose.*<ps> <-q> <gateway>'
assert_contains "${case_dir}/commands.log" 'compose.*<ps> <-q> <file>'
assert_contains "${case_dir}/commands.log" 'curl.*<http://127.0.0.1:25012/health>'
assert_full_port_env "${case_dir}/commands.log"
assert_contains "${case_dir}/commands.log" 'go cwd=.*/src/backend/gateway.*<-count=1> <-parallel> <1> <-timeout> <20m> <-tags> <live> <-run>'
go_run_regex="$(sed -n 's/.* <-run> <\([^>]*\)> <.*/\1/p' "${case_dir}/commands.log" | head -1)"
assert_eq "$go_run_regex" "$T055_REGEX"
assert_isolated_make_target

echo '== every compose path is absolute and env file is zero-byte =='
compose_line="$(grep -m1 '^docker <compose> ' "${case_dir}/commands.log")"
env_file="$(sed -n 's/.* <--env-file> <\([^>]*\)> .*/\1/p' <<<"$compose_line")"
compose_dir="$(sed -n 's/.* <--project-directory> <\([^>]*\)> .*/\1/p' <<<"$compose_line")"
compose_file="$(sed -n 's/.* <-f> <\([^>]*\)> .*/\1/p' <<<"$compose_line")"
assert_abs "$env_file"; assert_abs "$compose_dir"; assert_abs "$compose_file"
assert_contains "${case_dir}/commands.log" 'env_size=0'
assert_contains "${case_dir}/commands.log" 'compose.*<--project-name> <voice-a1-multi-[a-z0-9-]+>'
project_count="$(grep '^docker <compose> ' "${case_dir}/commands.log" | sed -n 's/.* <--project-name> <\([^>]*\)>.*/\1/p' | sort -u | wc -l | tr -d '[:space:]')"
assert_eq "$project_count" 1

echo '== failure diagnostics preserve status and cleanup is opt-in =='
case_dir="$(new_case failure)"
FAKE_GO_RC=23 run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 23
assert_contains "${case_dir}/commands.log" 'compose.*<ps>$'
assert_contains "${case_dir}/commands.log" 'compose.*<logs> <--no-color> <--timestamps>'
assert_contains "${case_dir}/stderr" 'diagnostic|failure|failed'
assert_not_contains "${case_dir}/commands.log" 'compose.*<down>'
go_line="$(grep -n '^go ' "${case_dir}/commands.log" | head -1 | cut -d: -f1)"
ps_line="$(grep -n 'compose.*<ps>$' "${case_dir}/commands.log" | head -1 | cut -d: -f1)"
logs_line="$(grep -n 'compose.*<logs>' "${case_dir}/commands.log" | head -1 | cut -d: -f1)"
[[ "$go_line" =~ ^[0-9]+$ && "$ps_line" =~ ^[0-9]+$ && "$logs_line" =~ ^[0-9]+$ ]] || fail 'missing diagnostic order entries'
(( ps_line > go_line && logs_line > ps_line )) || fail 'diagnostics must follow Go failure in ps then logs order'

case_dir="$(new_case cleanup)"
VOICE_A1_MULTI_ACCOUNT_CLEANUP=true run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 0
assert_contains "${case_dir}/commands.log" 'compose.*<down> <--remove-orphans>'
assert_not_contains "${case_dir}/commands.log" '--volumes|<-v>|volume prune|system prune'

echo 'All compose-a1-multi-account-proof tests passed.'
