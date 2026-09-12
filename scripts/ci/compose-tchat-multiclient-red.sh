#!/usr/bin/env bash
# Isolated three-phase GREEN runner for the T-CHAT multi-client closure contract.
# It proves active sockets while NATS is unavailable, then restart recovery, and
# never attaches to the shared local `voice` project.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
COMPOSE_FILE_PATH="${ROOT}/docker-compose.yml"
TEST_NAME='TestComposeTChatBlockedDMTransportClosure_live'
umask 077

for compose_var in \
  COMPOSE_PROJECT_NAME COMPOSE_PROFILES COMPOSE_FILE COMPOSE_ENV_FILES \
  COMPOSE_PATH_SEPARATOR COMPOSE_CONVERT_WINDOWS_PATHS COMPOSE_DISABLE_ENV_FILE; do
  if [[ -n "${!compose_var:-}" ]]; then
    echo "refusing caller-supplied ${compose_var}; this proof owns its Compose environment" >&2
    exit 2
  fi
done

port_base="${VOICE_TCHAT_CLOSURE_PORT_BASE:-$((27000 + RANDOM % 1000))}"
if [[ ! "$port_base" =~ ^[0-9]+$ ]] || (( port_base < 20000 || port_base > 65000 )); then
  echo "VOICE_TCHAT_CLOSURE_PORT_BASE must be an integer from 20000 through 65000" >&2
  exit 2
fi
if (( port_base + 16 > 65535 )); then
  echo "VOICE_TCHAT_CLOSURE_PORT_BASE leaves no room for the isolated 17-port range" >&2
  exit 2
fi

proof_id="$(date +%s)-${RANDOM}${RANDOM}"
project="voice-tchat-green-${proof_id}"
if [[ ! "$project" =~ ^voice-tchat-green-[a-z0-9-]+$ ]] || [[ "$project" == "voice" ]]; then
  echo "generated unsafe compose project name: $project" >&2
  exit 2
fi
if [[ -n "$(docker ps -aq --filter "label=com.docker.compose.project=${project}")" ]]; then
  echo "refusing compose project collision: ${project}" >&2
  exit 2
fi
for port in $(seq "$port_base" "$((port_base + 16))"); do
  if docker ps --format '{{.Ports}}' | grep -Eq "(:|\[::\]:)${port}->"; then
    echo "refusing occupied Docker host port: ${port}" >&2
    exit 2
  fi
done

tmp_parent="$(cd "${TMPDIR:-/tmp}" && pwd -P)"
state_dir="$(mktemp -d "${tmp_parent}/voice-tchat-green.XXXXXXXX")"
state_dir="$(cd "$state_dir" && pwd -P)"
state_path_posix="${state_dir}/state.json"
compose_env="${state_dir}/compose-empty.env"
: >"$compose_env"
if command -v cygpath >/dev/null 2>&1; then
  state_path="$(cygpath -am "$state_path_posix")"
else
  state_path="$state_path_posix"
fi

cleanup() {
  local status=$?
  if [[ "${VOICE_TCHAT_CLOSURE_CLEANUP:-false}" == "true" ]]; then
    compose down --remove-orphans || true
  fi
  case "$state_dir" in
    "$tmp_parent"/voice-tchat-green.*) rm -rf -- "$state_dir" ;;
    *) echo "refusing unsafe state cleanup path: $state_dir" >&2; status=1 ;;
  esac
  return "$status"
}
trap cleanup EXIT

export COMPOSE_PROJECT_NAME="$project"
unset COMPOSE_PROFILES COMPOSE_FILE COMPOSE_ENV_FILES \
  COMPOSE_PATH_SEPARATOR COMPOSE_CONVERT_WINDOWS_PATHS COMPOSE_DISABLE_ENV_FILE
export POSTGRES_PORT="$port_base"
export REDIS_PORT="$((port_base + 1))"
export NATS_PORT="$((port_base + 2))"
export NATS_HTTP_PORT="$((port_base + 3))"
export CLICKHOUSE_HTTP_PORT="$((port_base + 4))"
export LIVEKIT_PORT="$((port_base + 5))"
export LIVEKIT_RTC_TCP_PORT="$((port_base + 6))"
export LIVEKIT_RTC_UDP_PORT="$((port_base + 7))"
export CLAMAV_PORT="$((port_base + 8))"
export MINIO_PORT="$((port_base + 9))"
export MINIO_CONSOLE_PORT="$((port_base + 10))"
export NOTIFICATION_DEBUG_PORT="$((port_base + 11))"
export GATEWAY_PORT="$((port_base + 12))"
export WEB_PORT="$((port_base + 13))"
export DEVELOPER_PORTAL_PORT="$((port_base + 14))"
export ADMIN_PORT="$((port_base + 15))"
export VERIFICATION_STUB_PORT="$((port_base + 16))"
export VOICE_RUN_LIVE_COMPOSE=true
export VOICE_API_BASE_URL="http://127.0.0.1:${GATEWAY_PORT}"
export VOICE_AUTH_MAIL_STUB_URL="http://127.0.0.1:${VERIFICATION_STUB_PORT}"
export VOICE_TCHAT_CLOSURE_PROOF_ID="$proof_id"
export VOICE_TCHAT_CLOSURE_RUN_STARTED_UNIX_NANO="$(date +%s%N)"
export VOICE_TCHAT_CLOSURE_STATE_PATH="$state_path"

compose() {
  docker compose \
    --env-file "$compose_env" \
    --project-name "$project" \
    --project-directory "$ROOT" \
    -f "$COMPOSE_FILE_PATH" \
    --profile app \
    "$@"
}

wait_healthy() {
  local service="$1"
  local deadline=$((SECONDS + 180))
  local container health
  while (( SECONDS < deadline )); do
    container="$(compose ps -q "$service")"
    if [[ -n "$container" ]]; then
      health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container" 2>/dev/null || true)"
      if [[ "$health" == "healthy" ]]; then
        return 0
      fi
    fi
    sleep 2
  done
  echo "timed out waiting for ${service} in ${project}" >&2
  compose ps >&2 || true
  return 1
}

wait_gateway() {
  local deadline=$((SECONDS + 90))
  while (( SECONDS < deadline )); do
    if curl --fail --silent --show-error --max-time 3 "${VOICE_API_BASE_URL}/health" >/dev/null; then
      return 0
    fi
    sleep 2
  done
  echo "timed out waiting for gateway at ${VOICE_API_BASE_URL}" >&2
  return 1
}

run_phase() {
  export VOICE_TCHAT_CLOSURE_PHASE="$1"
  (
    cd "$ROOT/src/backend/gateway"
    go test -count=1 -parallel 1 -timeout 10m -run "^${TEST_NAME}$" ./...
  )
}

echo "T-CHAT GREEN project=${project} gateway=${VOICE_API_BASE_URL}"
compose config --quiet
compose up -d --build
wait_healthy gateway
wait_gateway

run_phase prepare
[[ -f "$state_path_posix" ]] || { echo "prepare did not create T-CHAT state" >&2; exit 1; }

compose stop nats
run_phase active

compose start nats
wait_healthy nats
compose restart chat messaging realtime
wait_healthy chat
wait_healthy messaging
wait_healthy realtime
wait_healthy gateway
wait_gateway

run_phase verify
