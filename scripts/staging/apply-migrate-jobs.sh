#!/usr/bin/env bash
# Apply golang-migrate Jobs for Go-owned DBs on staging Postgres (idempotent).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/lib/kubectl-configmap.sh
source "${ROOT}/scripts/staging/lib/kubectl-configmap.sh"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
MIGRATE_TAG="${VOICE_MIGRATE_IMAGE_TAG:-v4.18.1}"
PG_USER="${POSTGRES_USER:-voice}"
GIS_MIGRATIONS_ENABLED="${VOICE_ENABLE_GAME_INTEGRATION_DB:-false}"
case "${GIS_MIGRATIONS_ENABLED}" in
  true|false) ;;
  *) echo 'ERROR: VOICE_ENABLE_GAME_INTEGRATION_DB must be true or false' >&2; exit 2 ;;
esac

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

provision_gameintegration_runtime_role() {
  local job_name='voice-provision-gameintegration-runtime-role'
  local active
  local role_password_key runtime_url_key role_password runtime_url
  role_password_key="$(kubectl get secret voice-app-secrets -n "${NS}" -o jsonpath='{.data.GAME_INTEGRATION_DB_PASSWORD}' 2>/dev/null || true)"
  runtime_url_key="$(kubectl get secret voice-app-secrets -n "${NS}" -o jsonpath='{.data.GAME_INTEGRATION_DATABASE_URL}' 2>/dev/null || true)"
  if [ -z "${role_password_key}" ] || [ -z "${runtime_url_key}" ]; then
    echo 'ERROR: voice-app-secrets must contain operator-provisioned GIS runtime password and database URL keys' >&2
    exit 1
  fi
  if ! role_password="$(printf '%s' "${role_password_key}" | base64 -d 2>/dev/null)" || \
     ! runtime_url="$(printf '%s' "${runtime_url_key}" | base64 -d 2>/dev/null)" || \
     [[ ! "${role_password}" =~ ^[a-f0-9]{64}$ ]] || \
     [ "${runtime_url}" != "postgres://gameintegration_runtime:${role_password}@voice-postgres:5432/game_integration_db?sslmode=disable" ]; then
    echo 'ERROR: GIS runtime Secret must use a 64-character random hex password and the restricted game_integration_db URL' >&2
    exit 1
  fi

  kubectl_apply_configmap voice-gameintegration-runtime-role-sql "${NS}" \
    --from-file="provision.sql=${ROOT}/docker/postgres/provision-gameintegration-runtime-role.sql"
  if kubectl get job "${job_name}" -n "${NS}" >/dev/null 2>&1; then
    active="$(kubectl get job "${job_name}" -n "${NS}" -o jsonpath='{.status.active}' 2>/dev/null || echo 0)"
    if [ "${active:-0}" != 0 ]; then
      echo "ERROR: GIS runtime-role provision Job is already active: ${job_name}" >&2
      exit 1
    fi
    kubectl delete job "${job_name}" -n "${NS}" --wait=true
  fi
  substitute '' < "${ROOT}/deploy/templates/provision-gameintegration-runtime-role-job.yaml" | kubectl apply -f -
  if ! kubectl wait --for=condition=complete "job/${job_name}" -n "${NS}" --timeout=300s; then
    dump_migrate_job_logs "${job_name}"
    exit 1
  fi
}

finalize_gameintegration_runtime_role() {
  local job_name='voice-verify-gameintegration-db'
  local configmap_name='voice-gameintegration-db-verifier-sql'
  local active manifest dry_run

  if kubectl get job "${job_name}" -n "${NS}" >/dev/null 2>&1; then
    active="$(kubectl get job "${job_name}" -n "${NS}" -o jsonpath='{.status.active}' 2>/dev/null || echo 0)"
    if [ "${active:-0}" != 0 ]; then
      echo "ERROR: GIS privilege verifier Job is already active: ${job_name}" >&2
      exit 1
    fi
    kubectl delete job "${job_name}" -n "${NS}" --wait=true
  fi

  # Recreate the fixed source ConfigMap only after the prior Job has stopped,
  # so each verifier executes the current SQL as an immutable input.
  kubectl delete configmap "${configmap_name}" -n "${NS}" --ignore-not-found
  manifest="$(mktemp)"
  dry_run="$(kubectl_configmap_dry_run_flag)"
  if ! kubectl create configmap "${configmap_name}" --namespace="${NS}" \
    --from-file="verify.sql=${ROOT}/docker/postgres/verify-gameintegration-runtime-role.sql" \
    "${dry_run}" -o yaml >"${manifest}"; then
    rm -f "${manifest}"
    return 1
  fi
  if [ ! -s "${manifest}" ]; then
    echo "ERROR: kubectl generated an empty GIS verifier ConfigMap" >&2
    rm -f "${manifest}"
    return 1
  fi
  sed -i '/^data:/i immutable: true' "${manifest}"
  if kubectl apply -f "${manifest}"; then
    rm -f "${manifest}"
  else
    local status=$?
    rm -f "${manifest}"
    return "${status}"
  fi

  substitute '' < "${ROOT}/deploy/templates/verify-game-integration-db-job.yaml" | kubectl apply -f -
  if ! kubectl wait --for=condition=complete "job/${job_name}" -n "${NS}" --timeout=300s; then
    dump_migrate_job_logs "${job_name}"
    exit 1
  fi
}

