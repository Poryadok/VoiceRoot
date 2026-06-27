#!/usr/bin/env bash
# Apply golang-migrate SQL for Go-owned Postgres DBs on the local Compose network.
# auth_db (Path A): Flyway on Auth boot — not migrated here unless VOICE_MIGRATE_AUTH_DB=1.
# Usage: compose-migrate-all.sh [all|phase15|bot|story]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

postgres_container_id() {
  local cid
  cid="$(docker compose ps -q postgres 2>/dev/null || true)"
  if [[ -z "${cid}" ]]; then
    echo "postgres container not running; start: docker compose up -d" >&2
    exit 1
  fi
  echo "${cid}"
}

migrate_db() {
  local db="$1"
  echo "==> golang-migrate up: ${db}"
  # Share postgres network namespace — avoids compose DNS flakiness on CI runners.
  docker run --rm --network "container:${POSTGRES_CID}" \
    -v "${ROOT}/src/backend/migrations/${db}:/migrations" migrate/migrate \
    -path /migrations \
    -database "postgres://${POSTGRES_USER:-voice}:${POSTGRES_PASSWORD:-voice}@127.0.0.1:5432/${db}?sslmode=disable" up
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

POSTGRES_CID="$(postgres_container_id)"
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
