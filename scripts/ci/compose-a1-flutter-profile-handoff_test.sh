#!/usr/bin/env bash
# Offline shim tests for the isolated Flutter A1 runner.
# No command here reaches Docker, Flutter, curl, or the network.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SCRIPT="$ROOT/scripts/ci/compose-a1-flutter-profile-handoff.sh"
T055_TEST='test/t055_profile_switch_reconnect_inbox_e2e_live_test.dart'
T106_TEST='test/t106_account_soft_delete_e2e_live_test.dart'
T107_TEST='test/t107_folders_quick_access_e2e_live_test.dart'
PENDING_EMAIL_TEST='test/a1_pending_email_reentry_e2e_live_test.dart'
DEFAULT_MANIFEST_RESULT="$(printf '%s\n%s\n%s\n%s' "$T055_TEST" "$T106_TEST" "$T107_TEST" "$PENDING_EMAIL_TEST")"
REAL_BASH="${BASH:-$(command -v bash)}"

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
assert_abs() { [[ "$1" == /* || "$1" =~ ^[A-Za-z]:[/\\] ]] || fail "not absolute: $1"; }

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

assert_overlay_order() {
  local line="$1" file
  local -a files
  mapfile -t files < <(grep -oE '<-f> <[^>]+>' <<<"$line" | sed 's/^<-f> <//;s/>$//')
  assert_eq "${#files[@]}" 3
  assert_eq "${files[0]}" "${ROOT}/docker-compose.yml"
  assert_eq "${files[1]}" "${ROOT}/docker-compose.voice-space-media.yml"
  assert_eq "${files[2]}" "${ROOT}/docker-compose.phase0.yml"
  for file in "${files[@]}"; do assert_abs "$file"; done
}

assert_isolated_make_target() {
  local output_file="${TEST_TMP}/make-profile-handoff.out" rc count
  set +e
  make -n compose-a1-flutter-profile-handoff >"$output_file" 2>&1
  rc=$?
  set -e
  [[ "$rc" == 0 ]] || { cat "$output_file" >&2; fail 'missing compose-a1-flutter-profile-handoff target'; }
  count="$(grep -Fc 'scripts/ci/compose-a1-flutter-profile-handoff.sh' "$output_file" || true)"
  assert_eq "$count" 1
  if grep -Eq 'compose-e2e|compose-file-attachment|compose-a1-multi-account|docker compose' "$output_file"; then
    cat "$output_file" >&2
    fail 'profile-handoff target must not delegate to a shared compose target'
  fi
}

make_fake_tools() {
  local bin="$1"
  mkdir -p "$bin"
  cat >"${bin}/bash" <<'EOF'
#!/bin/sh
set -eu
if [ "${1:-}" = "${FAKE_MANIFEST_SCRIPT:-}" ]; then
  if [ "${FAKE_MANIFEST_RESULT+x}" != x ]; then
    echo 'FAKE_MANIFEST_RESULT required' >&2
    exit 2
  fi
  printf '%s' "$FAKE_MANIFEST_RESULT"
  exit 0
fi
exec "${REAL_BASH:?REAL_BASH required}" "$@"
EOF
  cat >"${bin}/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log="${FAKE_LOG:?FAKE_LOG required}"
if [[ "$*" == *'ratelimit:OTP:'* ]]; then exit 0; fi
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
if [[ "${1:-}" == inspect ]]; then
  if [[ "${FAKE_HEALTH_MODE:-}" == delayed ]]; then
    count_file="${FAKE_HEALTH_COUNT_FILE:?FAKE_HEALTH_COUNT_FILE required}"
    count=0
    [[ -f "$count_file" ]] && count="$(cat "$count_file")"
    count=$((count + 1))
    printf '%s' "$count" >"$count_file"
    if (( count % 2 == 1 )); then echo starting; else echo healthy; fi
  else
    echo healthy
  fi
  exit 0
fi
if [[ "${1:-}" == compose ]]; then
  printf 'env_PHASE0_FIXTURE_DIR=%s\n' "${PHASE0_FIXTURE_DIR:-}" >>"${log}"
  line="$*"
  previous=''
  for arg in "$@"; do
    if [[ "$previous" == --env-file ]]; then
      printf 'env_size=%s\n' "$(wc -c <"$arg")" >>"${log}"
    fi
    previous="$arg"
  done
  [[ "$line" == *' config '* ]] && exit 0
  [[ "$line" == *' up '* ]] && exit "${FAKE_UP_RC:-0}"
  [[ "$line" == *' ps -q '* ]] && { echo fake-container; exit 0; }
  [[ "$line" == *' logs '* ]] && {
    if [[ "$line" == *'compose-db-init'* ]]; then
      if [[ "${FAKE_STARTUP_LOG_MODE:-}" == hang ]]; then
        trap '' TERM
        command -p sleep 30
        printf 'startup_collector_survived_deadline\n' >>"$log"
      elif [[ "${FAKE_STARTUP_LOG_MODE:-}" == cap ]]; then
        head -c 70000 /dev/zero | tr '\0' x
      fi
      printf '%s\n' "${FAKE_STARTUP_LOG:-unrecognized startup detail}"
      printf 'startup_collector_finished\n' >>"$log"
    else
      echo fake diagnostics
    fi
    exit "${FAKE_LOGS_RC:-0}"
  }
  [[ "$line" == *' ps '* ]] && { echo fake compose ps; exit 0; }
  [[ "$line" == *' down '* ]] && exit "${FAKE_DOWN_RC:-0}"
  exit 0
fi
exit 0
EOF
cat >"${bin}/python3" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log="${FAKE_LOG:?FAKE_LOG required}"
printf 'python3' >>"${log}"
for arg in "$@"; do printf ' <%s>' "$arg" >>"${log}"; done
printf '\n' >>"${log}"
printf 'fixture_uid=%s\n' "${PHASE0_MATCHSQUAD_FIXTURE_UID:-}" >>"${log}"
if [[ "${1:-}" == */generate-phase0-fixture.py ]]; then
  if [[ "${FAKE_FIXTURE_RC:-0}" != 0 ]]; then exit "$FAKE_FIXTURE_RC"; fi
  mkdir -p "${2:?fixture destination required}"
elif [[ "${1:-}" == */restore-phase0-fixture-owner.py ]]; then
  :
fi
EOF
cat >"${bin}/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log="${FAKE_LOG:?FAKE_LOG required}"
printf 'curl' >>"${log}"
for arg in "$@"; do printf ' <%s>' "$arg" >>"${log}"; done
printf '\n' >>"${log}"
output=''
for ((i = 1; i <= $#; i += 1)); do
  if [[ "${!i}" == --output || "${!i}" == -o ]]; then
    next=$((i + 1))
    output="${!next}"
  fi
done
url=''
for arg in "$@"; do
  [[ "$arg" == http://* || "$arg" == https://* ]] && url="$arg"
done
status=200
body='{}'
if [[ "$url" == */api/v1/auth/register ]]; then
  body='{"session":{"access_token":"fake-a1-readiness-token"}}'
elif [[ "$url" == */api/v1/users/me ]]; then
  count_file="${FAKE_VISIBILITY_COUNT_FILE:?FAKE_VISIBILITY_COUNT_FILE required}"
  count=0
  [[ -f "$count_file" ]] && count="$(cat "$count_file")"
  count=$((count + 1))
  printf '%s' "$count" >"$count_file"
  if [[ "${FAKE_VISIBILITY_MODE:-}" == delayed && "$count" -eq 1 ]]; then
    status=503
    body='{"error":"unavailable","message":"account visibility unavailable"}'
  elif [[ "${FAKE_VISIBILITY_MODE:-}" == invalid ]]; then
    status=503
    body='{"error":"other_unavailable"}'
  fi
fi
body="$(printf '%s' "$body" | tr -d '\\')"
[[ -z "$output" ]] || printf '%s' "$body" >"$output"
printf '%s' "$status"
exit "${FAKE_CURL_RC:-0}"
EOF
  cat >"${bin}/flutter" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log="${FAKE_LOG:?FAKE_LOG required}"
printf 'flutter cwd=%s' "$PWD" >>"${log}"
for arg in "$@"; do printf ' <%s>' "$arg" >>"${log}"; done
printf '\n' >>"${log}"
printf 'env_VOICE_RUN_LIVE_INTEGRATION=%s\n' "${VOICE_RUN_LIVE_INTEGRATION:-}" >>"${log}"
printf 'env_VOICE_API_BASE_URL=%s\n' "${VOICE_API_BASE_URL:-}" >>"${log}"
exit "${FAKE_FLUTTER_RC:-0}"
EOF
  cat >"${bin}/make" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log="${FAKE_LOG:?FAKE_LOG required}"
printf 'make' >>"${log}"
for arg in "$@"; do printf ' <%s>' "$arg" >>"${log}"; done
printf '\n' >>"${log}"
EOF
cat >"${bin}/sleep" <<'EOF'
#!/usr/bin/env bash
if [[ "${FAKE_REAL_SLEEP:-}" == true ]]; then exec /bin/sleep "$@"; fi
exit 0
EOF
  chmod +x "${bin}/bash" "${bin}/docker" "${bin}/python3" "${bin}/curl" "${bin}/flutter" "${bin}/make" "${bin}/sleep"
}

new_case() {
  local work
  work="$(mktemp -d "${TEST_TMP}/profile-handoff-$1.XXXXXXXX")"
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
      FAKE_DOCKER_MODE="${FAKE_DOCKER_MODE:-}" FAKE_FLUTTER_RC="${FAKE_FLUTTER_RC:-0}" \
      FAKE_UP_RC="${FAKE_UP_RC:-0}" FAKE_LOGS_RC="${FAKE_LOGS_RC:-0}" \
      FAKE_STARTUP_LOG="${FAKE_STARTUP_LOG:-}" FAKE_STARTUP_LOG_MODE="${FAKE_STARTUP_LOG_MODE:-}" \
      FAKE_REAL_SLEEP="${FAKE_REAL_SLEEP:-}" \
      FAKE_HEALTH_MODE="${FAKE_HEALTH_MODE:-}" FAKE_HEALTH_COUNT_FILE="${work}/health-count" \
      FAKE_VISIBILITY_MODE="${FAKE_VISIBILITY_MODE:-}" FAKE_VISIBILITY_COUNT_FILE="${work}/visibility-count" \
      FAKE_MANIFEST_SCRIPT="${ROOT}/scripts/ci/e2e-manifest.sh" \
      FAKE_MANIFEST_RESULT="${FAKE_MANIFEST_RESULT-$DEFAULT_MANIFEST_RESULT}" REAL_BASH="$REAL_BASH" \
      TMPDIR="${work}/tmp with spaces" VOICE_A1_FLUTTER_PROFILE_HANDOFF_PORT_BASE=25000 \
      "$@" "$REAL_BASH" "$SCRIPT" >"${work}/stdout" 2>"${work}/stderr"
  )
  local rc=$?
  set -e
  printf '%s\n' "$rc" >"${work}/rc"
  return 0
}

TEST_TMP="$(mktemp -d "${TMPDIR:-/tmp}/profile-handoff-runner-tests.XXXXXXXX")"
trap 'rm -rf -- "$TEST_TMP"' EXIT
assert_file "$SCRIPT"
assert_contains "$SCRIPT" 'e2e-manifest\.sh.*a1_flutter_profile_handoff'
assert_contains "$SCRIPT" 'GATEWAY_RATE_LIMIT_RULES_JSON='
assert_contains "$SCRIPT" '"OTP":\{"limit":0'

echo '== Compose startup failure emits only safe bounded categories before cleanup =='
case_dir="$(new_case startup-failure)"
startup_secret='FLUTTER_STARTUP_SECRET_SENTINEL'
FAKE_UP_RC=23 FAKE_LOGS_RC=47 \
  FAKE_STARTUP_LOG="password authentication failed SQLSTATE 55000; DSN=postgresql://sensitive:${startup_secret}@private-host/voice" \
  VOICE_A1_FLUTTER_PROFILE_HANDOFF_CLEANUP=true run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 23
assert_contains "${case_dir}/stderr" '^A1 Flutter startup diagnostic category=authentication_failed$'
assert_not_contains "${case_dir}/stdout" "$startup_secret|postgresql://|SQLSTATE|password authentication failed"
assert_not_contains "${case_dir}/stderr" "$startup_secret|postgresql://|SQLSTATE|password authentication failed"
assert_not_contains "${case_dir}/commands.log" '^flutter |^curl '
assert_contains "${case_dir}/commands.log" 'compose.*<logs> <--tail> <80> <--no-color> <--timestamps> <compose-db-init>'
assert_contains "${case_dir}/commands.log" 'compose.*<down> <--remove-orphans>'
startup_up_line="$(grep -n 'compose.*<up> <-d> <--build>' "${case_dir}/commands.log" | head -1 | cut -d: -f1)"
startup_logs_line="$(grep -n 'compose.*<logs> <--tail> <80>' "${case_dir}/commands.log" | head -1 | cut -d: -f1)"
startup_down_line="$(grep -n 'compose.*<down> <--remove-orphans>' "${case_dir}/commands.log" | head -1 | cut -d: -f1)"
[[ "$startup_up_line" =~ ^[0-9]+$ && "$startup_logs_line" =~ ^[0-9]+$ && "$startup_down_line" =~ ^[0-9]+$ ]] || fail 'startup order entries missing'
(( startup_up_line < startup_logs_line && startup_logs_line < startup_down_line )) || fail 'startup diagnostics must precede cleanup'
[[ -z "$(find "${case_dir}/tmp with spaces" -type d -name phase0-fixture -print -quit)" ]] || fail 'opted-in startup cleanup must remove fixture'

echo '== startup diagnostics retain default cleanup opt-in and unknown classification =='
case_dir="$(new_case startup-retained)"
FAKE_UP_RC=29 FAKE_STARTUP_LOG="$startup_secret" run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 29
assert_contains "${case_dir}/stderr" '^A1 Flutter startup diagnostic category=unclassified$'
assert_not_contains "${case_dir}/stderr" "$startup_secret"
assert_not_contains "${case_dir}/commands.log" 'compose.*<down>'
[[ -d "$(find "${case_dir}/tmp with spaces" -type d -name phase0-fixture -print -quit)" ]] || fail 'default cleanup must retain fixture'

echo '== startup classifier never reads beyond its output cap =='
case_dir="$(new_case startup-output-cap)"
FAKE_UP_RC=31 FAKE_STARTUP_LOG_MODE=cap FAKE_STARTUP_LOG="password authentication failed $startup_secret" \
  VOICE_A1_FLUTTER_PROFILE_HANDOFF_CLEANUP=true run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 31
assert_contains "${case_dir}/stderr" '^A1 Flutter startup diagnostic category=unclassified$'
assert_not_contains "${case_dir}/stderr" "$startup_secret|password authentication failed|xxxx"
assert_contains "${case_dir}/commands.log" '^startup_collector_finished$'

echo '== TERM-ignoring startup collector is killed without replacing startup status =='
case_dir="$(new_case startup-hard-timeout)"
FAKE_UP_RC=37 FAKE_STARTUP_LOG_MODE=hang VOICE_A1_FLUTTER_PROFILE_HANDOFF_CLEANUP=true run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 37
assert_contains "${case_dir}/stderr" '^A1 Flutter startup diagnostic category=unclassified$'
assert_not_contains "${case_dir}/commands.log" 'startup_collector_survived_deadline|startup_collector_finished'
assert_contains "${case_dir}/commands.log" 'compose.*<down> <--remove-orphans>'
if [[ "${1:-}" == --startup-diagnostics-only ]]; then
  echo 'Focused Flutter startup diagnostics regressions passed.'
  exit 0
fi

echo '== caller Compose selectors are rejected before Docker =='
case_dir="$(new_case ambient-project)"
run_runner "$case_dir" COMPOSE_PROJECT_NAME=ambient-project
assert_eq "$(cat "${case_dir}/rc")" 2
[[ ! -s "${case_dir}/commands.log" ]] || fail 'ambient project must be rejected before Docker'

echo '== other ambient Compose selectors are rejected before Docker =='
for selector in COMPOSE_PROFILES COMPOSE_FILE COMPOSE_ENV_FILES COMPOSE_PATH_SEPARATOR COMPOSE_CONVERT_WINDOWS_PATHS COMPOSE_DISABLE_ENV_FILE; do
  case_dir="$(new_case "ambient-${selector}")"
  run_runner "$case_dir" "${selector}=hostile"
  assert_eq "$(cat "${case_dir}/rc")" 2
  [[ ! -s "${case_dir}/commands.log" ]] || fail "${selector} must be rejected before Docker"
done

echo '== generated project collision and occupied range are rejected =='
case_dir="$(new_case collision)"
FAKE_DOCKER_MODE=collision run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 2
assert_contains "${case_dir}/commands.log" 'label=com.docker.compose.project=voice-a1-flutter-handoff-'
assert_not_contains "${case_dir}/commands.log" '<config>'
assert_not_contains "${case_dir}/commands.log" '<up>'
case_dir="$(new_case occupied)"
FAKE_DOCKER_MODE=occupied run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 2
assert_contains "${case_dir}/stderr" 'occupied Docker host port: 25003'

echo '== manifest allowlist rejects invalid entry sets before Docker =='
declare -a invalid_manifest_cases=(empty one two four blank absolute non-dart duplicate reversed wrong-paths)
for manifest_case in "${invalid_manifest_cases[@]}"; do
  case_dir="$(new_case "invalid-${manifest_case}")"
  case "$manifest_case" in
    empty) FAKE_MANIFEST_RESULT='' ;;
    one) FAKE_MANIFEST_RESULT="$T055_TEST" ;;
    two) FAKE_MANIFEST_RESULT="$T055_TEST
