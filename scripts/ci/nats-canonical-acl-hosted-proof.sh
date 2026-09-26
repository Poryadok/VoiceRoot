#!/usr/bin/env bash
# Prove the checked-in ACL against a disposable JWT/TLS hub and all four Jobs.
set -euo pipefail

if [[ "${GITHUB_ACTIONS:-}" != true ]]; then
  echo 'SKIP: canonical NATS ACL proof is hosted-only'
  exit 0
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
work="$(mktemp -d)"
umask 077
network="voice-nats-canonical-${RANDOM}"
cleanup() {
  docker rm -f voice-nats-canonical-hub voice-nats-canonical-chat voice-nats-canonical-bad-leaf >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

openssl req -x509 -newkey rsa:2048 -nodes -days 1 -keyout "$work/key.pem" -out "$work/cert.pem" \
  -subj '/CN=voice-nats' -addext 'subjectAltName=DNS:voice-nats,DNS:hub' >/dev/null 2>&1
(cd "$root/src/backend/pkg" && go run ./cmd/nats-jwt-issuer --namespace voice-staging \
  --tls-cert "$work/cert.pem" --tls-key "$work/key.pem" --tls-ca "$work/cert.pem" \
  "$root/deploy/nats/acl-intent.yaml" "$work/issued")
bundle_bytes="$(gzip -c "$work/issued/secrets.json" | base64 -w 0 | wc -c)"
echo "NATS restore bundle gzip+base64 bytes: $bundle_bytes"
if (( bundle_bytes > 49152 )); then
  echo 'FAIL: NATS restore bundle exceeds GitHub Environment secret limit' >&2
  exit 1
fi
mkdir -m 0700 -p "$work/fixture/creds"
extract_secret_key() {
  local secret="$1" key="$2" target="$3"
  jq -er --arg secret "$secret" --arg key "$key" \
    '.items[] | select(.metadata.name == $secret) | .data[$key]' "$work/issued/secrets.json" |
    base64 -d >"$target"
}
for key in operator.jwt account.jwt system-account.jwt account.public system-account.public; do
  extract_secret_key voice-nats-operator "$key" "$work/fixture/$key"
done
extract_secret_key voice-nats-bootstrap-credentials bootstrap.creds "$work/fixture/creds/bootstrap.creds"
bootstrap_jwt_bytes="$(awk '/^-----BEGIN NATS USER JWT-----$/{getline; print length($0); exit}' "$work/fixture/creds/bootstrap.creds")"
if [[ ! "$bootstrap_jwt_bytes" =~ ^[0-9]+$ ]] || (( bootstrap_jwt_bytes == 0 || bootstrap_jwt_bytes + 2048 >= 32768 )); then
  echo 'FAIL: canonical bootstrap JWT leaves insufficient CONNECT-line headroom' >&2
  exit 1
fi
echo "NATS bootstrap JWT bytes: $bootstrap_jwt_bytes"
for service in analytics auth bot chat file gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice; do
  extract_secret_key voice-nats-service-credentials "$service.creds" "$work/fixture/creds/$service.creds"
done
for job in realtime notification search analytics-chat; do
  sed -n '/^    #!\/bin\/sh$/,/^---$/p' "$root/deploy/templates/nats-${job}-bootstrap.yaml" |
    sed '/^---$/d;s/^    //' >"$work/${job}.sh"
  sh -n "$work/${job}.sh"
done

account="$(tr -d '\r\n' <"$work/fixture/account.public")"
account_jwt="$(tr -d '\r\n' <"$work/fixture/account.jwt")"
system_account="$(tr -d '\r\n' <"$work/fixture/system-account.public")"
system_account_jwt="$(tr -d '\r\n' <"$work/fixture/system-account.jwt")"

cat >"$work/hub.conf" <<EOF
operator: $work/fixture/operator.jwt
max_control_line: 32768
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
cat >"$work/chat-leaf.conf" <<EOF
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
docker run -d --name voice-nats-canonical-hub --network "$network" --network-alias hub \
  -v "$work:$work" nats:2.12-alpine -c "$work/hub.conf" >/dev/null
for _ in $(seq 1 30); do
  docker logs voice-nats-canonical-hub 2>&1 | grep -q 'Server is ready' && break
  sleep 1
done
docker logs voice-nats-canonical-hub 2>&1 | grep -q 'Server is ready' || {
  docker logs voice-nats-canonical-hub >&2; exit 1;
}

# Run the same ConfigMap scripts that staging/prod mount, using only the Job JWT.
for job in realtime notification search analytics-chat; do
  echo "bootstrap job starting: $job"
  if docker run --rm --network "$network" -v "$work:$work" \
    -e NATS_URL=nats://hub:4222 -e NATS_CREDS="$work/fixture/creds/bootstrap.creds" \
    natsio/nats-box:0.18.0 sh "$work/${job}.sh" </dev/null; then
    echo "bootstrap job ready: $job"
  else
    status=$?
    echo "FAIL: bootstrap job $job exited $status" >&2
    exit "$status"
  fi
done

bootstrap_info() {
  docker run --rm --network "$network" -v "$work:$work:ro" natsio/nats-box:0.18.0 \
    nats --server nats://hub:4222 --creds "$work/fixture/creds/bootstrap.creds" \
    --inbox-prefix _INBOX.voice.bootstrap.reply req --raw "$1" ''
}
matchmaking_info="$(bootstrap_info '$JS.API.CONSUMER.INFO.story_events.matchmaking_story_lfp_v2')" || {
  echo 'FAIL: matchmaking fixed consumer INFO request' >&2; exit 1;
}
printf '%s' "$matchmaking_info" |
  jq -e '.config.filter_subjects == ["story.lfp_created","story.lfp_response"] and .config.deliver_policy == "all"' >/dev/null || {
    printf '%s' "$matchmaking_info" | jq -c '{filters: .config.filter_subjects, delivery: .config.deliver_policy, error: .error}' >&2
    echo 'FAIL: matchmaking fixed consumer shape' >&2; exit 1;
  }
space_info="$(bootstrap_info '$JS.API.CONSUMER.INFO.subscription_events.space_subscription_entitlement')" || {
  echo 'FAIL: space fixed consumer INFO request' >&2; exit 1;
}
printf '%s' "$space_info" |
  jq -e '(.config.filter_subjects | sort) == ["subscription.space_pro_expired","subscription.space_pro_started"] and .config.deliver_policy == "new"' >/dev/null || {
    printf '%s' "$space_info" | jq -c '{filters: .config.filter_subjects, delivery: .config.deliver_policy, error: .error}' >&2
    echo 'FAIL: space fixed consumer shape' >&2; exit 1;
  }
user_info="$(bootstrap_info '$JS.API.CONSUMER.INFO.user_events.user-account-deletion-v1')" || {
  echo 'FAIL: user fixed consumer INFO request' >&2; exit 1;
}
printf '%s' "$user_info" |
  jq -e '(.config.deliver_subject // "") == "" and .config.filter_subject == "user.account_deleted" and .config.deliver_policy == "all"' >/dev/null || {
    printf '%s' "$user_info" | jq -c '{filter: .config.filter_subject, subject: .config.deliver_subject, delivery: .config.deliver_policy, error: .error}' >&2
    echo 'FAIL: user fixed consumer shape' >&2; exit 1;
  }

docker run -d --name voice-nats-canonical-chat --network "$network" -v "$work:$work:ro" \
  nats:2.12-alpine -c "$work/chat-leaf.conf" >/dev/null
for _ in $(seq 1 20); do
  chat_leaf_logs="$(docker logs voice-nats-canonical-chat 2>&1 || true)"
  if grep -Fq 'Server is ready' <<<"$chat_leaf_logs" && \
    grep -Fq 'Leafnode connection created for account: $G' <<<"$chat_leaf_logs"; then
    break
  fi
  sleep 1
done
chat_leaf_logs="$(docker logs voice-nats-canonical-chat 2>&1 || true)"
if ! grep -Fq 'Server is ready' <<<"$chat_leaf_logs"; then
  echo 'FAIL: chat leaf did not become locally ready within 20 seconds' >&2
  exit 1
fi
if ! grep -Fq 'Leafnode connection created for account: $G' <<<"$chat_leaf_logs"; then
  echo 'FAIL: chat leaf did not announce its remote leaf attach within 20 seconds' >&2
  exit 1
fi

# Use a synchronous JetStream publish so the assertion follows the hub's
# PubAck, rather than racing a fire-and-forget Core NATS publish against leaf
# interest propagation. The bounded remote-attach log is only a startup
# diagnostic; the PubAck below is the proof that the leaf route persisted the
# message. This app client has no credentials and can only reach NATS through
# the Chat leaf.
cat >"$work/chat-leaf-publish.go" <<'EOF'
package main

import (
  "errors"
  "fmt"
  "os"
  "strings"
  "time"

  "github.com/nats-io/nats.go"
)

func errorCategory(err error) string {
  message := strings.ToLower(err.Error())
  switch {
  case errors.Is(err, nats.ErrTimeout), strings.Contains(message, "timeout"):
    return "timeout"
  case strings.Contains(message, "no responders"):
    return "no_responders"
  case strings.Contains(message, "permission"):
    return "permission"
  case strings.Contains(message, "connection refused"):
    return "connection_refused"
  case strings.Contains(message, "stream not found"):
    return "stream_not_found"
  default:
    return "other"
  }
}

func main() {
  nc, err := nats.Connect("nats://127.0.0.1:4222",
    nats.Name("canonical-chat-leaf-proof"),
    nats.CustomInboxPrefix("_INBOX.voice.chat"),
    nats.Timeout(3*time.Second))
  if err != nil {
    fmt.Fprintf(os.Stderr, "FAIL: local Chat leaf connect error_category=%s\n", errorCategory(err))
    os.Exit(1)
  }
  defer nc.Close()

  js, err := nc.JetStream()
  if err != nil {
    fmt.Fprintf(os.Stderr, "FAIL: create Chat leaf JetStream publisher error_category=%s\n", errorCategory(err))
    os.Exit(1)
  }
  ack, err := js.Publish("chat.created", []byte("canonical-leaf-proof"), nats.AckWait(3*time.Second))
  if err != nil {
    fmt.Fprintf(os.Stderr, "FAIL: Chat leaf JetStream publish puback_error_category=%s\n", errorCategory(err))
    os.Exit(1)
  }
  if ack.Stream != "chat_events" || ack.Sequence != 1 {
    fmt.Fprintf(os.Stderr, "FAIL: Chat leaf PubAck mismatch stream=%s sequence=%d\n", ack.Stream, ack.Sequence)
    os.Exit(1)
  }
  fmt.Printf("stream=%s sequence=%d\n", ack.Stream, ack.Sequence)
}
EOF
if ! (cd "$root/src/backend/pkg" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$work/chat-leaf-publish" "$work/chat-leaf-publish.go"); then
  echo 'FAIL: build Chat leaf PubAck probe' >&2
  exit 1
fi
chat_leaf_puback="$(docker run --rm --network container:voice-nats-canonical-chat \
  -v "$work/chat-leaf-publish:/chat-leaf-publish:ro" alpine:3.22 /chat-leaf-publish)" || {
  echo 'FAIL: Chat leaf JetStream publish did not complete' >&2
  exit 1
}
if [[ "$chat_leaf_puback" != 'stream=chat_events sequence=1' ]]; then
  echo 'FAIL: Chat leaf JetStream publish returned an unexpected PubAck' >&2
  printf 'observed=%s\n' "$chat_leaf_puback" >&2
  exit 1
fi
chat_leaf_after="$(bootstrap_info '$JS.API.STREAM.INFO.chat_events')" || {
  echo 'FAIL: hub stream INFO after Chat leaf PubAck' >&2
  exit 1
}
if ! jq -e '.config.name == "chat_events" and .state.messages == 1 and .state.last_seq == 1' \
  <<<"$chat_leaf_after" >/dev/null; then
  echo 'FAIL: hub did not retain the exact Chat leaf PubAck message' >&2
  jq -c '{stream: .config.name, messages: .state.messages, last_seq: .state.last_seq, error: .error}' \
    <<<"$chat_leaf_after" >&2 || true
  exit 1
fi

# The same CA must reject a leaf URL whose hostname is absent from the hub SAN.
sed 's@nats-leaf://hub:7422@nats-leaf://voice-nats-canonical-hub:7422@' \
  "$work/chat-leaf.conf" >"$work/bad-leaf.conf"
docker run -d --name voice-nats-canonical-bad-leaf --network "$network" -v "$work:$work:ro" \
  nats:2.12-alpine -c "$work/bad-leaf.conf" >/dev/null
san_rejected=false
for _ in $(seq 1 10); do
  if docker logs voice-nats-canonical-bad-leaf 2>&1 | grep -q 'x509: certificate is valid for'; then
    san_rejected=true
    break
  fi
  sleep 1
done
if [[ "$san_rejected" != true ]]; then
  docker logs voice-nats-canonical-bad-leaf >&2
  echo 'FAIL: leaf accepted a hub hostname outside the TLS SAN' >&2
  exit 1
fi

# Auth and Chat use their own JWTs for a real fixed-delivery/ACK exchange.
# The same direct authenticated probe tests broker denial for neighboring
# publish, consumer administration, ACK, and request/reply inbox subjects.
cat >"$work/probe.go" <<'EOF'
package main

import (
  "fmt"
  "os"
  "strings"
  "time"
  "github.com/nats-io/nats.go"
)

func connect(name, prefix string) (*nats.Conn, <-chan error) {
  errors := make(chan error, 8)
  nc, err := nats.Connect("nats://hub:4222", nats.UserCredentials("/fixture/creds/"+name+".creds"),
    nats.CustomInboxPrefix(prefix), nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, e error) { errors <- e }))
  if err != nil { panic(err) }
  return nc, errors
}

