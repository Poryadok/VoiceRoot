#!/usr/bin/env bash
# Static fail-closed contract for the Kubernetes NATS JWT activation assets.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
fail() { echo "FAIL: NATS JWT rollout: $*" >&2; exit 1; }
require() { grep -Fq -- "$1" "$2" || fail "${3} (${2})"; }

for env in staging prod; do
  infra="$root/deploy/$env/infra.yaml"
  app="$root/deploy/$env/configmap-app.yaml"
  require 'resolver: MEMORY' "$infra" 'MEMORY resolver missing'
  require '$APP_ACCOUNT_PUBLIC_KEY: /etc/nats/jwt/account.jwt' "$infra" 'APP resolver public key preload missing'
  require '$SYSTEM_ACCOUNT_PUBLIC_KEY: /etc/nats/jwt/system-account.jwt' "$infra" 'SYS resolver public key preload missing'
  require 'system_account: $SYSTEM_ACCOUNT_PUBLIC_KEY' "$infra" 'system account public key missing'
  ! grep -Fq 'APP: /etc/nats/jwt' "$infra" || fail "literal APP resolver key in $env"
  ! grep -Fq 'SYS: /etc/nats/jwt' "$infra" || fail "literal SYS resolver key in $env"
  require 'port: 7422' "$infra" 'TLS leaf Service port missing'
done

# Every deployed NATS consumer owns a credential-scoped localhost leaf. The
# shared app ConfigMap intentionally has no NATS_URL, so a newly added consumer
# cannot silently bypass this inventory through a direct hub endpoint.
for env in staging prod; do
  services="$root/deploy/$env/services.yaml"
  ! grep -Fq 'nats://voice-nats:4222' "$services" || fail "direct hub client URL present in $env"
  ! grep -Eq '^NATS_URL:' "$root/deploy/$env/configmap-app.yaml" || fail "shared NATS URL present in $env ConfigMap"
  for service in auth analytics bot chat file matchmaking messaging moderation notification realtime role search social space story subscription user voice; do
    block="$(awk -v name="voice-$service" '
      /^---$/ { if (seen) exit }
      $0 == "  name: " name { seen=1 }
      seen { print }
    ' "$services")"
    [ -n "$block" ] || fail "voice-$service workload missing in $env"
    printf '%s\n' "$block" | grep -Fq 'nats://127.0.0.1:4222' || fail "$service loopback URL missing in $env"
    [ "$(printf '%s\n' "$block" | grep -Ec '^[[:space:]]*- name: nats-leaf$')" = 1 ] || fail "$service must have exactly one leaf in $env"
    printf '%s\n' "$block" | grep -Fq "key: $service.creds" || fail "$service credential key missing in $env"
    printf '%s\n' "$block" | grep -Fq "mountPath: /var/run/nats/creds/$service.creds" || fail "$service credential mount missing in $env"
    printf '%s\n' "$block" | grep -Fq 'defaultMode: 0400' || fail "$service credential mode missing in $env"
    printf '%s\n' "$block" | grep -Fq 'mountPath: /etc/nats/tls/ca.crt' || fail "$service CA mount missing in $env"
    printf '%s\n' "$block" | grep -Fq 'name: nats-leaf-config' || fail "$service leaf config mount missing in $env"
  done
done

for job in realtime notification search analytics-chat; do
  template="$root/deploy/templates/nats-${job}-bootstrap.yaml"
  require 'voice-nats-bootstrap-credentials' "$template" 'Job-only bootstrap secret missing'
  require 'bootstrap.creds' "$template" 'bootstrap credential key missing'
  require 'NATS_CREDS' "$template" 'bootstrap credential use missing'
  require 'nats() { command nats --creds "$NATS_CREDS" "$@"; }' "$template" 'bootstrap commands must pass quoted creds'
done

policy="$root/deploy/templates/network-policy-nats-hub.yaml"
require 'port: 7422' "$policy" 'leaf ingress rule missing'
require 'port: 4222' "$policy" 'bootstrap-only client ingress rule missing'
require 'voice-nats-analytics-chat-bootstrap' "$policy" 'analytics bootstrap allowlist missing'

contract="$root/deploy/nats/secret-contract.example.yaml"
for key in operator.jwt account.jwt system-account.jwt account.public system-account.public; do require "$key" "$contract" "operator secret key missing"; done

echo 'NATS JWT rollout contract passed.'
