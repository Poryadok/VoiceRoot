#!/bin/sh
set -eu

nats_url="${NATS_URL:?NATS_URL is required}"

nats() {
  if [ -n "${NATS_CREDS:-}" ]; then command nats --creds "$NATS_CREDS" --inbox-prefix _INBOX.voice.bootstrap.reply "$@"; else command nats "$@"; fi
}

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

# chat_events is a shared Chat+Space stream. Its canonical full subject shape
# belongs to the central bootstrap; Search only requires its Chat inputs.
require_stream_subjects() {
  name="$1"; shift
  info="$(nats --server "$nats_url" req --raw "\$JS.API.STREAM.INFO.$name" "" 2>&1)" || { echo "$info" >&2; exit 1; }
  if printf '%s' "$info" | jq -e '.error' >/dev/null; then
    echo "$info" >&2; exit 1
  fi
  actual="$(printf '%s' "$info" | jq -c '.config.subjects | sort')"
  for subject in "$@"; do
    printf '%s' "$actual" | jq -e --arg subject "$subject" 'index($subject) != null' >/dev/null \
      || { echo "missing required subject $subject in stream $name" >&2; exit 1; }
  done
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
  payload="$(jq -cn --arg stream "$stream_name" --arg durable "$durable" --arg filter "$filter" --arg target "$target" '{stream_name: $stream, action: "create", config: {name: $durable, durable_name: $durable, filter_subject: $filter, deliver_subject: $target, deliver_policy: "new", ack_policy: "explicit"}}')"
  result="$(nats --server "$nats_url" req --raw "\$JS.API.CONSUMER.CREATE.$stream_name.$durable" "$payload")"
  printf '%s' "$result" | jq -e --arg durable "$durable" '(.error | not) and .config.durable_name == $durable' >/dev/null || { echo "$result" >&2; exit 1; }
}

stream message_events message.sent message.edited message.deleted message.read message.read_receipt_revoked message.reaction_added message.reaction_removed message.mention_added message.pinned message.unpinned message.forwarded message.delivery_ack
stream user_events user.account_deleted user.profile_created user.profile_updated user.profile_switched user.verified user.presence_changed user.game_detected user.settings_changed
require_stream_subjects chat_events chat.created chat.member_changed chat.dm_peer_deleted
stream user_profile_projection user.search_profile_projection

consumer message_events search-indexer-message-v1 'message.>' _INBOX.voice.search.indexer.message
consumer user_events search-indexer-user-v1 'user.>' _INBOX.voice.search.indexer.user
consumer chat_events search-indexer-chat-v1 '>' _INBOX.voice.search.indexer.chat
consumer user_profile_projection search-user-profile-projection-v1 user.search_profile_projection _INBOX.voice.search.user-projection