func denied(nc *nats.Conn, errors <-chan error, subject string, subscribe bool) {
  if subscribe {
    if _, err := nc.SubscribeSync(subject); err != nil { panic(err) }
  } else if err := nc.Publish(subject, []byte("deny")); err != nil { panic(err) }
  if err := nc.Flush(); err != nil { panic(err) }
  select {
  case err := <-errors:
    if !strings.Contains(strings.ToLower(err.Error()), "permission") { panic(err) }
  case <-time.After(3*time.Second): panic("missing broker permission denial: "+subject)
  }
}

func main() {
  chat, chatErrors := connect("chat", "_INBOX.voice.chat")
  defer chat.Close()
  auth, _ := connect("auth", "_INBOX.voice.auth.requests")
  defer auth.Close()
  js, err := chat.JetStream()
  if err != nil { panic(err) }
  if _, err := js.ConsumerInfo("user_events", "chat_account_deleted"); err != nil { panic(err) }
  sub, err := chat.SubscribeSync("_INBOX.voice.chat.chat_account_deleted")
  if err != nil { panic(err) }
  if err := chat.Flush(); err != nil { panic(err) }
  if err := auth.Publish("user.account_deleted", []byte("canonical-ack-proof")); err != nil { panic(err) }
  if err := auth.Flush(); err != nil { panic(err) }
  msg, err := sub.NextMsg(5*time.Second)
  if err != nil { panic(err) }
  if err := msg.AckSync(nats.AckWait(2*time.Second)); err != nil { panic(err) }
  denied(chat, chatErrors, "user.account_deleted", false)
  denied(chat, chatErrors, "$JS.API.CONSUMER.CREATE.user_events.unreviewed", false)
  denied(chat, chatErrors, "$JS.API.CONSUMER.INFO.user_events.user-account-deletion-v1", false)
  denied(chat, chatErrors, "$JS.ACK.user_events.user-account-deletion-v1.1.1.1.1.1", false)
  denied(chat, chatErrors, "_INBOX.voice.messaging.foreign", true)
  fmt.Println("canonical JWT ACL positive delivery/ACK and neighboring denials passed")
  os.Exit(0)
}
EOF
(cd "$root/src/backend/pkg" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$work/probe" "$work/probe.go")
docker run --rm --network "$network" -v "$work/fixture:/fixture:ro" -v "$work/probe:/probe:ro" \
  alpine:3.22 /probe
bootstrap_info '$JS.API.CONSUMER.INFO.user_events.chat_account_deleted' |
  jq -e '.ack_floor.stream_seq == 1 and .num_ack_pending == 0' >/dev/null

echo 'PASS: canonical hosted JWT/TLS bootstrap and least-privilege ACL proof'
