#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
workflow="$root/.github/workflows/staging-nats-root-rotation.yml"
diagnose="$(sed -n '/name: Diagnose NATS rotation state (read-only)/,$p' "$workflow")"

[[ "$diagnose" == *"if: inputs.operation == 'diagnose'"* ]] || { echo 'diagnose must be explicitly gated' >&2; exit 1; }
[[ "$diagnose" == *'set +x'* && "$diagnose" == *'kubectl get'* && "$diagnose" == *'kubectl logs'* ]] || { echo 'diagnose must suppress tracing and inspect only metadata/logs' >&2; exit 1; }
if grep -Eq 'kubectl (apply|create|delete|patch|scale|replace|rollout|set|annotate|label|exec|cp)([[:space:]]|$)' <<<"$diagnose"; then
  echo 'diagnose must not mutate Kubernetes or execute in Pods' >&2
  exit 1
fi
if grep -Eq '(cat|tee|echo|printf)[[:space:]].*\$log|kubectl get secret|kubectl describe|\.data\[' <<<"$diagnose"; then
  echo 'diagnose must not expose logs, Secret data, or arbitrary descriptions' >&2
  exit 1
fi
grep -Fq 'rollback|diagnose)' "$workflow" || { echo 'diagnose must use the master-ref dispatch gate' >&2; exit 1; }
echo 'NATS_ROOT_DIAGNOSE_CONTRACT=PASS'
