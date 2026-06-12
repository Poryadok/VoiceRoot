#!/usr/bin/env bash
# Apply gateway_db DDL after databases exist (runs once on fresh volume).
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname gateway_db -f /docker-entrypoint-initdb.d/gateway_db_init.sql.snippet
