#!/usr/bin/env bash
# Apply golang-migrate Jobs for Go-owned DBs on staging Postgres (idempotent).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
MIGRATE_TAG="${VOICE_MIGRATE_IMAGE_TAG:-v4.18.1}"
PG_USER="${POSTGRES_USER:-voice}"

postgres_migrate_dsn() {
  local db="$1"
  printf 'postgres://%s:%s@voice-postgres:5432/%s?sslmode=disable' "$PG_USER" "$PG_PASS" "$db"
}

escape_sed_replacement() {
  printf '%s' "$1" | sed -e 's/[&|]/\\&/g'
}

substitute() {
  local dsn="$1"
  local esc_dsn
  esc_dsn="$(escape_sed_replacement "${dsn}")"
  sed -e "s|__K_NAMESPACE__|${NS}|g" \
      -e "s|__MIGRATE_IMAGE_TAG__|${MIGRATE_TAG}|g" \
      -e "s|__DATABASE_URL__|${esc_dsn}|g"
}

dump_migrate_job_logs() {
  local job_name="$1"
  echo "migrate job ${job_name} status:" >&2
  kubectl get job "${job_name}" -n "${NS}" -o wide >&2 || true
  kubectl describe job "${job_name}" -n "${NS}" >&2 || true
  local pod
  pod="$(kubectl get pods -n "${NS}" -l "job-name=${job_name}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [ -n "${pod}" ]; then
    kubectl describe pod "${pod}" -n "${NS}" >&2 || true
    kubectl logs "${pod}" -n "${NS}" --all-containers=true --tail=120 >&2 || true
  fi
}

apply_migrate() {
  local db_key="$1"
  local migrations_dir="$2"
  local template="$3"
  local job_name="$4"
  local cm_name="$5"

  if [ ! -d "${migrations_dir}" ]; then
    echo "skip migrate ${db_key}: missing ${migrations_dir}"
    return 0
  fi

  echo "Applying migrations ConfigMap ${cm_name} from ${migrations_dir}"
  kubectl create configmap "${cm_name}" -n "${NS}" \
    --from-file="${migrations_dir}" \
    --dry-run=client -o yaml | kubectl apply -f -

  if kubectl get job "${job_name}" -n "${NS}" >/dev/null 2>&1; then
    local succeeded
    succeeded="$(kubectl get job "${job_name}" -n "${NS}" -o jsonpath='{.status.succeeded}' 2>/dev/null || echo 0)"
    if [ "${succeeded:-0}" = "1" ]; then
      echo "migrate job ${job_name} already succeeded; skipping"
      return 0
    fi
    echo "deleting incomplete job ${job_name}"
    kubectl delete job "${job_name}" -n "${NS}" --ignore-not-found
  fi

  local dsn
  dsn="$(postgres_migrate_dsn "${db_key}")"

  echo "Applying migrate job ${job_name}"
  substitute "${dsn}" < "${template}" | kubectl apply -f -
  if ! kubectl wait --for=condition=complete "job/${job_name}" -n "${NS}" --timeout=300s; then
    dump_migrate_job_logs "${job_name}"
    exit 1
  fi
}

if ! kubectl get secret voice-app-secrets -n "${NS}" >/dev/null 2>&1; then
  echo "ERROR: secret voice-app-secrets missing in ${NS}; cannot run migrate jobs" >&2
  exit 1
fi

bash "${ROOT}/scripts/staging/sync-postgres-password.sh"

PG_PASS="$(kubectl get secret voice-app-secrets -n "${NS}" -o jsonpath='{.data.POSTGRES_PASSWORD}' | base64 -d)"
if [ -z "${PG_PASS}" ]; then
  echo "ERROR: voice-app-secrets has no POSTGRES_PASSWORD in ${NS}" >&2
  exit 1
fi

apply_migrate bot_db \
  "${ROOT}/src/backend/migrations/bot_db" \
  "${ROOT}/deploy/templates/migrate-bot-db-job.yaml" \
  voice-migrate-bot-db \
  voice-bot-db-migrations

apply_migrate story_db \
  "${ROOT}/src/backend/migrations/story_db" \
  "${ROOT}/deploy/templates/migrate-story-db-job.yaml" \
  voice-migrate-story-db \
  voice-story-db-migrations

apply_migrate moderation_db \
  "${ROOT}/src/backend/migrations/moderation_db" \
  "${ROOT}/deploy/templates/migrate-moderation-db-job.yaml" \
  voice-migrate-moderation-db \
  voice-moderation-db-migrations

apply_migrate subscription_db \
  "${ROOT}/src/backend/migrations/subscription_db" \
  "${ROOT}/deploy/templates/migrate-subscription-db-job.yaml" \
  voice-migrate-subscription-db \
  voice-subscription-db-migrations

echo "Staging DB migrate jobs complete."
