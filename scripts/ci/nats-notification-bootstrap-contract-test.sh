#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
BOOTSTRAP="${ROOT}/docker/nats/notification-bootstrap.sh"

[[ -x "${BOOTSTRAP}" ]] || { echo 'notification bootstrap must be executable' >&2; exit 1; }
for spec in \
  'message_events notif_msg_v2 message.> _INBOX.voice.notification.message' \
  'matchmaking_events notif_mm mm.> _INBOX.voice.notification.matchmaking' \
  'voice_events notif_voice voice.> _INBOX.voice.notification.voice' \
  'story_events notif_story story.> _INBOX.voice.notification.story' \
  'social_events notif_social social.> _INBOX.voice.notification.social' \
  'subscription_events notif_subscription subscription.> _INBOX.voice.notification.subscription' \
  'moderation_events notif_mod moderation.> _INBOX.voice.notification.moderation'; do
  grep -Fqx "consumer ${spec}" "${BOOTSTRAP}"
done
grep -Fq "consumer not found" "${BOOTSTRAP}"
grep -Fq 'incompatible consumer $stream_name/$durable' "${BOOTSTRAP}"
echo 'NATS Notification bootstrap contract OK'
