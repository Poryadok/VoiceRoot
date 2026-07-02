#!/usr/bin/env bash
# Create Voice application databases on staging Postgres (idempotent).
# Run after voice-postgres is Ready and voice-app-secrets exists.
set -euo pipefail

NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
PG_POD="${VOICE_POSTGRES_POD:-voice-postgres-0}"
PG_USER="${POSTGRES_USER:-voice}"
PG_DB="${POSTGRES_DB:-voice}"

databases=(
  auth_db user_db social_db chat_db messaging_db file_db space_db role_db
  notification_db matchmaking_db gateway_db search_db subscription_db
  moderation_db bot_db story_db
)

for db in "${databases[@]}"; do
  kubectl exec -n "$NS" "$PG_POD" -- \
    psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 \
    -c "SELECT 1 FROM pg_database WHERE datname = '${db}'" | grep -q 1 \
    && echo "exists: ${db}" \
    || {
      echo "creating: ${db}"
      kubectl exec -n "$NS" "$PG_POD" -- \
        psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${db}"
    }
done

echo "Postgres databases ready in ${NS}/${PG_POD}"
