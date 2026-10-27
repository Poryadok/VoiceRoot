#!/usr/bin/env bash
# Disposable Compose proof for Story expiry, archive, purge and File deletion.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
umask 077
if [[ -n "$(git -C "$ROOT" status --porcelain --untracked-files=all)" ]]; then
  echo "refusing dirty source tree for live proof" >&2
  exit 2
fi
source_head="$(git -C "$ROOT" rev-parse HEAD)"
if [[ -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
  echo "refusing caller-supplied Compose project" >&2
  exit 2
fi
proof_id="$(date +%s)-${RANDOM}${RANDOM}"
project="voice-story-ttl-delete-${proof_id}"
port_base="${VOICE_STORY_TTL_DELETE_PORT_BASE:-$((25000 + RANDOM % 1000))}"
if [[ ! "$port_base" =~ ^[0-9]+$ ]] || (( port_base < 20000 || port_base > 65000 )); then
  echo "invalid private port base" >&2
  exit 2
fi
if [[ -n "$(docker ps -aq --filter "label=com.docker.compose.project=${project}")" ]]; then
  echo "refusing Compose project collision" >&2
  exit 2
fi
for port in $(seq "$port_base" "$((port_base + 16))"); do
  if docker ps --format '{{.Ports}}' | grep -Eq "(:|\[::\]:)${port}->"; then
    echo "occupied private host port ${port}" >&2
    exit 2
  fi
done

tmp_parent="$(cd "${TMPDIR:-/tmp}" && pwd -P)"
state_dir="$(mktemp -d "${tmp_parent}/voice-story-ttl-delete.XXXXXXXX")"
fixture_dir="${state_dir}/phase0-fixture"
compose_env="${state_dir}/compose.env"
limits_file="${state_dir}/limits.yml"
started=false
compose() {
  docker compose --env-file "$compose_env" --project-name "$project" --project-directory "$ROOT" \
    -f "$ROOT/docker-compose.yml" -f "$ROOT/docker-compose.voice-space-media.yml" -f "$limits_file" --profile app "$@"
}
cleanup() {
  local status=$?
  if [[ "$started" == true ]]; then
    compose down --remove-orphans --volumes || status=1
  fi
  if [[ "$state_dir" == "$tmp_parent"/voice-story-ttl-delete.* ]]; then
    if [[ "${PHASE0_MATCHSQUAD_FIXTURE_UID:-}" == 65532 && -d "$fixture_dir" ]]; then
      python3 "$ROOT/scripts/dev/restore-phase0-fixture-owner.py" "$fixture_dir" || status=1
    fi
    if (( status == 0 )); then rm -rf -- "$state_dir"; fi
  fi
  exit "$status"
}
trap cleanup EXIT

unset PHASE0_MATCHSQUAD_FIXTURE_UID
if python3 -c 'import os,sys;sys.exit(0 if os.name=="posix" else 1)'; then
  export PHASE0_MATCHSQUAD_FIXTURE_UID=65532
fi
python3 "$ROOT/scripts/dev/generate-phase0-fixture.py" "$fixture_dir"
printf 'PHASE0_FIXTURE_DIR=%s\n' "$fixture_dir" > "$compose_env"
export PHASE0_FIXTURE_DIR="$fixture_dir"
export COMPOSE_PROJECT_NAME="$project" STORY_TTL_DEV=30s
export POSTGRES_PORT="$port_base" REDIS_PORT="$((port_base + 1))" NATS_PORT="$((port_base + 2))"
export NATS_HTTP_PORT="$((port_base + 3))" CLICKHOUSE_HTTP_PORT="$((port_base + 4))"
export LIVEKIT_PORT="$((port_base + 5))" LIVEKIT_RTC_TCP_PORT="$((port_base + 6))"
export LIVEKIT_RTC_UDP_PORT="$((port_base + 7))" CLAMAV_PORT="$((port_base + 8))"
export MINIO_PORT="$((port_base + 9))" MINIO_CONSOLE_PORT="$((port_base + 10))"
export NOTIFICATION_DEBUG_PORT="$((port_base + 11))" GATEWAY_PORT="$((port_base + 12))"
export WEB_PORT="$((port_base + 13))" DEVELOPER_PORTAL_PORT="$((port_base + 14))"
export ADMIN_PORT="$((port_base + 15))" VERIFICATION_STUB_PORT="$((port_base + 16))"
export MINIO_ROOT_USER=voice-minio MINIO_ROOT_PASSWORD=voice-minio-dev
export FILE_R2_ENDPOINT="http://host.docker.internal:${MINIO_PORT}" FILE_R2_REGION=us-east-1
export FILE_R2_ACCESS_KEY_ID="$MINIO_ROOT_USER" FILE_R2_SECRET_ACCESS_KEY="$MINIO_ROOT_PASSWORD"
export FILE_R2_BUCKET="voice-story-ttl-${proof_id}"
export VOICE_RUN_LIVE_COMPOSE=true VOICE_STORY_TTL_DELETE_LIVE_TEST=true
export VOICE_STORY_TTL_DELETE_PROOF_ID="$proof_id"
export VOICE_STORY_TTL_DELETE_STARTED_UNIX_NANO="$(date +%s%N)"
export VOICE_API_BASE_URL="http://127.0.0.1:${GATEWAY_PORT}"
export VOICE_AUTH_MAIL_STUB_URL="http://127.0.0.1:${VERIFICATION_STUB_PORT}"
export COMPOSE_PARALLEL_LIMIT=2

(cd "$ROOT/src/backend/gateway" && go test -c -o "$state_dir/gateway.test" .)
export VOICE_STORY_TTL_DELETE_TEST_SHA256="$(sha256sum "$state_dir/gateway.test" | cut -d' ' -f1)"
export VOICE_STORY_TTL_DELETE_SOURCE_HEAD="$source_head"
if [[ "$(git -C "$ROOT" rev-parse HEAD)" != "$source_head" ]] ||
   [[ -n "$(git -C "$ROOT" status --porcelain --untracked-files=all)" ]]; then
  echo "source changed during test compilation" >&2
  exit 2
fi

# Bound each private service; the existing shared projects are never modified.
printf 'services:\n' > "$limits_file"
while IFS= read -r service; do
  case "$service" in
    clickhouse|auth|clamav) memory=1536m; cpu=1.0 ;;
    postgres|minio|livekit) memory=768m; cpu=0.75 ;;
    *) memory=384m; cpu=0.5 ;;
  esac
  printf '  %s:\n    mem_limit: %s\n    cpus: "%s"\n' "$service" "$memory" "$cpu" >> "$limits_file"
