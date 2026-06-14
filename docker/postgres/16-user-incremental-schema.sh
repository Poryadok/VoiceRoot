#!/usr/bin/env bash
# Apply user_db incremental DDL on fresh volumes (privacy, phase 13).
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname user_db -f /docker-entrypoint-initdb.d/incremental_user_db.sql.snippet
