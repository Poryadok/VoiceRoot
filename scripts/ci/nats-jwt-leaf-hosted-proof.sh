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
  docker rm -f voice-nats-proof-hub voice-nats-proof-chat voice-nats-proof-chat-noack voice-nats-proof-invalid-jwt voice-nats-proof-wrong-ca voice-nats-proof-wrong-sni voice-nats-proof-bootstrap-reply voice-nats-proof-receive-one voice-nats-proof-receive-two voice-nats-proof-noack-receive-one voice-nats-proof-noack-receive-two >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

cat >"$work/leaf_receive.go" <<'EOF'
package main

import (
  "fmt"
  "os"
  "time"
  "github.com/nats-io/nats.go"
)

func main() {
  nc, err := nats.Connect("nats://127.0.0.1:4222")
  if err != nil { panic(err) }
  defer nc.Close()
  sub, err := nc.SubscribeSync("_INBOX.voice.chat.proof")
  if err != nil { panic(err) }
  if err := nc.Flush(); err != nil { panic(err) }
  mode := "receive"
  if len(os.Args) >= 2 { mode = os.Args[1] }
  if len(os.Args) >= 3 {
    if err := os.WriteFile(os.Args[2], []byte("ready\n"), 0600); err != nil { panic(err) }
  }
  msg, err := sub.NextMsg(10 * time.Second)
  if err != nil { panic(err) }
  meta, err := msg.Metadata()
  if err != nil { panic(err) }
  if mode == "ack" {
    if err := msg.AckSync(); err != nil { panic(err) }
  }
  fmt.Printf("stream=%s sequence=%d delivered=%d ack=%t\n", meta.Stream, meta.Sequence.Stream, meta.NumDelivered, mode == "ack")
  if mode == "deny-ack" {
    fmt.Printf("ack_subject=%s\n", msg.Reply)
    if err := msg.AckSync(); err == nil {
      panic("AckSync unexpectedly succeeded")
    } else {
      fmt.Printf("ack_error=%v\n", err)
    }
  }
}
EOF

wait_for_file() {
  local path="$1" label="$2"
  for _ in $(seq 1 20); do
    [[ -f "$path" ]] && return 0
    sleep 1
  done
  echo "FAIL: $label did not become ready" >&2
  return 1
}

cat >"$work/acl.yaml" <<'EOF'
version: 1
services:
  analytics: {publish: [proof.analytics], subscribe: [proof.analytics.delivery]}
  auth: {publish: [proof.auth], subscribe: [proof.auth.delivery]}
  bot: {publish: [proof.bot], subscribe: [proof.bot.delivery]}
  chat:
    no_response: true
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
  publish: ['$JS.API.STREAM.CREATE.chat_events', '$JS.API.STREAM.INFO.chat_events', '$JS.API.CONSUMER.CREATE.chat_events.proof_chat', '$JS.API.CONSUMER.DELETE.chat_events.proof_chat', '$JS.API.CONSUMER.INFO.chat_events.proof_chat']
  subscribe: [_INBOX.voice.bootstrap.reply.>]
EOF

