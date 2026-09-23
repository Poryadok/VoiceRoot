#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT"
cleanup() { docker compose down -v --remove-orphans; }
trap cleanup EXIT
docker compose up -d --wait nats
docker compose run --rm nats-realtime-bootstrap
for spec in \
  'message_events rt_realtime1_msg message.>' 'chat_events rt_realtime1_chat chat.>' \
  'user_events rt_realtime1_user user.presence_changed' 'social_events rt_realtime1_social social.user_blocked' \
  'role_events rt_realtime1_role role.>' 'voice_events rt_realtime1_voice voice.>' \
  'matchmaking_events rt_realtime1_matchmaking mm.>'; do
  set -- $spec
  info="$(docker compose run --rm --no-deps --entrypoint nats nats-realtime-bootstrap --server nats://nats:4222 consumer info "$1" "$2" --json)"
  printf '%s' "$info" | grep -Fq "\"filter_subject\": \"$3\""
  printf '%s' "$info" | grep -Fq '"ack_policy": "explicit"'
done
echo 'NATS Realtime bootstrap smoke OK'
