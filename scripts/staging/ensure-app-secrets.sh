#!/usr/bin/env bash
# Create voice-app-secrets in the staging namespace when missing.
# Idempotent: skips if the secret already exists (does not overwrite cluster state).
#
# Env:
#   VOICE_K8S_NAMESPACE              (default: voice-staging)
#   VOICE_STAGING_POSTGRES_PASSWORD  (default: voice)
#   AUTH_JWT_PRIVATE_KEY_FILE        (default: repo jwt-test-private.pem)
#   USER_R2_* / FILE_R2_*            optional object storage (empty = pods start, uploads may fail)
#
# CI: set secrets.STAGING_APP_SECRETS_YAML (base64 full Secret manifest) to apply custom values instead.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
SECRET_NAME="voice-app-secrets"

if [ -n "${STAGING_APP_SECRETS_YAML_B64:-}" ]; then
  echo "Applying ${SECRET_NAME} from STAGING_APP_SECRETS_YAML_B64 (merge)"
  echo "${STAGING_APP_SECRETS_YAML_B64}" | base64 -d | kubectl apply -f -
  exit 0
fi

if [ -n "${STAGING_APP_SECRETS_YAML:-}" ] && [ -f "${STAGING_APP_SECRETS_YAML}" ]; then
  echo "Applying ${SECRET_NAME} from ${STAGING_APP_SECRETS_YAML} (merge)"
  kubectl apply -f "${STAGING_APP_SECRETS_YAML}"
  exit 0
fi

if kubectl get secret "$SECRET_NAME" -n "$NS" >/dev/null 2>&1; then
  echo "Secret ${SECRET_NAME} already exists in ${NS} — skip bootstrap"
  exit 0
fi

PG_PASS="${VOICE_STAGING_POSTGRES_PASSWORD:-voice}"
CH_PASS="${VOICE_STAGING_CLICKHOUSE_PASSWORD:-voice-clickhouse-staging}"
JWT_FILE="${AUTH_JWT_PRIVATE_KEY_FILE:-${ROOT}/src/backend/auth/src/test/resources/jwt-test-private.pem}"

if [ ! -f "$JWT_FILE" ]; then
  echo "ERROR: JWT key not found at ${JWT_FILE} (set AUTH_JWT_PRIVATE_KEY_FILE)" >&2
  exit 1
fi

pg_url() {
  printf 'postgres://voice:%s@voice-postgres:5432/%s?sslmode=disable' "$PG_PASS" "$1"
}

USER_R2_ENDPOINT="${USER_R2_ENDPOINT:-}"
USER_R2_ACCESS_KEY_ID="${USER_R2_ACCESS_KEY_ID:-}"
USER_R2_SECRET_ACCESS_KEY="${USER_R2_SECRET_ACCESS_KEY:-}"
USER_R2_BUCKET="${USER_R2_BUCKET:-voice-staging-avatars}"
USER_R2_PUBLIC_BASE_URL="${USER_R2_PUBLIC_BASE_URL:-}"
FILE_R2_ENDPOINT="${FILE_R2_ENDPOINT:-}"
FILE_R2_ACCESS_KEY_ID="${FILE_R2_ACCESS_KEY_ID:-}"
FILE_R2_SECRET_ACCESS_KEY="${FILE_R2_SECRET_ACCESS_KEY:-}"
FILE_R2_BUCKET="${FILE_R2_BUCKET:-voice-staging-files}"

echo "Bootstrapping ${SECRET_NAME} in ${NS} (Postgres user voice, test JWT key)"

kubectl create secret generic "$SECRET_NAME" \
  --namespace="$NS" \
  --from-literal=POSTGRES_PASSWORD="$PG_PASS" \
  --from-literal=SOCIAL_DATABASE_URL="$(pg_url social_db)" \
  --from-literal=USER_DATABASE_URL="$(pg_url user_db)" \
  --from-literal=CHAT_DATABASE_URL="$(pg_url chat_db)" \
  --from-literal=SPACE_DATABASE_URL="$(pg_url space_db)" \
  --from-literal=MESSAGING_DATABASE_URL="$(pg_url messaging_db)" \
  --from-literal=FILE_DATABASE_URL="$(pg_url file_db)" \
  --from-literal=ROLE_DATABASE_URL="$(pg_url role_db)" \
  --from-literal=MATCHMAKING_DATABASE_URL="$(pg_url matchmaking_db)" \
  --from-literal=SEARCH_DATABASE_URL="$(pg_url search_db)" \
  --from-literal=NOTIFICATION_DATABASE_URL="$(pg_url notification_db)" \
  --from-literal=BOT_DATABASE_URL="$(pg_url bot_db)" \
  --from-literal=STORY_DATABASE_URL="$(pg_url story_db)" \
  --from-literal=MODERATION_DATABASE_URL="$(pg_url moderation_db)" \
  --from-literal=SUBSCRIPTION_DATABASE_URL="$(pg_url subscription_db)" \
  --from-literal=GATEWAY_DATABASE_URL="$(pg_url gateway_db)" \
  --from-literal=CLICKHOUSE_PASSWORD="$CH_PASS" \
  --from-literal=CLICKHOUSE_DSN="clickhouse://default:${CH_PASS}@voice-clickhouse:9000/voice" \
  --from-literal=ANALYTICS_ID_HASH_KEY="change-me-staging-analytics-hash" \
  --from-file=AUTH_JWT_PRIVATE_KEY="$JWT_FILE" \
  --from-literal=USER_R2_ENDPOINT="$USER_R2_ENDPOINT" \
  --from-literal=USER_R2_ACCESS_KEY_ID="$USER_R2_ACCESS_KEY_ID" \
  --from-literal=USER_R2_SECRET_ACCESS_KEY="$USER_R2_SECRET_ACCESS_KEY" \
  --from-literal=USER_R2_BUCKET="$USER_R2_BUCKET" \
  --from-literal=USER_R2_PUBLIC_BASE_URL="$USER_R2_PUBLIC_BASE_URL" \
  --from-literal=FILE_R2_ENDPOINT="$FILE_R2_ENDPOINT" \
  --from-literal=FILE_R2_ACCESS_KEY_ID="$FILE_R2_ACCESS_KEY_ID" \
  --from-literal=FILE_R2_SECRET_ACCESS_KEY="$FILE_R2_SECRET_ACCESS_KEY" \
  --from-literal=FILE_R2_BUCKET="$FILE_R2_BUCKET" \
  --from-literal=FCM_PROJECT_ID="" \
  --from-literal=FCM_SERVICE_ACCOUNT_JSON="" \
  --from-literal=APNS_KEY_ID="" \
  --from-literal=APNS_TEAM_ID="" \
  --from-literal=APNS_PRIVATE_KEY="" \
  --from-literal=APNS_BUNDLE_ID="voice.app" \
  --from-literal=APNS_VOIP_TOPIC="voice.app.voip" \
  --from-literal=APNS_PRODUCTION="false" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Created ${SECRET_NAME} in ${NS}"
