#!/usr/bin/env bash
# Offline regression test for the rendered NATS JetStream Compose gate.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
CHECK="${ROOT}/scripts/ci/compose-nats-jetstream-check.sh"
FIXTURES="${ROOT}/scripts/ci/testdata/compose-nats-jetstream"

fail() { echo "FAIL: $*" >&2; exit 1; }

command -v jq >/dev/null 2>&1 || fail "jq is required to run the Compose JetStream regression test"
[[ -x "$CHECK" ]] || fail "missing executable Compose JetStream check"

temp_dir="$(mktemp -d)"
trap 'rm -rf "$temp_dir"' EXIT
mkdir -p "$temp_dir/bin"

cat >"$temp_dir/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[[ "$1" == compose && "$2" == config && "$3" == --format && "$4" == json ]] || exit 64
cat "$COMPOSE_CONFIG_FIXTURE"
EOF
chmod +x "$temp_dir/bin/docker"

run_check() {
  local fixture="$1"
  local output="$2"
  COMPOSE_CONFIG_FIXTURE="$FIXTURES/$fixture" PATH="$temp_dir/bin:$PATH" "$CHECK" >"$output" 2>&1
}

accepted_output="$temp_dir/accepted.out"
run_check accepted.json "$accepted_output" || fail "JetStream-enabled rendered Compose fixture must pass: $(<"$accepted_output")"
grep -Fq 'account-delete NATS deployment invariants passed.' "$accepted_output" || \
  fail "accepted fixture must reach account-delete invariants"

rejected_output="$temp_dir/rejected.out"
if run_check missing-jetstream.json "$rejected_output"; then
  fail "rendered Compose fixture without -js must fail"
fi
grep -Fq 'rendered Docker Compose must define services.nats.command with the "-js" JetStream flag.' "$rejected_output" || \
  fail "missing -js fixture must explain the rejected JetStream predicate"

echo 'Compose NATS JetStream gate regression tests passed.'
