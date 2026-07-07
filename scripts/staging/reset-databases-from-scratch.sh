#!/usr/bin/env bash
# Drop/recreate Voice application databases on staging and re-apply golang-migrate from repo.
# Use after renaming migration files or when schema_migrations is out of sync.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
PG_POD="${VOICE_POSTGRES_POD:-voice-postgres-0}"
PG_USER="${POSTGRES_USER:-voice}"
MIGRATE_TAG="${VOICE_MIGRATE_IMAGE_TAG:-v4.18.1}"

databases=(
  auth_db user_db social_db chat_db messaging_db file_db space_db role_db
  notification_db matchmaking_db gateway_db search_db subscription_db
  moderation_db bot_db story_db
)

go_owned_dbs=(
  chat_db messaging_db bot_db story_db
  user_db social_db file_db space_db role_db notification_db
  matchmaking_db search_db moderation_db gateway_db subscription_db
)

echo "==> Waiting for Postgres pod ${NS}/${PG_POD}"
kubectl wait --for=condition=ready "pod/${PG_POD}" -n "${NS}" --timeout=180s

PG_PASS="$(kubectl get secret voice-app-secrets -n "${NS}" -o jsonpath='{.data.POSTGRES_PASSWORD}' | base64 -d)"

recreate_db() {
  local db="$1"
  echo "==> recreate database: ${db}"
  kubectl exec -n "${NS}" "${PG_POD}" -- \
    psql -U "${PG_USER}" -d voice -v ON_ERROR_STOP=1 \
    -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${db}' AND pid <> pg_backend_pid();" \
    >/dev/null 2>&1 || true
  kubectl exec -n "${NS}" "${PG_POD}" -- \
    psql -U "${PG_USER}" -d voice -v ON_ERROR_STOP=1 \
    -c "DROP DATABASE IF EXISTS ${db};"
  kubectl exec -n "${NS}" "${PG_POD}" -- \
    psql -U "${PG_USER}" -d voice -v ON_ERROR_STOP=1 \
    -c "CREATE DATABASE ${db};"
}

migrate_db() {
  local db="$1"
  local db_slug="${db//_/-}"
  local migrations_dir="${ROOT}/src/backend/migrations/${db}"
  local cm_name="voice-${db_slug}-migrations"
  local job_name="voice-migrate-${db_slug}"

  if [ ! -d "${migrations_dir}" ]; then
    echo "==> skip migrate ${db}: no ${migrations_dir}"
    return 0
  fi

  echo "==> migrate up: ${db}"
  kubectl create configmap "${cm_name}" -n "${NS}" \
    --from-file="${migrations_dir}" \
    --dry-run=client -o yaml | kubectl apply -f -

  kubectl delete job "${job_name}" -n "${NS}" --ignore-not-found

  local dsn="postgres://${PG_USER}:${PG_PASS}@voice-postgres:5432/${db}?sslmode=disable"
  kubectl apply -n "${NS}" -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job_name}
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: ${job_name}
    app.kubernetes.io/component: migration
spec:
  ttlSecondsAfterFinished: 86400
  backoffLimit: 2
  template:
    metadata:
      labels:
        app.kubernetes.io/name: ${job_name}
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: migrate/migrate:${MIGRATE_TAG}
          args:
            - -path=/migrations
            - -database=${dsn}
            - up
          volumeMounts:
            - name: migrations
              mountPath: /migrations
              readOnly: true
      volumes:
        - name: migrations
          configMap:
            name: ${cm_name}
EOF
  kubectl wait --for=condition=complete "job/${job_name}" -n "${NS}" --timeout=300s
}

for db in "${databases[@]}"; do
  recreate_db "${db}"
done

for db in "${go_owned_dbs[@]}"; do
  migrate_db "${db}"
done

echo "==> auth_db: Flyway will run on next Auth pod restart"
kubectl rollout restart deployment/voice-auth -n "${NS}" || true

echo "==> Restarting app tier so services reconnect to fresh databases"
bash "${ROOT}/scripts/staging/rollout-app-tier.sh"

echo "Staging database reset complete (${NS})."
