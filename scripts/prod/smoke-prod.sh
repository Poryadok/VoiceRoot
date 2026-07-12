#!/usr/bin/env bash
# Post-deploy smoke for production (gateway health + minimal API).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/prod/load-prod-domains.sh
source "${ROOT}/scripts/prod/load-prod-domains.sh"

export VOICE_STAGING_URL="https://${VOICE_GATEWAY_INGRESS_HOST}"
bash "${ROOT}/scripts/staging/smoke-staging.sh"