$T106_TEST" ;;
    four) FAKE_MANIFEST_RESULT="$T055_TEST
$T106_TEST
    $T107_TEST
test/other_profile_test.dart" ;;
    blank) FAKE_MANIFEST_RESULT=$'\n' ;;
    absolute) FAKE_MANIFEST_RESULT="/tmp/t055_profile_switch_reconnect_inbox_e2e_live_test.dart
$T106_TEST" ;;
    non-dart) FAKE_MANIFEST_RESULT="test/t055_profile_switch_reconnect_inbox_e2e_live_test.txt
$T106_TEST" ;;
    duplicate) FAKE_MANIFEST_RESULT="$T055_TEST
$T055_TEST" ;;
    reversed) FAKE_MANIFEST_RESULT="$T106_TEST
$T055_TEST" ;;
    wrong-paths) FAKE_MANIFEST_RESULT='test/other_profile_test.dart
test/another_profile_test.dart' ;;
  esac
  FAKE_MANIFEST_RESULT="$FAKE_MANIFEST_RESULT" run_runner "$case_dir"
  assert_eq "$(cat "${case_dir}/rc")" 2
  [[ ! -s "${case_dir}/commands.log" ]] || fail "${manifest_case} manifest must be rejected before Docker"
done
unset FAKE_MANIFEST_RESULT

