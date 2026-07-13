#!/usr/bin/env bash
# Ordered rollout for a subset or full app tier (user/space deadlock aware).
# NEEDS_USER_SPACE_ROLLOUT=true runs tiers 2-3 from rollout-app-tier.sh.
# Otherwise restarts only deployments mapped from CHANGED_SERVICES.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
# shellcheck source=scripts/staging/rollout-helpers.sh
source "${ROOT}/scripts/staging/rollout-helpers.sh"

if [ "${NEEDS_FULL_ROLLOUT:-false}" = "true" ]; then
  echo "Full ordered rollout (NEEDS_FULL_ROLLOUT)"
  bash "${ROOT}/scripts/staging/rollout-app-tier.sh"
  exit 0
fi

if [ "${NEEDS_USER_SPACE_ROLLOUT:-false}" = "true" ]; then
  echo "User/space ordered rollout"
  bash "${ROOT}/scripts/staging/rollout-user-space-tier.sh"
  exit 0
fi

bash "${ROOT}/scripts/staging/deploy-changed.sh"
bash "${ROOT}/scripts/staging/repair-auth-flyway.sh"
recreate_deploy voice-auth 900s