(cd "$root/src/backend/pkg" && go run ./cmd/nats-jwt-fixture "$work/acl.yaml" "$work/fixture")
account="$(tr -d '\r\n' <"$work/fixture/account.public")"
account_jwt="$(tr -d '\r\n' <"$work/fixture/account.jwt")"
system_account="$(tr -d '\r\n' <"$work/fixture/system-account.public")"
system_account_jwt="$(tr -d '\r\n' <"$work/fixture/system-account.jwt")"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -keyout "$work/key.pem" -out "$work/cert.pem" \
  -subj '/CN=hub' -addext 'subjectAltName=DNS:hub' >/dev/null 2>&1
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -keyout "$work/wrong-ca-key.pem" -out "$work/wrong-ca.pem" \
  -subj '/CN=wrong-ca' >/dev/null 2>&1

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
cat >"$work/noack-leaf.conf" <<EOF
listen: 127.0.0.1:4222
leafnodes {
  remotes = [{
    urls: ["nats-leaf://hub:7422"]
    account: "\$G"
    credentials: "$work/fixture/creds/chat-noack.creds"
    tls {
      ca_file: "$work/cert.pem"
      handshake_first: true
    }
  }]
}
EOF
cat >"$work/wrong-ca-leaf.conf" <<EOF
listen: 127.0.0.1:4222
leafnodes { remotes = [{ urls: ["nats-leaf://hub:7422"] account: "\$G" credentials: "$work/fixture/creds/chat.creds" tls { ca_file: "$work/wrong-ca.pem"; handshake_first: true } }] }
EOF
cat >"$work/wrong-sni-leaf.conf" <<EOF
listen: 127.0.0.1:4222
leafnodes { remotes = [{ urls: ["nats-leaf://wrong-sni:7422"] account: "\$G" credentials: "$work/fixture/creds/chat.creds" tls { ca_file: "$work/cert.pem"; handshake_first: true } }] }
EOF
sed '0,/eyJ/s/eyJ/eyK/' "$work/fixture/creds/chat.creds" >"$work/invalid-chat.creds"
cat >"$work/invalid-jwt-leaf.conf" <<EOF
listen: 127.0.0.1:4222
leafnodes { remotes = [{ urls: ["nats-leaf://hub:7422"] account: "\$G" credentials: "$work/invalid-chat.creds" tls { ca_file: "$work/cert.pem"; handshake_first: true } }] }
EOF

docker network create "$network" >/dev/null
docker run -d --name voice-nats-proof-hub --network "$network" --network-alias hub --network-alias wrong-sni -v "$work:$work" nats:2.12-alpine -c "$work/hub.conf" >/dev/null
for _ in $(seq 1 30); do docker logs voice-nats-proof-hub 2>&1 | grep -q 'Server is ready' && break; sleep 1; done
if ! docker logs voice-nats-proof-hub 2>&1 | grep -q 'Server is ready'; then
  echo 'FAIL: JWT resolver hub did not become ready' >&2
  docker logs voice-nats-proof-hub >&2
  exit 1
fi

# A valid service credential with an unrelated CA must never join the hub.
wrong_ca_before=""
docker run -d --name voice-nats-proof-wrong-ca --network "$network" -v "$work:$work:ro" nats:2.12-alpine -c "$work/wrong-ca-leaf.conf" >/dev/null
for _ in $(seq 1 8); do
  wrong_ca_logs="$(docker logs voice-nats-proof-wrong-ca 2>&1 || true)"
  wrong_ca_delta="${wrong_ca_logs#"$wrong_ca_before"}"
  grep -Eqi '(certificate|tls|x509).*(unknown|verify|failed|error)|tls.*(unknown|verify|failed|error)' <<<"$wrong_ca_delta" && break
  sleep 1
done
if ! grep -Eqi '(certificate|tls|x509).*(unknown|verify|failed|error)|tls.*(unknown|verify|failed|error)' <<<"${wrong_ca_delta:-}"; then
  echo 'FAIL: wrong-CA leaf did not report a fresh TLS join rejection' >&2
  printf '%s\n' "${wrong_ca_delta:-}" >&2
  exit 1
fi
docker rm -f voice-nats-proof-wrong-ca >/dev/null

# This leaf trusts the issuing CA and has valid service credentials, but its
# URL hostname is deliberately absent from the hub certificate SAN.
wrong_sni_before=""
docker run -d --name voice-nats-proof-wrong-sni --network "$network" -v "$work:$work:ro" nats:2.12-alpine -c "$work/wrong-sni-leaf.conf" >/dev/null
for _ in $(seq 1 8); do
  wrong_sni_logs="$(docker logs voice-nats-proof-wrong-sni 2>&1 || true)"
  wrong_sni_delta="${wrong_sni_logs#"$wrong_sni_before"}"
  grep -Eqi '(certificate|tls|x509).*(name|hostname|verify|failed|error)|tls.*(name|hostname|verify|failed|error)' <<<"$wrong_sni_delta" && break
  sleep 1
