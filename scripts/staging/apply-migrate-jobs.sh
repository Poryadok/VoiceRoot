#!/usr/bin/env bash
# Apply golang-migrate Jobs for Go-owned DBs on staging Postgres (idempotent).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/lib/kubectl-configmap.sh
source "${ROOT}/scripts/staging/lib/kubectl-configmap.sh"
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
  local job_status_jsonpath pod_status_jsonpath
  job_status_jsonpath='jsonpath=job active={.status.active} failed={.status.failed} succeeded={.status.succeeded}{"\n"}{range .status.conditions[*]}job.condition type={.type} reason={.reason}{"\n"}{end}'
  pod_status_jsonpath='jsonpath=pod phase={.status.phase}{"\n"}{range .status.containerStatuses[*]}container name={.name} terminated.reason={.state.terminated.reason} waiting.reason={.state.waiting.reason} exitCode={.state.terminated.exitCode}{"\n"}{end}'

  echo "migrate job ${job_name} status:" >&2
  kubectl get job "${job_name}" -n "${NS}" -o "${job_status_jsonpath}" >&2 || true
  local pod
  pod="$(kubectl get pods -n "${NS}" -l "job-name=${job_name}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [ -n "${pod}" ]; then
    kubectl get pod "${pod}" -n "${NS}" -o "${pod_status_jsonpath}" >&2 || true
  fi
}

migration_content_hash() {
  local dir="$1"
  if [ ! -d "${dir}" ]; then
    return 1
  fi
  find "${dir}" -type f -name '*.sql' -print0 | sort -z | xargs -0 sha256sum | sha256sum | awk '{print $1}'
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

  local content_hash stored_hash succeeded
  content_hash="$(migration_content_hash "${migrations_dir}")"
  stored_hash="$(kubectl get configmap "${cm_name}" -n "${NS}" -o jsonpath='{.metadata.annotations.voice\.io/migration-content-hash}' 2>/dev/null || true)"
  succeeded=0

  if kubectl get job "${job_name}" -n "${NS}" >/dev/null 2>&1; then
    succeeded="$(kubectl get job "${job_name}" -n "${NS}" -o jsonpath='{.status.succeeded}' 2>/dev/null || echo 0)"
    if [ "${succeeded:-0}" = "1" ] && [ "${stored_hash}" = "${content_hash}" ]; then
      echo "migrate job ${job_name} already succeeded for current migrations; skipping"
      return 0
    fi
    if [ "${succeeded:-0}" = "1" ] && [ -n "${stored_hash}" ] && [ "${stored_hash}" != "${content_hash}" ]; then
      echo "migrate job ${job_name} succeeded but migrations changed (${stored_hash} -> ${content_hash}); re-running"
    fi
    echo "deleting incomplete or stale job ${job_name}"
    kubectl delete job "${job_name}" -n "${NS}" --ignore-not-found
  fi

  echo "Applying migrations ConfigMap ${cm_name} from ${migrations_dir}"
  kubectl_apply_configmap "${cm_name}" "${NS}" \
    --from-file="${migrations_dir}"
  kubectl annotate configmap "${cm_name}" -n "${NS}" \
    "voice.io/migration-content-hash=${content_hash}" --overwrite

  echo "Applying migrate job ${job_name}"
  if grep -Fq '__DATABASE_URL__' "${template}"; then
    local dsn
    dsn="$(postgres_migrate_dsn "${db_key}")"
    substitute "${dsn}" < "${template}" | kubectl apply -f -
  else
    substitute '' < "${template}" | kubectl apply -f -
  fi
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

apply_migrate voice_db \
  "${ROOT}/src/backend/migrations/voice_db" \
  "${ROOT}/deploy/templates/migrate-voice-db-job.yaml" \
  voice-migrate-voice-db \
  voice-voice-db-migrations

echo "Staging DB migrate jobs complete."
