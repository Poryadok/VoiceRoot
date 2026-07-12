#!/usr/bin/env bash
# Ordered rollout to break user<->space gRPC startup deadlock on cold start.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
PATCH_FILE="${ROOT}/.local/patch-user-space-addr.json"

describe_deploy_failure() {
  local dep="$1"
  echo "ERROR: deployment/${dep} rollout failed; diagnostics:" >&2
  kubectl get pods -n "$NS" -l "app=${dep}" -o wide >&2 || true
  local pod
  for pod in $(kubectl get pods -n "$NS" -l "app=${dep}" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null); do
    kubectl describe pod "$pod" -n "$NS" 2>&1 | tail -80 >&2 || true
  done
  pod="$(kubectl get pods -n "$NS" -l "app=${dep}" \
    -o jsonpath='{.items[?(@.status.containerStatuses[0].state.running)].metadata.name}' 2>/dev/null \
    | awk 'NR==1{print; exit}')"
  if [ -z "${pod}" ]; then
    pod="$(kubectl get pods -n "$NS" -l "app=${dep}" \
      -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  fi
  if [ -n "${pod}" ]; then
    kubectl logs -n "$NS" "$pod" --tail=80 --all-containers=true 2>&1 | tail -80 >&2 || true
    kubectl logs -n "$NS" "$pod" --previous --tail=80 --all-containers=true 2>&1 | tail -80 >&2 || true
  fi
}

wait_deploy() {
  local dep="$1"
  local timeout="${2:-300s}"
  if kubectl rollout status "deployment/${dep}" -n "$NS" --timeout="${timeout}"; then
    return 0
  fi
  describe_deploy_failure "$dep"
  return 1
}

# Scale-to-zero then back: reliable on single-node staging for heavy JVM images.
recreate_deploy() {
  local dep="$1"
  local timeout="${2:-600s}"
  echo "Recreate ${dep} (scale 0 → 1, timeout ${timeout})"
  kubectl scale "deployment/${dep}" -n "$NS" --replicas=0
  kubectl wait --for=delete pod -l "app=${dep}" -n "$NS" --timeout=180s 2>/dev/null || true
  kubectl scale "deployment/${dep}" -n "$NS" --replicas=1
  wait_deploy "$dep" "$timeout"
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
  wait_deploy "$d"
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

bash "${ROOT}/scripts/staging/repair-auth-flyway.sh"
recreate_deploy voice-auth 600s

echo "Ordered rollout complete."