done
if ! grep -Eqi '(certificate|tls|x509).*(name|hostname|verify|failed|error)|tls.*(name|hostname|verify|failed|error)' <<<"${wrong_sni_delta:-}"; then
  echo 'FAIL: wrong-SNI leaf did not report a fresh TLS name rejection' >&2
  printf '%s\n' "${wrong_sni_delta:-}" >&2
  exit 1
fi
docker rm -f voice-nats-proof-wrong-sni >/dev/null

# Only the service JWT is corrupt here: CA and SNI are valid, so the fresh
# failure must be authentication rather than a transport fallback.
invalid_jwt_before=""
docker run -d --name voice-nats-proof-invalid-jwt --network "$network" -v "$work:$work:ro" nats:2.12-alpine -c "$work/invalid-jwt-leaf.conf" >/dev/null
for _ in $(seq 1 8); do
  invalid_jwt_logs="$(docker logs voice-nats-proof-invalid-jwt 2>&1 || true)"
  invalid_jwt_delta="${invalid_jwt_logs#"$invalid_jwt_before"}"
  grep -Eqi '(authorization|authentication|jwt|signature).*(denied|invalid|failed|error)|authorization.*violation' <<<"$invalid_jwt_delta" && break
  sleep 1
done
if ! grep -Eqi '(authorization|authentication|jwt|signature).*(denied|invalid|failed|error)|authorization.*violation' <<<"${invalid_jwt_delta:-}"; then
  echo 'FAIL: invalid-JWT leaf did not report a fresh authentication rejection' >&2
  printf '%s\n' "${invalid_jwt_delta:-}" >&2
  exit 1
fi
docker rm -f voice-nats-proof-invalid-jwt >/dev/null

# Only the Job credential may create the stream. Its reply subscription is an
# exact credential-owned subject, never a broad _INBOX wildcard.
bootstrap_reply="_INBOX.voice.bootstrap.reply.stream_create"
docker run -d --name voice-nats-proof-bootstrap-reply --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" sub --count 1 "$bootstrap_reply" >/dev/null
docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply pub --reply "$bootstrap_reply" '$JS.API.STREAM.CREATE.chat_events' '{"name":"chat_events","subjects":["chat.created"],"storage":"file","retention":"limits"}' >/dev/null
docker wait voice-nats-proof-bootstrap-reply >/dev/null
if ! docker logs voice-nats-proof-bootstrap-reply 2>&1 | grep -Fq 'chat_events'; then
  echo 'FAIL: bootstrap stream-create reply was not received on the scoped child inbox' >&2
  exit 1
fi

# Bootstrap creates the one fixed proof durable before the application leaf
# joins. The service identity has only INFO and its own ACK namespace; it
# cannot create, update, or delete this durable.
docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer add chat_events proof_chat \
  --filter chat.created --target _INBOX.voice.chat.proof --ack explicit --ack-wait 1s --deliver new --defaults >/dev/null
proof_consumer="$(docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer info chat_events proof_chat --json)"
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

# Subscribe before the service publish, deliberately omit the first ACK, then
# receive the same fixed durable delivery again and acknowledge it. The client
# has no credentials and can reach NATS only through the chat leaf namespace.
docker run -d --name voice-nats-proof-receive-one --network container:voice-nats-proof-chat \
  -v "$root/src/backend/pkg:/repo:ro" -v "$work:$work:ro" golang:1.24-alpine \
  sh -ec "cd /repo && go run $work/leaf_receive.go receive $work/receive-one.ready" >/dev/null
wait_for_file "$work/receive-one.ready" 'first chat leaf receiver'

# This client has no credentials: its only path is the local leaf namespace.
docker run --rm --network container:voice-nats-proof-chat natsio/nats-box:0.18.0 \
  nats --server nats://127.0.0.1:4222 pub chat.created proof >/dev/null
