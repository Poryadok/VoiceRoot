#!/usr/bin/env bash
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname space_db -f /docker-entrypoint-initdb.d/space_db_subscription.sql.snippet
