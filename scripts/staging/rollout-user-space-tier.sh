#!/usr/bin/env bash
# User/space ordered rollout (tiers 2-3) without full app-tier restart.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
PATCH_FILE="${ROOT}/.local/patch-user-space-addr.json"

wait_deploy() {
  local dep="$1"
  local timeout="${2:-300s}"
  kubectl rollout status "deployment/${dep}" -n "$NS" --timeout="${timeout}"
}

patch_user_skip_space() {
  mkdir -p "${ROOT}/.local"
  cat >"${PATCH_FILE}" <<'EOF'
[
  {
    "op": "add",
    "path": "/spec/template/spec/containers/0/env/-",
    "value": {
      "name": "SPACE_GRPC_ADDR",
      "value": ""
    }
  }
]
EOF
  kubectl patch deployment voice-user -n "$NS" --type=json --patch-file "${PATCH_FILE}"
}

restore_user_space_addr() {
  kubectl set env deployment/voice-user -n "$NS" SPACE_GRPC_ADDR- 2>/dev/null || true
}

echo "User/space tier rollout: update images then ordered restart"

bash "${ROOT}/scripts/staging/deploy-changed.sh"

echo "Tier 2: user before space (break user<->space dial deadlock)"
kubectl scale deployment/voice-space -n "$NS" --replicas=0
patch_user_skip_space
kubectl rollout restart deployment/voice-user -n "$NS"
wait_deploy voice-user

echo "Tier 3: space"
kubectl scale deployment/voice-space -n "$NS" --replicas=1
wait_deploy voice-space
restore_user_space_addr
kubectl rollout restart deployment/voice-user -n "$NS"
wait_deploy voice-user

echo "User/space tier rollout complete."
