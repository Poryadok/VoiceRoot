#!/usr/bin/env bash
# Offline contract test for compose smoke rate-limit cleanup.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SCRIPT="${ROOT}/scripts/ci/compose-e2e-smoke.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }

[[ -f "$SCRIPT" ]] || fail "missing compose smoke runner"

otp_pattern_line="$(grep -n -F '"ratelimit:OTP:*"' "$SCRIPT" || true)"
[[ -n "$otp_pattern_line" ]] || fail 'compose smoke must clear stale OTP rate-limit keys before Flutter smoke'

pattern_block="$(sed -n '/for pattern in \\/,/done/p' "$SCRIPT")"
[[ "$pattern_block" == *'"ratelimit:OTP:*"'* ]] || fail 'OTP cleanup must belong to the Redis rate-limit pattern loop'
[[ "$pattern_block" == *'redis-cli --scan --pattern "${pattern}"'* ]] || fail 'rate-limit cleanup must scan each declared pattern'
[[ "$pattern_block" == *'redis-cli DEL "${key}"'* ]] || fail 'rate-limit cleanup must delete every scanned key'

echo 'All compose-e2e-smoke tests passed.'
