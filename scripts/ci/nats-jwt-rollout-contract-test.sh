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
  require '__NATS_APP_RESOLVER_PRELOAD__' "$infra" 'APP resolver render marker missing'
  require '__NATS_SYS_RESOLVER_PRELOAD__' "$infra" 'SYS resolver render marker missing'
  require 'system_account: __NATS_SYSTEM_ACCOUNT__' "$infra" 'system account render marker missing'
  require 'name: nats-config-renderer' "$infra" 'non-root config renderer missing'
  require 'image: __IMAGE_REGISTRY__/nats-hub-config-renderer:__IMAGE_TAG__' "$infra" 'renderer image reference missing'
  require 'name: nats-resolver-input' "$infra" 'init-only resolver input missing'
  require 'emptyDir: {medium: Memory, sizeLimit: 1Mi}' "$infra" 'memory-only rendered config volume missing'
  require 'runAsNonRoot: true' "$infra" 'renderer must run as non-root'
  require 'readOnlyRootFilesystem: true' "$infra" 'renderer root filesystem must be read-only'
  require 'automountServiceAccountToken: false' "$infra" 'service account token must be disabled'
  require 'path: "/healthz?js-enabled-only=true"' "$infra" 'JetStream readiness probe missing'
  ! grep -Fq '$APP_ACCOUNT_PUBLIC_KEY: /etc/nats/jwt/account.jwt' "$infra" || fail "literal APP resolver path in $env"
  ! grep -Fq '$SYSTEM_ACCOUNT_PUBLIC_KEY: /etc/nats/jwt/system-account.jwt' "$infra" || fail "literal SYS resolver path in $env"
  ! grep -Fq 'nats-config-validate' "$infra" || fail "mutable standalone validator present in $env"
  require 'port: 7422' "$infra" 'TLS leaf Service port missing'
done

for env in staging prod; do
  apply="$root/scripts/$env/apply-app-manifests.sh"
  require 'require_nats_bootstrap' "$apply" 'app-only rollout must require completed bootstrap Jobs'
  require 'voice-nats-realtime-bootstrap' "$apply" 'central stream bootstrap prerequisite missing'
done

for env in staging prod; do
  infra_apply="$root/scripts/$env/apply-infra.sh"
  require 'account.public system-account.public' "$infra_apply" 'APP/SYS public-key preflight missing'
  require 'tls.crt tls.key ca.crt' "$infra_apply" 'hub TLS preflight missing'
  require 'bootstrap.creds' "$infra_apply" 'bootstrap credential preflight missing'
  require 'voice-nats-service-credentials missing required' "$infra_apply" 'per-service credential preflight missing'
  require 'openssl verify -CAfile' "$infra_apply" 'hub TLS chain preflight missing'
  require '-checkhost voice-nats' "$infra_apply" 'hub TLS DNS SAN preflight missing'
  policy_line="$(grep -n 'network-policy-nats-hub.yaml' "$infra_apply" | head -1 | cut -d: -f1 || true)"
  hub_line="$(grep -n 'infra.yaml' "$infra_apply" | head -1 | cut -d: -f1 || true)"
  [ -n "$policy_line" ] && [ -n "$hub_line" ] && [ "$policy_line" -lt "$hub_line" ] || fail "direct-hub policy must precede hub apply in $env"
done

renderer="$root/src/backend/pkg/cmd/nats-hub-config-renderer"
require 'FROM nats@sha256:' "$renderer/Dockerfile" 'renderer validator base image must be pinned by digest'
require 'USER 10000:10000' "$renderer/Dockerfile" 'renderer image must use a non-root user'

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
  require 'command nats --creds "$NATS_CREDS" "$@"' "$template" 'bootstrap commands must pass quoted creds'
done

analytics_chat="$root/deploy/templates/nats-analytics-chat-bootstrap.yaml"
require "consumer user_events chat_account_deleted user.account_deleted" "$analytics_chat" 'Chat account-deleted fixed consumer missing'
require "consumer user_events chat_account_deleted user.account_deleted _INBOX.voice.chat.chat_account_deleted all ''" "$analytics_chat" 'Chat account-deleted durable contract is not fixed explicit/all'

policy="$root/deploy/templates/network-policy-nats-hub.yaml"
require 'port: 7422' "$policy" 'leaf ingress rule missing'
require 'port: 4222' "$policy" 'bootstrap-only client ingress rule missing'
require 'voice-nats-analytics-chat-bootstrap' "$policy" 'analytics bootstrap allowlist missing'

contract="$root/deploy/nats/secret-contract.example.yaml"
for key in operator.jwt account.jwt system-account.jwt account.public system-account.public; do require "$key" "$contract" "operator secret key missing"; done

echo 'NATS JWT rollout contract passed.'
