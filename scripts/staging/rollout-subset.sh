#!/usr/bin/env bash
# Ordered rollout for a subset or full app tier (user/space deadlock aware).
# NEEDS_USER_SPACE_ROLLOUT=true runs tiers 2-3 from rollout-app-tier.sh.
# Otherwise restarts only deployments mapped from CHANGED_SERVICES.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"

if [ "${NEEDS_FULL_ROLLOUT:-false}" = "true" ]; then
  echo "Full ordered rollout (NEEDS_FULL_ROLLOUT)"
  bash "${ROOT}/scripts/staging/rollout-app-tier.sh"
  exit 0
fi

if [ "${NEEDS_USER_SPACE_ROLLOUT:-false}" = "true" ]; then
  echo "User/space ordered rollout"
  # Tier 2-3 only from rollout-app-tier — invoke full script (safest for deadlock).
  bash "${ROOT}/scripts/staging/rollout-app-tier.sh"
  exit 0
fi

bash "${ROOT}/scripts/staging/deploy-changed.sh"
bash "${ROOT}/scripts/staging/repair-auth-flyway.sh" 2>/dev/null || true
if kubectl get deployment voice-auth -n "${NS}" >/dev/null 2>&1; then
  kubectl scale deployment/voice-auth -n "${NS}" --replicas=1
  kubectl rollout status deployment/voice-auth -n "${NS}" --timeout=600s
fi
