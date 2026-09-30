#!/usr/bin/env bash
# Contract for the explicit staging NATS root generation switch. The fixture
# contains inert base64 placeholders; no real signing or workload secret is used.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ROTATE="${ROOT}/scripts/staging/rotate-nats-root.sh"
WORKFLOW="${ROOT}/.github/workflows/staging-nats-root-rotation.yml"
fail() { echo "FAIL: $*" >&2; exit 1; }
test -x "$ROTATE" || fail 'NATS root rotation entrypoint must be executable'

# The trusted staging runner does not have Go preinstalled. Resolve and build
# the exact-source validator before kubectl is configured or activation starts.
source_line="$(grep -n 'name: Download exact master source archive' "$WORKFLOW" | cut -d: -f1 || true)"
setup_line="$(grep -n 'uses: actions/setup-go@v5' "$WORKFLOW" | cut -d: -f1 || true)"
validator_line="$(grep -n 'name: Verify NATS proof validator toolchain' "$WORKFLOW" | cut -d: -f1 || true)"
kubectl_line="$(grep -n 'name: Configure staging kubectl' "$WORKFLOW" | cut -d: -f1 || true)"
activate_line="$(grep -n 'name: Activate NATS root generation' "$WORKFLOW" | cut -d: -f1 || true)"
[[ "$source_line" =~ ^[0-9]+$ && "$setup_line" =~ ^[0-9]+$ &&
  "$validator_line" =~ ^[0-9]+$ && "$kubectl_line" =~ ^[0-9]+$ &&
  "$activate_line" =~ ^[0-9]+$ ]] || fail 'rotation workflow must prepare Go validator'
((source_line < setup_line && setup_line < validator_line &&
  validator_line < kubectl_line && kubectl_line < activate_line)) ||
  fail 'Go validator must be ready before Kubernetes access or mutation'
grep -Fq 'go-version-file: src/backend/pkg/go.mod' "$WORKFLOW" ||
  fail 'Go version must come from the exact master source archive'
grep -Fq "if: inputs.operation == 'activate' || inputs.operation == 'rollback'" "$WORKFLOW" ||
  fail 'exact-master Realtime image must be verified for activation and rollback'

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin"
cat >"$work/bin/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${KUBECTL_LOG:?}"
case "$1" in
  get|create|apply|patch|replace|delete|scale|set|annotate|label|wait|rollout|logs|config|version) ;;
  *) echo "mock kubectl rejects unexpected verb: $1" >&2; exit 2 ;;
esac
if [[ "$1" == create || "$1" == apply || "$1" == patch || "$1" == replace || "$1" == delete || "$1" == scale || "$1" == set || "$1" == annotate || "$1" == label || "$1" == rollout && "$2" == restart ]] && [[ "$*" != *'--dry-run'* ]]; then
  printf 'mutation %s\n' "$*" >>"${KUBECTL_MUTATIONS:?}"
  if printf '%s' "$*" | grep -Eq 'phase=rotating|"phase"[[:space:]]*:[[:space:]]*"rotating"'; then printf 'event phase=rotating\n' >>"$KUBECTL_LOG"; fi
  if printf '%s' "$*" | grep -Eq 'phase=active|"phase"[[:space:]]*:[[:space:]]*"active"'; then printf 'event phase=active\n' >>"$KUBECTL_LOG"; fi
