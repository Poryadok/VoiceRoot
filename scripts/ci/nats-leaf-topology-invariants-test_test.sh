#!/usr/bin/env bash
# Regression checks for the disabled NATS leaf topology static gate.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
CHECK="${ROOT}/scripts/ci/nats-leaf-topology-invariants-test.sh"
TEMPLATE="${ROOT}/deploy/nats/leaf-sidecar.template.yaml"
fail() { echo "FAIL: $*" >&2; exit 1; }

[[ -x "$CHECK" ]] || fail "static topology gate must be executable"
"$CHECK" >/dev/null || fail "complete disabled template must pass the static gate"

temp_dir="$(mktemp -d)"
trap 'cp "$temp_dir/template" "$TEMPLATE"; rm -rf "$temp_dir"' EXIT
cp "$TEMPLATE" "$temp_dir/template"

sed -i 's|nats://127.0.0.1:4222|nats://voice-nats:4222|' "$TEMPLATE"
if "$CHECK" >"$temp_dir/direct-hub.out" 2>&1; then
  fail "direct hub application URL must fail"
fi
grep -Fq 'loopback-only application URL' "$temp_dir/direct-hub.out" || fail "direct hub rejection must explain its invariant"
cp "$temp_dir/template" "$TEMPLATE"

sed -i 's|defaultMode: 0400|defaultMode: 0444|' "$TEMPLATE"
if "$CHECK" >"$temp_dir/mode.out" 2>&1; then
  fail "credential mode wider than 0400 must fail"
fi
grep -Fq 'credential file mode' "$temp_dir/mode.out" || fail "mode rejection must explain its invariant"

cp "$temp_dir/template" "$TEMPLATE"
sed -i 's|account: "\$G"|account: "__VOICE_NATS_ACCOUNT_PUBLIC_KEY__"|' "$TEMPLATE"
if "$CHECK" >"$temp_dir/account.out" 2>&1; then
  fail "config-mode remote account mapping must fail"
fi
grep -Fq 'config-mode remote account mapping' "$temp_dir/account.out" || fail "account rejection must explain its invariant"
cp "$temp_dir/template" "$TEMPLATE"

sed -i 's|account: "\$G"|account: "wrong-local-account"|' "$TEMPLATE"
printf '\n        # account: "$G"\n' >>"$TEMPLATE"
if "$CHECK" >"$temp_dir/comment-account.out" 2>&1; then
  fail "comment-only account decoy must fail"
fi
grep -Fq 'isolated local leaf account mapping' "$temp_dir/comment-account.out" || fail "comment decoy must explain its invariant"
cp "$temp_dir/template" "$TEMPLATE"

sed -i 's|account: "\$G"|account = "$G"|' "$TEMPLATE"
if "$CHECK" >"$temp_dir/account-equals.out" 2>&1; then
  fail "alternate account syntax must fail"
fi
grep -Fq 'exactly one active leaf account mapping' "$temp_dir/account-equals.out" || fail "alternate syntax must explain its invariant"
cp "$temp_dir/template" "$TEMPLATE"

sed -i '/account: "\$G"/a\        account: "$G"' "$TEMPLATE"
if "$CHECK" >"$temp_dir/duplicate-account.out" 2>&1; then
  fail "duplicate account mapping must fail"
fi
grep -Fq 'exactly one active leaf account mapping' "$temp_dir/duplicate-account.out" || fail "duplicate account must explain its invariant"
cp "$temp_dir/template" "$TEMPLATE"

sed -i 's|credentials: /var/run/nats/creds/__VOICE_SERVICE__\.creds|credentials: /var/run/nats/creds/shared.creds|' "$TEMPLATE"
if "$CHECK" >"$temp_dir/wrong-credentials.out" 2>&1; then
  fail "wrong service credential mapping must fail"
fi
grep -Fq 'single service credential path' "$temp_dir/wrong-credentials.out" || fail "credential rejection must explain its invariant"
cp "$temp_dir/template" "$TEMPLATE"

sed -i 's|ca_file: /etc/nats/tls/ca\.crt|ca_file: /etc/nats/tls/wrong-ca.crt|' "$TEMPLATE"
if "$CHECK" >"$temp_dir/wrong-ca.out" 2>&1; then
  fail "wrong hub CA mapping must fail"
fi
grep -Fq 'hub CA verification' "$temp_dir/wrong-ca.out" || fail "CA rejection must explain its invariant"
cp "$temp_dir/template" "$TEMPLATE"

sed -i 's|server_name: "__VOICE_NATS_HUB_SERVER_NAME__"|server_name: "wrong-hub"|' "$TEMPLATE"
if "$CHECK" >"$temp_dir/wrong-sni.out" 2>&1; then
  fail "wrong hub SNI mapping must fail"
fi
grep -Fq 'hub TLS server name' "$temp_dir/wrong-sni.out" || fail "SNI rejection must explain its invariant"

echo 'NATS leaf topology gate regression tests passed.'
