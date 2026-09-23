#!/usr/bin/env bash
# Static contract for the unselected per-service NATS leaf activation template.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
template="${root}/deploy/nats/leaf-sidecar.template.yaml"
proof="${root}/scripts/ci/nats-jwt-leaf-hosted-proof.sh"
fail() { echo "FAIL: nats leaf topology invariant: $*" >&2; exit 1; }

[[ -f "$template" ]] || fail "missing disabled leaf-sidecar template"

require() { grep -Fq -- "$1" "$template" || fail "missing $2"; }
reject() { ! grep -Fq -- "$1" "$template" || fail "forbidden $2"; }

require '__VOICE_SERVICE__' 'service identity placeholder'
require 'nats://127.0.0.1:4222' 'loopback-only application URL'
grep -A4 -F -- '- name: __VOICE_SERVICE__' "$template" | grep -Fq -- 'name: NATS_URL' || fail 'application NATS environment patch'
grep -A5 -F -- '- name: __VOICE_SERVICE__' "$template" | grep -Fq -- 'value: "nats://127.0.0.1:4222"' || fail 'application loopback endpoint patch'
require 'listen: 127.0.0.1:4222' 'loopback-only NATS client listener'
active_remote_lines="$(grep -Ec '^[[:space:]]*urls[[:space:]]*:' "$template" || true)"
[[ "$active_remote_lines" == "1" ]] || fail 'exactly one active leaf remote'
grep -Eq '^[[:space:]]*urls[[:space:]]*:[[:space:]]*\["nats-leaf://__VOICE_NATS_HUB_HOST__:7422"\][[:space:]]*$' "$template" || fail 'TLS leaf hub endpoint'
grep -Eq '^[[:space:]]*ca_file[[:space:]]*:[[:space:]]*/etc/nats/tls/ca\.crt[[:space:]]*$' "$template" || fail 'hub CA verification'
grep -Eq '^[[:space:]]*server_name[[:space:]]*:[[:space:]]*"__VOICE_NATS_HUB_SERVER_NAME__"[[:space:]]*$' "$template" || fail 'hub TLS server name'
require 'credentials: /var/run/nats/creds/__VOICE_SERVICE__.creds' 'single service credential path'
require 'mountPath: /var/run/nats/creds/__VOICE_SERVICE__.creds' 'single service credential mount'
require 'key: __VOICE_SERVICE__.creds' 'single service credential key'
require 'defaultMode: 0400' 'credential file mode'
reject 'account: "__VOICE_NATS_ACCOUNT_PUBLIC_KEY__"' 'config-mode remote account mapping'

# Account selection is a NATS directive, not a comment or a generic string.
# The configuration-mode leaf has one remote and keeps its local listener in
# the global account; the remote credential selects the authenticated account.
active_account_lines="$(grep -Ec '^[[:space:]]*account[[:space:]]*:' "$template" || true)"
[[ "$active_account_lines" == "1" ]] || fail 'exactly one active leaf account mapping'
grep -Eq '^[[:space:]]*account[[:space:]]*:[[:space:]]*"\$G"[[:space:]]*$' "$template" || fail 'isolated local leaf account mapping'
active_credentials_lines="$(grep -Ec '^[[:space:]]*credentials[[:space:]]*:' "$template" || true)"
[[ "$active_credentials_lines" == "1" ]] || fail 'exactly one active leaf credential mapping'
grep -Eq '^[[:space:]]*credentials[[:space:]]*:[[:space:]]*/var/run/nats/creds/__VOICE_SERVICE__\.creds[[:space:]]*$' "$template" || fail 'exact service credential mapping'
active_ca_lines="$(grep -Ec '^[[:space:]]*ca_file[[:space:]]*:' "$template" || true)"
[[ "$active_ca_lines" == "1" ]] || fail 'exactly one active hub CA mapping'
active_sni_lines="$(grep -Ec '^[[:space:]]*server_name[[:space:]]*:[[:space:]]*"__VOICE_NATS_HUB_SERVER_NAME__"[[:space:]]*$' "$template" || true)"
[[ "$active_sni_lines" == "1" ]] || fail 'exactly one active hub SNI mapping'
active_tls_first_lines="$(grep -Ec '^[[:space:]]*handshake_first[[:space:]]*:' "$template" || true)"
[[ "$active_tls_first_lines" == "1" ]] || fail 'exactly one TLS-first leaf handshake'
grep -Eq '^[[:space:]]*handshake_first[[:space:]]*:[[:space:]]*true[[:space:]]*$' "$template" || fail 'TLS-first leaf handshake'
require 'mountPath: /etc/nats/jwt/operator.jwt' 'operator JWT leaf mount'
require 'mountPath: /etc/nats/jwt/account.jwt' 'account JWT leaf mount'
tr '\n' ' ' <"$template" | grep -Fiq 'fixed, centrally pre-provisioned consumers' || fail 'missing fixed consumer constraint'
grep -Eq 'restart only (the )?nats-leaf sidecar' "$template" || fail 'missing credential rotation seam'
reject '$JS.API.>' 'broad JetStream administration grant'
reject 'insecure: true' 'insecure TLS verification'
reject 'insecure_skip_verify: true' 'insecure TLS verification'
reject 'nats://voice-nats:4222' 'direct hub application endpoint'

# The hosted proof must exercise concrete JetStream API subjects. A wildcard
# API permission would make that proof meaningless, and broad inbox grants
# would permit arbitrary request/reply traffic.
[[ -f "$proof" ]] || fail 'missing hosted JWT leaf proof'
proof_reject() { ! grep -Fq -- "$1" "$proof" || fail "hosted proof contains forbidden $2"; }
proof_require() { grep -Fq -- "$1" "$proof" || fail "hosted proof missing $2"; }
proof_reject '$JS.API.>' 'broad JetStream API wildcard'
proof_reject '_INBOX.>' 'broad inbox wildcard'
proof_reject '--ack-wait' 'nats-box-version-dependent consumer flag'
proof_require 'chat-noack.creds' 'no-ACK credential proof'
proof_require 'AckSync' 'synchronous no-ACK denial attempt'
proof_require 'ack_subject=' 'exact ACK subject evidence'
proof_require '$JS.API.CONSUMER.CREATE.chat_events.proof_chat' 'exact proof consumer create subject'
proof_require '"ack_wait":1000000000' 'one-second proof consumer ack wait'
proof_require '"max_deliver":-1' 'unbounded proof consumer redelivery budget'
proof_require 'mkdir -p "$work/receiver"' 'credential-free writable receiver directory'
proof_require '-v "$work/receiver:/receiver" alpine:3.22' 'receiver-only writable volume'
proof_reject '-v "$work:$work:ro" alpine:3.22' 'fixture credential mount in unauthenticated receiver'

for target in docker-compose.yml deploy/staging deploy/prod; do
  ! grep -R -Fq -- 'leaf-sidecar.template.yaml' "${root}/${target}" 2>/dev/null || fail "selected by ${target}"
done

echo 'NATS leaf topology invariants passed.'
