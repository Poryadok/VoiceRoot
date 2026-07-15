#!/usr/bin/env bash
# Ordered rollout for a subset or full app tier (user/space deadlock aware).
# User/space ordered tier runs automatically when apply-app-manifests ran (app-only/full)
# or when user/space images change (images-only). Manual NEEDS_USER_SPACE_ROLLOUT is optional.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
MODE="${DEPLOY_MODE:-app-only}"
# shellcheck source=scripts/staging/rollout-helpers.sh
source "${ROOT}/scripts/staging/rollout-helpers.sh"

needs_user_space_rollout() {
  if [ "${NEEDS_USER_SPACE_ROLLOUT:-false}" = "true" ]; then
    return 0
  fi
  # apply-app-manifests rewrites every deployment image; user must not dial space mid-rollout.
  if [ "${MODE}" != "images-only" ]; then
    return 0
  fi
  case ",${CHANGED_SERVICES}," in
    *,user,*|*,space,*) return 0 ;;
  esac
  return 1
}

if [ "${NEEDS_FULL_ROLLOUT:-false}" = "true" ]; then
  echo "Full ordered rollout (NEEDS_FULL_ROLLOUT)"
  bash "${ROOT}/scripts/staging/rollout-app-tier.sh"
  exit 0
fi

if needs_user_space_rollout; then
  echo "User/space ordered rollout (mode=${MODE})"
  bash "${ROOT}/scripts/staging/rollout-user-space-tier.sh"
  exit 0
fi

bash "${ROOT}/scripts/staging/deploy-changed.sh"
bash "${ROOT}/scripts/staging/repair-auth-flyway.sh"
recreate_deploy voice-auth 900s
