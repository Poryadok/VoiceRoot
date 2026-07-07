#!/usr/bin/env bash
# Ordered rollout to break user<->space gRPC startup deadlock on cold start.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
PATCH_FILE="${ROOT}/.local/patch-user-space-addr.json"

wait_deploy() {
  kubectl rollout status "deployment/$1" -n "$NS" --timeout=300s
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

echo "Tier 0: infra"
kubectl wait --for=condition=ready pod/voice-postgres-0 -n "$NS" --timeout=120s

echo "Tier 1: leaf gRPC services"
for d in voice-social voice-role voice-search voice-notification voice-bot voice-voice voice-realtime voice-subscription voice-analytics voice-federation; do
  kubectl rollout restart "deployment/${d}" -n "$NS" || true
done
for d in voice-social voice-role voice-search voice-notification voice-bot voice-voice voice-realtime voice-subscription voice-analytics voice-federation; do
  wait_deploy "$d" || true
done

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

echo "Tier 4: dependents"
for d in voice-chat voice-file voice-matchmaking voice-messaging voice-moderation voice-story; do
  kubectl scale deployment/"${d}" -n "$NS" --replicas=1
  kubectl rollout restart "deployment/${d}" -n "$NS"
  wait_deploy "$d"
done

kubectl rollout restart deployment/voice-auth -n "$NS"
wait_deploy voice-auth

echo "Ordered rollout complete."
