#!/usr/bin/env bash
# Apply subscription_db DDL after databases exist (runs once on fresh volume).
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname subscription_db -f /docker-entrypoint-initdb.d/subscription_db_init.sql.snippet
