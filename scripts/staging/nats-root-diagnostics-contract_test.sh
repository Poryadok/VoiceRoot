#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
workflow="$root/.github/workflows/staging-nats-root-rotation.yml"
diagnose="$root/scripts/staging/diagnose-nats-root.sh"
grep -Fq "if: inputs.operation == 'diagnose'" "$workflow" || { echo 'diagnose must be gated' >&2; exit 1; }
grep -Fq 'run: bash scripts/staging/diagnose-nats-root.sh' "$workflow" || { echo 'diagnose must run reviewed script' >&2; exit 1; }
grep -Fq 'rollback|diagnose)' "$workflow" || { echo 'diagnose must use the master-ref gate' >&2; exit 1; }
grep -Fq 'set +x' "$diagnose" || { echo 'diagnose must suppress tracing' >&2; exit 1; }
if grep -Eq 'kubectl (apply|create|delete|patch|scale|replace|rollout|set|annotate|label|exec|cp)([[:space:]]|$)' "$diagnose"; then
  echo 'diagnose must not mutate Kubernetes or execute in Pods' >&2
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf -- "$work"' EXIT
cat >"$work/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$1" >>"${KUBE_VERBS:?}"
case "$1" in
  get)
    case "$2" in
      configmap) echo '{"data":{"phase":"rotating","generation":"r20260930a1","previousGeneration":"legacy","unrelated":"SENSITIVE_SENTINEL"}}' ;;
      deployment) echo '{"spec":{"replicas":1,"template":{"metadata":{"annotations":{"voice.io/nats-generation":"legacy"}},"spec":{"volumes":[{"name":"jsdata","persistentVolumeClaim":{"claimName":"voice-nats-jsdata"}},{"name":"nats-operator-jwt","secret":{"secretName":"voice-nats-operator"}},{"name":"nats-hub-tls","secret":{"secretName":"voice-nats-hub-tls"}}]}}},"status":{"readyReplicas":1}}' ;;
      deployments) echo '{"items":[{"metadata":{"name":"voice-realtime"},"spec":{"replicas":0,"template":{"metadata":{"annotations":{"voice.io/nats-generation":"r20260930a1"}},"spec":{"volumes":[{"name":"nats-service-creds","secret":{"secretName":"voice-nats-service-credentials-r20260930a1"}},{"name":"nats-hub-tls","secret":{"secretName":"voice-nats-hub-tls-r20260930a1"}}]}}},"status":{"readyReplicas":0}}]}' ;;
      service) echo '{"spec":{"selector":{"app":"voice-nats-pvc-candidate"},"ports":[{"name":"client","port":4222,"targetPort":4222}]}}' ;;
      job)
        if [[ "$3" == voice-nats-notification-bootstrap ]]; then
          echo '{"metadata":{"annotations":{"voice.io/nats-generation":"legacy"}},"status":{"failed":1,"conditions":[{"type":"Failed","status":"True","reason":"TOKEN123456"}]},"data":"SENSITIVE_SENTINEL"}'
        else
          echo '{"metadata":{"annotations":{"voice.io/nats-generation":"legacy"}},"status":{"succeeded":1}}'
        fi ;;
      pods)
        if [[ "$*" == *voice-nats-notification-bootstrap* ]]; then
          echo '{"items":[{"metadata":{"name":"notification-bootstrap-pod"},"status":{"phase":"Running","conditions":[{"type":"Ready","status":"False","reason":"TOKEN123456"}],"containerStatuses":[{"name":"bootstrap","ready":false,"restartCount":11,"state":{"waiting":{"reason":"CrashLoopBackOff"}},"lastState":{"terminated":{"reason":"TOKEN123456","exitCode":1}}}]}}]}'
        else
          echo '{"items":[]}'
        fi ;;
      events) echo '{"items":[{"reason":"BackOff","count":11,"message":"SENSITIVE_SENTINEL"},{"reason":"TOKEN123456","count":1,"message":"SENSITIVE_SENTINEL"}]}' ;;
      *) exit 2 ;;
    esac ;;
  logs)
    if [[ "$2" == job/voice-nats-realtime-bootstrap ]]; then
      echo 'incompatible configuration for stream social_events SENSITIVE_SENTINEL'
    else
      echo 'authentication failed SENSITIVE_SENTINEL'
    fi ;;
  *) echo 'mutation or unexpected kubectl verb' >&2; exit 2 ;;
esac
EOF
chmod 700 "$work/kubectl"
KUBE_VERBS="$work/verbs" PATH="$work:$PATH" bash "$diagnose" >"$work/output"
grep -Fq '"job":"voice-nats-notification-bootstrap"' "$work/output" || { echo 'middle bootstrap missing' >&2; exit 1; }
grep -Fq '"failed":1' "$work/output" || { echo 'middle bootstrap failure hidden' >&2; exit 1; }
grep -Fq 'NATS_DIAGNOSTIC_LOG=voice-nats-notification-bootstrap:AUTH' "$work/output" || { echo 'middle bootstrap log category missing' >&2; exit 1; }
grep -Fq 'NATS_DIAGNOSTIC_LOG=voice-nats-realtime-bootstrap:INCOMPATIBLE_STREAM' "$work/output" || { echo 'incompatible stream category missing' >&2; exit 1; }
grep -Fq '"lastExitCode":1' "$work/output" || { echo 'last termination hidden' >&2; exit 1; }
grep -Fq '"eventReasons":[{"reason":"BackOff","count":11},{"reason":"OTHER","count":1}]' "$work/output" || { echo 'safe event reasons missing' >&2; exit 1; }
grep -Fq '"reason":"OTHER"' "$work/output" || { echo 'unrecognized reason not redacted' >&2; exit 1; }
grep -Fq 'NATS_DIAGNOSTIC_LOG=voice-nats-realtime-permissions-preflight:nats-leaf:AUTH' "$work/output" || { echo 'leaf log category missing' >&2; exit 1; }
if grep -Eq 'SENSITIVE_SENTINEL|TOKEN123456' "$work/output" || grep -Ev '^(get|logs)$' "$work/verbs"; then
  echo 'diagnose leaked details or issued a mutating verb' >&2
  exit 1
fi
[[ "$(grep -c '^get$' "$work/verbs")" -ge 9 ]] || { echo 'diagnose did not inspect all Jobs' >&2; exit 1; }
echo 'NATS_ROOT_DIAGNOSE_CONTRACT=PASS'
