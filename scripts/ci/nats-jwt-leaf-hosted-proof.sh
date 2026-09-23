#!/usr/bin/env bash
# Runs only on a Linux hosted runner. It proves the real wire path: an
# unauthenticated app client shares a network namespace with a leaf; the leaf
# authenticates to a JWT-resolver hub over TLS with its one service credential.
set -euo pipefail

if [[ "${GITHUB_ACTIONS:-}" != "true" ]]; then
  echo "SKIP: NATS JWT leaf runtime proof is hosted-only"
  exit 0
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
work="$(mktemp -d)"
network="voice-nats-proof-$RANDOM"
cleanup() {
  docker rm -f voice-nats-proof-hub voice-nats-proof-chat voice-nats-proof-bootstrap-reply >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

cat >"$work/acl.yaml" <<'EOF'
version: 1
services:
  analytics: {publish: [proof.analytics], subscribe: [proof.analytics.delivery]}
  auth: {publish: [proof.auth], subscribe: [proof.auth.delivery]}
  bot: {publish: [proof.bot], subscribe: [proof.bot.delivery]}
  chat:
    publish: [chat.created, '$JS.API.STREAM.INFO.chat_events', '$JS.API.CONSUMER.INFO.chat_events.proof_chat', '$JS.ACK.chat_events.proof_chat.>']
    subscribe: [proof.chat.delivery, _INBOX.voice.chat.proof]
  file: {publish: [proof.file], subscribe: [proof.file.delivery]}
  gateway: {publish: [proof.gateway], subscribe: [proof.gateway.delivery]}
  matchmaking: {publish: [proof.matchmaking], subscribe: [proof.matchmaking.delivery]}
  messaging: {publish: [proof.messaging], subscribe: [proof.messaging.delivery]}
  moderation: {publish: [proof.moderation], subscribe: [proof.moderation.delivery]}
  notification: {publish: [proof.notification], subscribe: [proof.notification.delivery]}
  realtime: {publish: [proof.realtime], subscribe: [proof.realtime.delivery]}
  role: {publish: [proof.role], subscribe: [proof.role.delivery]}
  search: {publish: [proof.search], subscribe: [proof.search.delivery]}
  social: {publish: [proof.social], subscribe: [proof.social.delivery]}
  space: {publish: [proof.space], subscribe: [proof.space.delivery]}
  story: {publish: [proof.story], subscribe: [proof.story.delivery]}
  subscription: {publish: [proof.subscription], subscribe: [proof.subscription.delivery]}
  user: {publish: [proof.user], subscribe: [proof.user.delivery]}
  voice: {publish: [proof.voice], subscribe: [proof.voice.delivery]}
bootstrap:
  publish: ['$JS.API.STREAM.CREATE.chat_events', '$JS.API.STREAM.INFO.chat_events', '$JS.API.CONSUMER.CREATE.chat_events.proof_chat', '$JS.API.CONSUMER.INFO.chat_events.proof_chat']
  subscribe: [_INBOX.voice.bootstrap.reply]
EOF

(cd "$root/src/backend/pkg" && go run ./cmd/nats-jwt-fixture "$work/acl.yaml" "$work/fixture")
account="$(tr -d '\r\n' <"$work/fixture/account.public")"
account_jwt="$(tr -d '\r\n' <"$work/fixture/account.jwt")"
system_account="$(tr -d '\r\n' <"$work/fixture/system-account.public")"
system_account_jwt="$(tr -d '\r\n' <"$work/fixture/system-account.jwt")"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -keyout "$work/key.pem" -out "$work/cert.pem" \
  -subj '/CN=hub' -addext 'subjectAltName=DNS:hub' >/dev/null 2>&1

cat >"$work/hub.conf" <<EOF
operator: $work/fixture/operator.jwt
resolver: MEMORY
resolver_preload: { $account: "$account_jwt", $system_account: "$system_account_jwt" }
system_account: $system_account
jetstream { store_dir: "/tmp/nats-js" }
leafnodes {
  listen: 0.0.0.0:7422
  tls {
    cert_file: "$work/cert.pem"
    key_file: "$work/key.pem"
    ca_file: "$work/cert.pem"
    handshake_first: true
  }
}
EOF
cat >"$work/leaf.conf" <<EOF
listen: 127.0.0.1:4222
leafnodes {
  remotes = [{
    urls: ["nats-leaf://hub:7422"]
    account: "\$G"
    credentials: "$work/fixture/creds/chat.creds"
    tls {
      ca_file: "$work/cert.pem"
      handshake_first: true
    }
  }]
}
EOF

docker network create "$network" >/dev/null
docker run -d --name voice-nats-proof-hub --network "$network" --network-alias hub -v "$work:$work" nats:2.12-alpine -c "$work/hub.conf" >/dev/null
for _ in $(seq 1 30); do docker logs voice-nats-proof-hub 2>&1 | grep -q 'Server is ready' && break; sleep 1; done
if ! docker logs voice-nats-proof-hub 2>&1 | grep -q 'Server is ready'; then
  echo 'FAIL: JWT resolver hub did not become ready' >&2
  docker logs voice-nats-proof-hub >&2
  exit 1
fi

# Only the Job credential may create the stream. Its reply subscription is an
# exact credential-owned subject, never a broad _INBOX wildcard.
docker run -d --name voice-nats-proof-bootstrap-reply --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" sub --count 1 _INBOX.voice.bootstrap.reply >/dev/null
docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" pub --reply _INBOX.voice.bootstrap.reply '$JS.API.STREAM.CREATE.chat_events' '{"name":"chat_events","subjects":["chat.created"],"storage":"file","retention":"limits"}' >/dev/null
docker wait voice-nats-proof-bootstrap-reply >/dev/null

# Bootstrap creates the one fixed proof durable before the application leaf
# joins. The service identity has only INFO and its own ACK namespace; it
# cannot create, update, or delete this durable.
docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" consumer add chat_events proof_chat \
  --filter chat.created --target _INBOX.voice.chat.proof --ack explicit --ack-wait 1s --deliver new --defaults >/dev/null
proof_consumer="$(docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" consumer info chat_events proof_chat --json)"
if [[ "$(jq -r '.config.filter_subject' <<<"$proof_consumer")" != chat.created || "$(jq -r '.config.deliver_subject' <<<"$proof_consumer")" != _INBOX.voice.chat.proof || "$(jq -r '.config.ack_policy' <<<"$proof_consumer")" != explicit ]]; then
  echo 'FAIL: bootstrap did not provision the fixed proof durable exactly' >&2
  exit 1
