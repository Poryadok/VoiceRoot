#!/usr/bin/env bash
set -euo pipefail
# Apply matchmaking_db DDL after databases exist (runs once on fresh volume).
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname matchmaking_db -f /docker-entrypoint-initdb.d/matchmaking_db_init.sql.snippet
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname matchmaking_db -f /docker-entrypoint-initdb.d/matchmaking_db_search_sessions.sql.snippet
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname matchmaking_db -f /docker-entrypoint-initdb.d/matchmaking_db_matches.sql.snippet
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname matchmaking_db -f /docker-entrypoint-initdb.d/matchmaking_db_ratings.sql.snippet
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname matchmaking_db -f /docker-entrypoint-initdb.d/matchmaking_db_search_nudge.sql.snippet
