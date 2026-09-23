#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT"
PROJECT="voice-nats-notification-bootstrap-${GITHUB_RUN_ID:-ci}-$$"
compose() { docker compose -p "$PROJECT" "$@"; }
cleanup() { compose down -v --remove-orphans; }
trap cleanup EXIT
compose up -d --wait nats
compose run --rm nats-notification-bootstrap
for spec in \
  'message_events notif_msg_v2 message.>' 'matchmaking_events notif_mm mm.>' \
  'voice_events notif_voice voice.>' 'story_events notif_story story.>' \
  'social_events notif_social social.>' 'subscription_events notif_subscription subscription.>' \
  'moderation_events notif_mod moderation.>'; do
  set -- $spec
  info="$(compose run --rm --no-deps --entrypoint nats nats-notification-bootstrap --server nats://nats:4222 req --raw "\$JS.API.CONSUMER.INFO.$1.$2" "")"
  [[ "$(printf '%s' "$info" | jq -r '.config.filter_subject')" == "$3" ]]
  [[ "$(printf '%s' "$info" | jq -r '.config.ack_policy')" == explicit ]]
done
compose run --rm --no-deps --entrypoint nats nats-notification-bootstrap --server nats://nats:4222 consumer rm social_events notif_social --force
compose run --rm --no-deps --entrypoint nats nats-notification-bootstrap --server nats://nats:4222 consumer add social_events notif_social --filter social.friend_request --target _INBOX.voice.notification.social --ack explicit --deliver new --defaults
if compose run --rm nats-notification-bootstrap; then
  echo 'expected bootstrap to reject narrowed consumer drift' >&2
  exit 1
fi
echo 'NATS Notification bootstrap smoke OK'
