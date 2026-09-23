#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT"
PROJECT="voice-nats-analytics-chat-${GITHUB_RUN_ID:-ci}-$$"
compose() { docker compose -p "$PROJECT" "$@"; }
cleanup() { compose down -v --remove-orphans; }
trap cleanup EXIT

compose up -d --wait nats
compose run --rm nats-realtime-bootstrap
compose run --rm nats-analytics-chat-bootstrap

for spec in \
  'message_events analytics_v2_msg message.>' \
  'user_events analytics_v2_user user.>' \
  'chat_events analytics_v2_chat >' \
  'matchmaking_events analytics_v2_mm mm.>' \
  'voice_events analytics_v2_voice voice.>' \
  'story_events analytics_v2_story story.>' \
  'bot_events analytics_v2_bot bot.>' \
  'social_events analytics_v2_social social.>' \
  'role_events analytics_v2_role role.>' \
  'file_events analytics_v2_file file.>' \
  'subscription_events analytics_v2_subscription subscription.>' \
  'moderation_events analytics_v2_moderation moderation.>' \
  'analytics_events analytics_v2_telemetry analytics.>' \
  'user_events chat_account_deleted user.account_deleted'; do
  set -- $spec
  info="$(compose run --rm --no-deps --entrypoint nats nats-analytics-chat-bootstrap --server nats://nats:4222 req --raw "\$JS.API.CONSUMER.INFO.$1.$2" "")"
  [[ "$(printf '%s' "$info" | jq -r '.config.filter_subject')" == "$3" ]]
  [[ "$(printf '%s' "$info" | jq -r '.config.ack_policy')" == explicit ]]
done

compose run --rm --no-deps --entrypoint nats nats-analytics-chat-bootstrap --server nats://nats:4222 consumer rm user_events chat_account_deleted --force
compose run --rm --no-deps --entrypoint nats nats-analytics-chat-bootstrap --server nats://nats:4222 consumer add user_events chat_account_deleted --filter user.account_deleted --target _INBOX.voice.chat.chat_account_deleted --ack explicit --deliver new --defaults
if compose run --rm nats-analytics-chat-bootstrap; then
  echo 'expected bootstrap to reject chat durable delivery-policy drift' >&2
  exit 1
fi
echo 'NATS Analytics/Chat bootstrap smoke OK'