apply_gameintegration_migrations() {
  provision_gameintegration_runtime_role
  apply_migrate game_integration_db \
    "${ROOT}/src/backend/migrations/game_integration_db" \
    "${ROOT}/deploy/templates/migrate-game-integration-db-job.yaml" \
    voice-migrate-gameintegration-db \
    voice-gameintegration-db-migrations
  # apply_migrate may return early for an unchanged successful Job. Always
  # finalize grants after it returns, on both fresh and no-op migration paths.
  finalize_gameintegration_runtime_role
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

# User owns lifecycle/search tombstone migration 000017.  It must finish before
# any rollout enables USER_ACCOUNT_DELETE_CONSUMER_ENABLED; rollback disables
# that flag and never down-migrates this schema.
apply_migrate user_db \
  "${ROOT}/src/backend/migrations/user_db" \
  "${ROOT}/deploy/templates/migrate-user-db-job.yaml" \
  voice-migrate-user-db \
  voice-user-db-migrations

apply_migrate search_db \
  "${ROOT}/src/backend/migrations/search_db" \
  "${ROOT}/deploy/templates/migrate-search-db-job.yaml" \
  voice-migrate-search-db \
  voice-search-db-migrations

# These service-owned schemas were absent from the staging migration runner.
# Use each service's existing URL SecretRef so migration credentials stay out of
# rendered Job specs, shell arguments, and logs.
apply_migrate social_db \
  "${ROOT}/src/backend/migrations/social_db" \
  "${ROOT}/deploy/templates/migrate-social-db-job.yaml" \
  voice-migrate-social-db \
  voice-social-db-migrations

apply_migrate chat_db \
  "${ROOT}/src/backend/migrations/chat_db" \
  "${ROOT}/deploy/templates/migrate-chat-db-job.yaml" \
  voice-migrate-chat-db \
  voice-chat-db-migrations

apply_migrate messaging_db \
  "${ROOT}/src/backend/migrations/messaging_db" \
  "${ROOT}/deploy/templates/migrate-messaging-db-job.yaml" \
  voice-migrate-messaging-db \
  voice-messaging-db-migrations

apply_migrate file_db \
  "${ROOT}/src/backend/migrations/file_db" \
  "${ROOT}/deploy/templates/migrate-file-db-job.yaml" \
  voice-migrate-file-db \
  voice-file-db-migrations

apply_migrate space_db \
  "${ROOT}/src/backend/migrations/space_db" \
  "${ROOT}/deploy/templates/migrate-space-db-job.yaml" \
  voice-migrate-space-db \
  voice-space-db-migrations

apply_migrate role_db \
  "${ROOT}/src/backend/migrations/role_db" \
  "${ROOT}/deploy/templates/migrate-role-db-job.yaml" \
  voice-migrate-role-db \
  voice-role-db-migrations

apply_migrate notification_db \
  "${ROOT}/src/backend/migrations/notification_db" \
  "${ROOT}/deploy/templates/migrate-notification-db-job.yaml" \
  voice-migrate-notification-db \
  voice-notification-db-migrations

apply_migrate matchmaking_db \
  "${ROOT}/src/backend/migrations/matchmaking_db" \
  "${ROOT}/deploy/templates/migrate-matchmaking-db-job.yaml" \
  voice-migrate-matchmaking-db \
  voice-matchmaking-db-migrations

apply_migrate voice_db \
  "${ROOT}/src/backend/migrations/voice_db" \
  "${ROOT}/deploy/templates/migrate-voice-db-job.yaml" \
  voice-migrate-voice-db \
  voice-voice-db-migrations

if [ "${GIS_MIGRATIONS_ENABLED}" = true ]; then
  # The GIS runtime role is provisioned with the existing least-privilege SQL
  # before its schema grants are created by the privileged migration Job.
  apply_gameintegration_migrations
else
  echo 'Skipping internal Game Integration migrations (VOICE_ENABLE_GAME_INTEGRATION_DB=false)'
fi

echo "Staging DB migrate jobs complete."
