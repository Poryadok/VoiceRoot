#!/usr/bin/env bash
# Apply golang-migrate SQL for Go-owned Postgres DBs on the local Compose network.
# auth_db (Path A): Flyway on Auth boot — not migrated here unless VOICE_MIGRATE_AUTH_DB=1.
# Usage: compose-migrate-all.sh [all|e2e|bot|story|voice]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

POSTGRES_USER="${POSTGRES_USER:-voice}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-voice}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
GAME_INTEGRATION_DB_PASSWORD="${GAME_INTEGRATION_DB_PASSWORD:-voice-compose-gameintegration-runtime}"
export POSTGRES_USER POSTGRES_PASSWORD GAME_INTEGRATION_DB_PASSWORD
# Compose publishes Postgres to the host; reach it from one-off migrate containers.
MIGRATE_PG_HOST="${VOICE_MIGRATE_PG_HOST:-host.docker.internal}"
MIGRATE_DOCKER_HOST_ARGS=(--add-host=host.docker.internal:host-gateway)

postgres_container_running() {
  local cid
  cid="$(docker compose ps -q postgres 2>/dev/null || true)"
  if [[ -z "${cid}" ]]; then
    echo "postgres container not running; start: docker compose up -d" >&2
    exit 1
  fi
  local running
  running="$(docker inspect -f '{{.State.Running}}' "${cid}" 2>/dev/null || echo false)"
  if [[ "${running}" != "true" ]]; then
    echo "postgres container is not running (id=${cid})" >&2
    docker compose ps postgres || true
    docker compose logs postgres --tail 80 || true
    exit 1
  fi
}

wait_postgres_tcp() {
  local i
  for i in $(seq 1 30); do
    if docker run --rm "${MIGRATE_DOCKER_HOST_ARGS[@]}" \
      postgres:18-alpine pg_isready -h "${MIGRATE_PG_HOST}" -p "${POSTGRES_PORT}" -U "${POSTGRES_USER}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  echo "postgres TCP not ready on ${MIGRATE_PG_HOST}:${POSTGRES_PORT}" >&2
  docker compose ps postgres || true
  docker compose logs postgres --tail 80 || true
  exit 1
}

migrate_db() {
  local db="$1"
  if [[ "$db" == space_db ]]; then
    # Build from this checkout, never guess a published installer tag. The
    # resulting image includes exact compiled SQL and native source identities.
    local source_revision installer_image
    source_revision="$(git rev-parse HEAD)"
    installer_image="voice-native-space-installer:local-${source_revision}"
    docker build --build-arg "SOURCE_REVISION=${SOURCE_REVISION:-}" \
      -f src/backend/space/Dockerfile.native-migrate -t "$installer_image" .
    local evidence_dir prior_file run_log image_id
    evidence_dir="${SPACE_NATIVE_EVIDENCE_DIR:-${ROOT}/tmp/native-space-installer}"
    mkdir -p "$evidence_dir"
    chmod 700 "$evidence_dir"
    prior_file="${SPACE_NATIVE_PRIOR_EVIDENCE:-${evidence_dir}/prior.jsonl}"
    local prior_args=() prior_mount=()
    if [[ -f "$prior_file" ]]; then
      prior_args=(--mode=verify --prior-evidence=/evidence/prior.jsonl)
      prior_mount=(--mount "type=bind,source=${prior_file},target=/evidence/prior.jsonl,readonly")
    fi
    run_log="$(umask 077; mktemp "${evidence_dir}/run.XXXXXX.jsonl")"
    if PGHOST="$MIGRATE_PG_HOST" PGPORT="$POSTGRES_PORT" PGUSER="$POSTGRES_USER" PGPASSWORD="$POSTGRES_PASSWORD" PGSSLMODE=disable \
      docker run --rm "${MIGRATE_DOCKER_HOST_ARGS[@]}" "${prior_mount[@]}" \
      -e PGHOST -e PGPORT -e PGUSER -e PGPASSWORD -e PGSSLMODE "$installer_image" "${prior_args[@]}" >"$run_log"; then
      cat "$run_log"
      if [[ ! -f "$prior_file" ]]; then
        # Only the successful fresh command can produce this original artifact.
        # Verify output never overwrites creation/transition evidence.
        cp "$run_log" "${prior_file}.next"
        chmod 600 "${prior_file}.next"
        mv "${prior_file}.next" "$prior_file"
        image_id="$(docker image inspect "$installer_image" --format '{{.Id}}')"
        printf '{"image_id":"%s","source_revision":"%s"}\n' "$image_id" "${SOURCE_REVISION:-}" >"${evidence_dir}/image.json"
        chmod 600 "${evidence_dir}/image.json"
      fi
    else
      local failure=$?
      cat "$run_log"
      return "$failure"
    fi
    return
  fi
  echo "==> golang-migrate up: ${db}"
  local dsn
  dsn="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${MIGRATE_PG_HOST}:${POSTGRES_PORT}/${db}?sslmode=disable"
  if [[ "$db" == bot_db ]]; then
    # The same project-scoped volume and helper serve CLI and app bootstrap.
    # Never recapture an already-migrated source into a newly missing artifact.
    docker compose --profile app run --rm --no-deps --build \
      --entrypoint python3 compose-db-init \
      /usr/local/bin/provision-bot-membership-source.py /native-bot-evidence/source.uuid -- \
      migrate -path /migrations/bot_db \
      -database "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/bot_db?sslmode=disable" up
    return
  fi
  docker run --rm "${MIGRATE_DOCKER_HOST_ARGS[@]}" \
    -v "${ROOT}/src/backend/migrations/${db}:/migrations" migrate/migrate \
    -path /migrations \
    -database "${dsn}" up
}

