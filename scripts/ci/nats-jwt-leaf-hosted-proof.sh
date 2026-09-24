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
  docker rm -f voice-nats-proof-hub voice-nats-proof-chat voice-nats-proof-chat-noack voice-nats-proof-invalid-jwt voice-nats-proof-wrong-ca voice-nats-proof-wrong-sni voice-nats-proof-bootstrap-reply voice-nats-proof-receive-one voice-nats-proof-receive-two voice-nats-proof-drift-receive voice-nats-proof-noack-receive-one voice-nats-proof-noack-receive-two >/dev/null 2>&1 || true
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
  options := []nats.Option{}
  endpoint := "nats://127.0.0.1:4222"
  if len(os.Args) >= 5 && (os.Args[1] == "direct-noack" || os.Args[1] == "direct-receive" || os.Args[1] == "direct-deny") {
    endpoint = os.Args[3]
    // The fixture exposes its authenticated hub CLIENT listener on plaintext
    // 4222. TLS is deliberately restricted to the 7422 leaf listener, so a
    // direct CLIENT probe must not add a TLS option to this endpoint.
    options = append(options, nats.UserCredentials(os.Args[4]))
  }
  asyncErr := make(chan error, 1)
  options = append(options, nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) { select { case asyncErr <- err: default: } }))
  nc, err := nats.Connect(endpoint, options...)
  if err != nil { panic(err) }
  defer nc.Close()
  if len(os.Args) >= 6 && os.Args[1] == "direct-deny" {
    if err := nc.Publish(os.Args[5], []byte("denied")); err != nil { panic(err) }
    if err := nc.Flush(); err != nil { panic(err) }
    select { case err := <-asyncErr: fmt.Printf("publish_denied=%v\n", err); case <-time.After(2*time.Second): panic("missing publish permission denial") }
    return
  }
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
  if mode == "nak-then-ack" {
    fmt.Printf("ack_subject=%s\n", msg.Reply)
    if err := msg.Nak(); err != nil { panic(err) }
    if err := nc.Flush(); err != nil { panic(err) }
    fmt.Printf("stream=%s sequence=%d delivered=%d ack=false\n", meta.Stream, meta.Sequence.Stream, meta.NumDelivered)
    redelivery, err := sub.NextMsg(10 * time.Second)
    if err != nil { panic(err) }
    redeliveryMeta, err := redelivery.Metadata()
    if err != nil { panic(err) }
    fmt.Printf("stream=%s sequence=%d delivered=%d ack=true\n", redeliveryMeta.Stream, redeliveryMeta.Sequence.Stream, redeliveryMeta.NumDelivered)
    if err := redelivery.Ack(); err != nil { panic(err) }
    if err := nc.Flush(); err != nil { panic(err) }
    return
  }
  if mode == "ack" {
    if err := msg.AckSync(); err != nil { panic(err) }
  }
  if mode == "nak" {
    if err := msg.Nak(); err != nil { panic(err) }
    if err := nc.Flush(); err != nil { panic(err) }
  }
  fmt.Printf("stream=%s sequence=%d delivered=%d ack=%t\n", meta.Stream, meta.Sequence.Stream, meta.NumDelivered, mode == "ack")
  if mode == "deny-ack" {
    fmt.Printf("ack_subject=%s\n", msg.Reply)
    if err := msg.AckSync(nats.AckWait(500 * time.Millisecond)); err == nil {
      panic("AckSync unexpectedly succeeded")
    } else {
      fmt.Printf("ack_error=%v\n", err)
    }
  }
  if mode == "direct-noack" {
    fmt.Printf("ack_subject=%s\n", msg.Reply)
    if err := msg.Ack(); err != nil { panic(err) }
    if err := nc.Flush(); err != nil { panic(err) }
    select {
    case err := <-asyncErr:
      fmt.Printf("ack_error=%v\n", err)
    case <-time.After(2 * time.Second):
      panic("missing ACK publish permission denial")
    }
    fmt.Printf("stream=%s sequence=%d delivered=%d ack=false\n", meta.Stream, meta.Sequence.Stream, meta.NumDelivered)
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
# Compile the receiver once before any readiness barrier. Recompiling it in
# each disposable container makes a subscription-ready assertion depend on
# module-download timing rather than the authenticated leaf path.
mkdir -p "$work/receiver"
(cd "$root/src/backend/pkg" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$work/receiver/leaf-receive" "$work/leaf_receive.go")
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
leafnodes {
  remotes = [{
    urls: ["nats-leaf://hub:7422"]
    account: "\$G"
    credentials: "$work/fixture/creds/chat.creds"
    tls {
      ca_file: "$work/wrong-ca.pem"
      handshake_first: true
    }
  }]
}
EOF
cat >"$work/wrong-sni-leaf.conf" <<EOF
listen: 127.0.0.1:4222
leafnodes {
  remotes = [{
    urls: ["nats-leaf://wrong-sni:7422"]
    account: "\$G"
    credentials: "$work/fixture/creds/chat.creds"
    tls {
      ca_file: "$work/cert.pem"
      handshake_first: true
    }
  }]
}
EOF
sed '0,/eyJ/s/eyJ/eyK/' "$work/fixture/creds/chat.creds" >"$work/invalid-chat.creds"
cat >"$work/invalid-jwt-leaf.conf" <<EOF
listen: 127.0.0.1:4222
leafnodes {
  remotes = [{
    urls: ["nats-leaf://hub:7422"]
    account: "\$G"
    credentials: "$work/invalid-chat.creds"
    tls {
      ca_file: "$work/cert.pem"
      handshake_first: true
    }
  }]
}
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
# cannot create, update, or delete this durable. Use the stable JetStream API
# payload rather than a nats-box CLI flag whose availability varies by image.
create_proof_consumer() {
  docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
    nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply \
    req --raw '$JS.API.CONSUMER.CREATE.chat_events.proof_chat' \
    '{"stream_name":"chat_events","config":{"name":"proof_chat","durable_name":"proof_chat","deliver_subject":"_INBOX.voice.chat.proof","deliver_policy":"new","ack_policy":"explicit","ack_wait":1000000000,"max_deliver":-1,"filter_subject":"chat.created"}}' | \
    jq -e '(.error | not) and .config.name == "proof_chat" and .config.ack_wait == 1000000000 and .config.max_deliver == -1' >/dev/null
}
create_proof_consumer
proof_consumer="$(docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer info chat_events proof_chat --json)"
if [[ "$(jq -r '.config.filter_subject' <<<"$proof_consumer")" != chat.created || "$(jq -r '.config.deliver_subject' <<<"$proof_consumer")" != _INBOX.voice.chat.proof || "$(jq -r '.config.ack_policy' <<<"$proof_consumer")" != explicit || "$(jq -r '.config.max_deliver' <<<"$proof_consumer")" != -1 ]]; then
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

# Subscribe before the service publish, keep the authenticated leaf client
# subscribed across a negative ACK, then receive the same fixed durable
# delivery again and acknowledge it. Keeping one subscription avoids an
# interest-handoff race while proving deterministic redelivery. The client has
# no credentials and can reach NATS only through the chat leaf.
docker run -d --name voice-nats-proof-receive-one --network container:voice-nats-proof-chat \
  -v "$work/receiver:/receiver" alpine:3.22 /receiver/leaf-receive nak-then-ack /receiver/receive-one.ready >/dev/null
if ! wait_for_file "$work/receiver/receive-one.ready" 'first chat leaf receiver'; then
  docker logs voice-nats-proof-receive-one >&2 || true
  exit 1
fi
redelivery_leaf_before="$(docker logs voice-nats-proof-chat 2>&1 || true)"
redelivery_hub_before="$(docker logs voice-nats-proof-hub 2>&1 || true)"

# This client has no credentials: its only path is the local leaf namespace.
docker run --rm --network container:voice-nats-proof-chat natsio/nats-box:0.18.0 \
  nats --server nats://127.0.0.1:4222 pub chat.created proof >/dev/null
docker wait voice-nats-proof-receive-one >/dev/null
receive_one="$(docker logs voice-nats-proof-receive-one 2>&1)"
if ! grep -Fqx 'stream=chat_events sequence=1 delivered=1 ack=false' <<<"$receive_one" || ! grep -Fqx 'stream=chat_events sequence=1 delivered=2 ack=true' <<<"$receive_one"; then
  echo 'FAIL: fixed durable did not NAK, redeliver, then acknowledge the leaf publish' >&2
  printf '%s\n' "$receive_one" >&2
  docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer info chat_events proof_chat --json >&2 || true
  leaf_logs="$(docker logs voice-nats-proof-chat 2>&1 || true)"; hub_logs="$(docker logs voice-nats-proof-hub 2>&1 || true)"
  printf '%s\n%s\n' "${leaf_logs#"$redelivery_leaf_before"}" "${hub_logs#"$redelivery_hub_before"}" >&2
  exit 1
fi
proof_after_ack="$(docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer info chat_events proof_chat --json)"
if ! jq -e '.delivered.consumer_seq == 2 and .delivered.stream_seq == 1 and .ack_floor.consumer_seq == 2 and .ack_floor.stream_seq == 1 and .num_ack_pending == 0' <<<"$proof_after_ack" >/dev/null; then
  echo 'FAIL: hub did not confirm the redelivered proof message acknowledged' >&2
  exit 1
fi

# A bootstrap contract must reject a fixed durable whose delivery target drifts.
# The service has no mutation grant and cannot repair this state itself.
docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer rm chat_events proof_chat --force >/dev/null
docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply \
  req --raw '$JS.API.CONSUMER.CREATE.chat_events.proof_chat' \
  '{"stream_name":"chat_events","config":{"name":"proof_chat","durable_name":"proof_chat","deliver_subject":"_INBOX.voice.chat.drift","deliver_policy":"new","ack_policy":"explicit","ack_wait":1000000000,"max_deliver":-1,"filter_subject":"chat.created"}}' | jq -e '(.error | not) and .config.deliver_subject == "_INBOX.voice.chat.drift"' >/dev/null
if docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer info chat_events proof_chat --json | \
  jq -e '.config.deliver_subject == "_INBOX.voice.chat.proof"' >/dev/null; then
  echo 'FAIL: drifted fixed durable was accepted as the canonical binding' >&2
  exit 1
fi
docker run -d --name voice-nats-proof-drift-receive --network container:voice-nats-proof-chat -v "$work/receiver:/receiver" alpine:3.22 \
  /receiver/leaf-receive ack /receiver/drift-receive.ready >/dev/null
if ! wait_for_file "$work/receiver/drift-receive.ready" 'canonical drift receiver'; then
  docker logs voice-nats-proof-drift-receive >&2 || true
  exit 1
fi
docker run --rm --network container:voice-nats-proof-chat natsio/nats-box:0.18.0 \
  nats --server nats://127.0.0.1:4222 pub chat.created drift-proof >/dev/null
drift_receiver_exit="$(docker wait voice-nats-proof-drift-receive)"
if [[ "$drift_receiver_exit" == 0 ]]; then
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
create_proof_consumer

# Core publish success alone is not an authorization or persistence proof. The
# bootstrap-only identity observes the exact JetStream stream state and proves
# that the leaf-authenticated publish was captured as sequence one.
stream_info="$(docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
  nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply stream info chat_events --json)"
if [[ "$(jq -r '.state.messages' <<<"$stream_info")" != 2 || "$(jq -r '.state.first_seq' <<<"$stream_info")" != 1 || "$(jq -r '.state.last_seq' <<<"$stream_info")" != 2 ]]; then
  echo 'FAIL: leaf-authenticated initial and post-drift publishes were not captured as sequences one and two' >&2
  exit 1
fi

# Direct authenticated client evidence avoids a leaf log-absence heuristic.
denied_subject="user.account_deleted"
denied_result="$(docker run --rm --network "$network" -v "$work/receiver:/receiver" -v "$work:$work:ro" alpine:3.22 /receiver/leaf-receive direct-deny ignored nats://hub:4222 "$work/fixture/creds/chat.creds" "$denied_subject")"
if ! grep -Eqi 'publish_denied=.*permission' <<<"$denied_result"; then
  echo 'FAIL: direct chat credential did not receive broker denial for neighbouring subject' >&2
  printf '%s\n' "$denied_result" >&2
  exit 1
fi

# Service-side mutation is rejected by the authenticated chat CLIENT. A
# client-side broker error is authoritative; leaf and hub log absence is not.
for denied_js_subject in '$JS.API.CONSUMER.CREATE.chat_events.denied' '$JS.API.CONSUMER.INFO.chat_events.neighbour' '$JS.ACK.chat_events.neighbour.1.1.1.1.1' '$JS.API.STREAM.CREATE.denied' '$JS.API.STREAM.UPDATE.chat_events' '$JS.API.STREAM.DELETE.chat_events' '$SYS.REQ.SERVER.PING'; do
  denied_result="$(docker run --rm --network "$network" -v "$work/receiver:/receiver" -v "$work:$work:ro" alpine:3.22 /receiver/leaf-receive direct-deny ignored nats://hub:4222 "$work/fixture/creds/chat.creds" "$denied_js_subject")"
  if ! grep -Eqi 'publish_denied=.*permission' <<<"$denied_result"; then
    echo "FAIL: direct chat credential did not receive broker denial for $denied_js_subject" >&2
    printf '%s\n' "$denied_result" >&2
    exit 1
  fi
done

# The no-ACK identity connects directly as an authenticated hub CLIENT. Leaf
# ACK subjects are exempt from leaf publish checks, so this is the enforceable
# JWT permission boundary for an exact ACK publish denial.
hub_log_before="$(docker logs voice-nats-proof-hub 2>&1 || true)"
docker run -d --name voice-nats-proof-noack-receive-one --network "$network" \
  -v "$work/receiver:/receiver" -v "$work:$work:ro" alpine:3.22 /receiver/leaf-receive direct-noack /receiver/noack-receive-one.ready nats://hub:4222 "$work/fixture/creds/chat-noack.creds" >/dev/null
if ! wait_for_file "$work/receiver/noack-receive-one.ready" 'no-ACK chat leaf receiver'; then
  docker logs voice-nats-proof-noack-receive-one >&2 || true
  exit 1
fi
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
  echo 'FAIL: direct no-ACK identity did not expose its exact proof ACK subject' >&2
  printf '%s\n' "$noack_receive_one" >&2
  exit 1
fi
noack_state="$(docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" --inbox-prefix _INBOX.voice.bootstrap.reply consumer info chat_events proof_chat --json)"
if ! jq -e '.ack_floor.stream_seq == 2 and .num_ack_pending == 1' <<<"$noack_state" >/dev/null; then
  echo 'FAIL: direct no-ACK publish advanced the consumer acknowledgement floor' >&2
  exit 1
fi
for attempt in {1..5}; do
  hub_logs="$(docker logs voice-nats-proof-hub 2>&1 || true)"
  hub_log_delta="${hub_logs#"$hub_log_before"}"
  noack_denial_log_delta="$hub_log_delta"
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
docker run -d --name voice-nats-proof-noack-receive-two --network "$network" \
  -v "$work/receiver:/receiver" -v "$work:$work:ro" alpine:3.22 /receiver/leaf-receive direct-receive /receiver/noack-receive-two.ready nats://hub:4222 "$work/fixture/creds/chat-noack.creds" >/dev/null
if ! wait_for_file "$work/receiver/noack-receive-two.ready" 'no-ACK redelivery receiver'; then
  docker logs voice-nats-proof-noack-receive-two >&2 || true
  exit 1
fi
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