docker wait voice-nats-proof-receive-one >/dev/null
receive_one="$(docker logs voice-nats-proof-receive-one 2>&1)"
if ! grep -Fqx 'stream=chat_events sequence=1 delivered=1 ack=false' <<<"$receive_one"; then
  echo 'FAIL: fixed durable did not receive the first leaf publish unacknowledged' >&2
  printf '%s\n' "$receive_one" >&2
  exit 1
fi
docker run -d --name voice-nats-proof-receive-two --network container:voice-nats-proof-chat \
  -v "$root/src/backend/pkg:/repo:ro" -v "$work:$work:ro" golang:1.24-alpine \
  sh -ec "cd /repo && go run $work/leaf_receive.go ack $work/receive-two.ready" >/dev/null
wait_for_file "$work/receive-two.ready" 'second chat leaf receiver'
docker wait voice-nats-proof-receive-two >/dev/null
receive_two="$(docker logs voice-nats-proof-receive-two 2>&1)"
if ! grep -Fqx 'stream=chat_events sequence=1 delivered=2 ack=true' <<<"$receive_two"; then
  echo 'FAIL: fixed durable did not redeliver then acknowledge the leaf publish' >&2
  printf '%s\n' "$receive_two" >&2
  exit 1
fi

# A bootstrap contract must reject a fixed durable whose delivery target drifts.
# The service has no mutation grant and cannot repair this state itself.
docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer rm chat_events proof_chat --force >/dev/null
docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer add chat_events proof_chat \
  --filter chat.created --target _INBOX.voice.chat.drift --ack explicit --deliver new --defaults >/dev/null
if docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer info chat_events proof_chat --json | \
  jq -e '.config.deliver_subject == "_INBOX.voice.chat.proof"' >/dev/null; then
  echo 'FAIL: drifted fixed durable was accepted as the canonical binding' >&2
  exit 1
fi
docker run -d --name voice-nats-proof-drift-receive --network container:voice-nats-proof-chat -v "$root/src/backend/pkg:/repo:ro" -v "$work:$work:ro" golang:1.24-alpine \
  sh -ec "cd /repo && go run $work/leaf_receive.go ack $work/drift-receive.ready" >/dev/null
wait_for_file "$work/drift-receive.ready" 'canonical drift receiver'
docker run --rm --network container:voice-nats-proof-chat natsio/nats-box:0.18.0 \
  nats --server nats://127.0.0.1:4222 pub chat.created drift-proof >/dev/null
if docker wait voice-nats-proof-drift-receive >/dev/null 2>&1; then
  echo 'FAIL: leaf consumed a drifted durable through the canonical target' >&2
  exit 1
fi
drift_info="$(docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer info chat_events proof_chat --json)"
drift_stream="$(docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply stream info chat_events --json)"
if [[ "$(jq -r '.config.deliver_subject' <<<"$drift_info")" != _INBOX.voice.chat.drift || "$(jq -r '.state.last_seq' <<<"$drift_stream")" != 2 ]]; then
  echo 'FAIL: post-drift event was not captured exclusively by the drifted durable state' >&2
  exit 1
fi
docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer rm chat_events proof_chat --force >/dev/null
docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer add chat_events proof_chat \
  --filter chat.created --target _INBOX.voice.chat.proof --ack explicit --ack-wait 1s --deliver new --defaults >/dev/null

