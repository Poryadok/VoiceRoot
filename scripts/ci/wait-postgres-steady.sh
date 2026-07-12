#!/usr/bin/env bash
# Wait until Postgres is steady after docker-entrypoint-initdb.d on a fresh volume.
# The official postgres image stops and restarts once after init scripts; probing only
# for created databases can succeed in that window and then fail with "shutting down".
# Usage: wait-postgres-steady.sh [REPO_ROOT]
set -euo pipefail

ROOT="${1:-.}"
cd "$ROOT"

postgres_probe_ok() {
  docker compose exec -T postgres pg_isready -U voice -d voice >/dev/null 2>&1 || return 1
  local dbs
  dbs="$(docker compose exec -T postgres psql -U voice -d voice -tAc \
    "SELECT count(*) FROM pg_database WHERE datname IN ('chat_db','user_db','auth_db')" 2>/dev/null || true)"
  [[ "$dbs" == "3" ]] || return 1
  docker compose exec -T postgres psql -U voice -d user_db -tAc "SELECT 1" >/dev/null 2>&1
}

stable=0
for _ in $(seq 1 90); do
  if postgres_probe_ok; then
    stable=$((stable + 1))
    if [[ "$stable" -ge 3 ]]; then
      exit 0
    fi
  else
    stable=0
  fi
  sleep 2
done

echo "Postgres not ready after init" >&2
docker compose ps
docker compose logs postgres --tail 80 >&2 || true
exit 1
