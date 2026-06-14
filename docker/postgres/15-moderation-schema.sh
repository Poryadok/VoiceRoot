#!/usr/bin/env bash
# Apply moderation_db DDL after databases exist (runs once on fresh volume).
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname moderation_db -f /docker-entrypoint-initdb.d/moderation_db_init.sql.snippet
