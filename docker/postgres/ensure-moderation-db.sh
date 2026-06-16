#!/usr/bin/env sh
# Idempotent moderation_db bootstrap for compose volumes created before Phase 14.
set -eu

exists="$(psql -v ON_ERROR_STOP=1 -tAc "SELECT 1 FROM pg_database WHERE datname = 'moderation_db'")"
if [ "$exists" != "1" ]; then
  psql -v ON_ERROR_STOP=1 -c "CREATE DATABASE moderation_db;"
fi

psql -v ON_ERROR_STOP=1 --dbname moderation_db -f /schema/moderation_db_init.sql.snippet
if [ -f /schema/incremental_moderation_story_reports.sql.snippet ]; then
  psql -v ON_ERROR_STOP=1 --dbname moderation_db -f /schema/incremental_moderation_story_reports.sql.snippet
fi