fi
manifest=''
for ((i=1; i<=$#; i++)); do
  if [[ "${!i}" == -f ]] && ((i < $#)); then
    next=$((i+1))
    manifest="${!next}"
  fi
done
if [[ "$*" == *'--dry-run'* ]]; then
  if [[ "$manifest" == - ]]; then cat >/dev/null; fi
elif [[ "$manifest" == - ]]; then
  # Keep only names/references/phase. Secret data never reaches the log.
  input="$(mktemp "${MOCK_KUBE_STATE_DIR:?}/manifest.XXXXXX")"
  cat >"$input"
  if jq -e . "$input" >/dev/null 2>&1; then
    jq -r '(.kind + " " + .metadata.name + " " + .metadata.namespace + " immutable=" + (.immutable | tostring)),
      ("namespace " + .metadata.namespace), (.data | keys[] | "key " + .)' "$input" >"${KUBECTL_RENDER_DIR:?}/latest.names"
  else
    awk '/^kind:/ {kind=$2} /^metadata:/ {metadata=1; next} metadata && /^  name:/ {print kind, $2, "voice-staging"} metadata && /^  namespace:/ {print "namespace", $2; metadata=0} /image:|secretName:|claimName:|voice.io\/nats-generation:|^[[:space:]]*(immutable|phase|generation|previousGeneration):/ {print}' "$input" \
      >"${KUBECTL_RENDER_DIR:?}/latest.names"
  fi
  rm -f "$input"
  expected_job_generation=r20260930a
  if [[ "${MOCK_GENERATION_STATE:-absent}" == active ]]; then expected_job_generation=legacy; fi
  if grep -Eq '^Job voice-nats-' "${KUBECTL_RENDER_DIR}/latest.names" && ! grep -Eq "voice.io/nats-generation: \"?${expected_job_generation}\"?" "${KUBECTL_RENDER_DIR}/latest.names"; then
    echo 'mock kubectl rejects a bootstrap Job without the exact generation annotation' >&2
    exit 2
  fi
  cat "${KUBECTL_RENDER_DIR}/latest.names" >>"${KUBECTL_RENDER_DIR}/metadata.names"
  if grep -Eq 'phase:.*rotating' "${KUBECTL_RENDER_DIR}/latest.names"; then printf 'event phase=rotating\n' >>"$KUBECTL_LOG"; fi
  if grep -Eq 'phase:.*active' "${KUBECTL_RENDER_DIR}/latest.names"; then printf 'event phase=active\n' >>"$KUBECTL_LOG"; fi
  if grep -Fq 'ConfigMap voice-nats-generation ' "${KUBECTL_RENDER_DIR}/latest.names"; then printf 'rotating\n' >"${MOCK_KUBE_STATE_DIR:?}/marker"; fi
  if grep -Fq 'PersistentVolumeClaim voice-nats-jsdata-r20260930a ' "${KUBECTL_RENDER_DIR}/latest.names"; then touch "${MOCK_KUBE_STATE_DIR:?}/target-pvc"; fi
  if grep -Fq 'Secret voice-nats-operator-r20260930a ' "${KUBECTL_RENDER_DIR}/latest.names"; then touch "${MOCK_KUBE_STATE_DIR:?}/target-secrets"; fi
elif [[ -f "$manifest" ]] && [[ "$manifest" == *.json ]] && [[ "$1" == create || "$1" == apply ]]; then
  jq -r '(.items // [.])[] | (.kind + " " + .metadata.name + " " + .metadata.namespace + " immutable=" + ((.immutable // false) | tostring))' "$manifest" \
    >>"${KUBECTL_RENDER_DIR:?}/metadata.names"
  if jq -e '.data.phase == "rotating"' "$manifest" >/dev/null 2>&1; then printf 'event phase=rotating\n' >>"$KUBECTL_LOG"; fi
  if jq -e '.data.phase == "active"' "$manifest" >/dev/null 2>&1; then printf 'event phase=active\n' >>"$KUBECTL_LOG"; fi
  if jq -e '[.items[]?.metadata.name] | index("voice-nats-operator-r20260930a") != null' "$manifest" >/dev/null 2>&1; then touch "${MOCK_KUBE_STATE_DIR:?}/target-secrets"; fi
elif [[ -f "$manifest" ]] && [[ "$1" == create || "$1" == apply ]]; then
  awk '/^kind:/ {kind=$2} /^metadata:/ {metadata=1; next} metadata && /^  name:/ {print kind, $2, "voice-staging"} metadata && /^  namespace:/ {print "namespace", $2; metadata=0}' "$manifest" \
    >>"${KUBECTL_RENDER_DIR:?}/metadata.names"
elif [[ -n "$manifest" && "$manifest" != - ]]; then
  echo 'mock kubectl rejects an unreadable manifest path' >&2
  exit 2
fi
if [[ "$1" == create && "$2" == secret && "$3" == generic ]]; then
  [[ "$*" == *'--dry-run=client'* && "$*" == *'--from-file=proof.creds='* && "$*" != *'--from-literal='* ]] || {
    echo 'mock kubectl rejects unsafe proof Secret create' >&2; exit 2;
  }
  printf '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"%s","namespace":"voice-staging"},"type":"Opaque","data":{"proof.creds":"eA=="}}\n' "$4"
fi
if [[ "$1" == patch && "$*" == *'configmap voice-nats-generation'* ]]; then
  marker_patch=''
  for ((i=1; i<=$#; i++)); do
    if [[ "${!i}" == -p ]] && ((i < $#)); then next=$((i+1)); marker_patch="${!next}"; fi
  done
  current_phase="${MOCK_GENERATION_STATE:-absent}"
  if [[ -f "${MOCK_KUBE_STATE_DIR:?}/marker" ]]; then current_phase="$(cat "$MOCK_KUBE_STATE_DIR/marker")"; fi
  jq -e --arg phase "$current_phase" --arg generation r20260930a --arg previous legacy '
    type == "array" and length == 6 and
    ([.[] | select(.op == "test") | .path] | sort) == ["/data/generation","/data/phase","/data/previousGeneration"] and
    ([.[] | select(.op == "replace") | .path] | sort) == ["/data/generation","/data/phase","/data/previousGeneration"] and
    all(.[] | select(.op == "test"); if .path == "/data/phase" then .value == $phase elif .path == "/data/generation" then .value == $generation else .value == $previous end)
  ' <<<"$marker_patch" >/dev/null || { echo 'mock kubectl rejects a marker patch without full JSON CAS' >&2; exit 2; }
  next_phase="$(jq -r '.[] | select(.op == "replace" and .path == "/data/phase") | .value' <<<"$marker_patch")"
  printf '%s\n' "$next_phase" >"${MOCK_KUBE_STATE_DIR:?}/marker"
  printf 'event phase=%s\n' "$next_phase" >>"$KUBECTL_LOG"
fi
if [[ "$1" == patch && "$2" == deployment && "$3" == voice-* ]]; then
  deployment_patch=''
  for ((i=1; i<=$#; i++)); do
    if [[ "${!i}" == -p ]] && ((i < $#)); then next=$((i+1)); deployment_patch="${!next}"; fi
  done
  jq -e '[.[] | select(.op == "test" and .path == "/metadata/resourceVersion" and .value == "100")] | length == 1' \
    <<<"$deployment_patch" >/dev/null || { echo 'mock kubectl rejects a Deployment patch without resourceVersion CAS' >&2; exit 2; }
  if [[ "$3" == voice-user ]] && jq -e 'any(.[]; .op == "add" and .path == "/spec/template/metadata/annotations/voice.io~1nats-user-space-bootstrap")' <<<"$deployment_patch" >/dev/null; then
    jq -e 'any(.[]; .op == "add" and .value == {name:"SPACE_GRPC_ADDR",value:""})' <<<"$deployment_patch" >/dev/null || exit 2
    touch "${MOCK_KUBE_STATE_DIR:?}/user-override"
  elif [[ "$3" == voice-user ]] && jq -e 'any(.[]; .op == "remove" and .path == "/spec/template/metadata/annotations/voice.io~1nats-user-space-bootstrap")' <<<"$deployment_patch" >/dev/null; then
    [[ "${MOCK_FAIL_STAGE:-}" != remove ]] || exit 1
    jq -e 'any(.[]; .op == "test" and (.value == {name:"SPACE_GRPC_ADDR"} or .value == {name:"SPACE_GRPC_ADDR",value:""}))' <<<"$deployment_patch" >/dev/null || exit 2
    rm -f "${MOCK_KUBE_STATE_DIR:?}/user-override"
  elif jq -e 'any(.[]; .path | endswith("/secret/secretName"))' <<<"$deployment_patch" >/dev/null; then
    if jq -e 'any(.[]; .value == "voice-nats-service-credentials-r20260930a")' <<<"$deployment_patch" >/dev/null; then
      printf '%s' '-r20260930a' >"${MOCK_KUBE_STATE_DIR:?}/leaf-${3#voice-}-suffix"
    else
      : >"${MOCK_KUBE_STATE_DIR:?}/leaf-${3#voice-}-suffix"
    fi
  fi
fi
if [[ "$1" == rollout && "$2" == status && "$3" == deployment/voice-space && "${MOCK_FAIL_STAGE:-}" == space ]]; then exit 1; fi
if [[ "$1" == rollout && "$2" == status && "$3" == deployment/voice-user && "${MOCK_FAIL_STAGE:-}" == restored-user && ! -f "${MOCK_KUBE_STATE_DIR:?}/user-override" ]]; then exit 1; fi
case "$*" in
  *'get jobs,networkpolicies,secrets -n voice-staging -l voice.io/nats-proof=true,voice.io/nats-proof-generation=r20260930a -o json'*)
    printf '%s\n' '{"apiVersion":"v1","kind":"List","items":[]}'
    ;;
  *'get configmap voice-nats-generation'*|*'get configmap/voice-nats-generation'*)
    if [[ -f "${MOCK_KUBE_STATE_DIR:?}/marker" ]]; then
      jq -n --rawfile phase "$MOCK_KUBE_STATE_DIR/marker" '{kind:"ConfigMap",metadata:{name:"voice-nats-generation",namespace:"voice-staging"},data:{phase:($phase|rtrimstr("\n")),generation:"r20260930a",previousGeneration:"legacy"}}'
    elif [[ "${MOCK_GENERATION_STATE:-absent}" == active ]]; then
      printf '{"kind":"ConfigMap","metadata":{"name":"voice-nats-generation","namespace":"voice-staging"},"data":{"phase":"active","generation":"r20260930a","previousGeneration":"legacy"}}\n'
    elif [[ "${MOCK_GENERATION_STATE:-absent}" == rotating ]]; then
      printf '{"kind":"ConfigMap","metadata":{"name":"voice-nats-generation","namespace":"voice-staging"},"data":{"phase":"rotating","generation":"r20260930a","previousGeneration":"legacy"}}\n'
    elif [[ "$*" != *'--ignore-not-found'* ]]; then
      echo 'Error from server (NotFound): configmaps "voice-nats-generation" not found' >&2
      exit 1
    fi
    ;;
  *'get secret voice-nats-acl-proof-'*|*'get networkpolicy voice-nats-acl-proof-'*|*'get job voice-nats-acl-proof-'*)
    echo 'Error from server (NotFound): temporary proof resource not found' >&2
    exit 1
    ;;
  *'get secret voice-nats-operator-r20260930a'*|*'get secret/voice-nats-operator-r20260930a'*)
    if [[ "${MOCK_PARTIAL_SET:-false}" != true && "${MOCK_GENERATION_STATE:-absent}" != active && ! -f "${MOCK_KUBE_STATE_DIR:?}/target-secrets" ]]; then
      if [[ "$*" != *'--ignore-not-found'* ]]; then echo 'Error from server (NotFound): secrets "voice-nats-operator-r20260930a" not found' >&2; exit 1; fi
    else
      jq -c '.items[] | select(.metadata.name == "voice-nats-operator-r20260930a")' "${MOCK_BUNDLE_FILE:?}"
    fi
    ;;
  *'get secret '*'-r20260930a'*|*'get secret/voice-nats-'*'-r20260930a'*)
    target=''
    for token in "$@"; do
      token="${token#secret/}"
      if [[ "$token" =~ ^voice-nats-(operator|hub-tls|bootstrap-credentials|service-credentials)-r20260930a$ ]]; then target="$token"; fi
    done
    if [[ "${MOCK_GENERATION_STATE:-absent}" != active && ! -f "${MOCK_KUBE_STATE_DIR:?}/target-secrets" ]]; then
      if [[ "$*" != *'--ignore-not-found'* ]]; then echo "Error from server (NotFound): secrets \"${target}\" not found" >&2; exit 1; fi
    else
      jq -c --arg name "$target" '.items[] | select(.metadata.name == $name)' "${MOCK_BUNDLE_FILE:?}"
    fi
    ;;
  *'get persistentvolumeclaim voice-nats-jsdata-r20260930a'*|*'get pvc voice-nats-jsdata-r20260930a'*|*'get pvc/voice-nats-jsdata-r20260930a'*)
    if [[ "${MOCK_TARGET_PVC_EXISTS:-false}" != true && "${MOCK_GENERATION_STATE:-absent}" != active && ! -f "${MOCK_KUBE_STATE_DIR:?}/target-pvc" ]]; then
      if [[ "$*" != *'--ignore-not-found'* ]]; then echo 'Error from server (NotFound): persistentvolumeclaims "voice-nats-jsdata-r20260930a" not found' >&2; exit 1; fi
    else
      printf '{"kind":"PersistentVolumeClaim","metadata":{"name":"voice-nats-jsdata-r20260930a","namespace":"voice-staging"},"spec":{"storageClassName":"local-path","accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}\n'
    fi
    ;;
  *'get pvc voice-nats-jsdata '*|*'get pvc/voice-nats-jsdata '*|*'get persistentvolumeclaim voice-nats-jsdata '*)
    jq -n --arg class "${MOCK_SOURCE_PVC_CLASS:-local-path}" '{kind:"PersistentVolumeClaim",metadata:{name:"voice-nats-jsdata",namespace:"voice-staging"},spec:{storageClassName:$class,accessModes:["ReadWriteOnce"],resources:{requests:{storage:"20Gi"}}}}'
    ;;
  *'get secret voice-nats-'*|*'get secret/voice-nats-'*)
    legacy=''
    for token in "$@"; do
      token="${token#secret/}"
      if [[ "$token" =~ ^voice-nats-(operator|hub-tls|bootstrap-credentials|service-credentials)$ ]]; then legacy="$token"; fi
    done
    [[ -n "$legacy" ]] || exit 2
    jq -c --arg name "$legacy" '.items[] | select(.metadata.name == ($name + "-r20260930a")) | .metadata.name = $name | del(.immutable)' "${MOCK_BUNDLE_FILE:?}"
    ;;
  *'get service voice-nats'*|*'get service/voice-nats'*)
    printf '{"kind":"Service","metadata":{"name":"voice-nats","namespace":"voice-staging"},"spec":{"selector":{"app":"%s"}}}\n' "${MOCK_LIVE_SELECTOR:-voice-nats-pvc-candidate}"
    ;;
  *'get configmap voice-app-config'*|*'get configmap/voice-app-config'*)
    printf '%s\n' '{"kind":"ConfigMap","metadata":{"name":"voice-app-config","namespace":"voice-staging"},"data":{"SPACE_GRPC_ADDR":"voice-space:9090"}}'
    ;;
  *'get job voice-nats-'*|*'get job/voice-nats-'*)
    job=''
    for token in "$@"; do
      token="${token#job/}"
      if [[ "$token" =~ ^voice-nats-(realtime|notification|search|analytics-chat)-bootstrap$ || "$token" == voice-nats-realtime-permissions-preflight ]]; then job="$token"; fi
    done
    [[ -n "$job" ]] || exit 2
    state=complete
    [[ "$job" != voice-nats-realtime-permissions-preflight || -z "${MOCK_FAIL_JOB:-}" ]] || state="$MOCK_FAIL_JOB"
    job_generation=r20260930a
    [[ "${MOCK_GENERATION_STATE:-absent}" != active ]] || job_generation=legacy
    jq -n --arg job "$job" --arg state "$state" --arg generation "$job_generation" '{kind:"Job",metadata:{name:$job,namespace:"voice-staging",annotations:{"voice.io/nats-generation":$generation}},status:(if $state == "complete" then {conditions:[{type:"Complete",status:"True"}],succeeded:1} elif $state == "deadline" then {conditions:[{type:"Failed",status:"True",reason:"DeadlineExceeded"}],failed:1} else {conditions:[],failed:1} end)}'
    ;;
  *'get pods -n voice-staging -l app=voice-user -o json'*)
    if [[ -f "${MOCK_KUBE_STATE_DIR:?}/user-override" ]]; then
      printf '%s\n' '{"items":[{"metadata":{"name":"voice-user-pod","labels":{"app":"voice-user"}},"spec":{"containers":[{"name":"user","env":[{"name":"SPACE_GRPC_ADDR"}]}]},"status":{"phase":"Running","containerStatuses":[{"name":"user","ready":true}]}}]}'
    else
      printf '%s\n' '{"items":[{"metadata":{"name":"voice-user-pod","labels":{"app":"voice-user"}},"spec":{"containers":[{"name":"user","env":[]}]},"status":{"phase":"Running","containerStatuses":[{"name":"user","ready":true}]}}]}'
    fi
    ;;
  *'get pods -n voice-staging -l app=voice-'*'-o name'*)
    # Empty list proves the scaled-down hub or leaf pod has exited.
    ;;
  *'get deployment voice-nats '*|*'get deployment/voice-nats '*)
    echo 'Error from server (NotFound): deployments "voice-nats" not found' >&2
    exit 1
    ;;
  *'get deployment voice-nats-pvc-candidate'*|*'get deployment/voice-nats-pvc-candidate'*)
    suffix=''
    if [[ "${MOCK_GENERATION_STATE:-absent}" == active ]]; then suffix='-r20260930a'; fi
    printf '{"kind":"Deployment","metadata":{"name":"voice-nats-pvc-candidate","namespace":"voice-staging","resourceVersion":"100"},"spec":{"replicas":1,"selector":{"matchLabels":{"app":"voice-nats-pvc-candidate"}},"template":{"metadata":{"labels":{"app":"voice-nats-pvc-candidate"}},"spec":{"containers":[{"name":"nats"}],"volumes":[{"name":"jsdata","persistentVolumeClaim":{"claimName":"%s"}},{"name":"nats-resolver-input","secret":{"secretName":"voice-nats-operator%s"}},{"name":"nats-operator-jwt","secret":{"secretName":"voice-nats-operator%s"}},{"name":"nats-hub-tls","secret":{"secretName":"voice-nats-hub-tls%s"}}]}}}}\n' "${MOCK_SOURCE_PVC:-voice-nats-jsdata}${suffix}" "$suffix" "$suffix" "$suffix"
    ;;
  *'get deployment voice-'*|*'get deployment/voice-'*)
    service=auth
    for token in "$@"; do
      token="${token#deployment/}"
      if [[ "$token" =~ ^voice-(auth|social|user|role|space|chat|file|messaging|voice|matchmaking|search|notification|realtime|bot|subscription|moderation|story|analytics)$ ]]; then
        service="${token#voice-}"
      fi
    done
    suffix=''
    if [[ -f "${MOCK_KUBE_STATE_DIR:?}/leaf-${service}-suffix" ]]; then
      suffix="$(cat "${MOCK_KUBE_STATE_DIR}/leaf-${service}-suffix")"
    elif [[ "${MOCK_GENERATION_STATE:-absent}" == active ]]; then suffix='-r20260930a'; fi
    image="ghcr.io/example/voice/${service}:0123456789abcdef0123456789abcdef01234567"
    if [[ "$service" == realtime ]]; then image="${MOCK_REALTIME_IMAGE:-$image}"; fi
    owned=false
    [[ "$service" != user || ! -f "${MOCK_KUBE_STATE_DIR:?}/user-override" ]] || owned=true
    jq -n --arg service "$service" --arg image "$image" --arg suffix "$suffix" --argjson owned "$owned" '{kind:"Deployment",metadata:{name:("voice-"+$service),namespace:"voice-staging",resourceVersion:"100"},spec:{replicas:1,template:{metadata:{annotations:(if $owned then {"voice.io/nats-user-space-bootstrap":"r20260930a"} else {} end)},spec:{containers:[{name:$service,image:$image,env:(if $service == "user" and $owned then [{name:"SPACE_GRPC_ADDR"}] else [] end),envFrom:(if $service == "user" then [{configMapRef:{name:"voice-app-config"}}] else [] end)},{name:"nats-leaf"}],volumes:[{name:"nats-service-creds",secret:{secretName:("voice-nats-service-credentials"+$suffix),items:[{key:($service+".creds"),path:($service+".creds")}] }},{name:"nats-hub-tls",secret:{secretName:("voice-nats-hub-tls"+$suffix),items:[{key:"ca.crt",path:"ca.crt"}]}}]}}}}'
    ;;
  *'logs job/voice-nats-acl-proof-'*)
    printf 'NATS_LIVE_ACL_PROOF=PASS generation=r20260930a acl_sha=%s\n' "${MOCK_ACL_SHA:?}"
    ;;
  *)
    if [[ "$1" == get ]]; then echo 'mock kubectl rejects an unexpected get resource' >&2; exit 2; fi
    ;;
