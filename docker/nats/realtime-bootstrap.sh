#!/bin/sh
set -eu

nats_url="${NATS_URL:?NATS_URL is required}"
stream() {
  name="$1"; shift
  subjects="$(IFS=,; echo "$*")"
  expected="$(printf '%s\n' "$@" | jq -R . | jq -sc 'sort')"
  if info="$(nats --server "$nats_url" req --raw "\$JS.API.STREAM.INFO.$name" "" 2>&1)"; then
    if printf '%s' "$info" | jq -e '.error' >/dev/null; then
      printf '%s' "$info" | jq -r '.error.description' | grep -qi 'stream not found' || { echo "$info" >&2; exit 1; }
    else
      actual="$(printf '%s' "$info" | jq -c '.config.subjects | sort')"
      [ "$actual" = "$expected" ] || { echo "incompatible subjects for stream $name" >&2; exit 1; }
      return
    fi
  else
    echo "$info" >&2; exit 1
  fi
  nats --server "$nats_url" stream add "$name" --subjects "$subjects" --storage file --retention limits --max-age 7d --defaults
}
consumer() {
  stream_name="$1"; durable="$2"; filter="$3"; target="$4"
  if info="$(nats --server "$nats_url" req --raw "\$JS.API.CONSUMER.INFO.$stream_name.$durable" "" 2>&1)"; then
    if printf '%s' "$info" | jq -e '.error' >/dev/null; then
      printf '%s' "$info" | jq -r '.error.description' | grep -qi 'consumer not found' || { echo "$info" >&2; exit 1; }
    else
      actual="$(printf '%s' "$info" | jq -c '[.config.filter_subject, .config.deliver_subject, .config.ack_policy, .config.deliver_policy]')"
      expected="$(jq -cn --arg filter "$filter" --arg target "$target" '[ $filter, $target, "explicit", "new" ]')"
      [ "$actual" = "$expected" ] || { echo "incompatible consumer $stream_name/$durable" >&2; exit 1; }
      return
    fi
  else
    echo "$info" >&2; exit 1
  fi
  nats --server "$nats_url" consumer add "$stream_name" "$durable" --filter "$filter" --target "$target" --ack explicit --deliver new --defaults
}
stream message_events message.sent message.edited message.deleted message.read message.read_receipt_revoked message.reaction_added message.reaction_removed message.mention_added message.pinned message.unpinned message.forwarded message.delivery_ack
stream chat_events chat.created chat.member_changed chat.dm_peer_deleted space.tree_changed space.created voice.room_created voice.room_deleted space.invite_created space.member_joined space.member_left space.updated space.deleted
stream file_events file.uploaded file.processed file.scan_infected file.expired file.downloaded
stream moderation_events moderation.report_created moderation.sanction_applied moderation.appeal_submitted
stream bot_events bot.registered bot.command_executed bot.webhook_delivered bot.webhook_failed
stream subscription_events subscription.plan_started subscription.plan_cancelled subscription.plan_expired subscription.downgrade subscription.payment_success subscription.payment_failed subscription.space_pro_started subscription.space_pro_expired subscription.grace_reminder
stream story_events story.created story.viewed story.reacted story.expired story.highlight_created story.lfp_created story.lfp_response
stream user_events user.account_deleted user.profile_created user.profile_updated user.profile_switched user.verified user.presence_changed user.game_detected user.settings_changed
stream social_events social.friend_request social.friend_accepted social.friend_removed social.user_blocked social.contacts_synced
stream role_events role.created role.updated role.deleted role.assigned role.revoked role.chat_override_set role.chat_override_removed role.voice_override_set role.voice_override_removed
stream voice_events 'voice.>'
stream matchmaking_events mm.search_started mm.search_cancelled mm.search_nudge mm.search_timeout mm.match_found mm.match_completed mm.rating_submitted mm.player_banned
consumer message_events rt_realtime1_msg 'message.>' _INBOX.voice.realtime1.message
consumer chat_events rt_realtime1_chat 'chat.>' _INBOX.voice.realtime1.chat
consumer user_events rt_realtime1_user user.presence_changed _INBOX.voice.realtime1.user
consumer social_events rt_realtime1_social social.user_blocked _INBOX.voice.realtime1.social
consumer role_events rt_realtime1_role 'role.>' _INBOX.voice.realtime1.role
consumer voice_events rt_realtime1_voice 'voice.>' _INBOX.voice.realtime1.voice
consumer matchmaking_events rt_realtime1_matchmaking 'mm.>' _INBOX.voice.realtime1.matchmaking
