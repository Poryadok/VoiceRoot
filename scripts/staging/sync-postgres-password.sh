#!/usr/bin/env bash
# Sync Postgres voice user password from voice-app-secrets (idempotent).
# Staging PVC may retain an older password while CI merges STAGING_APP_SECRETS_YAML_B64.
set -euo pipefail

NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
PG_POD="${VOICE_POSTGRES_POD:-voice-postgres-0}"
PG_USER="${POSTGRES_USER:-voice}"
PG_DB="${POSTGRES_DB:-voice}"

escape_pg_literal() {
  printf '%s' "$1" | sed "s/'/''/g"
}

postgres_secret_password() {
  kubectl get secret voice-app-secrets -n "${NS}" -o jsonpath='{.data.POSTGRES_PASSWORD}' 2>/dev/null \
    | base64 -d 2>/dev/null || true
}

postgres_remote_auth_ok() {
  local pass
  pass="$(postgres_secret_password)"
  if [ -z "${pass}" ]; then
    return 1
  fi
  kubectl exec -n "${NS}" "${PG_POD}" -- env PGPASSWORD="${pass}" \
    psql -h voice-postgres -U "${PG_USER}" -d "${PG_DB}" -v ON_ERROR_STOP=1 -c 'SELECT 1' \
    >/dev/null 2>&1
}

sync_postgres_password_from_secret() {
  local pass escaped
  pass="$(postgres_secret_password)"
  if [ -z "${pass}" ]; then
    echo "WARN: POSTGRES_PASSWORD missing in voice-app-secrets; skip password sync" >&2
    return 1
  fi
  escaped="$(escape_pg_literal "${pass}")"
  echo "Syncing Postgres ${PG_USER} password from voice-app-secrets..."
  kubectl exec -n "${NS}" "${PG_POD}" -- \
    psql -U "${PG_USER}" -d "${PG_DB}" -v ON_ERROR_STOP=1 \
    -c "ALTER USER \"${PG_USER}\" WITH PASSWORD '${escaped}'"
}

if ! kubectl get pod "${PG_POD}" -n "${NS}" >/dev/null 2>&1; then
  echo "skip postgres password sync: pod ${PG_POD} missing in ${NS}"
  exit 0
fi

if postgres_remote_auth_ok; then
  echo "Postgres remote auth already matches voice-app-secrets"
  exit 0
fi

echo "Postgres remote auth mismatch; syncing password from voice-app-secrets..."
if ! sync_postgres_password_from_secret; then
  echo "ERROR: failed to sync Postgres password in ${NS}/${PG_POD}" >&2
  exit 1
fi

if ! postgres_remote_auth_ok; then
  echo "ERROR: Postgres remote auth still failing after password sync" >&2
  kubectl logs -n "${NS}" "${PG_POD}" --tail=80 >&2 || true
  exit 1
fi

echo "Postgres password sync complete."
