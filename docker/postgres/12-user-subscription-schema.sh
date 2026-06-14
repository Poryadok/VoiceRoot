#!/usr/bin/env bash
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname user_db -f /docker-entrypoint-initdb.d/user_db_subscription.sql.snippet
