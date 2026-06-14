#!/usr/bin/env bash
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname search_db -f /docker-entrypoint-initdb.d/search_db_init.sql.snippet
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname search_db -f /docker-entrypoint-initdb.d/search_db_phase13_verification.sql.snippet
