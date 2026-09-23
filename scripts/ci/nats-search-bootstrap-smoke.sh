#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT"
PROJECT="voice-nats-search-bootstrap-${GITHUB_RUN_ID:-ci}-$$"
compose() { docker compose -p "$PROJECT" "$@"; }
cleanup() { compose down -v --remove-orphans; }
trap cleanup EXIT

compose up -d --wait nats
compose run --rm nats-search-bootstrap
for spec in \
  'message_events search-indexer-message-v1 message.>' \
  'user_events search-indexer-user-v1 user.>' \
  'chat_events search-indexer-chat-v1 >' \
  'user_profile_projection search-user-profile-projection-v1 user.search_profile_projection'; do
  set -- $spec
  info="$(compose run --rm --no-deps --entrypoint nats nats-search-bootstrap --server nats://nats:4222 req --raw "\$JS.API.CONSUMER.INFO.$1.$2" "")"
  [[ "$(printf '%s' "$info" | jq -r '.config.filter_subject')" == "$3" ]]
  [[ "$(printf '%s' "$info" | jq -r '.config.ack_policy')" == explicit ]]
done

compose run --rm --no-deps --entrypoint nats nats-search-bootstrap --server nats://nats:4222 consumer rm chat_events search-indexer-chat-v1 --force
compose run --rm --no-deps --entrypoint nats nats-search-bootstrap --server nats://nats:4222 consumer add chat_events search-indexer-chat-v1 --filter chat.created --target _INBOX.voice.search.indexer.chat --ack explicit --deliver new --defaults
if compose run --rm nats-search-bootstrap; then
  echo 'expected Search bootstrap to reject consumer filter drift' >&2
  exit 1
fi
echo 'NATS Search bootstrap smoke OK'
