#!/usr/bin/env bash
# Apply gateway_db schema for /api/v1/version (idempotent).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
PG_POD="${VOICE_POSTGRES_POD:-voice-postgres-0}"
PG_USER="${POSTGRES_USER:-voice}"

SQL_FILE="${ROOT}/src/backend/migrations/gateway_db/000001_client_versions.up.sql"
if [ ! -f "${SQL_FILE}" ]; then
  echo "ERROR: missing ${SQL_FILE}" >&2
  exit 1
fi

echo "Ensuring gateway_db client_versions schema in ${NS}/${PG_POD}..."
kubectl exec -n "${NS}" -i "${PG_POD}" -- \
  psql -U "${PG_USER}" -d gateway_db -v ON_ERROR_STOP=1 < "${SQL_FILE}"
echo "gateway_db schema ready."
