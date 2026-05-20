#!/usr/bin/env bash
# Apply messaging_db DDL after databases exist (runs once on fresh volume).
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname messaging_db -f /docker-entrypoint-initdb.d/messaging_db_init.sql.snippet
