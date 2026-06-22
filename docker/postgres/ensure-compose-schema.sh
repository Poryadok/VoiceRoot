#!/usr/bin/env sh
# Idempotent schema bootstrap for compose volumes created before newer migrations.
set -eu

SCHEMA_DIR="${SCHEMA_DIR:-/schema}"

ensure_database() {
  db="$1"
  exists="$(psql -v ON_ERROR_STOP=1 -tAc "SELECT 1 FROM pg_database WHERE datname = '${db}'")"
  if [ "$exists" != "1" ]; then
    psql -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${db};"
  fi
}

apply_incremental() {
  db="$1"
  file="$2"
  psql -v ON_ERROR_STOP=1 --dbname "$db" -f "${SCHEMA_DIR}/${file}"
}

for db in auth_db user_db social_db chat_db messaging_db file_db space_db role_db \
  notification_db matchmaking_db gateway_db search_db subscription_db moderation_db bot_db story_db; do
  ensure_database "$db"
done

matchmaking_ready="$(psql -v ON_ERROR_STOP=1 -tAc "SELECT to_regclass('public.games') IS NOT NULL" --dbname matchmaking_db)"
if [ "$matchmaking_ready" != "t" ]; then
  psql -v ON_ERROR_STOP=1 --dbname matchmaking_db -f "${SCHEMA_DIR}/matchmaking_db_init.sql.snippet"
  psql -v ON_ERROR_STOP=1 --dbname matchmaking_db -f "${SCHEMA_DIR}/matchmaking_db_search_sessions.sql.snippet"
  psql -v ON_ERROR_STOP=1 --dbname matchmaking_db -f "${SCHEMA_DIR}/matchmaking_db_matches.sql.snippet"
  psql -v ON_ERROR_STOP=1 --dbname matchmaking_db -f "${SCHEMA_DIR}/matchmaking_db_ratings.sql.snippet"
  psql -v ON_ERROR_STOP=1 --dbname matchmaking_db -f "${SCHEMA_DIR}/matchmaking_db_search_nudge.sql.snippet"
fi

apply_incremental chat_db incremental_chat_db.sql.snippet
apply_incremental matchmaking_db incremental_matchmaking_db.sql.snippet
apply_incremental messaging_db incremental_messaging_db.sql.snippet
apply_incremental user_db incremental_user_db.sql.snippet

role_ready="$(psql -v ON_ERROR_STOP=1 -tAc "SELECT to_regclass('public.roles') IS NOT NULL" --dbname role_db)"
if [ "$role_ready" != "t" ]; then
  psql -v ON_ERROR_STOP=1 --dbname role_db -f "${SCHEMA_DIR}/role_db_init.sql.snippet"
fi
apply_incremental role_db incremental_role_db.sql.snippet

file_ready="$(psql -v ON_ERROR_STOP=1 -tAc "SELECT to_regclass('public.files') IS NOT NULL" --dbname file_db)"
if [ "$file_ready" != "t" ]; then
  psql -v ON_ERROR_STOP=1 --dbname file_db -f "${SCHEMA_DIR}/file_db_init.sql.snippet"
fi
apply_incremental file_db file_db_premium_upload.sql.snippet

search_ready="$(psql -v ON_ERROR_STOP=1 -tAc "SELECT to_regclass('public.message_search_documents') IS NOT NULL" --dbname search_db)"
if [ "$search_ready" != "t" ]; then
  psql -v ON_ERROR_STOP=1 --dbname search_db -f "${SCHEMA_DIR}/search_db_init.sql.snippet"
fi
psql -v ON_ERROR_STOP=1 --dbname search_db -f "${SCHEMA_DIR}/search_db_phase13_verification.sql.snippet"

gateway_ready="$(psql -v ON_ERROR_STOP=1 -tAc "SELECT to_regclass('public.client_versions') IS NOT NULL" --dbname gateway_db)"
if [ "$gateway_ready" != "t" ]; then
  psql -v ON_ERROR_STOP=1 --dbname gateway_db -f "${SCHEMA_DIR}/gateway_db_init.sql.snippet"
fi
