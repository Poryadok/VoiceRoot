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
if [[ "$verb" == rollout ]]; then
  printf '%s %s\n' "$verb" "${3:-}" >>"${KUBE_CALLS:?}"
else
  printf '%s %s\n' "$verb" "${2:-}" >>"${KUBE_CALLS:?}"
fi
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
        replicas=0 ready=0
        if [[ "$name" == voice-auth || "$name" == voice-social || "$name" == voice-user || "${MOCK_RESUME_ALL:-0}" == 1 && "$name" == voice-* ]]; then replicas=1; fi
        if [[ "$name" == "${MOCK_BAD_PREFIX:-never}" ]]; then replicas=1; fi
        if [[ "$name" == voice-auth || "$name" == voice-social ]]; then ready=1; fi
        if [[ "$name" == "${MOCK_BAD_PARTIAL:-never}" ]]; then ready=1; fi
        jq -cn --arg name "$name" --arg service "$service" --arg creds "$creds" --argjson replicas "$replicas" --argjson ready "$ready" '{kind:"Deployment",metadata:{name:$name,namespace:"voice-staging",generation:2},spec:{replicas:$replicas,selector:{matchLabels:{app:$name}},template:{metadata:{labels:{app:$name}},spec:{containers:[{name:"nats-leaf"}],volumes:[{name:"nats-service-creds",secret:{secretName:$creds,items:[{key:($service+".creds"),path:($service+".creds")}]}} ,{name:"nats-hub-tls",secret:{secretName:"voice-nats-hub-tls",items:[{key:"ca.crt",path:"ca.crt"}]}}]}}},status:{readyReplicas:$ready,observedGeneration:2}}' ;;
      pods)
        if [[ "$*" == *app=voice-nats-pvc-candidate* ]]; then
          echo '{"items":[{"metadata":{"annotations":{"voice.io/nats-generation":"legacy"}},"status":{"phase":"Running","containerStatuses":[{"ready":true}]}}]}'
        elif [[ "$*" == *'app=voice-nats -o json'* && "${MOCK_OLD_HUB_POD:-0}" == 1 ]]; then
          echo '{"items":[{"metadata":{"name":"old-hub-lingering"},"status":{"phase":"Terminating"}}]}'
        elif [[ "$*" == *app=voice-auth* || "$*" == *app=voice-social* || "$*" == *app=voice-user* ]]; then
          args="$*"; app="${args#*app=}"; app="${app%% *}"
          app_ready=true
          [[ "$app" == voice-user ]] && app_ready=false
          pod_creds=voice-nats-service-credentials
          [[ "$app" == "${MOCK_BAD_POD:-never}" ]] && pod_creds=voice-nats-service-credentials-r20260930a1
          jq -cn --arg app "$app" --arg creds "$pod_creds" --argjson appReady "$app_ready" '{items:[{metadata:{name:($app+"-pod"),labels:{app:$app}},spec:{volumes:[{name:"nats-service-creds",secret:{secretName:$creds}},{name:"nats-hub-tls",secret:{secretName:"voice-nats-hub-tls"}}]},status:{phase:"Running",containerStatuses:[{name:"nats-leaf",ready:true},{name:($app|ltrimstr("voice-")),ready:$appReady}]}}]}'
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
run_case || { cat "$work/error" >&2; exit 1; }
grep -Fxq 'NATS_RECOVERY=LEGACY_ACTIVE' "$work/output" || { echo 'recovery success marker missing' >&2; exit 1; }
[[ "$(wc -l <"$KUBE_SCALES")" == 15 && "$(wc -l <"$KUBE_ROLLOUTS")" == 18 && "$(wc -l <"$KUBE_PATCHES")" == 1 ]] || { echo 'recovery mutation count differs' >&2; exit 1; }
expected='auth social user role space chat file messaging voice matchmaking search notification realtime bot subscription moderation story analytics'
actual="$(sed 's|deployment/voice-||' "$KUBE_ROLLOUTS" | paste -sd ' ' -)"
[[ "$actual" == "$expected" ]] || { echo 'recovery leaf readiness order differs' >&2; exit 1; }
for service in role space chat file messaging voice matchmaking search notification realtime bot subscription moderation story analytics; do
  printf 'scale deployment/voice-%s\n' "$service"
done >"$work/expected-order"
for service in $expected; do printf 'rollout deployment/voice-%s\n' "$service"; done >>"$work/expected-order"
grep -E '^(scale|rollout) deployment/voice-' "$KUBE_CALLS" >"$work/actual-order"
cmp -s "$work/expected-order" "$work/actual-order" || { echo 'recovery must start all dependencies before readiness waits' >&2; exit 1; }
[[ "$(tail -1 "$KUBE_CALLS")" == 'patch configmap' ]] || { echo 'marker changed before readiness' >&2; exit 1; }
if grep -Ev '^(get|scale|rollout|patch) ' "$KUBE_CALLS"; then echo 'unapproved Kubernetes verb' >&2; exit 1; fi

export MOCK_HUB_READY=0
if run_case; then echo 'unready hub did not block recovery' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'unready hub caused mutation' >&2; exit 1; }
unset MOCK_HUB_READY
export MOCK_OLD_HUB_POD=1
if run_case; then echo 'lingering old hub Pod did not block recovery' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'lingering old hub caused mutation' >&2; exit 1; }
unset MOCK_OLD_HUB_POD
export MOCK_PHASE=active
if run_case; then echo 'wrong marker did not block recovery' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'wrong marker caused mutation' >&2; exit 1; }
unset MOCK_PHASE
export MOCK_BAD_LEAF=voice-chat
if run_case; then echo 'partial target leaf reference did not block recovery' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'partial target reference caused mutation' >&2; exit 1; }
unset MOCK_BAD_LEAF
export MOCK_BAD_POD=voice-user
if run_case; then echo 'mixed-generation running Pod did not block recovery' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'mixed-generation Pod caused mutation' >&2; exit 1; }
unset MOCK_BAD_POD
export MOCK_BAD_PARTIAL=voice-space
if run_case; then echo 'unexpected partial state did not block recovery' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'unexpected partial state caused mutation' >&2; exit 1; }
unset MOCK_BAD_PARTIAL
export MOCK_BAD_PREFIX=voice-space
if run_case; then echo 'non-prefix partial state did not block recovery' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'non-prefix state caused mutation' >&2; exit 1; }
unset MOCK_BAD_PREFIX
export MOCK_FAIL_ROLLOUT=voice-chat
if run_case; then echo 'failed leaf readiness did not block recovery' >&2; exit 1; fi
[[ ! -s "$KUBE_PATCHES" ]] || { echo 'marker changed after failed rollout' >&2; exit 1; }
[[ "$(tail -1 "$KUBE_SCALES")" == 'deployment/voice-analytics' ]] || { echo 'all dependencies were not started before readiness failure' >&2; exit 1; }
unset MOCK_FAIL_ROLLOUT
export MOCK_RESUME_ALL=1
run_case || { cat "$work/error" >&2; exit 1; }
[[ ! -s "$KUBE_SCALES" && "$(wc -l <"$KUBE_ROLLOUTS")" == 18 && "$(wc -l <"$KUBE_PATCHES")" == 1 ]] || { echo 'retry re-scaled an already started leaf or missed readiness' >&2; exit 1; }
unset MOCK_RESUME_ALL

echo 'NATS_LEGACY_RECOVERY_CONTRACT=PASS'
