#!/usr/bin/env bash
# Static contract for the unselected per-service NATS leaf activation template.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
template="${root}/deploy/nats/leaf-sidecar.template.yaml"
fail() { echo "FAIL: nats leaf topology invariant: $*" >&2; exit 1; }

[[ -f "$template" ]] || fail "missing disabled leaf-sidecar template"

require() { grep -Fq -- "$1" "$template" || fail "missing $2"; }
reject() { ! grep -Fq -- "$1" "$template" || fail "forbidden $2"; }

require '__VOICE_SERVICE__' 'service identity placeholder'
require 'nats://127.0.0.1:4222' 'loopback-only application URL'
require 'listen: 127.0.0.1:4222' 'loopback-only NATS client listener'
require 'nats-leaf://__VOICE_NATS_HUB_HOST__:7422' 'TLS leaf hub endpoint'
require 'ca_file: /etc/nats/tls/ca.crt' 'hub CA verification'
require 'server_name: "__VOICE_NATS_HUB_SERVER_NAME__"' 'hub TLS server name'
require 'credentials: /var/run/nats/creds/__VOICE_SERVICE__.creds' 'single service credential path'
require 'mountPath: /var/run/nats/creds/__VOICE_SERVICE__.creds' 'single service credential mount'
require 'key: __VOICE_SERVICE__.creds' 'single service credential key'
require 'defaultMode: 0400' 'credential file mode'
require 'mountPath: /etc/nats/jwt/operator.jwt' 'operator JWT leaf mount'
require 'mountPath: /etc/nats/jwt/account.jwt' 'account JWT leaf mount'
tr '\n' ' ' <"$template" | grep -Fiq 'fixed, centrally pre-provisioned consumers' || fail 'missing fixed consumer constraint'
grep -Eq 'restart only (the )?nats-leaf sidecar' "$template" || fail 'missing credential rotation seam'
reject '$JS.API.>' 'broad JetStream administration grant'
reject 'insecure: true' 'insecure TLS verification'
reject 'NATS_URL' 'live application NATS URL mutation'

for target in docker-compose.yml deploy/staging deploy/prod; do
  ! grep -R -Fq -- 'leaf-sidecar.template.yaml' "${root}/${target}" 2>/dev/null || fail "selected by ${target}"
done

echo 'NATS leaf topology invariants passed.'
