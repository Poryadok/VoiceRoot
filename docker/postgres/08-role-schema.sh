#!/usr/bin/env bash
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname role_db -f /docker-entrypoint-initdb.d/role_db_init.sql.snippet
