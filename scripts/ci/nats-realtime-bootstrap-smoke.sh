#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT"
PROJECT="voice-nats-bootstrap-${GITHUB_RUN_ID:-ci}-$$"
compose() { docker compose -p "$PROJECT" "$@"; }
cleanup() { compose down -v --remove-orphans; }
trap cleanup EXIT
compose up -d --wait nats
compose run --rm nats-realtime-bootstrap
for spec in \
  'message_events rt_realtime1_msg message.>' 'chat_events rt_realtime1_chat chat.>' \
  'user_events rt_realtime1_user user.presence_changed' 'social_events rt_realtime1_social social.user_blocked' \
  'role_events rt_realtime1_role role.>' 'voice_events rt_realtime1_voice voice.>' \
  'matchmaking_events rt_realtime1_matchmaking mm.>'; do
  set -- $spec
  info="$(compose run --rm --no-deps --entrypoint nats nats-realtime-bootstrap --server nats://nats:4222 req --raw "\$JS.API.CONSUMER.INFO.$1.$2" "")"
  [[ "$(printf '%s' "$info" | jq -r '.config.filter_subject')" == "$3" ]]
  [[ "$(printf '%s' "$info" | jq -r '.config.ack_policy')" == explicit ]]
done
compose run --rm --no-deps --entrypoint nats nats-realtime-bootstrap --server nats://nats:4222 consumer rm role_events rt_realtime1_role --force
compose run --rm --no-deps --entrypoint nats nats-realtime-bootstrap --server nats://nats:4222 consumer add role_events rt_realtime1_role --filter role.created --target _INBOX.voice.realtime1.role --ack explicit --deliver new --defaults
if compose run --rm nats-realtime-bootstrap; then
  echo 'expected bootstrap to reject broad consumer drift' >&2
  exit 1
fi
echo 'NATS Realtime bootstrap smoke OK'
