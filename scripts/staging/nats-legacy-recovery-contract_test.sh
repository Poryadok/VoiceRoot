#!/usr/bin/env bash
# Inert Kubernetes fixture: no cluster, credential, or PVC content is used.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
workflow="$root/.github/workflows/staging-nats-root-rotation.yml"
rotate="$root/scripts/staging/rotate-nats-root.sh"
grep -Fq "if: inputs.operation == 'recover-legacy'" "$workflow" || { echo 'recovery dispatch missing' >&2; exit 1; }
grep -Fq 'r20260930a1' "$workflow" || { echo 'recovery generation not fixed' >&2; exit 1; }
work="$(mktemp -d)"
trap 'rm -rf -- "$work"' EXIT

cat >"$work/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
verb="$1"
printf '%s %s\n' "$verb" "${2:-}" >>"${KUBE_CALLS:?}"
case "$verb" in
  get)
    kind="$2" name="${3:-}"
    case "$kind" in
      configmap)
        jq -cn --arg phase "${MOCK_PHASE:-rotating}" '{kind:"ConfigMap",metadata:{name:"voice-nats-generation",namespace:"voice-staging",resourceVersion:"100"},data:{phase:$phase,generation:"r20260930a1",previousGeneration:"legacy"}}' ;;
      service)
        echo '{"kind":"Service","metadata":{"name":"voice-nats","namespace":"voice-staging"},"spec":{"selector":{"app":"voice-nats-pvc-candidate"},"ports":[{"name":"client","port":4222,"targetPort":4222}]}}' ;;
      secret)
        case "$name" in
          voice-nats-operator) keys='["operator.jwt","account.jwt","system-account.jwt","account.public","system-account.public"]' ;;
          voice-nats-hub-tls) keys='["tls.crt","tls.key","ca.crt"]' ;;
          voice-nats-bootstrap-credentials) keys='["bootstrap.creds"]' ;;
          voice-nats-service-credentials) keys='["analytics.creds","auth.creds","bot.creds","chat.creds","file.creds","gateway.creds","matchmaking.creds","messaging.creds","moderation.creds","notification.creds","realtime.creds","role.creds","search.creds","social.creds","space.creds","story.creds","subscription.creds","user.creds","voice.creds"]' ;;
          *) exit 2 ;;
        esac
        jq -cn --arg name "$name" --argjson keys "$keys" '{kind:"Secret",metadata:{name:$name,namespace:"voice-staging"},type:"Opaque",data:($keys | map({key:.,value:"eA=="}) | from_entries)}' ;;
      pvc)
        echo '{"kind":"PersistentVolumeClaim","metadata":{"name":"voice-nats-jsdata","namespace":"voice-staging"},"status":{"phase":"Bound"}}' ;;
      deployment)
        if [[ "$name" == voice-nats ]]; then echo 'NotFound' >&2; exit 1; fi
        if [[ "$name" == voice-nats-pvc-candidate ]]; then
          jq -cn --argjson ready "${MOCK_HUB_READY:-1}" '{kind:"Deployment",metadata:{name:"voice-nats-pvc-candidate",namespace:"voice-staging",generation:2},spec:{replicas:1,selector:{matchLabels:{app:"voice-nats-pvc-candidate"}},template:{metadata:{labels:{app:"voice-nats-pvc-candidate"}},spec:{volumes:[{name:"jsdata",persistentVolumeClaim:{claimName:"voice-nats-jsdata"}},{name:"nats-resolver-input",secret:{secretName:"voice-nats-operator"}},{name:"nats-operator-jwt",secret:{secretName:"voice-nats-operator"}},{name:"nats-hub-tls",secret:{secretName:"voice-nats-hub-tls"}}]}}},status:{readyReplicas:$ready,observedGeneration:2}}'
          exit
        fi
        service="${name#voice-}"
        creds=voice-nats-service-credentials
        if [[ "$name" == "${MOCK_BAD_LEAF:-never}" ]]; then creds=voice-nats-service-credentials-r20260930a1; fi
        jq -cn --arg name "$name" --arg service "$service" --arg creds "$creds" '{kind:"Deployment",metadata:{name:$name,namespace:"voice-staging"},spec:{replicas:0,selector:{matchLabels:{app:$name}},template:{metadata:{labels:{app:$name}},spec:{containers:[{name:"nats-leaf"}],volumes:[{name:"nats-service-creds",secret:{secretName:$creds,items:[{key:($service+".creds"),path:($service+".creds")}]}} ,{name:"nats-hub-tls",secret:{secretName:"voice-nats-hub-tls",items:[{key:"ca.crt",path:"ca.crt"}]}}]}}}}' ;;
      pods)
        if [[ "$*" == *app=voice-nats-pvc-candidate* ]]; then
          echo '{"items":[{"metadata":{"annotations":{"voice.io/nats-generation":"legacy"}},"status":{"phase":"Running","containerStatuses":[{"ready":true}]}}]}'
        else echo '{"items":[]}'
        fi ;;
      *) echo 'unexpected read' >&2; exit 2 ;;
    esac ;;
  scale)
    [[ "$2" == deployment/voice-* && "$*" == *'--replicas=1'* ]] || exit 2
    printf '%s\n' "$2" >>"${KUBE_SCALES:?}" ;;
  rollout)
    [[ "$2" == status && "$3" == deployment/voice-* ]] || exit 2
    printf '%s\n' "$3" >>"${KUBE_ROLLOUTS:?}"
    [[ "$3" != "deployment/${MOCK_FAIL_ROLLOUT:-never}" ]] || exit 1 ;;
  patch)
    [[ "$2" == configmap && "$3" == voice-nats-generation ]] || exit 2
    payload=''
    for ((i=1; i<=$#; i++)); do
      if [[ "${!i}" == -p ]]; then next=$((i+1)); payload="${!next}"; fi
    done
    jq -e 'type == "array" and length == 7 and
      ([.[] | select(.op == "test") | .path] | sort) == ["/data/generation","/data/phase","/data/previousGeneration","/metadata/resourceVersion"] and
      ([.[] | select(.op == "replace") | .path] | sort) == ["/data/generation","/data/phase","/data/previousGeneration"] and
      any(.[]; .op == "test" and .path == "/metadata/resourceVersion" and .value == "100") and
      any(.[]; .op == "test" and .path == "/data/phase" and .value == "rotating") and
      any(.[]; .op == "test" and .path == "/data/generation" and .value == "r20260930a1") and
      any(.[]; .op == "test" and .path == "/data/previousGeneration" and .value == "legacy") and
      any(.[]; .op == "replace" and .path == "/data/phase" and .value == "active") and
      any(.[]; .op == "replace" and .path == "/data/generation" and .value == "legacy") and
      any(.[]; .op == "replace" and .path == "/data/previousGeneration" and .value == "r20260930a1")' <<<"$payload" >/dev/null || exit 2
    printf 'marker patched\n' >>"${KUBE_PATCHES:?}" ;;
  *) echo 'forbidden Kubernetes mutation' >&2; exit 2 ;;
esac
EOF
chmod 700 "$work/kubectl"
export PATH="$work:$PATH" KUBE_CALLS="$work/calls" KUBE_SCALES="$work/scales" KUBE_ROLLOUTS="$work/rollouts" KUBE_PATCHES="$work/patches"

run_case() {
  : >"$KUBE_CALLS"; : >"$KUBE_SCALES"; : >"$KUBE_ROLLOUTS"; : >"$KUBE_PATCHES"
  VOICE_K8S_NAMESPACE=voice-staging bash "$rotate" --recover-legacy r20260930a1 >"$work/output" 2>"$work/error"
}
run_case
grep -Fxq 'NATS_RECOVERY=LEGACY_ACTIVE' "$work/output" || { echo 'recovery success marker missing' >&2; exit 1; }
[[ "$(wc -l <"$KUBE_SCALES")" == 18 && "$(wc -l <"$KUBE_ROLLOUTS")" == 18 && "$(wc -l <"$KUBE_PATCHES")" == 1 ]] || { echo 'recovery mutation count differs' >&2; exit 1; }
expected='auth social user role space chat file messaging voice matchmaking search notification realtime bot subscription moderation story analytics'
actual="$(sed 's|deployment/voice-||' "$KUBE_SCALES" | paste -sd ' ' -)"
[[ "$actual" == "$expected" ]] || { echo 'recovery leaf order differs' >&2; exit 1; }
[[ "$(tail -1 "$KUBE_CALLS")" == 'patch configmap' ]] || { echo 'marker changed before readiness' >&2; exit 1; }
if grep -Ev '^(get|scale|rollout|patch) ' "$KUBE_CALLS"; then echo 'unapproved Kubernetes verb' >&2; exit 1; fi

export MOCK_HUB_READY=0
if run_case; then echo 'unready hub did not block recovery' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'unready hub caused mutation' >&2; exit 1; }
unset MOCK_HUB_READY
export MOCK_PHASE=active
if run_case; then echo 'wrong marker did not block recovery' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'wrong marker caused mutation' >&2; exit 1; }
unset MOCK_PHASE
export MOCK_BAD_LEAF=voice-chat
if run_case; then echo 'partial target leaf reference did not block recovery' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'partial target reference caused mutation' >&2; exit 1; }
unset MOCK_BAD_LEAF
export MOCK_FAIL_ROLLOUT=voice-chat
if run_case; then echo 'failed leaf readiness did not block recovery' >&2; exit 1; fi
[[ ! -s "$KUBE_PATCHES" ]] || { echo 'marker changed after failed rollout' >&2; exit 1; }
unset MOCK_FAIL_ROLLOUT

echo 'NATS_LEGACY_RECOVERY_CONTRACT=PASS'
