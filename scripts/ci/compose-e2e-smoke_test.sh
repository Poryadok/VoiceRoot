#!/usr/bin/env bash
# Offline contract test for compose smoke rate-limit cleanup.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SCRIPT="${ROOT}/scripts/ci/compose-e2e-smoke.sh"
COMPOSE_FILE="${ROOT}/docker-compose.yml"

fail() { echo "FAIL: $*" >&2; exit 1; }

[[ -f "$SCRIPT" ]] || fail "missing compose smoke runner"
[[ -f "$COMPOSE_FILE" ]] || fail "missing compose fixture"

# The all-feature Flutter suite creates more than three disposable accounts from
# one loopback IP. The Compose fixture therefore disables Gateway's shared-IP
# OTP bucket, while retaining the separate Redis-backed Auth per-account
# throttle. This must remain a fixture default, not a production Gateway change.
gateway_rules_line="$(grep -F 'GATEWAY_RATE_LIMIT_RULES_JSON:' "$COMPOSE_FILE" || true)"
[[ "$gateway_rules_line" == *'"Auth":{"limit":0,"window":"15m"}'* ]] || \
  fail 'compose fixture must retain its Auth login/register override'
[[ "$gateway_rules_line" == *'"OTP":{"limit":0,"window":"10m"}'* ]] || \
  fail 'compose fixture must disable only Gateway OTP rate limiting for Flutter smoke'
[[ "$gateway_rules_line" == *'${GATEWAY_RATE_LIMIT_RULES_JSON:-'* ]] || \
  fail 'compose fixture must preserve an explicit GATEWAY_RATE_LIMIT_RULES_JSON override'

auth_redis_line="$(grep -F 'SPRING_DATA_REDIS_HOST: redis' "$COMPOSE_FILE" || true)"
[[ -n "$auth_redis_line" ]] || \
  fail 'compose fixture must retain Auth Redis wiring for the per-account OTP throttle'

otp_pattern_line="$(grep -n -F '"ratelimit:OTP:*"' "$SCRIPT" || true)"
[[ -n "$otp_pattern_line" ]] || fail 'compose smoke must clear stale OTP rate-limit keys before Flutter smoke'

pattern_block="$(sed -n '/for pattern in \\/,/done/p' "$SCRIPT")"
[[ "$pattern_block" == *'"ratelimit:OTP:*"'* ]] || fail 'OTP cleanup must belong to the Redis rate-limit pattern loop'
[[ "$pattern_block" == *'redis-cli --scan --pattern "${pattern}"'* ]] || fail 'rate-limit cleanup must scan each declared pattern'
[[ "$pattern_block" == *'redis-cli DEL "${key}"'* ]] || fail 'rate-limit cleanup must delete every scanned key'

echo 'All compose-e2e-smoke tests passed.'