provision_gameintegration_runtime_role() {
  echo "==> provision GIS runtime database role"
  PGPASSWORD="${POSTGRES_PASSWORD}" docker run --rm -i "${MIGRATE_DOCKER_HOST_ARGS[@]}" \
    -e PGPASSWORD -e GAME_INTEGRATION_DB_PASSWORD \
    -v "${ROOT}/docker/postgres/provision-gameintegration-runtime-role.sql:/provision.sql:ro" \
    postgres:18-alpine psql -v ON_ERROR_STOP=1 \
      -h "${MIGRATE_PG_HOST}" -p "${POSTGRES_PORT}" -U "${POSTGRES_USER}" \
      -d postgres -f /provision.sql
}

run_e2e() {
  migrate_db chat_db
  migrate_db messaging_db
}

run_bot() {
  migrate_db bot_db
}

run_story() {
  migrate_db story_db
}

run_voice() {
  migrate_db voice_db
}

run_other_go_owned() {
  provision_gameintegration_runtime_role
  local dbs=(
    user_db social_db file_db space_db role_db notification_db realtime_db
    matchmaking_db search_db moderation_db gateway_db subscription_db game_integration_db
  )
  local db
  for db in "${dbs[@]}"; do
    migrate_db "${db}"
  done
}

run_auth_optional() {
  if [[ "${VOICE_MIGRATE_AUTH_DB:-}" == "1" ]]; then
    echo "==> auth_db Path B (golang-migrate); set AUTH_FLYWAY_ENABLED=false for Auth"
    migrate_db auth_db
  else
    echo "==> auth_db Path A (default): Flyway on Auth container boot — skipped"
    echo "    To use golang-migrate instead: VOICE_MIGRATE_AUTH_DB=1 $0"
  fi
}

run_all() {
  run_e2e
  run_bot
  run_story
  run_voice
  run_other_go_owned
  run_auth_optional
}

postgres_container_running
wait_postgres_tcp
MODE="${1:-all}"

case "${MODE}" in
  all) run_all ;;
  e2e) run_e2e ;;
  bot) run_bot ;;
  story) run_story ;;
  voice) run_voice ;;
  *)
    echo "usage: $0 [all|e2e|bot|story|voice]" >&2
    exit 1
    ;;
esac

echo "compose migrate (${MODE}) OK"