done < <(docker compose --env-file "$compose_env" --project-name "$project" --project-directory "$ROOT" \
  -f "$ROOT/docker-compose.yml" -f "$ROOT/docker-compose.voice-space-media.yml" --profile app config --services)

echo "Story TTL delete proof project=${project}"
compose config --quiet
started=true
compose up -d --build
deadline=$((SECONDS + 180))
until curl --fail --silent --max-time 3 "${VOICE_API_BASE_URL}/health" >/dev/null; do
  if (( SECONDS >= deadline )); then
    compose ps --all >&2
    exit 1
  fi
  sleep 2
done
for service in story file gateway; do
  container="$(compose ps -q "$service")"
  if [[ -z "$container" ]]; then
    echo "missing private ${service} container" >&2
    exit 1
  fi
  image="$(docker inspect --format '{{.Image}}' "$container")"
  if [[ "$image" != sha256:* ]]; then
    echo "unbound private ${service} image" >&2
    exit 1
  fi
  export "VOICE_STORY_TTL_DELETE_$(tr '[:lower:]' '[:upper:]' <<< "$service")_IMAGE_ID=$image"
done
if [[ "$(git -C "$ROOT" rev-parse HEAD)" != "$source_head" ]] ||
   [[ -n "$(git -C "$ROOT" status --porcelain --untracked-files=all)" ]] ||
   [[ "$(sha256sum "$state_dir/gateway.test" | cut -d' ' -f1)" != "$VOICE_STORY_TTL_DELETE_TEST_SHA256" ]]; then
  echo "proof inputs changed after private build" >&2
  exit 1
fi
"$state_dir/gateway.test" -test.v -test.parallel=1 -test.timeout=8m -test.run='^TestComposeStoryTTLDeleteFile_live$'
