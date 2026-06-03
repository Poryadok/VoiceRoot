#!/usr/bin/env bash
# Apply file_db DDL after databases exist (runs once on fresh volume).
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname file_db -f /docker-entrypoint-initdb.d/file_db_init.sql.snippet
