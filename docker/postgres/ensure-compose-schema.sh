#!/bin/sh
# Idempotent schema bootstrap for compose on every `docker compose up`.
# 1) ensure databases exist
# 2) apply idempotent legacy SQL patches (old volumes / partial entrypoint init)
# 3) golang-migrate up for all Go-owned DBs (canonical: src/backend/migrations/*)
set -eu

SCHEMA_DIR="${SCHEMA_DIR:-/schema}"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"

ensure_database() {
  db="$1"
  exists="$(psql -v ON_ERROR_STOP=1 -tAc "SELECT 1 FROM pg_database WHERE datname = '${db}'")"
  if [ "$exists" != "1" ]; then
    psql -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${db};"
  fi
}

apply_if_exists() {
  db="$1"
  file="$2"
  if [ ! -f "${SCHEMA_DIR}/${file}" ]; then
    return 0
  fi
  has_sm="$(psql -v ON_ERROR_STOP=1 -tAc "SELECT to_regclass('public.schema_migrations') IS NOT NULL" --dbname "$db")"
  if [ "$has_sm" = "t" ]; then
    return 0
  fi
  tables="$(psql -v ON_ERROR_STOP=1 -tAc "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'" --dbname "$db")"
  if [ "$tables" -eq 0 ]; then
    return 0
  fi
  echo "==> legacy patch ${db} <- ${file}"
  psql -v ON_ERROR_STOP=1 --dbname "$db" -f "${SCHEMA_DIR}/${file}"
}

for db in auth_db user_db social_db chat_db messaging_db file_db space_db role_db \
  notification_db matchmaking_db gateway_db search_db subscription_db moderation_db bot_db story_db; do
  ensure_database "$db"
done

# Idempotent deltas for volumes created before golang-migrate tracking.
apply_if_exists chat_db incremental_chat_db.sql.snippet
apply_if_exists messaging_db incremental_messaging_db.sql.snippet
apply_if_exists user_db incremental_user_db.sql.snippet
apply_if_exists role_db incremental_role_db.sql.snippet
apply_if_exists matchmaking_db incremental_matchmaking_db.sql.snippet
apply_if_exists space_db incremental_space_db.sql.snippet
apply_if_exists story_db incremental_story_db.sql.snippet
apply_if_exists file_db incremental_file_db.sql.snippet
apply_if_exists file_db file_db_premium_upload.sql.snippet
apply_if_exists bot_db incremental_bot_db.sql.snippet
apply_if_exists moderation_db incremental_moderation_db.sql.snippet
apply_if_exists search_db search_db_verification.sql.snippet

export MIGRATIONS_DIR
sh /usr/local/bin/compose-migrate-dbs.sh
