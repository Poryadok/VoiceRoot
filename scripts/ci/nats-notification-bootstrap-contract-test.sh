#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
BOOTSTRAP="${ROOT}/docker/nats/notification-bootstrap.sh"
K8S_BOOTSTRAP="${ROOT}/deploy/templates/nats-notification-bootstrap.yaml"
REALTIME_BOOTSTRAP="${ROOT}/docker/nats/realtime-bootstrap.sh"
COMPOSE="${ROOT}/docker-compose.yml"
SMOKE="${ROOT}/scripts/ci/nats-notification-bootstrap-smoke.sh"

[[ -x "${BOOTSTRAP}" ]] || { echo 'notification bootstrap must be executable' >&2; exit 1; }
cmp <(sed -n '/^    #!\/bin\/sh$/,$p' "${K8S_BOOTSTRAP}" | sed '/^---$/,$d' | sed 's/^    //') "${BOOTSTRAP}"
for spec in \
  "message_events notif_msg_v2 'message.>' _INBOX.voice.notification.message" \
  "matchmaking_events notif_mm 'mm.>' _INBOX.voice.notification.matchmaking" \
  "voice_events notif_voice 'voice.>' _INBOX.voice.notification.voice" \
  "story_events notif_story 'story.>' _INBOX.voice.notification.story" \
  "social_events notif_social 'social.>' _INBOX.voice.notification.social" \
  "subscription_events notif_subscription 'subscription.>' _INBOX.voice.notification.subscription" \
  "moderation_events notif_mod 'moderation.>' _INBOX.voice.notification.moderation"; do
  grep -Fqx "consumer ${spec}" "${BOOTSTRAP}"
done
grep -Fq "consumer not found" "${BOOTSTRAP}"
grep -Fq 'incompatible consumer $stream_name/$durable' "${BOOTSTRAP}"
! grep -Fq 'nats --server "$nats_url" stream add' "${BOOTSTRAP}"
for stream in \
  'stream message_events message.sent message.edited message.deleted message.read message.read_receipt_revoked message.reaction_added message.reaction_removed message.mention_added message.pinned message.unpinned message.forwarded message.delivery_ack' \
  'stream matchmaking_events mm.search_started mm.search_cancelled mm.search_nudge mm.search_timeout mm.match_found mm.match_completed mm.rating_submitted mm.player_banned' \
  'stream voice_events voice.call_incoming voice.call_accepted voice.call_declined voice.call_missed voice.call_ended voice.state_changed voice.screen_share_started voice.screen_share_stopped voice.call_started voice.member_joined' \
  'stream story_events story.created story.viewed story.reacted story.expired story.highlight_created story.lfp_created story.lfp_response' \
  'stream social_events social.friend_request social.friend_accepted social.friend_removed social.user_blocked social.contacts_synced' \
  'stream subscription_events subscription.plan_started subscription.plan_cancelled subscription.plan_expired subscription.downgrade subscription.payment_success subscription.payment_failed subscription.space_pro_started subscription.space_pro_expired subscription.grace_reminder subscription.entitlement_changed' \
  'stream moderation_events moderation.report_created moderation.sanction_applied moderation.appeal_submitted'; do
  grep -Fqx "$stream" "${BOOTSTRAP}"
  grep -Fqx "$stream" "${REALTIME_BOOTSTRAP}"
done
grep -Fqx '  nats-notification-bootstrap:' "${COMPOSE}"
notification_section="$(sed -n '/^  nats-notification-bootstrap:$/,/^  [^ ]/p' "${COMPOSE}")"
printf '%s\n' "${notification_section}" | grep -Fqx '      nats-realtime-bootstrap:'
printf '%s\n' "${notification_section}" | grep -Fqx '        condition: service_completed_successfully'
grep -Fqx 'compose run --rm nats-realtime-bootstrap' "${SMOKE}"
PROD_APPLY="${ROOT}/scripts/prod/apply-infra.sh"
awk '
  /kubectl wait --for=condition=complete job\/voice-nats-realtime-bootstrap/ { central = NR }
  /kubectl delete job voice-nats-notification-bootstrap/ { notification = NR }
  END { exit !(central && notification && central < notification) }
' "${PROD_APPLY}"

# Staging provisions realtime and notification durables in this explicit order
# through the shared helper used by the fresh-install NATS bootstrap.
STAGING_APPLY="${ROOT}/scripts/staging/apply-infra.sh"
grep -Fq 'for bootstrap in realtime notification search analytics-chat; do' "${STAGING_APPLY}"
grep -Fq 'kubectl wait --for=condition=complete "job/voice-nats-${bootstrap}-bootstrap"' "${STAGING_APPLY}"
grep -Fq 'run_nats_bootstrap_jobs' "${STAGING_APPLY}"
for source in message_events_consumer.go matchmaking_events_consumer.go voice_events_consumer.go story_events_consumer.go social_events_consumer.go subscription_events_consumer.go moderation_events_consumer.go; do
  grep -Fq 'bindPreprovisionedConsumer(' "${ROOT}/src/backend/notification/${source}"
done
echo 'NATS Notification bootstrap contract OK'
