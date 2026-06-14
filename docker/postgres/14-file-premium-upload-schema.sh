#!/usr/bin/env bash
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname file_db -f /docker-entrypoint-initdb.d/file_db_premium_upload.sql.snippet
