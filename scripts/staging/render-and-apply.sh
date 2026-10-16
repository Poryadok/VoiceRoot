#!/usr/bin/env bash
# Every ordinary version change uses the root-established paused transaction.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export VOICE_IMAGE_REGISTRY="${VOICE_IMAGE_REGISTRY:-ghcr.io/voiceroot/voiceroot}"
export VOICE_IMAGE_TAG="${VOICE_IMAGE_TAG:?VOICE_IMAGE_TAG required}"
export VOICE_K8S_NAMESPACE="${VOICE_K8S_NAMESPACE:-voice-staging}"
export DEPLOY_MODE="${DEPLOY_MODE:-full}"
export VOICE_NATS_TARGET_SOURCE="${ROOT}"
if [[ -z "${VOICE_NATS_PRESERVATION_RECEIPT:-}" ]]; then
  echo 'ERROR: version-bound NATS preservation receipt is required' >&2
  exit 1
fi
# Read-only target/ACL/identity checks precede the single-use marker claim.
# No legacy ingress/migration/bootstrap/replica-one path can bypass this entry.
python3 -I -S "${ROOT}/scripts/staging/nats-rollout-preservation/runner.py"
echo 'Staging target applied paused. Root preservation verification/resume is required.'
