#!/usr/bin/env bash
set -euo pipefail
# Apply notification_db DDL after databases exist (runs once on fresh volume).
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname notification_db -f /docker-entrypoint-initdb.d/notification_db_init.sql.snippet