esac
EOF
chmod +x "$work/bin/kubectl"
cat >"$work/bin/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[[ "${MOCK_GO_PROOF_VALIDATION_FAIL:-false}" != true ]] || exit 127
[[ "$*" == *'run ./cmd/nats-proof-credential --check-min-validity 30m'* && "$*" == *'--generation r20260930a --namespace voice-staging'* ]] || exit 2
EOF
chmod +x "$work/bin/go"

generation=r20260930a
umask 077
MSYS2_ARG_CONV_EXCL='/CN=' openssl req -x509 -newkey rsa:2048 -nodes -days 1 \
  -subj '/CN=voice-nats' -addext 'subjectAltName=DNS:voice-nats' \
  -keyout "$work/tls.key" -out "$work/tls.crt" >/dev/null 2>&1
jq -n --arg g "$generation" --rawfile cert "$work/tls.crt" --rawfile key "$work/tls.key" '
  {apiVersion:"v1",kind:"List",items:[
    {apiVersion:"v1",kind:"Secret",metadata:{name:("voice-nats-operator-"+$g),namespace:"voice-staging"},type:"Opaque",immutable:true,data:{"operator.jwt":"eA==","account.jwt":"eA==","system-account.jwt":"eA==","account.public":"eA==","system-account.public":"eA=="}},
    {apiVersion:"v1",kind:"Secret",metadata:{name:("voice-nats-hub-tls-"+$g),namespace:"voice-staging"},type:"Opaque",immutable:true,data:{"tls.crt":($cert|@base64),"tls.key":($key|@base64),"ca.crt":($cert|@base64)}},
    {apiVersion:"v1",kind:"Secret",metadata:{name:("voice-nats-bootstrap-credentials-"+$g),namespace:"voice-staging"},type:"Opaque",immutable:true,data:{"bootstrap.creds":"eA=="}},
    {apiVersion:"v1",kind:"Secret",metadata:{name:("voice-nats-service-credentials-"+$g),namespace:"voice-staging"},type:"Opaque",immutable:true,data:(["analytics","auth","bot","chat","file","gateway","matchmaking","messaging","moderation","notification","realtime","role","search","social","space","story","subscription","user","voice"] | map({key:(.+".creds"),value:"eA=="}) | from_entries)}
  ]}' >"$work/bundle.json"
