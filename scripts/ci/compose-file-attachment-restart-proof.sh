#!/usr/bin/env bash
# Isolated A1 attachment durability proof. Docker orchestration lives here, not
# in Go: this script owns one generated compose project and never addresses the
# shared local `voice` project.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
umask 077

if [[ -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
  echo "refusing caller-supplied COMPOSE_PROJECT_NAME; this proof generates its own isolated project" >&2
  exit 2
fi

proof_id="$(date +%s)-${RANDOM}${RANDOM}"
project="voice-a1-file-restart-${proof_id}"
if [[ ! "$project" =~ ^voice-a1-file-restart-[a-z0-9-]+$ ]] || [[ "$project" == "voice" ]]; then
  echo "generated unsafe compose project name: $project" >&2
  exit 2
fi
if [[ -n "$(docker ps -aq --filter "label=com.docker.compose.project=${project}")" ]]; then
  echo "refusing compose project collision: ${project}" >&2
  exit 2
fi

port_base="${VOICE_FILE_ATTACHMENT_RESTART_PORT_BASE:-$((24000 + RANDOM % 1000))}"
if [[ ! "$port_base" =~ ^[0-9]+$ ]] || (( port_base < 20000 || port_base > 65000 )); then
  echo "VOICE_FILE_ATTACHMENT_RESTART_PORT_BASE must be an integer from 20000 through 65000" >&2
  exit 2
fi
if (( port_base + 16 > 65535 )); then
  echo "VOICE_FILE_ATTACHMENT_RESTART_PORT_BASE leaves no room for the isolated compose port range" >&2
  exit 2
fi

for port in $(seq "$port_base" "$((port_base + 16))"); do
  if docker ps --format '{{.Ports}}' | grep -Eq "(:|\[::\]:)${port}->"; then
    echo "refusing occupied Docker host port: ${port}" >&2
    exit 2
  fi
done

tmp_parent="$(cd "${TMPDIR:-/tmp}" && pwd -P)"
state_dir="$(mktemp -d "${tmp_parent}/voice-file-restart-proof.XXXXXXXX")"
state_dir="$(cd "$state_dir" && pwd -P)"
state_path_posix="${state_dir}/state.json"
compose_env="${state_dir}/compose-empty.env"
: > "$compose_env" # Explicit --env-file prevents an ambient repository .env from participating.

if command -v cygpath >/dev/null 2>&1; then
  state_path="$(cygpath -am "$state_path_posix")"
else
  state_path="$state_path_posix"
fi

cleanup() {
  local status=$?
  if [[ "${VOICE_FILE_ATTACHMENT_RESTART_CLEANUP:-false}" == "true" ]]; then
    # Exact generated project only; never delete named volumes.
    docker compose --env-file "$compose_env" --project-name "$project" --project-directory "$ROOT" -f "$ROOT/docker-compose.yml" down --remove-orphans || true
  fi
  [[ "$state_dir" == "$tmp_parent"/voice-file-restart-proof.* ]] || {
    echo "refusing unsafe state cleanup path: $state_dir" >&2
    return "$status"
  }
  rm -rf -- "$state_dir"
  return "$status"
}
trap cleanup EXIT

export COMPOSE_PROJECT_NAME="$project"
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
export MINIO_ROOT_USER="voice-minio"
export MINIO_ROOT_PASSWORD="voice-minio-dev"
export FILE_R2_ENDPOINT="http://host.docker.internal:${MINIO_PORT}"
export FILE_R2_REGION="us-east-1"
export FILE_R2_ACCESS_KEY_ID="$MINIO_ROOT_USER"
export FILE_R2_SECRET_ACCESS_KEY="$MINIO_ROOT_PASSWORD"
export FILE_R2_BUCKET="voice-restart-${proof_id}"
export VOICE_RUN_LIVE_COMPOSE="true"
export VOICE_API_BASE_URL="http://127.0.0.1:${GATEWAY_PORT}"
export VOICE_FILE_ATTACHMENT_RESTART_PROOF_ID="$proof_id"
export VOICE_FILE_ATTACHMENT_RESTART_RUN_STARTED_UNIX_NANO="$(date +%s%N)"
export VOICE_FILE_ATTACHMENT_RESTART_STATE_PATH="$state_path"

manifest="$ROOT/.github/ci/e2e-features.yml"
mapfile -t restart_tests < <(bash "$ROOT/scripts/ci/e2e-manifest.sh" "$manifest" restart_proof_gateway)
if (( ${#restart_tests[@]} != 1 )) || [[ "${restart_tests[0]}" != "TestComposeFileAttachmentRestartProof_live" ]]; then
  echo "restart_proof_gateway manifest must contain exactly TestComposeFileAttachmentRestartProof_live" >&2
  exit 2
fi
restart_test="${restart_tests[0]}"

compose() {
  docker compose --env-file "$compose_env" --project-name "$project" --project-directory "$ROOT" -f "$ROOT/docker-compose.yml" "$@"
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

echo "A1 restart proof project=${project} gateway=${VOICE_API_BASE_URL}"
compose --profile app config --quiet
set +e
compose --profile app up -d --build
initial_up_status=$?
set -e
if (( initial_up_status != 0 )); then
  compose ps --all >&2 || true
  compose logs --no-color --timestamps compose-db-init >&2 || true
  exit "$initial_up_status"
fi
wait_healthy file
wait_healthy gateway
wait_gateway

export VOICE_FILE_ATTACHMENT_RESTART_PHASE=prepare
(
  cd "$ROOT/src/backend/gateway"
  go test -count=1 -parallel 1 -timeout 10m -run "^${restart_test}$" ./...
)
[[ -f "$state_path_posix" ]] || { echo "prepare did not create restart-proof state" >&2; exit 1; }

# Restart the durable attachment producer and storage owner separately. The
# subsequent verify phase must prove the attachment survives both restarts.
compose restart file
wait_healthy file
compose restart messaging
wait_healthy messaging
wait_healthy gateway
wait_gateway

export VOICE_FILE_ATTACHMENT_RESTART_PHASE=verify
(
  cd "$ROOT/src/backend/gateway"
  go test -count=1 -parallel 1 -timeout 10m -run "^${restart_test}$" ./...
)
