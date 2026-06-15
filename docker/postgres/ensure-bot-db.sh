#!/usr/bin/env sh
set -eu

SCHEMA_DIR="${SCHEMA_DIR:-/schema}"

ensure_database() {
  db="$1"
  exists="$(psql -v ON_ERROR_STOP=1 -tAc "SELECT 1 FROM pg_database WHERE datname = '${db}'")"
  if [ "$exists" != "1" ]; then
    psql -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${db};"
  fi
}

ensure_database bot_db
bot_ready="$(psql -v ON_ERROR_STOP=1 -tAc "SELECT to_regclass('public.bots') IS NOT NULL" --dbname bot_db)"
if [ "$bot_ready" != "t" ]; then
  psql -v ON_ERROR_STOP=1 --dbname bot_db -f "${SCHEMA_DIR}/bot_db_init.sql.snippet"
fi