printf 'inert-proof-credential\n' >"$work/proof.creds"
acl_sha="$(sha256sum "$ROOT/deploy/nats/acl-intent.yaml" | cut -d' ' -f1)"
run_rotation() {
  local rc=0
  : >"$work/kubectl.log"
  : >"$work/mutations.log"
  mkdir -p "$work/rendered"
  mkdir -p "$work/state"
  rm -f "$work/state/marker" "$work/state/target-secrets" "$work/state/target-pvc" "$work/state/user-override" "$work/state"/leaf-*-suffix
  : >"$work/rendered/metadata.names"
  PATH="$work/bin:$PATH" \
    KUBECTL_LOG="$work/kubectl.log" KUBECTL_MUTATIONS="$work/mutations.log" KUBECTL_RENDER_DIR="$work/rendered" \
    MOCK_KUBE_STATE_DIR="$work/state" MOCK_BUNDLE_FILE="$work/bundle.json" \
    VOICE_K8S_NAMESPACE="${TEST_NAMESPACE:-voice-staging}" \
    VOICE_NATS_STORAGE_CLASS=local-path VOICE_NATS_STORAGE_SIZE=20Gi \
    VOICE_IMAGE_TAG=0123456789abcdef0123456789abcdef01234567 \
    VOICE_IMAGE_REGISTRY=ghcr.io/example/voice \
    MOCK_PARTIAL_SET="${MOCK_PARTIAL_SET:-false}" \
    MOCK_TARGET_PVC_EXISTS="${MOCK_TARGET_PVC_EXISTS:-false}" \
    MOCK_LIVE_SELECTOR="${MOCK_LIVE_SELECTOR:-voice-nats-pvc-candidate}" \
    MOCK_SOURCE_PVC="${MOCK_SOURCE_PVC:-voice-nats-jsdata}" \
    MOCK_SOURCE_PVC_CLASS="${MOCK_SOURCE_PVC_CLASS:-local-path}" \
    MOCK_REALTIME_IMAGE="${MOCK_REALTIME_IMAGE:-}" \
    MOCK_GO_PROOF_VALIDATION_FAIL="${MOCK_GO_PROOF_VALIDATION_FAIL:-false}" \
    MOCK_FAIL_JOB="${MOCK_FAIL_JOB:-}" \
    MOCK_FAIL_STAGE="${MOCK_FAIL_STAGE:-}" \
    MOCK_GENERATION_STATE="${MOCK_GENERATION_STATE:-absent}" \
    MOCK_ACL_SHA="$acl_sha" GITHUB_RUN_ID=123456 GITHUB_RUN_ATTEMPT=1 \
    GITHUB_SHA=0123456789abcdef0123456789abcdef01234567 \
    VOICE_NATS_PROOF_IMAGE_REGISTRY=ghcr.io/poryadok/voiceroot \
    VOICE_NATS_PROOF_IMAGE_TAG=0123456789abcdef0123456789abcdef01234567 \
    VOICE_NATS_PROOF_IMAGE_PUBLIC=true \
    bash "$ROTATE" "$@" >"$work/output" 2>&1 || rc=$?
  if grep -Eq 'eA==|inert-proof-credential|BEGIN (RSA )?PRIVATE KEY' "$work/output"; then
    fail 'rotation output exposed fixture credential bytes'
  fi
  return "$rc"
}

