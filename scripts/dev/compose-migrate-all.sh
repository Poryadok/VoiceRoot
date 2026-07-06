#!/usr/bin/env bash
# Apply golang-migrate SQL for Go-owned Postgres DBs on the local Compose network.
# auth_db (Path A): Flyway on Auth boot — not migrated here unless VOICE_MIGRATE_AUTH_DB=1.
# Usage: compose-migrate-all.sh [all|phase15|bot|story]
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
      postgres:16-alpine pg_isready -h "${MIGRATE_PG_HOST}" -p "${POSTGRES_PORT}" -U "${POSTGRES_USER}" >/dev/null 2>&1; then
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
  echo "==> golang-migrate up: ${db}"
  local dsn
  dsn="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${MIGRATE_PG_HOST}:${POSTGRES_PORT}/${db}?sslmode=disable"
  docker run --rm "${MIGRATE_DOCKER_HOST_ARGS[@]}" \
    -v "${ROOT}/src/backend/migrations/${db}:/migrations" migrate/migrate \
    -path /migrations \
    -database "${dsn}" up
}

run_phase15() {
  migrate_db chat_db
  migrate_db messaging_db
}

run_bot() {
  migrate_db bot_db
}

run_story() {
  migrate_db story_db
}

run_other_go_owned() {
  local dbs=(
    user_db social_db file_db space_db role_db notification_db
    matchmaking_db search_db moderation_db gateway_db subscription_db
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
  run_phase15
  run_bot
  run_story
  run_other_go_owned
  run_auth_optional
}

postgres_container_running
wait_postgres_tcp
MODE="${1:-all}"

case "${MODE}" in
  all) run_all ;;
  phase15) run_phase15 ;;
  bot) run_bot ;;
  story) run_story ;;
  *)
    echo "usage: $0 [all|phase15|bot|story]" >&2
    exit 1
    ;;
esac

echo "compose migrate (${MODE}) OK"
