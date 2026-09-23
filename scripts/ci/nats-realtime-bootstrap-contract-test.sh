#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
BOOTSTRAP="${ROOT}/docker/nats/realtime-bootstrap.sh"
K8S_BOOTSTRAP="${ROOT}/deploy/templates/nats-realtime-bootstrap.yaml"
COMPOSE="${ROOT}/docker-compose.yml"

fail() { echo "FAIL: $*" >&2; exit 1; }
require() { grep -Fqx -- "$1" "$2" || fail "missing exact contract line in ${2#"${ROOT}/"}: $1"; }

[[ -x "$BOOTSTRAP" ]] || fail "Compose bootstrap script must be executable"
cmp <(sed -n '/^    #!\/bin\/sh$/,$p' "$K8S_BOOTSTRAP" | sed '/^---$/,$d' | sed 's/^    //') "$BOOTSTRAP" \
  || fail "Kubernetes bootstrap script must exactly match the Compose bootstrap script"

for stream in \
  'stream message_events message.sent message.edited message.deleted message.read message.read_receipt_revoked message.reaction_added message.reaction_removed message.mention_added message.pinned message.unpinned message.forwarded message.delivery_ack' \
  'stream chat_events chat.created chat.member_changed chat.dm_peer_deleted' \
  'stream user_events user.account_deleted user.profile_created user.profile_updated user.profile_switched user.verified user.presence_changed user.game_detected user.settings_changed' \
  'stream social_events social.friend_request social.friend_accepted social.friend_removed social.user_blocked social.contacts_synced' \
  'stream role_events role.created role.updated role.deleted role.assigned role.revoked role.chat_override_set role.chat_override_removed role.voice_override_set role.voice_override_removed' \
  'stream voice_events voice.>' \
  'stream matchmaking_events mm.search_started mm.search_cancelled mm.search_nudge mm.search_timeout mm.match_found mm.match_completed mm.rating_submitted mm.player_banned'; do
  require "$stream" "$BOOTSTRAP"
done

for consumer in \
  'consumer message_events rt_realtime1_msg message.> _INBOX.voice.realtime1.message' \
  'consumer chat_events rt_realtime1_chat chat.> _INBOX.voice.realtime1.chat' \
  'consumer user_events rt_realtime1_user user.presence_changed _INBOX.voice.realtime1.user' \
  'consumer social_events rt_realtime1_social social.user_blocked _INBOX.voice.realtime1.social' \
  'consumer role_events rt_realtime1_role role.> _INBOX.voice.realtime1.role' \
  'consumer voice_events rt_realtime1_voice voice.> _INBOX.voice.realtime1.voice' \
  'consumer matchmaking_events rt_realtime1_matchmaking mm.> _INBOX.voice.realtime1.matchmaking'; do
  require "$consumer" "$BOOTSTRAP"
done

require '  nats-realtime-bootstrap:' "$COMPOSE"
require '        condition: service_completed_successfully' "$COMPOSE"
for manifest in "${ROOT}/deploy/staging/services.yaml" "${ROOT}/deploy/prod/services.yaml"; do
  require '              value: realtime-1' "$manifest"
done

for source in \
  chat_events_consumer.go matchmaking_events_consumer.go message_events_consumer.go \
  role_events_consumer.go social_events_consumer.go user_events_consumer.go voice_events_consumer.go; do
  file="${ROOT}/src/backend/realtime/${source}"
  grep -Fq 'nats.Bind(' "$file" || fail "${source} must bind its centrally provisioned durable"
  ! grep -Fq 'nats.Durable(' "$file" || fail "${source} must not auto-create a durable"
  ! grep -Fq 'nats.BindStream(' "$file" || fail "${source} must not create or update stream state"
done

echo 'NATS Realtime bootstrap contract OK'