assert_no_mutation() {
  if grep -E '^(apply|create|delete|patch|replace|scale|set|annotate|label) ' "$work/kubectl.log" | grep -Ev -- '--dry-run'; then
    fail 'rejected rotation issued a Kubernetes mutation'
  fi
  if grep -Eq '^rollout restart ' "$work/kubectl.log"; then
    fail 'rejected rotation restarted a workload'
  fi
}

assert_nats_only_mutations() {
  local operation
  while IFS= read -r operation; do
    if [[ "$operation" =~ ^mutation[[:space:]]+(apply|create|replace)[[:space:]]+.*-f[[:space:]]+ ]]; then
      continue
    fi
    if [[ "$operation" =~ ^mutation\ (patch|scale|set|rollout\ restart)\ .*deployment[/[:space:]]+voice-(nats-pvc-candidate|auth|social|user|role|space|chat|file|messaging|voice|matchmaking|search|notification|realtime|bot|subscription|moderation|story|analytics)([[:space:]]|$) ]]; then
      continue
    fi
    if [[ "$operation" =~ ^mutation\ (create|patch|replace)\ .*configmap[/[:space:]]+voice-nats-generation([[:space:]]|$) ]]; then
      continue
    fi
    if [[ "$operation" =~ ^mutation\ create\ .*secret\ .*voice-nats-(operator|hub-tls|bootstrap-credentials|service-credentials)-r20260930a([[:space:]]|$) ]]; then
      continue
    fi
    if [[ "$operation" =~ ^mutation\ create\ .*(pvc|persistentvolumeclaim)\ .*voice-nats-jsdata-r20260930a([[:space:]]|$) ]]; then
      continue
    fi
    if [[ "$operation" =~ ^mutation\ (create|delete)\ .*job[/[:space:]]+voice-nats-(realtime|notification|search|analytics-chat)-bootstrap([[:space:]]|$) || "$operation" =~ ^mutation\ (create|delete)\ .*job[/[:space:]]+voice-nats-realtime-permissions-preflight([[:space:]]|$) ]]; then
      continue
    fi
    if [[ "$operation" =~ ^mutation\ delete\ (job|networkpolicy|secret)\ voice-nats-acl-proof-r20260930a-[a-z0-9-]+\ -n\ voice-staging([[:space:]]|$) ]]; then
      continue
    fi
    fail "rotation used an unexpected Kubernetes mutation outside its NATS allowlist: ${operation}"
  done <"$work/mutations.log"
}

if run_rotation --activate '../voice-prod' "$work/bundle.json" "$work/proof.creds"; then
  fail 'path-like generation must fail'
fi
assert_no_mutation
if run_rotation --activate r20260930A "$work/bundle.json" "$work/proof.creds"; then
  fail 'generation must match r[0-9]{8}[a-z0-9]{0,8}'
fi
assert_no_mutation
if TEST_NAMESPACE=voice-prod run_rotation --activate "$generation" "$work/bundle.json" "$work/proof.creds"; then
  fail 'rotation must reject every namespace except voice-staging'
fi
assert_no_mutation
if MOCK_GO_PROOF_VALIDATION_FAIL=true run_rotation --activate "$generation" "$work/bundle.json" "$work/proof.creds"; then
  fail 'rotation must reject a failed proof validator before changing Kubernetes'
fi
assert_no_mutation
if MOCK_PARTIAL_SET=true run_rotation --activate "$generation" "$work/bundle.json" "$work/proof.creds"; then
  fail 'rotation must reject a partial target generation'
fi
assert_no_mutation
if MOCK_TARGET_PVC_EXISTS=true run_rotation --activate "$generation" "$work/bundle.json" "$work/proof.creds"; then
  fail 'rotation must reject a preexisting target PVC'
fi
assert_no_mutation
if MOCK_LIVE_SELECTOR=voice-postgres run_rotation --activate "$generation" "$work/bundle.json" "$work/proof.creds"; then
  fail 'rotation must reject a wrong live voice-nats Service selector'
