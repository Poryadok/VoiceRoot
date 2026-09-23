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

echo 'NATS leaf topology gate regression tests passed.'
