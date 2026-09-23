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
stream matchmaking_events mm.search_started mm.search_cancelled mm.search_nudge mm.search_timeout mm.match_found mm.match_completed mm.rating_submitted mm.player_banned
stream voice_events 'voice.>'
stream story_events story.created story.viewed story.reacted story.expired story.highlight_created story.lfp_created story.lfp_response
stream social_events social.friend_request social.friend_accepted social.friend_removed social.user_blocked social.contacts_synced
stream subscription_events subscription.entitlement_changed
stream moderation_events moderation.report_created moderation.sanction_applied moderation.appeal_submitted

consumer message_events notif_msg_v2 'message.>' _INBOX.voice.notification.message
consumer matchmaking_events notif_mm 'mm.>' _INBOX.voice.notification.matchmaking
consumer voice_events notif_voice 'voice.>' _INBOX.voice.notification.voice
consumer story_events notif_story 'story.>' _INBOX.voice.notification.story
consumer social_events notif_social 'social.>' _INBOX.voice.notification.social
consumer subscription_events notif_subscription 'subscription.>' _INBOX.voice.notification.subscription
consumer moderation_events notif_mod 'moderation.>' _INBOX.voice.notification.moderation
