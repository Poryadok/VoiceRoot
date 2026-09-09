#!/usr/bin/env bash
# Regression guard for the staging full-rollout dependency order.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT="${ROOT}/scripts/staging/rollout-app-tier.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

line_number() {
  local pattern="$1"
  grep -nF "${pattern}" "${SCRIPT}" | head -1 | cut -d: -f1
}

auth_repair="$(line_number 'bash "${ROOT}/scripts/staging/repair-auth-flyway.sh"')"
auth_ready="$(line_number 'recreate_deploy voice-auth 900s')"
user_restart="$(line_number 'kubectl rollout restart deployment/voice-user -n "$NS"')"

[[ -n "${auth_repair}" ]] || fail "full rollout must repair Auth migrations"
[[ -n "${auth_ready}" ]] || fail "full rollout must wait for recreated Auth"
[[ -n "${user_restart}" ]] || fail "full rollout must restart User"

(( auth_repair < auth_ready )) || fail "Auth repair must precede Auth recreation"
(( auth_ready < user_restart )) || fail "Auth must be ready before the first User restart"

echo "Full rollout restores Auth before User."
