#!/usr/bin/env bash
# Bootstrap independent production credentials without writing their values to stdout.
set -euo pipefail

NS="${VOICE_K8S_NAMESPACE:-voice-prod}"
SECRET_NAME=voice-app-secrets

if kubectl get secret "${SECRET_NAME}" -n "${NS}" >/dev/null 2>&1; then
  echo "Secret ${SECRET_NAME} already exists in ${NS}; preserving it."
  exit 0
fi

command -v openssl >/dev/null
workdir="$(mktemp -d)"
trap 'rm -rf "${workdir}"' EXIT
umask 077

random_hex() {
  openssl rand -hex "$1"
}

pg_password="$(random_hex 32)"
clickhouse_password="$(random_hex 32)"
staff_token="$(random_hex 32)"
jwt_file="${workdir}/auth-jwt-private.pem"
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 -out "${jwt_file}" 2>/dev/null

pg_url() {
  printf 'postgres://voice:%s@voice-postgres:5432/%s?sslmode=disable' "${pg_password}" "$1"
}

install -d -m 700 /etc/voice
printf 'PROD_STAFF_TOKEN=%s\n' "${staff_token}" >/etc/voice/operator.env
chmod 600 /etc/voice/operator.env

kubectl create secret generic "${SECRET_NAME}" \
  --namespace "${NS}" \
  --from-literal=POSTGRES_PASSWORD="${pg_password}" \
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
  --from-literal=VOICE_DATABASE_URL="$(pg_url voice_db)" \
  --from-file=AUTH_JWT_PRIVATE_KEY="${jwt_file}" \
  --from-literal=AUTH_TOTP_ENCRYPTION_KEY="$(random_hex 32)" \
  --from-literal=ACCOUNT_DELETE_TOKEN_SECRET="$(random_hex 32)" \
  --from-literal=AUTH_RESEND_API_KEY= \
  --from-literal='AUTH_RESEND_FROM=Voice <noreply@voice.invalid>' \
  --from-literal=USER_R2_ENDPOINT= \
  --from-literal=USER_R2_ACCESS_KEY_ID= \
  --from-literal=USER_R2_SECRET_ACCESS_KEY= \
  --from-literal=USER_R2_BUCKET=voice-prod-avatars \
  --from-literal=USER_R2_PUBLIC_BASE_URL= \
  --from-literal=FILE_R2_ENDPOINT= \
  --from-literal=FILE_R2_ACCESS_KEY_ID= \
  --from-literal=FILE_R2_SECRET_ACCESS_KEY= \
  --from-literal=FILE_R2_BUCKET=voice-prod-files \
  --from-literal=FCM_PROJECT_ID= \
  --from-literal=FCM_SERVICE_ACCOUNT_JSON= \
  --from-literal=APNS_KEY_ID= \
  --from-literal=APNS_TEAM_ID= \
  --from-literal=APNS_PRIVATE_KEY= \
  --from-literal=APNS_BUNDLE_ID=voice.app \
  --from-literal=APNS_VOIP_TOPIC=voice.app.voip \
  --from-literal=APNS_PRODUCTION=false \
  --from-literal=CLICKHOUSE_PASSWORD="${clickhouse_password}" \
  --from-literal="CLICKHOUSE_DSN=clickhouse://default:${clickhouse_password}@voice-clickhouse:9000/voice" \
  --from-literal=ANALYTICS_ID_HASH_KEY="$(random_hex 32)" \
  --from-literal=LIVEKIT_API_KEY="$(random_hex 8)" \
  --from-literal=LIVEKIT_API_SECRET="$(random_hex 32)" \
  --from-literal=PADDLE_WEBHOOK_SECRET="$(random_hex 32)" \
  --from-literal="GATEWAY_STATIC_TOKENS_JSON={\"${staff_token}\":{\"user_id\":\"00000000-0000-0000-0000-000000000099\",\"profile_id\":\"00000000-0000-0000-0000-000000000199\",\"roles\":[\"staff\"]}}"

echo "Created ${SECRET_NAME} in ${NS}; operator token is root-only at /etc/voice/operator.env."
