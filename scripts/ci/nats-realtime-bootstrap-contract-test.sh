#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
BOOTSTRAP="${ROOT}/docker/nats/realtime-bootstrap.sh"
K8S_BOOTSTRAP="${ROOT}/deploy/templates/nats-realtime-bootstrap.yaml"
NOTIFICATION_BOOTSTRAP="${ROOT}/docker/nats/notification-bootstrap.sh"
K8S_NOTIFICATION_BOOTSTRAP="${ROOT}/deploy/templates/nats-notification-bootstrap.yaml"
COMPOSE="${ROOT}/docker-compose.yml"
MANIFEST="${ROOT}/deploy/nats/jetstream-publisher-streams.yaml"
STAGING_INFRA="${ROOT}/scripts/staging/apply-infra.sh"
PROD_INFRA="${ROOT}/scripts/prod/apply-infra.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }
require() { grep -Fqx -- "$1" "$2" || fail "missing exact contract line in ${2#"${ROOT}/"}: $1"; }
require_stream_contract() {
  name="$1" subjects="$2" max_age="$3"
  block="$(awk -v name="$name" '
    $0 == "  - name: " name { found = 1; next }
    found && /^  - name:/ { exit }
    found { print }
  ' "$MANIFEST")"
  printf '%s\n' "$block" | grep -Fqx "    subjects: $subjects" || fail "${name} subjects must match its central contract"
  printf '%s\n' "$block" | grep -Fqx '    retention: limits' || fail "${name} retention must be limits"
  printf '%s\n' "$block" | grep -Fqx "    max_age: $max_age" || fail "${name} max age must match its central contract"
  printf '%s\n' "$block" | grep -Fqx '    storage: file' || fail "${name} storage must be file"
}

[[ -x "$BOOTSTRAP" ]] || fail "Compose bootstrap script must be executable"
cmp <(sed -n '/^    #!\/bin\/sh$/,$p' "$K8S_BOOTSTRAP" | sed '/^---$/,$d' | sed 's/^    //') "$BOOTSTRAP" \
  || fail "Kubernetes bootstrap script must exactly match the Compose bootstrap script"

for stream in \
  'stream message_events message.sent message.edited message.deleted message.read message.read_receipt_revoked message.reaction_added message.reaction_removed message.mention_added message.pinned message.unpinned message.forwarded message.delivery_ack' \
  'stream chat_events chat.created chat.member_changed chat.dm_peer_deleted space.tree_changed space.created voice.room_created voice.room_deleted space.invite_created space.member_joined space.member_left space.updated space.deleted' \
  'stream file_events file.uploaded file.processed file.scan_infected file.expired file.downloaded' \
  'stream moderation_events moderation.report_created moderation.sanction_applied moderation.appeal_submitted' \
  'stream bot_events bot.registered bot.command_executed bot.webhook_delivered bot.webhook_failed' \
  'stream subscription_events subscription.plan_started subscription.plan_cancelled subscription.plan_expired subscription.downgrade subscription.payment_success subscription.payment_failed subscription.space_pro_started subscription.space_pro_expired subscription.grace_reminder' \
  'stream story_events story.created story.viewed story.reacted story.expired story.highlight_created story.lfp_created story.lfp_response' \
  'stream user_events user.account_deleted user.profile_created user.profile_updated user.profile_switched user.verified user.presence_changed user.game_detected user.settings_changed' \
  'stream social_events social.friend_request social.friend_accepted social.friend_removed social.user_blocked social.contacts_synced' \
  'stream role_events role.created role.updated role.deleted role.assigned role.revoked role.chat_override_set role.chat_override_removed role.voice_override_set role.voice_override_removed' \
  'stream voice_events voice.call_incoming voice.call_accepted voice.call_declined voice.call_missed voice.call_ended voice.state_changed voice.screen_share_started voice.screen_share_stopped voice.call_started voice.member_joined' \
  'stream matchmaking_events mm.search_started mm.search_cancelled mm.search_nudge mm.search_timeout mm.match_found mm.match_completed mm.rating_submitted mm.player_banned'; do
  require "$stream" "$BOOTSTRAP"
