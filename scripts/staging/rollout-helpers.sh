#!/usr/bin/env bash
# Shared kubectl rollout helpers for staging/prod deploy scripts.
set -euo pipefail

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
    kubectl logs -n "$NS" "$pod" --tail=120 --all-containers=true 2>&1 | tail -120 >&2 || true
    kubectl logs -n "$NS" "$pod" --previous --tail=120 --all-containers=true 2>&1 | tail -120 >&2 || true
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
  local timeout="${2:-900s}"
  echo "Recreate ${dep} (scale 0 → 1, timeout ${timeout})"
  kubectl scale "deployment/${dep}" -n "$NS" --replicas=0
  kubectl wait --for=delete pod -l "app=${dep}" -n "$NS" --timeout=180s 2>/dev/null || true
  kubectl scale "deployment/${dep}" -n "$NS" --replicas=1
  wait_deploy "$dep" "$timeout"
}