# Core publish success alone is not an authorization or persistence proof. The
# bootstrap-only identity observes the exact JetStream stream state and proves
# that the leaf-authenticated publish was captured as sequence one.
stream_info="$(docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply stream info chat_events --json)"
if [[ "$(jq -r '.state.messages' <<<"$stream_info")" != 2 || "$(jq -r '.state.first_seq' <<<"$stream_info")" != 1 || "$(jq -r '.state.last_seq' <<<"$stream_info")" != 2 ]]; then
  echo 'FAIL: leaf-authenticated initial and post-drift publishes were not captured as sequences one and two' >&2
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
for denied_js_subject in '$JS.API.CONSUMER.CREATE.chat_events.denied' '$JS.API.CONSUMER.INFO.chat_events.neighbour' '$JS.ACK.chat_events.neighbour.1.1.1.1.1' '$JS.API.STREAM.CREATE.denied' '$JS.API.STREAM.UPDATE.chat_events' '$JS.API.STREAM.DELETE.chat_events' '$SYS.REQ.SERVER.PING'; do
  leaf_log_before="$(docker logs voice-nats-proof-chat 2>&1 || true)"
  hub_log_before="$(docker logs voice-nats-proof-hub 2>&1 || true)"
  docker run --rm --network container:voice-nats-proof-chat natsio/nats-box:0.18.0 nats --server nats://127.0.0.1:4222 pub "$denied_js_subject" denied >/dev/null 2>&1 || true
  for _ in $(seq 1 8); do
    leaf_logs="$(docker logs voice-nats-proof-chat 2>&1 || true)"
    hub_logs="$(docker logs voice-nats-proof-hub 2>&1 || true)"
    denial_log_delta="${leaf_logs#"$leaf_log_before"}"$'\n'"${hub_logs#"$hub_log_before"}"
    grep -Fq "$denied_js_subject" <<<"$denial_log_delta" && grep -Eqi 'permission.*(violation|denied)' <<<"$denial_log_delta" && break
    sleep 1
  done
  if ! grep -Fq "$denied_js_subject" <<<"$denial_log_delta" || ! grep -Eqi 'permission.*(violation|denied)' <<<"$denial_log_delta"; then
    echo "FAIL: chat leaf did not freshly deny $denied_js_subject" >&2; exit 1
  fi
done

# The no-ACK chat identity has the same proof read rights but no permission to
# publish an acknowledgement. It must receive a new fixed-durable delivery,
# fail AckSync on the exact server-generated ACK subject, and leave that
# message unacknowledged for redelivery.
docker run -d --name voice-nats-proof-chat-noack --network "$network" -v "$work:$work:ro" nats:2.12-alpine -c "$work/noack-leaf.conf" >/dev/null
for attempt in {1..15}; do
  if docker logs voice-nats-proof-chat-noack 2>&1 | grep -Fq 'Server is ready'; then
    break
  fi
  if ! docker inspect --format '{{.State.Running}}' voice-nats-proof-chat-noack 2>/dev/null | grep -qx true; then
    echo 'FAIL: no-ACK chat leaf exited before becoming ready' >&2
    docker logs voice-nats-proof-chat-noack >&2 || true
    exit 1
  fi
  sleep 1
done
if ! docker logs voice-nats-proof-chat-noack 2>&1 | grep -Fq 'Server is ready'; then
  echo 'FAIL: no-ACK chat leaf did not become ready within 15 seconds' >&2
  docker logs voice-nats-proof-chat-noack >&2 || true
  exit 1
fi

noack_leaf_log_before="$(docker logs voice-nats-proof-chat-noack 2>&1 || true)"
hub_log_before="$(docker logs voice-nats-proof-hub 2>&1 || true)"
docker run -d --name voice-nats-proof-noack-receive-one --network container:voice-nats-proof-chat-noack \
  -v "$root/src/backend/pkg:/repo:ro" -v "$work:$work:ro" golang:1.24-alpine \
  sh -ec "cd /repo && go run $work/leaf_receive.go deny-ack $work/noack-receive-one.ready" >/dev/null
wait_for_file "$work/noack-receive-one.ready" 'no-ACK chat leaf receiver'
docker run --rm --network container:voice-nats-proof-chat natsio/nats-box:0.18.0 \
  nats --server nats://127.0.0.1:4222 pub chat.created noack-proof >/dev/null
docker wait voice-nats-proof-noack-receive-one >/dev/null
noack_receive_one="$(docker logs voice-nats-proof-noack-receive-one 2>&1)"
if ! grep -Fqx 'stream=chat_events sequence=3 delivered=1 ack=false' <<<"$noack_receive_one"; then
  echo 'FAIL: no-ACK identity did not receive the new fixed durable delivery' >&2
  printf '%s\n' "$noack_receive_one" >&2
  exit 1
