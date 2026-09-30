#!/usr/bin/env bash
# Inert Kubernetes fixture: no cluster, credential, or PVC content is used.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
workflow="$root/.github/workflows/staging-nats-root-rotation.yml"
rotate="$root/scripts/staging/rotate-nats-root.sh"
grep -Fq "if: inputs.operation == 'recover-legacy'" "$workflow" || { echo 'recovery dispatch missing' >&2; exit 1; }
grep -Fq "if: inputs.operation == 'restore-user-cycle'" "$workflow" || { echo 'owned User cleanup dispatch missing' >&2; exit 1; }
grep -Fq 'r20260930a1' "$workflow" || { echo 'original recovery generation missing' >&2; exit 1; }
grep -Fq 'r20260930a2' "$workflow" || { echo 'all-stopped recovery generation missing' >&2; exit 1; }
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
        if [[ "$name" == voice-app-config ]]; then
          echo '{"kind":"ConfigMap","metadata":{"name":"voice-app-config","namespace":"voice-staging"},"data":{"SPACE_GRPC_ADDR":"voice-space:9090"}}'
        else
          jq -cn --arg phase "${MOCK_PHASE:-rotating}" --arg generation "${MOCK_TARGET_GENERATION:-r20260930a1}" '{kind:"ConfigMap",metadata:{name:"voice-nats-generation",namespace:"voice-staging",resourceVersion:"100"},data:{phase:$phase,generation:$generation,previousGeneration:"legacy"}}'
        fi ;;
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
        if [[ "$name" == voice-user ]]; then
          count=0
          if [[ -f "${KUBE_USER_GET_COUNT:?}" ]]; then read -r count <"$KUBE_USER_GET_COUNT"; fi
          count=$((count + 1))
          printf '%s\n' "$count" >"$KUBE_USER_GET_COUNT"
          if [[ "${MOCK_CLEAN_AT_BOOTSTRAP:-0}" == 1 && "$count" -ge 3 ]]; then rm -f "${KUBE_USER_OVERRIDE:?}"; fi
        fi
        creds=voice-nats-service-credentials
        if [[ "$name" == "${MOCK_BAD_LEAF:-never}" ]]; then creds=voice-nats-service-credentials-r20260930a1; fi
        replicas=0 ready=0
        if [[ -e "${KUBE_STARTED_DIR:?}/$name" || ( "${MOCK_ALL_STOPPED:-0}" != 1 && ( "$name" == voice-auth || "$name" == voice-social || "$name" == voice-user || "${MOCK_RESUME_ALL:-0}" == 1 && "$name" == voice-* ) ) ]]; then replicas=1; fi
        if [[ "$name" == "${MOCK_BAD_PREFIX:-never}" ]]; then replicas=1; fi
        if [[ "$name" == voice-auth || "$name" == voice-social ]]; then ready=1; fi
        if [[ "$name" == voice-user && ( -e "${KUBE_USER_OVERRIDE:?}" || -e "${KUBE_SPACE_READY:?}" || "${MOCK_CLEAN_READY:-0}" == 1 ) ]]; then ready=1; fi
        if [[ "$name" == voice-space && ( -e "${KUBE_SPACE_READY:?}" || "${MOCK_CLEAN_READY:-0}" == 1 ) ]]; then ready=1; fi
        if [[ "$name" == "${MOCK_BAD_PARTIAL:-never}" ]]; then ready=1; fi
        owned=false
        [[ "$name" == voice-user && -e "${KUBE_USER_OVERRIDE:?}" ]] && owned=true
        empty_owner=false
        if [[ "$name" == voice-user && "${MOCK_EMPTY_OWNER:-0}" == 1 ]]; then empty_owner=true; fi
        owner='' owner_present=false
        if [[ "$owned" == true ]]; then owner="${MOCK_TARGET_GENERATION:-r20260930a1}"; owner_present=true; fi
        if [[ "$empty_owner" == true ]]; then owner_present=true; fi
        annotations='{}'
        if [[ "$owner_present" == true ]]; then annotations="$(jq -cn --arg owner "$owner" '{"voice.io/nats-user-space-bootstrap":$owner}')"; fi
        jq -cn --arg name "$name" --arg service "$service" --arg creds "$creds" --argjson replicas "$replicas" --argjson ready "$ready" --argjson owned "$owned" --argjson annotations "$annotations" --argjson bad_override "${MOCK_BAD_OVERRIDE:-0}" '{kind:"Deployment",metadata:{name:$name,namespace:"voice-staging",generation:2,resourceVersion:"100"},spec:{replicas:$replicas,selector:{matchLabels:{app:$name}},template:{metadata:{labels:{app:$name},annotations:$annotations},spec:{containers:[{name:"nats-leaf"},{name:$service,env:(if $owned then (if $bad_override == 1 then [{name:"SPACE_GRPC_ADDR",value:"nonempty"}] else [{name:"SPACE_GRPC_ADDR"}] end) else [] end),envFrom:[{configMapRef:{name:"voice-app-config"}}]}],volumes:[{name:"nats-service-creds",secret:{secretName:$creds,items:[{key:($service+".creds"),path:($service+".creds")}]}} ,{name:"nats-hub-tls",secret:{secretName:"voice-nats-hub-tls",items:[{key:"ca.crt",path:"ca.crt"}]}}]}}},status:{readyReplicas:$ready,updatedReplicas:$ready,observedGeneration:2}}' ;;
      pods)
        if [[ "$*" == *app=voice-nats-pvc-candidate* ]]; then
          echo '{"items":[{"metadata":{"annotations":{"voice.io/nats-generation":"legacy"}},"status":{"phase":"Running","containerStatuses":[{"ready":true}]}}]}'
        elif [[ "$*" == *'app=voice-nats -o json'* && "${MOCK_OLD_HUB_POD:-0}" == 1 ]]; then
          echo '{"items":[{"metadata":{"name":"old-hub-lingering"},"status":{"phase":"Terminating"}}]}'
        elif [[ "$*" == *app=voice-auth* || "$*" == *app=voice-social* || "$*" == *app=voice-user* ]]; then
          args="$*"; app="${args#*app=}"; app="${app%% *}"
          if [[ "${MOCK_ALL_STOPPED:-0}" == 1 && ! -e "${KUBE_STARTED_DIR:?}/${app}" ]]; then
            echo '{"items":[]}'
            exit
          fi
          app_ready=true
          [[ "$app" == voice-user ]] && app_ready=false
          pod_creds=voice-nats-service-credentials
          [[ "$app" == "${MOCK_BAD_POD:-never}" ]] && pod_creds=voice-nats-service-credentials-r20260930a1
          owned=false
          [[ "$app" == voice-user && ( -e "${KUBE_USER_OVERRIDE:?}" || "${MOCK_LINGERING_OVERRIDE_POD:-0}" == 1 ) ]] && owned=true
          jq -cn --arg app "$app" --arg creds "$pod_creds" --argjson owned "$owned" --argjson appReady "$app_ready" '{items:[{metadata:{name:($app+"-pod"),labels:{app:$app}},spec:{volumes:[{name:"nats-service-creds",secret:{secretName:$creds}},{name:"nats-hub-tls",secret:{secretName:"voice-nats-hub-tls"}}],containers:[{name:($app|ltrimstr("voice-")),env:(if $owned then [{name:"SPACE_GRPC_ADDR"}] else [] end)}]},status:{phase:"Running",containerStatuses:[{name:"nats-leaf",ready:true},{name:($app|ltrimstr("voice-")),ready:$appReady}]}}]}'
        else echo '{"items":[]}'
        fi ;;
      *) echo 'unexpected read' >&2; exit 2 ;;
    esac ;;
  scale)
    [[ "$2" == deployment/voice-* && "$*" == *'--replicas=1'* ]] || exit 2
    printf '%s\n' "$2" >>"${KUBE_SCALES:?}"
    touch "${KUBE_STARTED_DIR:?}/${2#deployment/}" ;;
  rollout)
    [[ "$2" == status && "$3" == deployment/voice-* ]] || exit 2
    printf '%s\n' "$3" >>"${KUBE_ROLLOUTS:?}"
    if [[ "$3" == deployment/voice-user && -e "${KUBE_USER_OVERRIDE:?}" && "${MOCK_FAIL_STAGE:-}" == temporary-user ]]; then exit 1; fi
    if [[ "$3" == deployment/voice-space && "${MOCK_FAIL_STAGE:-}" == space ]]; then exit 1; fi
    if [[ "$3" == deployment/voice-space && -e "${KUBE_USER_OVERRIDE:?}" ]]; then touch "${KUBE_SPACE_READY:?}"; fi
    if [[ "$3" == deployment/voice-user && ! -e "${KUBE_USER_OVERRIDE:?}" && "${MOCK_FAIL_STAGE:-}" == restored-user ]]; then exit 1; fi
    [[ "$3" != "deployment/${MOCK_FAIL_ROLLOUT:-never}" ]] || exit 1 ;;
  patch)
    payload=''
    for ((i=1; i<=$#; i++)); do
      if [[ "${!i}" == -p ]]; then next=$((i+1)); payload="${!next}"; fi
    done
    if [[ "$2" == deployment && "$3" == voice-user ]]; then
      jq -e 'type == "array" and any(.[]; .op == "test" and .path == "/metadata/resourceVersion" and .value == "100") and
        all(.[]; .path == "/metadata/resourceVersion" or .path == "/spec/template/spec/containers/1/env/-" or .path == "/spec/template/spec/containers/1/env/0" or .path == "/spec/template/metadata/annotations/voice.io~1nats-user-space-bootstrap")' <<<"$payload" >/dev/null || exit 2
      if jq -e --arg generation "${MOCK_TARGET_GENERATION:-r20260930a1}" 'any(.[]; .op == "add" and .path == "/spec/template/metadata/annotations/voice.io~1nats-user-space-bootstrap" and .value == $generation)' <<<"$payload" >/dev/null; then
        jq -e 'any(.[]; .op == "add" and .path == "/spec/template/spec/containers/1/env/-" and .value == {name:"SPACE_GRPC_ADDR",value:""})' <<<"$payload" >/dev/null || exit 2
        [[ "${MOCK_FAIL_STAGE:-}" != apply ]] || exit 1
        touch "${KUBE_USER_OVERRIDE:?}"
      elif jq -e 'any(.[]; .op == "remove" and .path == "/spec/template/metadata/annotations/voice.io~1nats-user-space-bootstrap")' <<<"$payload" >/dev/null; then
        jq -e 'any(.[]; .op == "test" and .path == "/spec/template/spec/containers/1/env/0" and .value == {name:"SPACE_GRPC_ADDR"}) and any(.[]; .op == "remove" and .path == "/spec/template/spec/containers/1/env/0")' <<<"$payload" >/dev/null || exit 2
        [[ "${MOCK_FAIL_STAGE:-}" != remove ]] || exit 1
        rm -f "${KUBE_USER_OVERRIDE:?}"
      else exit 2
      fi
      exit
    fi
    [[ "$2" == configmap && "$3" == voice-nats-generation ]] || exit 2
    jq -e --arg generation "${MOCK_TARGET_GENERATION:-r20260930a1}" 'type == "array" and length == 7 and
      ([.[] | select(.op == "test") | .path] | sort) == ["/data/generation","/data/phase","/data/previousGeneration","/metadata/resourceVersion"] and
      ([.[] | select(.op == "replace") | .path] | sort) == ["/data/generation","/data/phase","/data/previousGeneration"] and
      any(.[]; .op == "test" and .path == "/metadata/resourceVersion" and .value == "100") and
      any(.[]; .op == "test" and .path == "/data/phase" and .value == "rotating") and
      any(.[]; .op == "test" and .path == "/data/generation" and .value == $generation) and
      any(.[]; .op == "test" and .path == "/data/previousGeneration" and .value == "legacy") and
      any(.[]; .op == "replace" and .path == "/data/phase" and .value == "active") and
      any(.[]; .op == "replace" and .path == "/data/generation" and .value == "legacy") and
      any(.[]; .op == "replace" and .path == "/data/previousGeneration" and .value == $generation)' <<<"$payload" >/dev/null || exit 2
    printf 'marker patched\n' >>"${KUBE_PATCHES:?}" ;;
  *) echo 'forbidden Kubernetes mutation' >&2; exit 2 ;;
esac
EOF
chmod 700 "$work/kubectl"
mkdir "$work/started"
export PATH="$work:$PATH" KUBE_CALLS="$work/calls" KUBE_SCALES="$work/scales" KUBE_ROLLOUTS="$work/rollouts" KUBE_PATCHES="$work/patches" KUBE_STARTED_DIR="$work/started" KUBE_USER_OVERRIDE="$work/user-override" KUBE_SPACE_READY="$work/space-ready" KUBE_USER_GET_COUNT="$work/user-gets"

run_case() {
  local generation="${MOCK_TARGET_GENERATION:-r20260930a1}"
  : >"$KUBE_CALLS"; : >"$KUBE_SCALES"; : >"$KUBE_ROLLOUTS"; : >"$KUBE_PATCHES"
  rm -f "$KUBE_STARTED_DIR"/* "$KUBE_USER_OVERRIDE" "$KUBE_SPACE_READY" "$KUBE_USER_GET_COUNT"
  [[ "${MOCK_START_OWNED:-0}" != 1 ]] || touch "$KUBE_USER_OVERRIDE"
  VOICE_K8S_NAMESPACE=voice-staging bash "$rotate" --"${MOCK_OPERATION:-recover-legacy}" "$generation" >"$work/output" 2>"$work/error"
}
run_case || { cat "$work/error" >&2; exit 1; }
grep -Fxq 'NATS_RECOVERY=LEGACY_ACTIVE' "$work/output" || { echo 'recovery success marker missing' >&2; exit 1; }
grep -Fxq 'NATS_USER_SPACE_OVERRIDE=APPLIED' "$work/output" || { echo 'temporary fail-closed User override not applied' >&2; exit 1; }
grep -Fxq 'NATS_USER_SPACE_OVERRIDE=REMOVED' "$work/output" || { echo 'temporary User override not removed' >&2; exit 1; }
[[ ! -e "$KUBE_USER_OVERRIDE" ]] || { echo 'User override remains after successful recovery' >&2; exit 1; }
[[ "$(wc -l <"$KUBE_SCALES")" == 15 && "$(wc -l <"$KUBE_ROLLOUTS")" == 21 && "$(wc -l <"$KUBE_PATCHES")" == 1 ]] || { echo 'recovery mutation count differs' >&2; exit 1; }
expected='auth social user role space chat file messaging voice matchmaking search notification realtime bot subscription moderation story analytics'
actual="$(sed 's|deployment/voice-||' "$KUBE_ROLLOUTS" | paste -sd ' ' -)"
[[ "$actual" == "user space user $expected" ]] || { echo 'recovery leaf readiness order differs' >&2; exit 1; }
for service in role space chat file messaging voice matchmaking search notification realtime bot subscription moderation story analytics; do
  printf 'scale deployment/voice-%s\n' "$service"
done >"$work/expected-order"
printf 'rollout deployment/voice-user\nrollout deployment/voice-space\nrollout deployment/voice-user\n' >>"$work/expected-order"
for service in $expected; do printf 'rollout deployment/voice-%s\n' "$service"; done >>"$work/expected-order"
grep -E '^(scale|rollout) deployment/voice-' "$KUBE_CALLS" >"$work/actual-order"
cmp -s "$work/expected-order" "$work/actual-order" || { echo 'recovery must start all dependencies before readiness waits' >&2; exit 1; }
[[ "$(tail -1 "$KUBE_CALLS")" == 'patch configmap' ]] || { echo 'marker changed before readiness' >&2; exit 1; }
[[ "$(grep -c '^patch deployment$' "$KUBE_CALLS")" == 2 ]] || { echo 'User override was not applied and removed exactly once' >&2; exit 1; }
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
export MOCK_EMPTY_OWNER=1
if run_case; then echo 'pre-existing empty ownership annotation did not block recovery' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'empty ownership annotation caused mutation' >&2; exit 1; }
unset MOCK_EMPTY_OWNER
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
for stage in apply temporary-user space remove restored-user; do
  export MOCK_FAIL_STAGE="$stage"
  if run_case; then echo "cycle stage ${stage} failure did not block recovery" >&2; exit 1; fi
  [[ ! -s "$KUBE_PATCHES" ]] || { echo "cycle stage ${stage} changed marker" >&2; exit 1; }
  if [[ "$stage" == remove ]]; then
    [[ -e "$KUBE_USER_OVERRIDE" ]] || { echo 'failed cleanup was not retained for explicit recovery' >&2; exit 1; }
    grep -Fq 'temporary User override cleanup failed' "$work/error" || { echo 'failed cleanup was not diagnosed' >&2; exit 1; }
  else
    [[ ! -e "$KUBE_USER_OVERRIDE" ]] || { echo "cycle stage ${stage} leaked temporary override" >&2; exit 1; }
  fi
done
unset MOCK_FAIL_STAGE
export MOCK_RESUME_ALL=1
export MOCK_CLEAN_READY=1 MOCK_LINGERING_OVERRIDE_POD=1
if run_case; then echo 'lingering temporary User Pod did not block active marker' >&2; exit 1; fi
[[ ! -s "$KUBE_PATCHES" && $(grep -c '^patch deployment$' "$KUBE_CALLS" || true) == 0 ]] || { echo 'lingering temporary User Pod caused mutation' >&2; exit 1; }
unset MOCK_CLEAN_READY MOCK_LINGERING_OVERRIDE_POD
run_case || { cat "$work/error" >&2; exit 1; }
[[ ! -s "$KUBE_SCALES" && "$(wc -l <"$KUBE_ROLLOUTS")" == 21 && "$(wc -l <"$KUBE_PATCHES")" == 1 ]] || { echo 'retry re-scaled an already started leaf or missed readiness' >&2; exit 1; }
export MOCK_START_OWNED=1
run_case || { cat "$work/error" >&2; exit 1; }
[[ ! -s "$KUBE_SCALES" && ! -e "$KUBE_USER_OVERRIDE" && "$(grep -c '^patch deployment$' "$KUBE_CALLS")" == 1 && "$(wc -l <"$KUBE_PATCHES")" == 1 ]] || { echo 'interrupted owned override did not resume and clean up' >&2; exit 1; }
export MOCK_OPERATION=restore-user-cycle
export MOCK_CLEAN_AT_BOOTSTRAP=1
if run_case; then echo 'restore-user-cycle reapplied a vanished ownership override' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" && $(grep -c '^patch deployment$' "$KUBE_CALLS" || true) == 0 ]] || { echo 'restore-user-cycle clean-state race caused mutation' >&2; exit 1; }
unset MOCK_CLEAN_AT_BOOTSTRAP
export MOCK_BAD_OVERRIDE=1
if run_case; then echo 'nonempty User override was accepted' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'nonempty User override caused mutation' >&2; exit 1; }
unset MOCK_BAD_OVERRIDE
run_case || { cat "$work/error" >&2; exit 1; }
[[ ! -s "$KUBE_SCALES" && ! -e "$KUBE_USER_OVERRIDE" && "$(grep -c '^patch deployment$' "$KUBE_CALLS")" == 1 && "$(wc -l <"$KUBE_PATCHES")" == 1 ]] || { echo 'restore-user-cycle changed unrelated state or missed cleanup' >&2; exit 1; }
unset MOCK_START_OWNED
if run_case; then echo 'restore-user-cycle accepted an unowned User Deployment' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'unowned restore-user-cycle caused mutation' >&2; exit 1; }
unset MOCK_OPERATION
unset MOCK_RESUME_ALL

export MOCK_ALL_STOPPED=1
if run_case; then echo 'original recovery generation accepted a fully stopped state' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'unsupported a1 all-stopped state caused mutation' >&2; exit 1; }
export MOCK_TARGET_GENERATION=r20260930a2
unset MOCK_ALL_STOPPED
if run_case; then echo 'a2 recovery accepted a started leaf outside its all-stopped precondition' >&2; exit 1; fi
[[ ! -s "$KUBE_SCALES" && ! -s "$KUBE_PATCHES" ]] || { echo 'unsupported a2 partial state caused mutation' >&2; exit 1; }
export MOCK_ALL_STOPPED=1
run_case || { cat "$work/error" >&2; exit 1; }
grep -Fxq 'NATS_RECOVERY=LEGACY_ACTIVE' "$work/output" || { echo 'a2 recovery success marker missing' >&2; exit 1; }
[[ ! -e "$KUBE_USER_OVERRIDE" && "$(wc -l <"$KUBE_SCALES")" == 18 && "$(wc -l <"$KUBE_ROLLOUTS")" == 21 && "$(wc -l <"$KUBE_PATCHES")" == 1 ]] || { echo 'a2 all-stopped recovery did not restore leaves and clear User override' >&2; exit 1; }
! grep -Eq 'run_jobs|voice-nats-realtime-permissions-preflight|voice-nats-acl-proof' "$KUBE_CALLS" || { echo 'recovery reran bootstrap or proof' >&2; exit 1; }

echo 'NATS_LEGACY_RECOVERY_CONTRACT=PASS'