fi
assert_no_mutation
if MOCK_SOURCE_PVC=voice-postgres-pgdata run_rotation --activate "$generation" "$work/bundle.json" "$work/proof.creds"; then
  fail 'rotation must reject an unexpected source PVC mount'
fi
assert_no_mutation
# The old deployed Realtime image can predate the preflight entrypoint. The
# one-shot Job must run the image validated against the workflow SHA instead.
MOCK_REALTIME_IMAGE=ghcr.io/example/voice/realtime:latest run_rotation --activate "$generation" "$work/bundle.json" "$work/proof.creds" ||
  fail 'legacy deployed Realtime image must not determine the preflight Job image'
grep -Fq 'image: ghcr.io/poryadok/voiceroot/realtime:0123456789abcdef0123456789abcdef01234567' "$work/rendered/metadata.names" ||
  fail 'preflight Job must use the validated exact-master Realtime image'
! grep -Fq 'image: ghcr.io/example/voice/realtime:latest' "$work/rendered/metadata.names" ||
  fail 'preflight Job must never use the deployed legacy Realtime image'
grep -Fxq 'NATS_USER_SPACE_OVERRIDE=APPLIED' "$work/output" || fail 'activation did not break the User/Space startup cycle'
grep -Fxq 'NATS_USER_SPACE_OVERRIDE=REMOVED' "$work/output" || fail 'activation left the User override installed'
assert_nats_only_mutations
if jq 'del(.items[] | select(.metadata.name == "voice-nats-service-credentials-r20260930a") | .data["chat.creds"])' "$work/bundle.json" >"$work/incomplete.json" && run_rotation --activate "$generation" "$work/incomplete.json" "$work/proof.creds"; then
  fail 'rotation must reject a bundle missing a required service key'
fi
assert_no_mutation
if jq '(.items[0].metadata.namespace) = "voice-prod"' "$work/bundle.json" >"$work/wrong-bundle-namespace.json" && run_rotation --activate "$generation" "$work/wrong-bundle-namespace.json" "$work/proof.creds"; then
  fail 'rotation must reject a bundle whose namespace differs from voice-staging'
fi
assert_no_mutation
if jq '(.items[0].immutable) = false' "$work/bundle.json" >"$work/mutable.json" && run_rotation --activate "$generation" "$work/mutable.json" "$work/proof.creds"; then
  fail 'rotation must reject a mutable generation Secret'
fi
assert_no_mutation
if jq '.items += [{apiVersion:"v1",kind:"Secret",metadata:{name:"voice-app-secrets",namespace:"voice-staging"},type:"Opaque",immutable:true,data:{x:"eA=="}}]' "$work/bundle.json" >"$work/extra-secret.json" && run_rotation --activate "$generation" "$work/extra-secret.json" "$work/proof.creds"; then
  fail 'rotation must reject bundles with a fifth or non-NATS Secret'
fi
assert_no_mutation

# A regular full infra apply must fail closed while the root generation is
# rotating, before it can apply unrelated infrastructure or app workloads.
: >"$work/kubectl.log"
if PATH="$work/bin:$PATH" KUBECTL_LOG="$work/kubectl.log" KUBECTL_RENDER_DIR="$work/rendered" \
  KUBECTL_MUTATIONS="$work/mutations.log" MOCK_KUBE_STATE_DIR="$work/state" MOCK_BUNDLE_FILE="$work/bundle.json" \
  MOCK_GENERATION_STATE=rotating VOICE_K8S_NAMESPACE=voice-staging \
  VOICE_NATS_STORAGE_CLASS=local-path VOICE_NATS_STORAGE_SIZE=20Gi \
  VOICE_IMAGE_TAG=0123456789abcdef0123456789abcdef01234567 \
  bash "${ROOT}/scripts/staging/apply-infra.sh" >"$work/output" 2>&1; then
  fail 'ordinary full infra apply must reject a rotating NATS generation'
fi
assert_no_mutation
grep -Fq 'NATS generation rotation is in progress' "$work/output" || fail 'full deploy must fail specifically on the rotating marker'

# The generated deployment/app templates and regular full deploy must follow
# this marker, while the stable application DNS must remain voice-nats.
if ! run_rotation --activate "$generation" "$work/bundle.json" "$work/proof.creds"; then
  fail "valid staging activation did not complete under mock kubectl: $(tail -1 "$work/output")"
fi
assert_nats_only_mutations
! grep -E '^namespace ' "$work/rendered/metadata.names" | grep -Ev '^namespace voice-staging$' || fail 'rotation rendered a resource outside voice-staging'
for resource in voice-nats-operator voice-nats-hub-tls voice-nats-bootstrap-credentials voice-nats-service-credentials; do
  grep -Fq "Secret ${resource}-${generation} voice-staging" "$work/rendered/metadata.names" || \
    grep -Eq "^mutation (create|apply) .*secret.*${resource}-${generation}" "$work/mutations.log" || \
    fail "activation must create immutable ${resource}-${generation}"
done
grep -Fq "PersistentVolumeClaim voice-nats-jsdata-${generation} voice-staging" "$work/rendered/metadata.names" || \
  grep -Eq "^mutation (create|apply) .*persistentvolumeclaim.*voice-nats-jsdata-${generation}" "$work/mutations.log" || \
  fail 'activation must provision a generation PVC'
grep -Eq '(phase[=:\\" ]+rotating|phase=rotating)' "$work/kubectl.log" "$work/mutations.log" "$work/rendered/metadata.names" || fail 'rotation must record the rotating phase before hub replacement'
grep -Eq '(phase[=:\\" ]+active|phase=active)' "$work/kubectl.log" "$work/mutations.log" "$work/rendered/metadata.names" || fail 'successful rotation must record the active phase'
grep -Fq "$generation" "$work/mutations.log" "$work/rendered/metadata.names" || fail 'generation marker must bind the exact token'
hub_patches="$(grep -E '^patch .*deployment[/[:space:]]+voice-nats-pvc-candidate' "$work/kubectl.log" || true)"
[[ -n "$hub_patches" ]] || fail 'candidate hub Deployment must be patched'
for ref in "voice-nats-jsdata-${generation}" "voice-nats-operator-${generation}" "voice-nats-hub-tls-${generation}"; do
  [[ "$hub_patches" == *"$ref"* ]] || fail "hub must bind ${ref}"
