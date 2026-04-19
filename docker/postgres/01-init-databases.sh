#!/usr/bin/env bash
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname "${POSTGRES_DB}" <<-EOSQL
	CREATE DATABASE auth_db;
	CREATE DATABASE user_db;
	CREATE DATABASE social_db;
	CREATE DATABASE chat_db;
	CREATE DATABASE messaging_db;
EOSQL