echo '== config/up/health and exactly four constrained Flutter tests =='
case_dir="$(new_case happy)"
FAKE_REAL_SLEEP=true FAKE_HEALTH_MODE=delayed PHASE0_FIXTURE_DIR=hostile run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 0
assert_contains "${case_dir}/commands.log" '^env_PHASE0_FIXTURE_DIR=.*/phase0-fixture$'
assert_not_contains "${case_dir}/commands.log" '^env_PHASE0_FIXTURE_DIR=hostile$'
[[ -d "$(find "${case_dir}/tmp with spaces" -type d -name phase0-fixture -print -quit)" ]] || fail 'fixture must remain while Compose project is retained'
assert_contains "${case_dir}/commands.log" 'compose.*<--profile> <app> <config> <--quiet>'
assert_contains "${case_dir}/commands.log" "^make <-C> <${ROOT}> <flutter-windows-prefetch-sqlite3> <flutter-linux-prefetch-sqlite3>$"
first_command="$(head -n 1 "${case_dir}/commands.log")"
assert_eq "$first_command" "make <-C> <${ROOT}> <flutter-windows-prefetch-sqlite3> <flutter-linux-prefetch-sqlite3>"
assert_contains "${case_dir}/commands.log" 'compose.*<--profile> <app> <up> <-d> <--build>'
assert_contains "${case_dir}/commands.log" 'compose.*<ps> <-q> <realtime>'
assert_contains "${case_dir}/commands.log" 'compose.*<ps> <-q> <gateway>'
inspect_count="$(grep -c '^docker <inspect>' "${case_dir}/commands.log" || true)"
[[ "$inspect_count" -ge 4 ]] || fail 'runner must retry non-healthy realtime and gateway containers'
assert_contains "${case_dir}/commands.log" 'curl.*<http://127.0.0.1:25012/health>'
assert_contains "${case_dir}/commands.log" 'curl.*api/v1/auth/register'
assert_contains "${case_dir}/commands.log" 'curl.*api/v1/users/me'
assert_full_port_env "${case_dir}/commands.log"
flutter_count="$(grep -c '^flutter ' "${case_dir}/commands.log" || true)"
assert_eq "$flutter_count" 1
flutter_line="$(grep '^flutter ' "${case_dir}/commands.log")"
mapfile -t flutter_dart_args < <(grep -oE '<test/[^>]+\.dart>' <<<"$flutter_line")
assert_eq "${#flutter_dart_args[@]}" 4
assert_eq "${flutter_dart_args[0]}" "<${T055_TEST}>"
assert_eq "${flutter_dart_args[1]}" "<${T106_TEST}>"
assert_eq "${flutter_dart_args[2]}" "<${T107_TEST}>"
assert_eq "${flutter_dart_args[3]}" "<${PENDING_EMAIL_TEST}>"
assert_contains "${case_dir}/commands.log" "flutter cwd=.*/src/frontend.*<test>.*<${T055_TEST}>.*<${T106_TEST}>.*<${T107_TEST}>.*<${PENDING_EMAIL_TEST}>"
assert_contains "${case_dir}/commands.log" 'flutter.*<--concurrency=1>'
assert_contains "${case_dir}/commands.log" 'flutter.*<--dart-define=VOICE_RUN_LIVE_INTEGRATION=true>'
assert_contains "${case_dir}/commands.log" 'flutter.*<--dart-define=VOICE_API_BASE_URL=http://127.0.0.1:25012>'
assert_contains "${case_dir}/commands.log" '^env_VOICE_RUN_LIVE_INTEGRATION=true$'
assert_contains "${case_dir}/commands.log" '^env_VOICE_API_BASE_URL=http://127.0.0.1:25012$'
assert_isolated_make_target

