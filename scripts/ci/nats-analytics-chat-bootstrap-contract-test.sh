#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
BOOTSTRAP="${ROOT}/deploy/templates/nats-analytics-chat-bootstrap.yaml"
COMPOSE_BOOTSTRAP="${ROOT}/docker/nats/analytics-chat-bootstrap.sh"
COMPOSE="${ROOT}/docker-compose.yml"
ANALYTICS="${ROOT}/src/backend/analytics/internal/consumer/runner.go"
CHAT="${ROOT}/src/backend/chat/account_deleted_consumer.go"

fail() { echo "FAIL: $*" >&2; exit 1; }
require() { grep -Fqx -- "    $1" "$BOOTSTRAP" || fail "missing exact bootstrap contract: $1"; }

for spec in \
  "consumer message_events analytics_v2_msg 'message.>' _INBOX.voice.analytics.analytics_v2_msg new analytics_v2_msg" \
  "consumer user_events analytics_v2_user 'user.>' _INBOX.voice.analytics.analytics_v2_user new analytics_v2_user" \
  "consumer chat_events analytics_v2_chat '>' _INBOX.voice.analytics.analytics_v2_chat new analytics_v2_chat" \
  "consumer matchmaking_events analytics_v2_mm 'mm.>' _INBOX.voice.analytics.analytics_v2_mm new analytics_v2_mm" \
  "consumer voice_events analytics_v2_voice 'voice.>' _INBOX.voice.analytics.analytics_v2_voice new analytics_v2_voice" \
  "consumer story_events analytics_v2_story 'story.>' _INBOX.voice.analytics.analytics_v2_story new analytics_v2_story" \
  "consumer bot_events analytics_v2_bot 'bot.>' _INBOX.voice.analytics.analytics_v2_bot new analytics_v2_bot" \
  "consumer social_events analytics_v2_social 'social.>' _INBOX.voice.analytics.analytics_v2_social new analytics_v2_social" \
  "consumer role_events analytics_v2_role 'role.>' _INBOX.voice.analytics.analytics_v2_role new analytics_v2_role" \
  "consumer file_events analytics_v2_file 'file.>' _INBOX.voice.analytics.analytics_v2_file new analytics_v2_file" \
  "consumer subscription_events analytics_v2_subscription 'subscription.>' _INBOX.voice.analytics.analytics_v2_subscription new analytics_v2_subscription" \
  "consumer moderation_events analytics_v2_moderation 'moderation.>' _INBOX.voice.analytics.analytics_v2_moderation new analytics_v2_moderation" \
  "consumer analytics_events analytics_v2_telemetry 'analytics.>' _INBOX.voice.analytics.analytics_v2_telemetry new analytics_v2_telemetry" \
  "consumer user_events chat_account_deleted user.account_deleted _INBOX.voice.chat.chat_account_deleted all ''"; do
  require "$spec"
done

grep -Fq 'CONSUMER.INFO.$stream_name.$durable' "$BOOTSTRAP" || fail 'bootstrap must inspect existing consumers'
grep -Fq '[.stream_name, .name, .config.durable_name,' "$BOOTSTRAP" || fail 'bootstrap must reject consumer identity drift'
grep -Fq 'incompatible consumer $stream_name/$durable' "$BOOTSTRAP" || fail 'bootstrap must reject durable drift'
grep -Fq -- '--deliver-group "$group"' "$BOOTSTRAP" || fail 'bootstrap must preserve Analytics queue groups'
grep -Fq 'nats-analytics-chat-bootstrap:' "$COMPOSE" || fail 'Compose must run the Analytics/Chat bootstrap'
grep -Fq './deploy/templates/nats-analytics-chat-bootstrap.yaml:/bootstrap/nats-analytics-chat-bootstrap.yaml:ro' "$COMPOSE" || fail 'Compose must mount the canonical Analytics/Chat bootstrap template read-only'
grep -Fq 'NATS_ANALYTICS_CHAT_BOOTSTRAP_TEMPLATE' "$COMPOSE_BOOTSTRAP" || fail 'Compose bootstrap must require the canonical template'
grep -Fq 'sed -n' "$COMPOSE_BOOTSTRAP" || fail 'Compose bootstrap must extract the canonical ConfigMap script'
grep -Fq 'mktemp' "$COMPOSE_BOOTSTRAP" || fail 'Compose bootstrap must materialize the extracted script before NATS request/reply calls'
grep -Fq '/bin/sh "$script" </dev/null' "$COMPOSE_BOOTSTRAP" || fail 'Compose bootstrap must keep NATS request/reply stdin away from shell source'
for source in "$ANALYTICS" "$CHAT"; do
  ! grep -Fq 'AddConsumer(' "$source" || fail "${source#"${ROOT}/"} must not create consumers"
  ! grep -Fq 'UpdateConsumer(' "$source" || fail "${source#"${ROOT}/"} must not update consumers"
done

echo 'NATS Analytics/Chat bootstrap contract OK'