fi

docker run -d --name voice-nats-proof-chat --network "$network" -v "$work:$work:ro" nats:2.12-alpine -c "$work/leaf.conf" >/dev/null
for attempt in {1..15}; do
  if docker logs voice-nats-proof-chat 2>&1 | grep -Fq 'Server is ready'; then
    break
  fi
  if ! docker inspect --format '{{.State.Running}}' voice-nats-proof-chat 2>/dev/null | grep -qx true; then
    echo 'FAIL: chat leaf exited before becoming ready' >&2
    docker logs voice-nats-proof-chat >&2 || true
    exit 1
  fi
  sleep 1
done
if ! docker logs voice-nats-proof-chat 2>&1 | grep -Fq 'Server is ready'; then
  echo 'FAIL: chat leaf did not become ready within 15 seconds' >&2
  docker logs voice-nats-proof-chat >&2 || true
  exit 1
fi

# This client has no credentials: its only path is the local leaf namespace.
docker run --rm --network container:voice-nats-proof-chat natsio/nats-box:0.18.0 \
  nats --server nats://127.0.0.1:4222 pub chat.created proof >/dev/null

# Core publish success alone is not an authorization or persistence proof. The
# bootstrap-only identity observes the exact JetStream stream state and proves
# that the leaf-authenticated publish was captured as sequence one.
stream_info="$(docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" stream info chat_events --json)"
if [[ "$(jq -r '.state.messages' <<<"$stream_info")" != 1 || "$(jq -r '.state.first_seq' <<<"$stream_info")" != 1 || "$(jq -r '.state.last_seq' <<<"$stream_info")" != 1 ]]; then
  echo 'FAIL: leaf-authenticated chat.created was not captured as stream sequence one' >&2
  exit 1
fi

# Core NATS publish permission failures are asynchronous, so `nats pub` can
# exit successfully after the server has rejected the message. Snapshot the
# leaf log before sending the real neighboring subject, then require its
# permission violation in only the newly appended log output.
denied_subject="user.account_deleted"
leaf_log_before="$(docker logs voice-nats-proof-chat 2>&1 || true)"
hub_log_before="$(docker logs voice-nats-proof-hub 2>&1 || true)"
docker run --rm --network container:voice-nats-proof-chat natsio/nats-box:0.18.0 \
  nats --server nats://127.0.0.1:4222 pub "$denied_subject" denied >/dev/null 2>&1 || true
for attempt in {1..5}; do
  leaf_logs="$(docker logs voice-nats-proof-chat 2>&1 || true)"
  leaf_log_delta="${leaf_logs#"$leaf_log_before"}"
  hub_logs="$(docker logs voice-nats-proof-hub 2>&1 || true)"
  hub_log_delta="${hub_logs#"$hub_log_before"}"
  denial_log_delta="$leaf_log_delta"$'\n'"$hub_log_delta"
  if grep -Eqi 'permission.*(violation|denied)' <<<"$denial_log_delta" && grep -Fq "$denied_subject" <<<"$denial_log_delta"; then
    break
  fi
  sleep 1
done
if ! grep -Eqi 'permission.*(violation|denied)' <<<"$denial_log_delta" || ! grep -Fq "$denied_subject" <<<"$denial_log_delta"; then
  echo 'FAIL: chat leaf or hub did not log denial for neighbouring subject' >&2
  printf '%s\n' "$denial_log_delta" >&2
  exit 1
fi

# Service-side mutation is also rejected by the chat identity.
if docker run --rm --network container:voice-nats-proof-chat natsio/nats-box:0.18.0 nats --server nats://127.0.0.1:4222 req --raw '$JS.API.STREAM.CREATE.denied' '{"name":"denied"}' >/dev/null 2>&1; then
  echo 'FAIL: chat leaf mutated JetStream' >&2; exit 1
fi
# The hub has no anonymous client path; direct unauthenticated access fails.
if docker run --rm --network "$network" natsio/nats-box:0.18.0 nats --server nats://hub:4222 pub chat.created denied >/dev/null 2>&1; then
  echo 'FAIL: direct hub accepted an unauthenticated client' >&2; exit 1
fi
echo 'PASS: hosted JWT resolver + TLS leaf + exact ACL proof'