echo '== account visibility waits only for its documented transient =='
case_dir="$(new_case visibility-delayed)"
FAKE_VISIBILITY_MODE=delayed run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 0
assert_eq "$(cat "${case_dir}/visibility-count")" 2
case_dir="$(new_case visibility-invalid)"
FAKE_VISIBILITY_MODE=invalid run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 1
assert_eq "$(cat "${case_dir}/visibility-count")" 1
flutter_count="$(grep -c '^flutter ' "${case_dir}/commands.log" || true)"
assert_eq "$flutter_count" 0

echo '== every Compose path is absolute and disposable media fixture is wired =='
compose_line="$(grep -m1 '^docker <compose> ' "${case_dir}/commands.log")"
assert_overlay_order "$compose_line"
env_file="$(sed -n 's/.* <--env-file> <\([^>]*\)> .*/\1/p' <<<"$compose_line")"
compose_dir="$(sed -n 's/.* <--project-directory> <\([^>]*\)> .*/\1/p' <<<"$compose_line")"
compose_file="$(sed -n 's/.* <-f> <\([^>]*\)> .*/\1/p' <<<"$compose_line")"
assert_abs "$env_file"; assert_abs "$compose_dir"; assert_abs "$compose_file"
assert_contains "${case_dir}/commands.log" 'env_size=[1-9][0-9]*'
assert_contains "${case_dir}/commands.log" 'python3 .*<.*scripts/dev/generate-phase0-fixture.py>'
assert_contains "${case_dir}/commands.log" '^fixture_uid=65532$'
assert_contains "${case_dir}/commands.log" 'docker <compose> .*<-f> <.*docker-compose.voice-space-media.yml>'
assert_contains "${case_dir}/commands.log" '^env_PHASE0_FIXTURE_DIR=.*/phase0-fixture$'
assert_contains "${case_dir}/commands.log" 'compose.*<--project-name> <voice-a1-flutter-handoff-[a-z0-9-]+>'
project_count="$(grep '^docker <compose> ' "${case_dir}/commands.log" | sed -n 's/.* <--project-name> <\([^>]*\)>.*/\1/p' | sort -u | wc -l | tr -d '[:space:]')"
assert_eq "$project_count" 1