done
! grep -Eq '^delete (pvc|persistentvolumeclaim) |^delete secret voice-nats-(operator|hub-tls|bootstrap-credentials|service-credentials)( |$)' "$work/kubectl.log" || fail 'activation must retain previous Secrets and PVC for rollback'
! grep -Eq '^delete namespace |^delete (secret|pvc|persistentvolumeclaim) (voice-postgres|voice-minio|voice-app)' "$work/kubectl.log" || fail 'rotation must not delete non-NATS resources'
! grep -Eq '^patch service voice-nats |^delete service voice-nats ' "$work/kubectl.log" || fail 'rotation must preserve stable voice-nats Service and DNS'
! grep -Eq 'render-and-apply.sh|apply-infra.sh|apply-app-manifests.sh' "$work/kubectl.log" || fail 'NATS-only rotation must not invoke full infrastructure/app apply'
! grep -Eq '^(apply|create|delete|patch|replace|scale|set|annotate|label) .*voice-(postgres|redis|minio|clickhouse|livekit|app-secrets)|^delete namespace |^apply .*deploy/staging/(infra|services|gateway)' "$work/kubectl.log" || fail 'rotation mutated a non-NATS resource'
! awk '$1 ~ /^(Service|StatefulSet|Deployment|Secret|PersistentVolumeClaim|ConfigMap|Job|NetworkPolicy)$/ && $2 !~ /^voice-nats/ {bad=1} END {exit !bad}' "$work/rendered/metadata.names" || fail 'rotation applied a non-NATS manifest'
for service in auth social user role space chat file messaging voice matchmaking search notification realtime bot subscription moderation story analytics; do
  grep -Eq "^(scale|patch) .*deployment[/[:space:]]+voice-${service}.*(replicas[=: ]+0|\\\"replicas\\\":0)" "$work/kubectl.log" || fail "${service} leaf must stop before hub switch"
  grep -Eq "^(scale|patch) .*deployment[/[:space:]]+voice-${service}.*(replicas[=: ]+1|\\\"replicas\\\":1)" "$work/kubectl.log" || fail "${service} leaf must restart after bootstrap"
  leaf_patches="$(grep -E "^patch .*deployment[/[:space:]]+voice-${service}( |$)" "$work/kubectl.log" || true)"
  [[ "$leaf_patches" == *"voice-nats-service-credentials-${generation}"* ]] || fail "${service} leaf must bind new service credentials"
  [[ "$leaf_patches" == *"voice-nats-hub-tls-${generation}"* ]] || fail "${service} leaf must bind new TLS CA"
done
! grep -Eq '^(scale|patch) .*deployment[/[:space:]]+voice-gateway' "$work/kubectl.log" || fail 'Gateway has no staging NATS leaf and must stay outside rotation'
for bootstrap in realtime notification search analytics-chat; do
  grep -Fq "Job voice-nats-${bootstrap}-bootstrap voice-staging" "$work/rendered/metadata.names" || fail "new generation must apply ${bootstrap} bootstrap Job"
  grep -Eq "^delete job[/[:space:]]+voice-nats-${bootstrap}-bootstrap" "$work/kubectl.log" || fail "new generation must replace stale ${bootstrap} Job"
  grep -Eq "^get job voice-nats-${bootstrap}-bootstrap " "$work/kubectl.log" || fail "new generation must verify ${bootstrap} bootstrap completion"
done
grep -Fq 'Job voice-nats-realtime-permissions-preflight voice-staging' "$work/rendered/metadata.names" || fail 'new generation must apply Realtime permissions preflight'
grep -Eq '^get job voice-nats-realtime-permissions-preflight ' "$work/kubectl.log" || fail 'new generation must verify Realtime permissions preflight completion'
proof_name="$(awk '$1 == "Secret" && $2 ~ /^voice-nats-acl-proof-/ {print $2}' "$work/rendered/metadata.names" | sort -u)"
[[ "$proof_name" =~ ^voice-nats-acl-proof-r20260930a-[a-z0-9-]+$ ]] || fail 'activation must create a uniquely named proof Secret'
for kind in Secret NetworkPolicy Job; do
  [[ "$(grep -Ec "^${kind} ${proof_name} voice-staging" "$work/rendered/metadata.names" || true)" == 1 ]] || fail "activation must create one temporary proof ${kind}"
done
grep -Fq "Secret ${proof_name} voice-staging immutable=true" "$work/rendered/metadata.names" || fail 'temporary proof Secret must be immutable'
for kind in job networkpolicy secret; do
  [[ "$(grep -Ec "^delete ${kind} ${proof_name} -n voice-staging " "$work/kubectl.log" || true)" == 1 ]] || fail "activation must delete temporary proof ${kind} exactly once"
  grep -Eq "^get ${kind} ${proof_name} -n voice-staging " "$work/kubectl.log" || fail "activation must verify temporary proof ${kind} is absent"
done
grep -Fq "NATS_LIVE_ACL_PROOF=PASS generation=${generation} acl_sha=${acl_sha}" "$work/output" || fail 'activation must report the exact live proof PASS token'
hub_ready_line="$(grep -nm1 -E '^rollout status deployment/voice-nats-pvc-candidate' "$work/kubectl.log" | cut -d: -f1)"
[[ -n "$hub_ready_line" ]] || fail 'rotation must wait for the new hub'
rotating_line="$(grep -nm1 '^event phase=rotating$' "$work/kubectl.log" | cut -d: -f1)"
hub_patch_line="$(grep -nm1 -E '^patch .*deployment[/[:space:]]+voice-nats-pvc-candidate' "$work/kubectl.log" | cut -d: -f1)"
[[ -n "$rotating_line" && -n "$hub_patch_line" && "$rotating_line" -lt "$hub_patch_line" ]] || fail 'rotating marker must precede hub volume switch'
last_stop_line="$(grep -nE '^(scale|patch) .*deployment[/[:space:]]+voice-(auth|social|user|role|space|chat|file|messaging|voice|matchmaking|search|notification|realtime|bot|subscription|moderation|story|analytics).*(replicas[=: ]+0|\\\"replicas\\\":0)' "$work/kubectl.log" | tail -1 | cut -d: -f1)"
[[ -n "$last_stop_line" && "$last_stop_line" -lt "$hub_patch_line" ]] || fail 'all 18 leaves must stop before hub volume switch'
last_bootstrap_line="$(grep -nE '^get job voice-nats-((realtime|notification|search|analytics-chat)-bootstrap|realtime-permissions-preflight)' "$work/kubectl.log" | tail -1 | cut -d: -f1)"
proof_logs_line="$(grep -nF "logs job/${proof_name} " "$work/kubectl.log" | sed -n '1p' | cut -d: -f1)"
first_start_line="$(grep -nm1 -E '^(scale|patch) .*deployment[/[:space:]]+voice-(auth|social|user|role|space|chat|file|messaging|voice|matchmaking|search|notification|realtime|bot|subscription|moderation|story|analytics).*(replicas[=: ]+1|\\\"replicas\\\":1)' "$work/kubectl.log" | cut -d: -f1)"
[[ -n "$last_bootstrap_line" && -n "$first_start_line" && "$last_bootstrap_line" -lt "$first_start_line" ]] || fail 'all bootstrap Jobs must complete before a leaf starts'
[[ -n "$proof_logs_line" && "$last_bootstrap_line" -lt "$proof_logs_line" && "$proof_logs_line" -lt "$first_start_line" ]] || fail 'live ACL proof must complete between bootstrap and first leaf restart'
last_leaf_start_line="$(grep -nE '^scale deployment/voice-(auth|social|user|role|space|chat|file|messaging|voice|matchmaking|search|notification|realtime|bot|subscription|moderation|story|analytics) .*--replicas=1' "$work/kubectl.log" | tail -1 | cut -d: -f1)"
first_leaf_ready_line="$(grep -nm1 -E '^rollout status deployment/voice-(auth|social|user|role|space|chat|file|messaging|voice|matchmaking|search|notification|realtime|bot|subscription|moderation|story|analytics)( |$)' "$work/kubectl.log" | cut -d: -f1)"
[[ -n "$last_leaf_start_line" && -n "$first_leaf_ready_line" && "$last_leaf_start_line" -lt "$first_leaf_ready_line" ]] || fail 'rotation must start mutually dependent leaves before readiness waits'
active_line="$(grep -n '^event phase=active$' "$work/kubectl.log" | tail -1 | cut -d: -f1)"
last_leaf_ready_line="$(grep -nE '^rollout status deployment/voice-(auth|social|user|role|space|chat|file|messaging|voice|matchmaking|search|notification|realtime|bot|subscription|moderation|story|analytics)( |$)' "$work/kubectl.log" | tail -1 | cut -d: -f1)"
[[ -n "$active_line" && -n "$last_leaf_ready_line" && "$last_leaf_ready_line" -lt "$active_line" ]] || fail 'active marker must follow leaf rollout convergence'
grep -Eq 'voice-nats-generation|generation.*r20260930a' "$work/mutations.log" "$work/rendered/metadata.names" || fail 'active generation marker must be recorded'