done

for consumer in \
  "consumer message_events rt_realtime1_msg 'message.>' _INBOX.voice.realtime1.message" \
  "consumer chat_events rt_realtime1_chat 'chat.>' _INBOX.voice.realtime1.chat" \
  'consumer user_events rt_realtime1_user user.presence_changed _INBOX.voice.realtime1.user' \
  'consumer social_events rt_realtime1_social social.user_blocked _INBOX.voice.realtime1.social' \
  "consumer role_events rt_realtime1_role 'role.>' _INBOX.voice.realtime1.role" \
  "consumer voice_events rt_realtime1_voice 'voice.>' _INBOX.voice.realtime1.voice" \
  "consumer matchmaking_events rt_realtime1_matchmaking 'mm.>' _INBOX.voice.realtime1.matchmaking"; do
  require "$consumer" "$BOOTSTRAP"
done

require '  nats-realtime-bootstrap:' "$COMPOSE"
require '        condition: service_completed_successfully' "$COMPOSE"
require '  - name: voice_events' "$MANIFEST"
require '    subjects: [voice.call_incoming, voice.call_accepted, voice.call_declined, voice.call_missed, voice.call_ended, voice.state_changed, voice.screen_share_started, voice.screen_share_stopped, voice.call_started, voice.member_joined]' "$MANIFEST"
require_stream_contract analytics_events '[analytics.>]' 168h
require_stream_contract user_profile_projection '[user.search_profile_projection]' 0s
require "stream analytics_events 'analytics.>'" "$BOOTSTRAP"
require 'stream_with_max_age user_profile_projection 0 user.search_profile_projection' "$BOOTSTRAP"
require 'stream voice_events voice.call_incoming voice.call_accepted voice.call_declined voice.call_missed voice.call_ended voice.state_changed voice.screen_share_started voice.screen_share_stopped voice.call_started voice.member_joined' "$NOTIFICATION_BOOTSTRAP"
cmp <(sed -n '/^    #!\/bin\/sh$/,$p' "$K8S_NOTIFICATION_BOOTSTRAP" | sed '/^---$/,$d' | sed 's/^    //') "$NOTIFICATION_BOOTSTRAP" \
  || fail "Kubernetes notification bootstrap script must exactly match the Compose notification bootstrap script"
grep -Fq '[(.config.subjects | sort), .config.storage, .config.retention, .config.max_age]' "$BOOTSTRAP" || fail "bootstrap must validate storage, retention and max age"
grep -Fq -- '--argjson max_age "$max_age"' "$BOOTSTRAP" || fail "bootstrap must validate per-stream max age"
sh -n "$BOOTSTRAP" || fail "Compose bootstrap must be valid POSIX shell"
if ! awk '
  $1 == "stream" {
    for (i = 3; i <= NF; i++) {
      subject = $i
      gsub(/^\047|\047$/, "", subject)
      if ((subject ~ /[*>]/ && !($2 == "analytics_events" && subject == "analytics.>")) || (subject in owners && owners[subject] != $2)) {
        exit 1
      }
      owners[subject] = $2
    }
  }
' "$BOOTSTRAP"; then
  fail "publisher streams must have disjoint, exact subjects"
fi
for service in social user matchmaking role voice analytics; do
  section="$(sed -n "/^  ${service}:$/,/^  [^ ]/p" "$COMPOSE")"
  printf '%s\n' "$section" | grep -Fqx '      nats-realtime-bootstrap:' || fail "${service} must wait for central NATS bootstrap"
  printf '%s\n' "$section" | grep -Fqx '        condition: service_completed_successfully' || fail "${service} bootstrap dependency must require success"
done
for infra in "$STAGING_INFRA" "$PROD_INFRA"; do
  grep -Fq 'deploy/templates/nats-realtime-bootstrap.yaml' "$infra" || fail "${infra#"${ROOT}/"} must apply central bootstrap"
  grep -Fq 'kubectl wait --for=condition=complete job/voice-nats-realtime-bootstrap' "$infra" || fail "${infra#"${ROOT}/"} must wait for central bootstrap"
done
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
