#!/usr/bin/env bash
# Add missing keys to an existing voice-app-secrets (idempotent).
# CI STAGING_APP_SECRETS_YAML and older cluster secrets may lack keys for newer services
# (database URLs, R2 object storage, ClickHouse).
set -euo pipefail

NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
SECRET_NAME="voice-app-secrets"

if ! kubectl get secret "$SECRET_NAME" -n "$NS" >/dev/null 2>&1; then
  exit 0
fi

secret_data_key() {
  kubectl get secret "$SECRET_NAME" -n "$NS" -o "jsonpath={.data.$1}" 2>/dev/null || true
}

args=()
add_if_missing() {
  local key="$1"
  local value="$2"
  if [ -n "$(secret_data_key "${key}")" ]; then
    return 0
  fi
  echo "Patching ${SECRET_NAME}: add ${key}"
  args+=(--from-literal="${key}=${value}")
}

# ClickHouse keys must be patched even when POSTGRES_PASSWORD is absent (StatefulSet
# references CLICKHOUSE_PASSWORD via secretKeyRef since analytics staging).
DEFAULT_CH_PASS="${VOICE_STAGING_CLICKHOUSE_PASSWORD:-voice-clickhouse-staging}"
CH_PASS="$(secret_data_key CLICKHOUSE_PASSWORD | base64 -d 2>/dev/null || true)"
if [ -z "${CH_PASS}" ]; then
  current_ch_dsn_for_pass="$(secret_data_key CLICKHOUSE_DSN | base64 -d 2>/dev/null || true)"
  if [[ "${current_ch_dsn_for_pass}" =~ ^clickhouse://[^:]*:([^@]+)@ ]]; then
    CH_PASS="${BASH_REMATCH[1]}"
  else
    CH_PASS="${DEFAULT_CH_PASS}"
  fi
  echo "Patching ${SECRET_NAME}: add CLICKHOUSE_PASSWORD"
  args+=(--from-literal="CLICKHOUSE_PASSWORD=${CH_PASS}")
fi

ch_dsn() {
  printf 'clickhouse://default:%s@voice-clickhouse:9000/voice' "$CH_PASS"
}

add_if_missing ANALYTICS_ID_HASH_KEY "change-me-staging-analytics-hash"

current_ch_dsn="$(secret_data_key CLICKHOUSE_DSN | base64 -d 2>/dev/null || true)"
expected_ch_dsn="$(ch_dsn)"
if [ -z "${current_ch_dsn}" ]; then
  echo "Patching ${SECRET_NAME}: add CLICKHOUSE_DSN"
  args+=(--from-literal="CLICKHOUSE_DSN=${expected_ch_dsn}")
elif [ "${current_ch_dsn}" != "${expected_ch_dsn}" ]; then
  echo "Patching ${SECRET_NAME}: update CLICKHOUSE_DSN (sync password)"
  args+=(--from-literal="CLICKHOUSE_DSN=${expected_ch_dsn}")
fi

PG_PASS="$(secret_data_key POSTGRES_PASSWORD | base64 -d 2>/dev/null || true)"
if [ -z "${PG_PASS}" ]; then
  echo "WARN: ${SECRET_NAME} has no POSTGRES_PASSWORD; skip Postgres database URL patch" >&2
else
  pg_url() {
    printf 'postgres://voice:%s@voice-postgres:5432/%s?sslmode=disable' "$PG_PASS" "$1"
  }

  sync_pg_url_if_needed() {
    local key="$1"
    local db="$2"
    local expected current
    expected="$(pg_url "${db}")"
    current="$(secret_data_key "${key}" | base64 -d 2>/dev/null || true)"
    if [ -z "${current}" ]; then
      add_if_missing "${key}" "${expected}"
    elif [ "${current}" != "${expected}" ]; then
      echo "Patching ${SECRET_NAME}: update ${key} (sync password)"
      args+=(--from-literal="${key}=${expected}")
    fi
  }

  # All *_DATABASE_URL keys referenced by deploy/staging/services.yaml (+ gateway).
  pg_url_keys=(
    SOCIAL_DATABASE_URL:social_db
    USER_DATABASE_URL:user_db
    CHAT_DATABASE_URL:chat_db
    SPACE_DATABASE_URL:space_db
    MESSAGING_DATABASE_URL:messaging_db
    FILE_DATABASE_URL:file_db
    ROLE_DATABASE_URL:role_db
    MATCHMAKING_DATABASE_URL:matchmaking_db
    SEARCH_DATABASE_URL:search_db
    NOTIFICATION_DATABASE_URL:notification_db
    BOT_DATABASE_URL:bot_db
    STORY_DATABASE_URL:story_db
    MODERATION_DATABASE_URL:moderation_db
    SUBSCRIPTION_DATABASE_URL:subscription_db
    GATEWAY_DATABASE_URL:gateway_db
  )
  for entry in "${pg_url_keys[@]}"; do
    sync_pg_url_if_needed "${entry%%:*}" "${entry#*:}"
  done
fi

# R2 keys for user/file services (empty credentials = pods start, uploads may fail).
add_if_missing USER_R2_ENDPOINT "${USER_R2_ENDPOINT:-}"
add_if_missing USER_R2_ACCESS_KEY_ID "${USER_R2_ACCESS_KEY_ID:-}"
add_if_missing USER_R2_SECRET_ACCESS_KEY "${USER_R2_SECRET_ACCESS_KEY:-}"
add_if_missing USER_R2_BUCKET "${USER_R2_BUCKET:-voice-staging-avatars}"
add_if_missing USER_R2_PUBLIC_BASE_URL "${USER_R2_PUBLIC_BASE_URL:-}"
add_if_missing FILE_R2_ENDPOINT "${FILE_R2_ENDPOINT:-}"
add_if_missing FILE_R2_ACCESS_KEY_ID "${FILE_R2_ACCESS_KEY_ID:-}"
add_if_missing FILE_R2_SECRET_ACCESS_KEY "${FILE_R2_SECRET_ACCESS_KEY:-}"
add_if_missing FILE_R2_BUCKET "${FILE_R2_BUCKET:-voice-staging-files}"

if ((${#args[@]} == 0)); then
  echo "${SECRET_NAME} keys complete"
  exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq required for patch-app-secrets-database-urls.sh" >&2
  exit 1
fi

string_data="{}"
for arg in "${args[@]}"; do
  case "${arg}" in
    --from-literal=*)
      kv="${arg#--from-literal=}"
      key="${kv%%=*}"
      value="${kv#*=}"
      string_data="$(jq -nc --argjson obj "${string_data}" --arg key "${key}" --arg value "${value}" \
        '$obj + {($key): $value}')"
      ;;
    *)
      echo "WARN: unsupported patch arg ${arg}" >&2
      ;;
  esac
done

patch_payload="$(jq -nc --argjson stringData "${string_data}" '{stringData: $stringData}')"
kubectl patch secret "$SECRET_NAME" -n "$NS" --type=merge -p "${patch_payload}"

echo "Patched ${SECRET_NAME} in ${NS}"
