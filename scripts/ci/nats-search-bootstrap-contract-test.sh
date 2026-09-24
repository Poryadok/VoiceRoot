#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
BOOTSTRAP="${ROOT}/docker/nats/search-bootstrap.sh"
K8S_BOOTSTRAP="${ROOT}/deploy/templates/nats-search-bootstrap.yaml"
SMOKE="${ROOT}/scripts/ci/nats-search-bootstrap-smoke.sh"
COMPOSE="${ROOT}/docker-compose.yml"
STAGING_APPLY="${ROOT}/scripts/staging/apply-infra.sh"
PROD_APPLY="${ROOT}/scripts/prod/apply-infra.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }
require() { grep -Fqx -- "$1" "$2" || fail "missing exact contract line in ${2#"${ROOT}/"}: $1"; }

[[ -x "$BOOTSTRAP" ]] || fail "Search Compose bootstrap script must be executable"
cmp <(sed -n '/^    #!\/bin\/sh$/,$p' "$K8S_BOOTSTRAP" | sed '/^---$/,$d' | sed 's/^    //') "$BOOTSTRAP" \
  || fail "Search Kubernetes bootstrap script must exactly match the Compose bootstrap script"

grep -Fq '  nats-search-bootstrap:' "$COMPOSE" \
  || fail "Compose must define the Search bootstrap service"
grep -Fq './docker/nats/search-bootstrap.sh:/bootstrap/search-bootstrap.sh:ro' "$COMPOSE" \
  || fail "Compose must mount the Search bootstrap script"
grep -Fq '      nats-realtime-bootstrap:' "$COMPOSE" \
  || fail "Search bootstrap must wait for the canonical shared-stream bootstrap"
grep -Fq '      nats-search-bootstrap:' "$COMPOSE" \
  || fail "Search must wait for its bootstrap service"
for apply in "$STAGING_APPLY" "$PROD_APPLY"; do
  grep -Fq 'voice-nats-search-bootstrap' "$apply" \
    || fail "${apply#"${ROOT}/"} must apply the Search bootstrap Job"
done

for stream in \
  'stream message_events message.sent message.edited message.deleted message.read message.read_receipt_revoked message.reaction_added message.reaction_removed message.mention_added message.pinned message.unpinned message.forwarded message.delivery_ack' \
  'stream user_events user.account_deleted user.account_restored user.guest_converted user.profile_created user.profile_updated user.profile_switched user.verified user.presence_changed user.game_detected user.settings_changed' \
  'stream user_profile_projection user.search_profile_projection'; do
  require "$stream" "$BOOTSTRAP"
done
require 'require_stream_subjects chat_events chat.created chat.member_changed chat.dm_peer_deleted' "$BOOTSTRAP"

for consumer in \
  "consumer message_events search-indexer-message-v1 'message.>' _INBOX.voice.search.indexer.message" \
  "consumer user_events search-indexer-user-v1 'user.>' _INBOX.voice.search.indexer.user" \
  "consumer chat_events search-indexer-chat-v1 '>' _INBOX.voice.search.indexer.chat" \
  'consumer user_profile_projection search-user-profile-projection-v1 user.search_profile_projection _INBOX.voice.search.user-projection'; do
  require "$consumer" "$BOOTSTRAP"
done

for source in \
  "${ROOT}/src/backend/search/internal/indexer/consumer.go" \
  "${ROOT}/src/backend/search/internal/profileprojection/jetstream_consumer.go"; do
  grep -Fq 'jetstreambind.Bind(' "$source" || fail "${source#"${ROOT}/"} must preflight then bind its centrally provisioned durable"
  ! grep -Fq 'AddConsumer(' "$source" || fail "${source#"${ROOT}/"} must not create a consumer"
  ! grep -Fq 'UpdateConsumer(' "$source" || fail "${source#"${ROOT}/"} must not mutate a consumer"
done
grep -Fq 'js.ConsumerInfo(stream, durable)' "${ROOT}/src/backend/search/internal/jetstreambind/preflight.go" \
  || fail "Search preflight must read the provisioned consumer"
grep -Fq 'nats.Bind(stream, durable)' "${ROOT}/src/backend/search/internal/jetstreambind/preflight.go" \
  || fail "Search preflight must bind only after exact validation"
grep -Fq 'nats.MaxReconnects(0)' "${ROOT}/src/backend/search/internal/jetstreambind/preflight.go" \
  || fail "Search must not implicitly reconnect around durable preflight"
grep -Fq 'func Watch(' "${ROOT}/src/backend/search/internal/jetstreambind/preflight.go" \
  || fail "Search must continuously revalidate provisioned durables"

grep -Fq "consumer add user_events search-indexer-user-v1 --filter '>'" "$SMOKE" \
  || fail "Search smoke must prove broad user-filter drift is rejected"

echo 'NATS Search bootstrap contract OK'