for failure in deadline failed-pod; do
  if MOCK_FAIL_JOB="$failure" run_rotation --activate "$generation" "$work/bundle.json" "$work/proof.creds"; then
    fail "terminal Realtime preflight ${failure} must stop activation"
  fi
  grep -Fq "NATS_JOB=FAILED job=voice-nats-realtime-permissions-preflight category=${failure}" "$work/output" ||
    fail "preflight ${failure} did not report a bounded failure category"
  ! grep -Eq '^scale deployment/voice-(auth|social|user|role|space|chat|file|messaging|voice|matchmaking|search|notification|realtime|bot|subscription|moderation|story|analytics) .*--replicas=1' "$work/kubectl.log" || fail 'failed preflight started a leaf'
  ! grep -Fxq 'event phase=active' "$work/kubectl.log" || fail 'failed preflight activated the marker'
  [[ ! -f "$work/state/user-override" ]] || fail 'failed preflight altered User override'
done
for failure in space restored-user remove; do
  if MOCK_FAIL_STAGE="$failure" run_rotation --activate "$generation" "$work/bundle.json" "$work/proof.creds"; then
    fail "User/Space ${failure} failure must stop activation"
  fi
  ! grep -Fxq 'event phase=active' "$work/kubectl.log" || fail 'User/Space failure activated the marker'
  if [[ "$failure" == remove ]]; then
    [[ -f "$work/state/user-override" ]] || fail 'failed removal lost the owned override evidence'
  else
    [[ ! -f "$work/state/user-override" ]] || fail 'User/Space failure left temporary override without a cleanup error'
  fi
done

if MOCK_GENERATION_STATE=active run_rotation --activate "$generation" "$work/bundle.json" "$work/proof.creds"; then
  fail 'repeated activation of the already active generation must fail closed'
fi
assert_no_mutation
if MOCK_GENERATION_STATE=active MOCK_SOURCE_PVC_CLASS='bad/class' run_rotation --rollback; then
  fail 'rollback must reject an unsafe retained PVC storage class before mutation'
fi
assert_no_mutation

MOCK_GENERATION_STATE=active run_rotation --rollback || fail "rollback to retained legacy generation failed: $(tail -1 "$work/output")"
assert_nats_only_mutations
last_leaf_start_line="$(grep -nE '^scale deployment/voice-(auth|social|user|role|space|chat|file|messaging|voice|matchmaking|search|notification|realtime|bot|subscription|moderation|story|analytics) .*--replicas=1' "$work/kubectl.log" | tail -1 | cut -d: -f1)"
first_leaf_ready_line="$(grep -nm1 -E '^rollout status deployment/voice-(auth|social|user|role|space|chat|file|messaging|voice|matchmaking|search|notification|realtime|bot|subscription|moderation|story|analytics)( |$)' "$work/kubectl.log" | cut -d: -f1)"
[[ -n "$last_leaf_start_line" && -n "$first_leaf_ready_line" && "$last_leaf_start_line" -lt "$first_leaf_ready_line" ]] || fail 'rollback must start mutually dependent leaves before readiness waits'
grep -Eq '(generation[=:\\" ]+legacy|generation=legacy)' "$work/mutations.log" "$work/rendered/metadata.names" || fail 'rollback must record legacy as the active generation'
grep -Eq '(phase[=:\\" ]+active|phase=active)' "$work/kubectl.log" "$work/mutations.log" "$work/rendered/metadata.names" || fail 'rollback must restore an active marker'
! grep -Eq '^delete (secret|pvc|persistentvolumeclaim) ' "$work/kubectl.log" || fail 'rollback must retain generation resources'
! grep -Eq '^patch service voice-nats |^delete service voice-nats ' "$work/kubectl.log" || fail 'rollback must preserve stable Service'
rollback_hub_patches="$(grep -E '^patch .*deployment[/[:space:]]+voice-nats-pvc-candidate' "$work/kubectl.log" || true)"
for ref in voice-nats-jsdata voice-nats-operator voice-nats-hub-tls; do
  [[ "$rollback_hub_patches" == *"$ref"* ]] || fail "rollback must restore retained hub ${ref}"
done
[[ "$rollback_hub_patches" != *"-${generation}"* ]] || fail 'rollback hub must not retain a new-generation volume ref'
for service in auth social user role space chat file messaging voice matchmaking search notification realtime bot subscription moderation story analytics; do
  grep -Eq "^patch .*deployment[/[:space:]]+voice-${service}.*voice-nats-service-credentials" "$work/kubectl.log" || fail "rollback must restore ${service} service credential ref"
  grep -Eq "^patch .*deployment[/[:space:]]+voice-${service}.*voice-nats-hub-tls" "$work/kubectl.log" || fail "rollback must restore ${service} TLS ref"
  ! grep -Eq "^patch .*deployment[/[:space:]]+voice-${service}.*voice-nats-(service-credentials|hub-tls)-${generation}" "$work/kubectl.log" || fail "rollback must remove ${service} new-generation refs"
done
grep -Eq '^rollout status deployment/voice-nats-pvc-candidate' "$work/kubectl.log" || fail 'rollback must wait for the retained hub'
if run_rotation --rollback; then
  fail 'rollback without an active generation marker must fail'
fi
assert_no_mutation

echo 'staging NATS root rotation contract: OK'