fi
noack_ack_subject="$(sed -n 's/^ack_subject=//p' <<<"$noack_receive_one")"
if [[ ! "$noack_ack_subject" =~ ^\$JS\.ACK\.chat_events\.proof_chat\.[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]] || ! grep -Eq '^ack_error=.+$' <<<"$noack_receive_one"; then
  echo 'FAIL: no-ACK identity did not report denial for its exact proof ACK subject' >&2
  printf '%s\n' "$noack_receive_one" >&2
  exit 1
fi
for attempt in {1..5}; do
  noack_leaf_logs="$(docker logs voice-nats-proof-chat-noack 2>&1 || true)"
  noack_leaf_log_delta="${noack_leaf_logs#"$noack_leaf_log_before"}"
  hub_logs="$(docker logs voice-nats-proof-hub 2>&1 || true)"
  hub_log_delta="${hub_logs#"$hub_log_before"}"
  noack_denial_log_delta="$noack_leaf_log_delta"$'\n'"$hub_log_delta"
  if grep -Fq "$noack_ack_subject" <<<"$noack_denial_log_delta" && grep -Eqi 'permission.*(violation|denied)' <<<"$noack_denial_log_delta"; then
    break
  fi
  sleep 1
done
if ! grep -Fq "$noack_ack_subject" <<<"${noack_denial_log_delta:-}" || ! grep -Eqi 'permission.*(violation|denied)' <<<"${noack_denial_log_delta:-}"; then
  echo 'FAIL: no-ACK AckSync denial lacked fresh broker evidence for the exact ACK subject' >&2
  printf '%s\n' "${noack_denial_log_delta:-}" >&2
  exit 1
fi
docker run -d --name voice-nats-proof-noack-receive-two --network container:voice-nats-proof-chat-noack \
  -v "$root/src/backend/pkg:/repo:ro" -v "$work:$work:ro" golang:1.24-alpine \
  sh -ec "cd /repo && go run $work/leaf_receive.go receive $work/noack-receive-two.ready" >/dev/null
wait_for_file "$work/noack-receive-two.ready" 'no-ACK redelivery receiver'
docker wait voice-nats-proof-noack-receive-two >/dev/null
noack_receive_two="$(docker logs voice-nats-proof-noack-receive-two 2>&1)"
if ! grep -Fqx 'stream=chat_events sequence=3 delivered=2 ack=false' <<<"$noack_receive_two"; then
  echo 'FAIL: denied no-ACK delivery was not redelivered unacknowledged' >&2
  printf '%s\n' "$noack_receive_two" >&2
  exit 1
fi

# The hub has no anonymous client path; direct unauthenticated access fails.
hub_log_before="$(docker logs voice-nats-proof-hub 2>&1 || true)"
if docker run --rm --network "$network" natsio/nats-box:0.18.0 nats --server nats://hub:4222 pub chat.created denied >/dev/null 2>&1; then
  echo 'FAIL: direct hub accepted an unauthenticated client' >&2; exit 1
fi
for _ in $(seq 1 8); do
  hub_log_delta="$(docker logs voice-nats-proof-hub 2>&1 || true)"
  hub_log_delta="${hub_log_delta#"$hub_log_before"}"
  grep -Eqi '(authorization|authentication).*(violation|denied|failed|required)|authentication.*error' <<<"$hub_log_delta" && break
  sleep 1
done
if ! grep -Eqi '(authorization|authentication).*(violation|denied|failed|required)|authentication.*error' <<<"$hub_log_delta"; then
  echo 'FAIL: direct unauthenticated hub rejection lacked fresh hub evidence' >&2
  printf '%s\n' "$hub_log_delta" >&2
  exit 1
fi
echo 'PASS: hosted JWT resolver + TLS leaf + exact ACL proof'