echo '== Flutter failure emits diagnostics and cleanup is exact/opt-in =='
case_dir="$(new_case failure)"
FAKE_FLUTTER_RC=23 run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 23
assert_contains "${case_dir}/commands.log" 'compose.*<ps>$'
assert_contains "${case_dir}/commands.log" 'compose.*<logs> <--no-color> <--timestamps>'
assert_contains "${case_dir}/stderr" 'diagnostic|failure|failed'
assert_not_contains "${case_dir}/commands.log" 'compose.*<down>'

case_dir="$(new_case cleanup)"
VOICE_A1_FLUTTER_PROFILE_HANDOFF_CLEANUP=true run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 0
assert_contains "${case_dir}/commands.log" 'restore-phase0-fixture-owner.py'
[[ -z "$(find "${case_dir}/tmp with spaces" -type d -name phase0-fixture -print -quit)" ]] || fail 'fixture must be removed after opted-in Compose cleanup'
assert_contains "${case_dir}/commands.log" 'compose.*<down> <--remove-orphans>'
assert_not_contains "${case_dir}/commands.log" '--volumes|<-v>|volume prune|system prune'
cleanup_line="$(grep '^docker <compose> ' "${case_dir}/commands.log" | grep '<down>' | tail -1)"
cleanup_env_file="$(sed -n 's/.* <--env-file> <\([^>]*\)> .*/\1/p' <<<"$cleanup_line")"
cleanup_project="$(sed -n 's/.* <--project-name> <\([^>]*\)> .*/\1/p' <<<"$cleanup_line")"
cleanup_directory="$(sed -n 's/.* <--project-directory> <\([^>]*\)> .*/\1/p' <<<"$cleanup_line")"
cleanup_compose_file="$(sed -n 's/.* <-f> <\([^>]*\)> .*/\1/p' <<<"$cleanup_line")"
cleanup_profile="$(sed -n 's/.* <--profile> <\([^>]*\)> .*/\1/p' <<<"$cleanup_line")"
assert_abs "$cleanup_env_file"; assert_abs "$cleanup_directory"; assert_abs "$cleanup_compose_file"
[[ "$cleanup_project" =~ ^voice-a1-flutter-handoff-[a-z0-9-]+$ ]] || fail "unsafe cleanup project: $cleanup_project"
[[ "$cleanup_project" != voice ]] || fail 'cleanup must never target the shared voice project'
assert_eq "$cleanup_profile" app
while IFS= read -r compose_identity_line; do
  assert_overlay_order "$compose_identity_line"
  identity_env_file="$(sed -n 's/.* <--env-file> <\([^>]*\)> .*/\1/p' <<<"$compose_identity_line")"
  identity_project="$(sed -n 's/.* <--project-name> <\([^>]*\)> .*/\1/p' <<<"$compose_identity_line")"
  identity_directory="$(sed -n 's/.* <--project-directory> <\([^>]*\)> .*/\1/p' <<<"$compose_identity_line")"
  identity_compose_file="$(sed -n 's/.* <-f> <\([^>]*\)> .*/\1/p' <<<"$compose_identity_line")"
  identity_profile="$(sed -n 's/.* <--profile> <\([^>]*\)> .*/\1/p' <<<"$compose_identity_line")"
  assert_eq "$identity_env_file" "$cleanup_env_file"
  assert_eq "$identity_project" "$cleanup_project"
  assert_eq "$identity_directory" "$cleanup_directory"
  assert_eq "$identity_compose_file" "$cleanup_compose_file"
  assert_eq "$identity_profile" "$cleanup_profile"
done < <(grep '^docker <compose> ' "${case_dir}/commands.log")

case_dir="$(new_case cleanup-down-failure)"
FAKE_DOWN_RC=47 VOICE_A1_FLUTTER_PROFILE_HANDOFF_CLEANUP=true run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 0
[[ -d "$(find "${case_dir}/tmp with spaces" -type d -name phase0-fixture -print -quit)" ]] || fail 'fixture must remain when opted-in Compose cleanup fails'
assert_contains "${case_dir}/stderr" 'cleanup failed.*fixture retained'

echo '== fixture generation failure removes the unowned temporary directory =='
case_dir="$(new_case fixture-failure)"
FAKE_FIXTURE_RC=17 run_runner "$case_dir"
assert_eq "$(cat "${case_dir}/rc")" 17
assert_not_contains "${case_dir}/commands.log" '<config>'
assert_not_contains "${case_dir}/commands.log" '<up>'

echo 'All compose-a1-flutter-profile-handoff tests passed.'
