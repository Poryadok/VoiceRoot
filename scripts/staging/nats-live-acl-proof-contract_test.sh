#!/usr/bin/env bash
# Synthetic contract for the staging-only live NATS ACL proof gate. No cluster,
# signing seed, real credential, or GitHub Environment secret is used here.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ROTATE="$ROOT/scripts/staging/rotate-nats-root.sh"
WORKFLOW="$ROOT/.github/workflows/staging-nats-root-rotation.yml"
GEN=r20260930a
ACL_SHA="$(sha256sum "$ROOT/deploy/nats/acl-intent.yaml" | cut -d' ' -f1)"
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
[[ -x "$ROTATE" ]] || fail 'rotation entrypoint is not executable'
grep -Fq 'image="$registry/realtime:$GITHUB_SHA"' "$WORKFLOW" || fail 'workflow proof image must use the exact master SHA'
grep -Fq 'docker --config "$docker_config" manifest inspect "$image"' "$WORKFLOW" || fail 'workflow must prove the exact Realtime image exists'
grep -Fq 'VOICE_NATS_PROOF_IMAGE_TAG=%s\n' "$WORKFLOW" || fail 'workflow must pass the exact proof image SHA to rotation'
grep -Fq 'STAGING_NATS_PROOF_CREDS_B64: ${{ secrets.STAGING_NATS_PROOF_CREDS_B64 }}' "$WORKFLOW" || fail 'workflow must use separate protected proof credential'
grep -Fq 'bash scripts/staging/rotate-nats-root.sh --activate "$ROTATION_GENERATION" "$bundle" "$proof"' "$WORKFLOW" || fail 'workflow must pass the decoded proof credential file'
image_check_line="$(grep -nF 'docker --config "$docker_config" manifest inspect "$image"' "$WORKFLOW" | sed -n '1p' | cut -d: -f1)"
activate_line="$(grep -nF 'bash scripts/staging/rotate-nats-root.sh --activate "$ROTATION_GENERATION" "$bundle" "$proof"' "$WORKFLOW" | sed -n '1p' | cut -d: -f1)"
((image_check_line < activate_line)) || fail 'workflow must verify proof image before any activation mutation'

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/state" "$work/rendered"
cat >"$work/bin/kubectl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$KUBECTL_LOG"
case "$1" in get|create|apply|patch|delete|scale|wait|rollout|logs) ;; *) echo 'unexpected kubectl verb' >&2; exit 2 ;; esac
case "$1" in create|apply|patch|delete|scale)
  [[ "$*" == *'--dry-run'* ]] || printf '%s\n' "$*" >>"$MUTATION_LOG" ;;
esac
manifest=''
for ((i=1; i<=$#; i++)); do
  if [[ "${!i}" == -f ]] && ((i < $#)); then next=$((i+1)); manifest="${!next}"; fi
done
if [[ "$manifest" == - ]]; then
  # The only JSON stdin is the ephemeral proof Secret. Never retain data.
  input="$(mktemp "$STATE/manifest.XXXXXX")"
  cat >"$input"
  if jq -e . "$input" >/dev/null 2>&1; then
    jq -r '
      (.kind + " " + .metadata.name + " " + .metadata.namespace),
      ("namespace " + .metadata.namespace),
      ("immutable " + .kind + " " + .metadata.name + " " + (.immutable | tostring)),
      (.data | keys[] | "key " + .),
      (.metadata.annotations["voice.io/nats-generation"] // empty | "annotation generation Secret " + .),
      (.metadata.labels["voice.io/nats-proof-run"] // empty | "proof-run Secret " + .)
    ' "$input" >"$RENDERED/latest"
  else
    # Record only Kubernetes metadata and refs from YAML. No Secret payload.
    awk '/^kind:/ {kind=$2} /^metadata:/ {metadata=1; next}
    metadata && /^  name:/ {name=$2; print kind, name, "voice-staging"}
    metadata && /^  namespace:/ {print "namespace", $2; metadata=0}
    /^[[:space:]]*proof.creds:/ {print "key proof.creds"}
    /voice.io\/nats-generation:/ {print "annotation generation", kind, $2}
    /voice.io\/nats-proof-run:/ {line=$0; sub(/^.*voice.io\/nats-proof-run:[[:space:]]*/, "", line); sub(/[},].*$/, "", line); print "proof-run", kind, line}
    /^[[:space:]]*app:/ {print "app-label", kind, $2}
    kind == "NetworkPolicy" && /matchLabels: \{app:/ {line=$0; sub(/^.*app:[[:space:]]*/, "", line); sub(/[},].*$/, "", line); print "policy-target", line}
    kind == "NetworkPolicy" && /^[[:space:]]*- podSelector:/ {print "policy-source-entry"}
    kind == "NetworkPolicy" && /port: [0-9]+/ {line=$0; sub(/^.*port:[[:space:]]*/, "", line); sub(/[^0-9].*$/, "", line); print "policy-port", line}
    kind == "NetworkPolicy" && /protocol:/ {line=$0; sub(/^.*protocol:[[:space:]]*/, "", line); sub(/[,}].*$/, "", line); print "policy-protocol", line}
    kind == "NetworkPolicy" && /namespaceSelector:|ipBlock:|podSelector: \{\}|- \{\}/ {print "policy-broad-selector"}
    /name: REALTIME_NATS_(HUB_URL|PROOF_URL)/ {line=$0; sub(/^.*name:[[:space:]]*/, "", line); sub(/[,}].*$/, "", line); print "proof-env", line}
    /^[[:space:]]*- name: (realtime-leaf|proof-leaf)/ {print "proof-container", $3}
    /image:/ {print "image", $2}
    /imagePullSecrets:/ {print "image-pull", $0}
    /^[[:space:]]*immutable:/ {print "immutable", kind, name, $2}
    /claimName:|secretName:/ {print "ref", $2}
    /^[[:space:]]*(phase|generation|previousGeneration):/ {print "marker", $1, $2}' \
    "$input" >"$RENDERED/latest"
  fi
  rm -f "$input"
  if [[ "$*" != *'--dry-run'* ]]; then
    cat "$RENDERED/latest" >>"$RENDERED/all"
    awk '$1 == "Secret" || $1 == "NetworkPolicy" || $1 == "Job" {print "event created", $1, $2}' "$RENDERED/latest" >>"$KUBECTL_LOG"
    if grep -q '^ConfigMap voice-nats-generation ' "$RENDERED/latest"; then printf 'rotating\n' >"$STATE/marker"; fi
    if grep -q '^Secret voice-nats-acl-proof-' "$RENDERED/latest"; then
      awk '$1 == "Secret" && $2 ~ /^voice-nats-acl-proof-/ {print $2}' "$RENDERED/latest" >>"$STATE/proof-secret-names"
    fi
  fi
elif [[ -n "$manifest" && "$manifest" != - && "$1" == create && "$*" != *'--dry-run'* ]]; then
  if [[ "$manifest" == *.json ]]; then
    jq -r '(.items // [.])[] | .kind + " " + .metadata.name + " " + .metadata.namespace' "$manifest" >>"$RENDERED/all"
  else
    echo 'unrecognized kubectl manifest path' >&2; exit 2
  fi
fi
if [[ "$1" == create && "$2" == secret && "$3" == generic ]]; then
  [[ "$*" == *'--dry-run=client'* && "$*" == *'--from-file=proof.creds='* && "$*" != *'--from-literal='* ]] || {
    echo 'mock rejects an unsafe proof Secret create' >&2; exit 2;
  }
  printf '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"%s","namespace":"voice-staging"},"type":"Opaque","data":{"proof.creds":"eA=="}}\n' "$4"
fi
if [[ "$1" == patch && "$2" == configmap && "$3" == voice-nats-generation ]]; then
  patch=''
  for ((i=1; i<=$#; i++)); do if [[ "${!i}" == -p ]] && ((i < $#)); then next=$((i+1)); patch="${!next}"; fi; done
  phase="$(jq -r '.[] | select(.op == "replace" and .path == "/data/phase") | .value' <<<"$patch")"
  printf '%s\n' "$phase" >"$STATE/marker"
  printf 'marker phase %s\n' "$phase" >>"$RENDERED/all"
fi
if [[ "$1" == patch && "$2" == deployment && "$3" == voice-* ]]; then
  patch=''
  for ((i=1; i<=$#; i++)); do if [[ "${!i}" == -p ]] && ((i < $#)); then next=$((i+1)); patch="${!next}"; fi; done
  jq -e 'any(.[]; .op == "test" and .path == "/metadata/resourceVersion" and .value == "100")' <<<"$patch" >/dev/null || exit 2
  if [[ "$3" == voice-user ]] && jq -e 'any(.[]; .op == "add" and .path == "/spec/template/metadata/annotations/voice.io~1nats-user-space-bootstrap")' <<<"$patch" >/dev/null; then
    touch "$STATE/user-override"
  elif [[ "$3" == voice-user ]] && jq -e 'any(.[]; .op == "remove" and .path == "/spec/template/metadata/annotations/voice.io~1nats-user-space-bootstrap")' <<<"$patch" >/dev/null; then
    rm -f "$STATE/user-override"
  elif jq -e 'any(.[]; .path | endswith("/secret/secretName"))' <<<"$patch" >/dev/null; then
    if jq -e 'any(.[]; .value == "voice-nats-service-credentials-r20260930a")' <<<"$patch" >/dev/null; then
      printf '%s' '-r20260930a' >"$STATE/leaf-${3#voice-}-suffix"
    else
      : >"$STATE/leaf-${3#voice-}-suffix"
    fi
  fi
fi
if [[ "$1" == delete && "$2" == secret && "$3" == voice-nats-acl-proof-* && "${MOCK_CLEANUP_FAIL:-false}" == true ]]; then
  echo 'simulated proof Secret cleanup error' >&2; exit 1
fi
if [[ "$1" == delete && "$2" == secret && "$3" == voice-nats-acl-proof-* ]]; then
  touch "$STATE/proof-secret-delete-attempted"
fi
if [[ "$1" == delete && "$3" == voice-nats-acl-proof-* && "${MOCK_LEFTOVER_MODE:-none}" != none ]]; then
  [[ "$*" == *'-n voice-staging'* ]] || { echo 'proof cleanup escaped staging namespace' >&2; exit 2; }
  if [[ "$2" == networkpolicy && "${MOCK_LEFTOVER_MODE:-none}" == delete_fail ]]; then
    echo 'simulated interrupted rollback policy deletion' >&2; exit 1
  fi
  touch "$STATE/leftover-$2-deleted"
fi
if [[ "$1" == wait && "$*" == *'job/voice-nats-acl-proof-'* && "${MOCK_PROOF_RESULT:-pass}" == deny ]]; then
  echo 'simulated proof Job denied' >&2; exit 1
fi
if [[ "$1" == logs && "$*" == *'voice-nats-acl-proof-'* ]]; then
  case "${MOCK_PROOF_RESULT:-pass}" in
    pass|cleanup-fail) printf 'NATS_LIVE_ACL_PROOF=PASS generation=r20260930a acl_sha=%s\n' "$MOCK_ACL_SHA" ;;
    malformed) printf 'NATS_LIVE_ACL_PROOF=PASS generation=legacy acl_sha=%s\n' "$MOCK_ACL_SHA" ;;
    deny) printf 'NATS_LIVE_ACL_PROOF=FAIL code=publish_denied\n' ;;
    missing) ;;
  esac
  exit 0
fi
proof_item() {
  jq -cn --arg kind "$1" --arg name voice-nats-acl-proof-r20260930a-123456-1 '
    {kind:$kind,metadata:{name:$name,namespace:"voice-staging",
      labels:{"voice.io/nats-proof":"true","voice.io/nats-proof-generation":"r20260930a","voice.io/nats-proof-run":$name},
      annotations:{"voice.io/nats-generation":"r20260930a"}}}'
}
case "$*" in
  *'get jobs,networkpolicies,secrets -n voice-staging -l voice.io/nats-proof=true,voice.io/nats-proof-generation=r20260930a -o json'*)
    items=()
    if [[ "${MOCK_LEFTOVER_MODE:-none}" != none ]]; then
      for entry in 'job Job' 'networkpolicy NetworkPolicy' 'secret Secret'; do
        read -r resource kind <<<"$entry"
        [[ -f "$STATE/leftover-$resource-deleted" ]] || items+=("$(proof_item "$kind")")
      done
    fi
    printf '%s\n' "${items[@]}" | jq -cs '{apiVersion:"v1",kind:"List",items:.}' | if [[ "${MOCK_LEFTOVER_MODE:-none}" == mismatch ]]; then
      jq '.items[2].metadata.name = "voice-postgres"'
    elif [[ "${MOCK_LEFTOVER_MODE:-none}" == duplicate ]]; then
      jq '.items += [.items[0]]'
    else
      cat
    fi
    ;;
  *'get configmap voice-nats-generation'*)
    if [[ "${MOCK_GENERATION_STATE:-absent}" == rotating || -f "$STATE/marker" ]]; then
      printf '%s\n' '{"kind":"ConfigMap","metadata":{"name":"voice-nats-generation","namespace":"voice-staging"},"data":{"phase":"rotating","generation":"r20260930a","previousGeneration":"legacy"}}'
      exit 0
    fi
    echo 'Error from server (NotFound): configmaps "voice-nats-generation" not found' >&2; exit 1 ;;
  *'get secret voice-proof-ghcr'*)
    if [[ "${MOCK_PULL_SECRET_BAD:-false}" == true ]]; then
      printf '%s\n' '{"kind":"Secret","metadata":{"name":"voice-proof-ghcr","namespace":"voice-staging"},"type":"Opaque","data":{".dockerconfigjson":"eA=="}}'
    else
      config='{"auths":{"ghcr.io":{"auth":"eA=="}}}'
      jq -cn --arg encoded "$(printf '%s' "$config" | base64 -w0)" '{kind:"Secret",metadata:{name:"voice-proof-ghcr",namespace:"voice-staging"},type:"kubernetes.io/dockerconfigjson",data:{".dockerconfigjson":$encoded}}'
    fi
    ;;
  *'get configmap voice-app-config'*)
    printf '%s\n' '{"kind":"ConfigMap","metadata":{"name":"voice-app-config","namespace":"voice-staging"},"data":{"SPACE_GRPC_ADDR":"voice-space:9090"}}' ;;
  *'get secret voice-nats-acl-proof-'*)
    if [[ "${MOCK_LEFTOVER_MODE:-none}" != none && ! -f "$STATE/leftover-secret-deleted" ]]; then
      proof_item Secret; exit 0
    fi
    if [[ "${MOCK_PROOF_REMAINS:-false}" == true && -f "$STATE/proof-secret-delete-attempted" ]]; then
      printf '%s\n' '{"kind":"Secret","metadata":{"name":"voice-nats-acl-proof-r20260930a-123461-1","namespace":"voice-staging"}}'
      exit 0
    fi
    echo 'Error from server (NotFound): temporary proof Secret not found' >&2; exit 1 ;;
  *'get secret '*'-r20260930a'*|*'get pvc voice-nats-jsdata-r20260930a'*)
    echo 'Error from server (NotFound): generation resource not found' >&2; exit 1 ;;
  *'get networkpolicy voice-nats-acl-proof-'*|*'get networkpolicy/voice-nats-acl-proof-'*)
    if [[ "${MOCK_LEFTOVER_MODE:-none}" != none && ! -f "$STATE/leftover-networkpolicy-deleted" ]]; then
      proof_item NetworkPolicy; exit 0
    fi
    echo 'Error from server (NotFound): temporary proof NetworkPolicy not found' >&2; exit 1 ;;
  *'get secret voice-nats-'*)
    name="$3"
    jq -c --arg name "$name" '.items[] | select(.metadata.name == ($name + "-r20260930a")) | .metadata.name = $name | del(.immutable)' "$BUNDLE" ;;
  *'get pvc voice-nats-jsdata '*)
    printf '%s\n' '{"kind":"PersistentVolumeClaim","metadata":{"name":"voice-nats-jsdata","namespace":"voice-staging"},"spec":{"storageClassName":"local-path","accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}' ;;
  *'get service voice-nats'*)
    printf '%s\n' '{"kind":"Service","metadata":{"name":"voice-nats","namespace":"voice-staging"},"spec":{"selector":{"app":"voice-nats-pvc-candidate"}}}' ;;
  *'get deployment voice-nats '*)
    echo 'Error from server (NotFound): deployments "voice-nats" not found' >&2; exit 1 ;;
  *'get deployment voice-nats-pvc-candidate'*)
    printf '%s\n' '{"kind":"Deployment","metadata":{"name":"voice-nats-pvc-candidate","namespace":"voice-staging","resourceVersion":"100"},"spec":{"replicas":1,"selector":{"matchLabels":{"app":"voice-nats-pvc-candidate"}},"template":{"metadata":{"labels":{"app":"voice-nats-pvc-candidate"}},"spec":{"containers":[{"name":"nats"}],"volumes":[{"name":"jsdata","persistentVolumeClaim":{"claimName":"voice-nats-jsdata"}},{"name":"nats-resolver-input","secret":{"secretName":"voice-nats-operator"}},{"name":"nats-operator-jwt","secret":{"secretName":"voice-nats-operator"}},{"name":"nats-hub-tls","secret":{"secretName":"voice-nats-hub-tls"}}]}}}}' ;;
  *'get deployment voice-'*)
    service="${3#voice-}"
    image="ghcr.io/example/voice/${service}:0123456789abcdef0123456789abcdef01234567"
    suffix=''; [[ ! -f "$STATE/leaf-${service}-suffix" ]] || suffix="$(cat "$STATE/leaf-${service}-suffix")"
    owned=false; [[ "$service" != user || ! -f "$STATE/user-override" ]] || owned=true
    jq -n --arg s "$service" --arg image "$image" --arg suffix "$suffix" --argjson owned "$owned" '{kind:"Deployment",metadata:{name:("voice-"+$s),namespace:"voice-staging",resourceVersion:"100"},spec:{replicas:1,template:{metadata:{annotations:(if $owned then {"voice.io/nats-user-space-bootstrap":"r20260930a"} else {} end)},spec:{containers:[{name:$s,image:$image,env:(if $s == "user" and $owned then [{name:"SPACE_GRPC_ADDR"}] else [] end),envFrom:(if $s == "user" then [{configMapRef:{name:"voice-app-config"}}] else [] end)},{name:"nats-leaf"}],volumes:[{name:"nats-service-creds",secret:{secretName:("voice-nats-service-credentials"+$suffix),items:[{key:($s+".creds"),path:($s+".creds")}] }},{name:"nats-hub-tls",secret:{secretName:("voice-nats-hub-tls"+$suffix),items:[{key:"ca.crt",path:"ca.crt"}]}}]}}}}' ;;
  *'get pods -n voice-staging -l app=voice-user -o json'*)
    if [[ -f "$STATE/user-override" ]]; then
      printf '%s\n' '{"items":[{"metadata":{"name":"voice-user-pod","labels":{"app":"voice-user"}},"spec":{"containers":[{"name":"user","env":[{"name":"SPACE_GRPC_ADDR"}]}]},"status":{"phase":"Running","containerStatuses":[{"name":"user","ready":true}]}}]}'
    else
      printf '%s\n' '{"items":[{"metadata":{"name":"voice-user-pod","labels":{"app":"voice-user"}},"spec":{"containers":[{"name":"user","env":[]}]},"status":{"phase":"Running","containerStatuses":[{"name":"user","ready":true}]}}]}'
    fi ;;
  *'get pods -n voice-staging -l app=voice-'*) ;;
  *'get job voice-nats-'*'-bootstrap'*|*'get job voice-nats-realtime-permissions-preflight '*)
    job="$3"; generation=r20260930a
    [[ "${MOCK_GENERATION_STATE:-absent}" != rotating ]] || generation=legacy
    jq -n --arg name "$job" --arg gen "$generation" '{kind:"Job",metadata:{name:$name,namespace:"voice-staging",annotations:{"voice.io/nats-generation":$gen}},status:{conditions:[{type:"Complete",status:"True"}],succeeded:1}}' ;;
  *'get job voice-nats-acl-proof-'*)
    if [[ "${MOCK_LEFTOVER_MODE:-none}" != none && ! -f "$STATE/leftover-job-deleted" ]]; then
      proof_item Job; exit 0
    fi
    echo 'Error from server (NotFound): proof job not found' >&2; exit 1 ;;
  *)
    [[ "$1" != get ]] || { echo 'unexpected Kubernetes read' >&2; exit 2; } ;;
esac
MOCK
chmod +x "$work/bin/kubectl"
cat >"$work/bin/go" <<'MOCK_GO'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${GO_LOG:?}"
[[ "$*" == *'run ./cmd/nats-proof-credential --check-min-validity 30m'* ]] || {
  echo 'mock issuer rejects missing 30-minute proof expiry preflight' >&2; exit 2;
}
[[ "$*" == *'--generation r20260930a --namespace voice-staging'* ]] || {
  echo 'mock issuer rejects wrong proof generation or namespace' >&2; exit 2;
}
credential=''
for ((i=1; i<=$#; i++)); do
  if [[ "${!i}" == --credential ]] && ((i < $#)); then next=$((i+1)); credential="${!next}"; fi
done
[[ -f "$credential" && -s "$credential" && "${MOCK_CREDENTIAL_VALID:-true}" == true ]] || {
  echo 'synthetic issuer rejects missing, malformed, or near-expiry proof credential' >&2; exit 1;
}
MOCK_GO
chmod +x "$work/bin/go"
cat >"$work/bin/docker" <<'MOCK_DOCKER'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${DOCKER_LOG:?}"
[[ "$1" == --config && "$3" == manifest && "$4" == inspect && "$5" == ghcr.io/poryadok/voiceroot/realtime:0123456789abcdef0123456789abcdef01234567 ]] || exit 2
jq -e '(.auths["ghcr.io"] // .auths["https://ghcr.io"]) | type == "object"' "$2/config.json" >/dev/null || exit 2
[[ "${MOCK_DOCKER_RESULT:-pass}" == pass ]] || exit 1
MOCK_DOCKER
chmod +x "$work/bin/docker"

# The four-Secret bundle is deliberately inert. TLS is local, ephemeral, and
# valid only to exercise the rotation preflight; all other payloads are "x".
umask 077
MSYS2_ARG_CONV_EXCL='/CN=' openssl req -x509 -newkey rsa:2048 -nodes -days 1 \
  -subj '/CN=voice-nats' -addext 'subjectAltName=DNS:voice-nats' \
  -keyout "$work/tls.key" -out "$work/tls.crt" >/dev/null 2>&1
jq -n --arg g "$GEN" --rawfile cert "$work/tls.crt" --rawfile key "$work/tls.key" '
  {apiVersion:"v1",kind:"List",items:[
    {apiVersion:"v1",kind:"Secret",metadata:{name:("voice-nats-operator-"+$g),namespace:"voice-staging"},type:"Opaque",immutable:true,data:{"operator.jwt":"eA==","account.jwt":"eA==","system-account.jwt":"eA==","account.public":"eA==","system-account.public":"eA=="}},
    {apiVersion:"v1",kind:"Secret",metadata:{name:("voice-nats-hub-tls-"+$g),namespace:"voice-staging"},type:"Opaque",immutable:true,data:{"tls.crt":($cert|@base64),"tls.key":($key|@base64),"ca.crt":($cert|@base64)}},
    {apiVersion:"v1",kind:"Secret",metadata:{name:("voice-nats-bootstrap-credentials-"+$g),namespace:"voice-staging"},type:"Opaque",immutable:true,data:{"bootstrap.creds":"eA=="}},
    {apiVersion:"v1",kind:"Secret",metadata:{name:("voice-nats-service-credentials-"+$g),namespace:"voice-staging"},type:"Opaque",immutable:true,data:(["analytics","auth","bot","chat","file","gateway","matchmaking","messaging","moderation","notification","realtime","role","search","social","space","story","subscription","user","voice"] | map({key:(.+".creds"),value:"eA=="}) | from_entries)}
  ]}' >"$work/bundle.json"
printf 'synthetic-proof-credential-do-not-log\n' >"$work/proof.creds"

run_rotation() {
  local rc=0
  : >"$work/kubectl.log"
  : >"$work/mutations.log"
  : >"$work/go.log"
  : >"$work/docker.log"
  : >"$work/rendered/all"
  rm -f "$work/state/marker" "$work/state/proof-secret-names" "$work/state/proof-secret-delete-attempted" \
    "$work/state/leftover-job-deleted" "$work/state/leftover-networkpolicy-deleted" "$work/state/leftover-secret-deleted" \
    "$work/state/user-override" "$work/state"/leaf-*-suffix
  PATH="$work/bin:$PATH" KUBECTL_LOG="$work/kubectl.log" MUTATION_LOG="$work/mutations.log" \
    RENDERED="$work/rendered" STATE="$work/state" BUNDLE="$work/bundle.json" \
    GO_LOG="$work/go.log" MOCK_CREDENTIAL_VALID="${MOCK_CREDENTIAL_VALID:-true}" \
    DOCKER_LOG="$work/docker.log" MOCK_DOCKER_RESULT="${MOCK_DOCKER_RESULT:-pass}" \
    MOCK_PULL_SECRET_BAD="${MOCK_PULL_SECRET_BAD:-false}" \
    MOCK_ACL_SHA="$ACL_SHA" MOCK_PROOF_RESULT="${MOCK_PROOF_RESULT:-pass}" \
    MOCK_CLEANUP_FAIL="${MOCK_CLEANUP_FAIL:-false}" \
    MOCK_PROOF_REMAINS="${MOCK_PROOF_REMAINS:-false}" \
    MOCK_GENERATION_STATE="${MOCK_GENERATION_STATE:-absent}" \
    MOCK_LEFTOVER_MODE="${MOCK_LEFTOVER_MODE:-none}" \
    GITHUB_RUN_ID="${MOCK_RUN_ID:-123456}" GITHUB_RUN_ATTEMPT=1 \
    GITHUB_SHA=0123456789abcdef0123456789abcdef01234567 \
    VOICE_NATS_PROOF_IMAGE_REGISTRY="${TEST_PROOF_REGISTRY-ghcr.io/poryadok/voiceroot}" \
    VOICE_NATS_PROOF_IMAGE_TAG="${TEST_PROOF_TAG:-0123456789abcdef0123456789abcdef01234567}" \
    VOICE_NATS_PROOF_IMAGE_PUBLIC="${TEST_PROOF_IMAGE_PUBLIC:-true}" \
    VOICE_NATS_PROOF_IMAGE_PULL_SECRET="${TEST_PROOF_PULL_SECRET:-}" \
    VOICE_K8S_NAMESPACE=voice-staging bash "$ROTATE" "$@" >"$work/output" 2>&1 || rc=$?
  if grep -Eq 'eA==|BEGIN (RSA )?PRIVATE KEY|STAGING_NATS_PROOF_CREDS_B64|synthetic-proof-credential-do-not-log' "$work/output"; then
    fail 'rotation printed credential or environment content'
  fi
  return "$rc"
}

assert_no_mutation() {
  [[ ! -s "$work/mutations.log" ]] || fail 'proof preflight failure mutated Kubernetes'
}

assert_rotating_without_leaves() {
  [[ -f "$work/state/marker" && "$(cat "$work/state/marker")" == rotating ]] || fail 'failed live proof must leave marker rotating'
  ! grep -Eq '^scale deployment/voice-(auth|social|user|role|space|chat|file|messaging|voice|matchmaking|search|notification|realtime|bot|subscription|moderation|story|analytics) .*--replicas=1' "$work/kubectl.log" || fail 'failed live proof restarted a leaf'
  ! grep -q '^marker phase active$' "$work/rendered/all" || fail 'failed live proof activated generation'
}

assert_proof_lifecycle() {
  local proof_name="$1" kind count
  [[ "$proof_name" =~ ^voice-nats-acl-proof-r20260930a-[a-z0-9-]+$ ]] || fail 'temporary proof name is not generation/run-specific'
  for kind in Secret NetworkPolicy Job; do
    count="$(grep -Ec "^${kind} ${proof_name} voice-staging$" "$work/rendered/all" || true)"
    [[ "$count" == 1 ]] || fail "proof ${kind} must be created exactly once with the shared unique name"
  done
  grep -q '^key proof.creds$' "$work/rendered/all" || fail 'temporary Secret must have proof.creds key'
  grep -Fq "immutable Secret ${proof_name} true" "$work/rendered/all" || fail 'temporary proof Secret must be immutable'
  grep -q '^annotation generation Job r20260930a$' "$work/rendered/all" || fail 'proof Job must bind exact generation'
  grep -Fq "proof-run Job ${proof_name}" "$work/rendered/all" || fail 'proof Job must carry its unique run label'
  grep -Fq "proof-run NetworkPolicy ${proof_name}" "$work/rendered/all" || fail 'NetworkPolicy must allow only the unique proof run label'
  [[ "$(grep -c '^policy-source-entry$' "$work/rendered/all" || true)" == 1 ]] || fail 'temporary policy must have exactly one source peer'
  [[ "$(grep -c '^policy-target ' "$work/rendered/all" || true)" == 1 ]] || fail 'temporary policy must have exactly one target selector'
  grep -q '^policy-target voice-nats-pvc-candidate$' "$work/rendered/all" || fail 'temporary policy must target only the NATS hub'
  ! grep -q '^policy-broad-selector$' "$work/rendered/all" || fail 'temporary policy has a broad selector'
  [[ "$(grep '^policy-port ' "$work/rendered/all" | awk '{print $2}' | sort -u | paste -sd, -)" == 4222,7422 ]] || fail 'temporary policy must allow only hub ports 4222 and 7422'
  [[ "$(grep '^policy-protocol ' "$work/rendered/all" | awk '{print $2}' | sort -u | paste -sd, -)" == TCP ]] || fail 'temporary policy must allow only TCP'
  ! grep -Eq '^app-label Job voice-(realtime|nats-pvc-candidate)' "$work/rendered/all" || fail 'proof Job spoofed an existing application selector'
  grep -q '^proof-container realtime-leaf$' "$work/rendered/all" || fail 'proof Job lacks ordinary Realtime leaf'
  grep -q '^proof-container proof-leaf$' "$work/rendered/all" || fail 'proof Job lacks independent proof leaf'
  grep -q '^proof-env REALTIME_NATS_HUB_URL$' "$work/rendered/all" || fail 'proof Job lacks direct hub verification endpoint'
  grep -q '^proof-env REALTIME_NATS_PROOF_URL$' "$work/rendered/all" || fail 'proof Job lacks independent proof leaf endpoint'
  grep -q '^image ghcr.io/poryadok/voiceroot/realtime:0123456789abcdef0123456789abcdef01234567$' "$work/rendered/all" || fail 'proof Job did not use the exact master SHA image'
  for kind in job networkpolicy secret; do
    count="$(grep -Ec "^delete ${kind} ${proof_name} -n voice-staging " "$work/kubectl.log" || true)"
    [[ "$count" == 1 ]] || fail "temporary proof ${kind} deletion was not attempted exactly once in voice-staging"
    grep -Eq "^get ${kind} ${proof_name} -n voice-staging " "$work/kubectl.log" || fail "temporary proof ${kind} absence was not checked after cleanup"
  done
  ! grep -Eq '^(create|apply|delete|patch) (namespace|pvc|persistentvolumeclaim) (voice-prod|voice-postgres|voice-minio|voice-app)' "$work/mutations.log" || fail 'proof touched an unrelated namespace or PVC'
  ! grep -Eq '^delete (namespace|pvc|persistentvolumeclaim) ' "$work/mutations.log" || fail 'proof deleted a namespace or PVC'
  [[ "$(grep -Ec '^create -n voice-staging -f -$|^create -f -$' "$work/kubectl.log" || true)" -ge 4 ]] || fail 'proof resources were not created through manifests'
}

# A missing proof file must be rejected before the rotating marker and any
# target Secret/PVC create. This is the first expected RED on the old script.
if run_rotation --activate "$GEN" "$work/bundle.json"; then
  fail 'missing proof credential was accepted'
fi
assert_no_mutation

if MOCK_CREDENTIAL_VALID=false run_rotation --activate "$GEN" "$work/bundle.json" "$work/proof.creds"; then
  fail 'malformed or near-expiry proof credential was accepted'
fi
assert_no_mutation
grep -Fq -- '--check-min-validity 30m' "$work/go.log" || fail 'rotation did not validate minimum proof credential lifetime'
if run_rotation --activate "$GEN" "$work/bundle.json" "$work/absent.creds"; then
  fail 'missing proof credential file was accepted'
fi
assert_no_mutation
if TEST_PROOF_TAG=latest run_rotation --activate "$GEN" "$work/bundle.json" "$work/proof.creds"; then
  fail 'mutable proof image was accepted'
fi
assert_no_mutation
if TEST_PROOF_REGISTRY='' run_rotation --activate "$GEN" "$work/bundle.json" "$work/proof.creds"; then
  fail 'missing proof image registry was accepted'
fi
assert_no_mutation
if TEST_PROOF_IMAGE_PUBLIC=false run_rotation --activate "$GEN" "$work/bundle.json" "$work/proof.creds"; then
  fail 'proof image without anonymous access or a verified Kubernetes pull Secret was accepted'
fi
assert_no_mutation
if TEST_PROOF_IMAGE_PUBLIC=false TEST_PROOF_PULL_SECRET=voice-proof-ghcr MOCK_PULL_SECRET_BAD=true \
  run_rotation --activate "$GEN" "$work/bundle.json" "$work/proof.creds"; then
  fail 'malformed Kubernetes image pull Secret was accepted'
fi
assert_no_mutation
if TEST_PROOF_IMAGE_PUBLIC=false TEST_PROOF_PULL_SECRET=voice-proof-ghcr MOCK_DOCKER_RESULT=fail \
  run_rotation --activate "$GEN" "$work/bundle.json" "$work/proof.creds"; then
  fail 'private proof image inaccessible to Kubernetes credentials was accepted'
fi
assert_no_mutation
grep -Fq 'manifest inspect ghcr.io/poryadok/voiceroot/realtime:0123456789abcdef0123456789abcdef01234567' "$work/docker.log" || fail 'private image preflight did not check the exact master SHA'

TEST_PROOF_IMAGE_PUBLIC=false TEST_PROOF_PULL_SECRET=voice-proof-ghcr \
  run_rotation --activate "$GEN" "$work/bundle.json" "$work/proof.creds" || \
  fail "valid private image pull Secret did not permit proof: $(tail -1 "$work/output")"
grep -Fq 'get secret voice-proof-ghcr -n voice-staging -o json' "$work/kubectl.log" || fail 'rotation did not read the namespaced Kubernetes pull Secret'
[[ "$(grep -Fc 'imagePullSecrets: [{name: voice-proof-ghcr}]' "$work/rendered/all")" -eq 2 ]] ||
  fail 'preflight and proof Jobs must both use the validated private image pull Secret'

# A PASS is necessary and sufficient only when it is from this newly created
# Job, after hub/bootstrap convergence and before any leaf is restarted.
run_rotation --activate "$GEN" "$work/bundle.json" "$work/proof.creds" || \
  fail "valid synthetic proof did not activate: $(tail -1 "$work/output")"
proof_name="$(awk '$1 == "Secret" && $2 ~ /^voice-nats-acl-proof-/ {print $2}' "$work/rendered/all" | sort -u)"
[[ -n "$proof_name" && "$proof_name" != *$'\n'* ]] || fail 'expected one temporary proof resource identity'
assert_proof_lifecycle "$proof_name"
hub_ready_line="$(grep -n '^rollout status deployment/voice-nats-pvc-candidate ' "$work/kubectl.log" | sed -n '1p' | cut -d: -f1)"
bootstrap_done_line="$(grep -n '^get job voice-nats-realtime-permissions-preflight ' "$work/kubectl.log" | tail -1 | cut -d: -f1)"
proof_job_line="$(grep -nF "event created Job ${proof_name}" "$work/kubectl.log" | sed -n '1p' | cut -d: -f1)"
proof_wait_line="$(grep -nF "job/${proof_name}" "$work/kubectl.log" | grep ':wait ' | sed -n '1p' | cut -d: -f1)"
proof_logs_line="$(grep -nF "job/${proof_name}" "$work/kubectl.log" | grep ':logs ' | sed -n '1p' | cut -d: -f1)"
first_leaf_start_line="$(grep -nE '^scale deployment/voice-(auth|social|user|role|space|chat|file|messaging|voice|matchmaking|search|notification|realtime|bot|subscription|moderation|story|analytics) .*--replicas=1' "$work/kubectl.log" | sed -n '1p' | cut -d: -f1)"
[[ -n "$hub_ready_line" && -n "$bootstrap_done_line" && -n "$proof_job_line" && -n "$proof_wait_line" && -n "$proof_logs_line" && -n "$first_leaf_start_line" ]] || fail 'proof order events are missing'
((hub_ready_line < bootstrap_done_line && bootstrap_done_line < proof_job_line && proof_job_line < proof_wait_line && proof_wait_line < proof_logs_line && proof_logs_line < first_leaf_start_line)) || fail "live proof did not gate leaf restart after hub/bootstrap (hub=$hub_ready_line bootstrap=$bootstrap_done_line job=$proof_job_line wait=$proof_wait_line logs=$proof_logs_line leaf=$first_leaf_start_line)"
[[ "$(cat "$work/state/marker")" == active ]] || fail 'valid live proof did not activate marker'
first_proof_name="$proof_name"

if MOCK_PROOF_RESULT=deny MOCK_RUN_ID=123457 run_rotation --activate "$GEN" "$work/bundle.json" "$work/proof.creds"; then
  fail 'denied live ACL proof was accepted'
fi
assert_rotating_without_leaves
proof_name="$(awk '$1 == "Secret" && $2 ~ /^voice-nats-acl-proof-/ {print $2}' "$work/rendered/all" | sort -u)"
assert_proof_lifecycle "$proof_name"
[[ "$proof_name" != "$first_proof_name" ]] || fail 'proof resource names were reused across workflow runs'

if MOCK_PROOF_RESULT=malformed MOCK_RUN_ID=123458 run_rotation --activate "$GEN" "$work/bundle.json" "$work/proof.creds"; then
  fail 'wrong-generation PASS token was accepted'
fi
assert_rotating_without_leaves

if MOCK_PROOF_RESULT=missing MOCK_RUN_ID=123459 run_rotation --activate "$GEN" "$work/bundle.json" "$work/proof.creds"; then
  fail 'absent PASS token was accepted'
fi
assert_rotating_without_leaves

if MOCK_CLEANUP_FAIL=true MOCK_RUN_ID=123460 run_rotation --activate "$GEN" "$work/bundle.json" "$work/proof.creds"; then
  fail 'proof cleanup failure was accepted'
fi
assert_rotating_without_leaves

if MOCK_PROOF_REMAINS=true MOCK_RUN_ID=123461 run_rotation --activate "$GEN" "$work/bundle.json" "$work/proof.creds"; then
  fail 'proof cleanup verification accepted a still-present Secret'
fi
assert_rotating_without_leaves

# A killed runner can leave all temporary resources in the staging namespace.
# Rollback must identify and remove only this generation's owned proof trio
# before touching the marker, hub, or application leaves.
leftover_name=voice-nats-acl-proof-r20260930a-123456-1
MOCK_LEFTOVER_MODE=owned MOCK_GENERATION_STATE=rotating run_rotation --rollback || \
  fail "rollback could not recover owned proof leftovers: $(tail -1 "$work/output")"
grep -Fq 'get jobs,networkpolicies,secrets -n voice-staging -l voice.io/nats-proof=true,voice.io/nats-proof-generation=r20260930a -o json' "$work/kubectl.log" || fail 'rollback did not list only owned proof resources for the marker generation'
for kind in job networkpolicy secret; do
  [[ "$(grep -Ec "^delete ${kind} ${leftover_name} -n voice-staging " "$work/kubectl.log" || true)" == 1 ]] || fail "rollback did not delete owned proof ${kind} exactly once"
  grep -Eq "^get ${kind} ${leftover_name} -n voice-staging -o json$" "$work/kubectl.log" || fail "rollback did not verify ${kind} NotFound"
done
grep -E '^delete job voice-nats-acl-proof-' "$work/kubectl.log" | grep -Fq -- '--cascade=foreground' || fail 'rollback must wait for proof Pod foreground deletion'
for kind in job networkpolicy secret; do
  grep -E "^delete ${kind} voice-nats-acl-proof-" "$work/kubectl.log" | grep -Fq -- '--wait=true' || fail "rollback must wait for ${kind} deletion"
done
job_delete_line="$(grep -nF "delete job ${leftover_name} " "$work/kubectl.log" | sed -n '1p' | cut -d: -f1)"
policy_delete_line="$(grep -nF "delete networkpolicy ${leftover_name} " "$work/kubectl.log" | sed -n '1p' | cut -d: -f1)"
secret_delete_line="$(grep -nF "delete secret ${leftover_name} " "$work/kubectl.log" | sed -n '1p' | cut -d: -f1)"
marker_patch_line="$(grep -n '^patch configmap voice-nats-generation ' "$work/kubectl.log" | sed -n '1p' | cut -d: -f1)"
first_scale_line="$(grep -n '^scale deployment/' "$work/kubectl.log" | sed -n '1p' | cut -d: -f1)"
((job_delete_line < policy_delete_line && policy_delete_line < secret_delete_line && secret_delete_line < marker_patch_line && secret_delete_line < first_scale_line)) || fail 'rollback did not clean proof Job, policy, Secret before changing marker or workloads'
[[ "$(grep -c '^get jobs,networkpolicies,secrets -n voice-staging -l voice.io/nats-proof=true,voice.io/nats-proof-generation=r20260930a -o json$' "$work/kubectl.log" || true)" -ge 2 ]] || fail 'rollback must re-list and prove no owned proof resource remains'
! grep -Eq '^delete (job|networkpolicy|secret) (voice-postgres|voice-minio|voice-app)' "$work/kubectl.log" || fail 'rollback deleted an unrelated resource'

assert_rollback_stopped_before_workloads() {
  ! grep -Eq '^patch configmap voice-nats-generation |^scale deployment/|^patch deployment ' "$work/kubectl.log" || fail 'unsafe proof cleanup reached marker or workload mutation'
}
for bad in mismatch duplicate; do
  if MOCK_LEFTOVER_MODE="$bad" MOCK_GENERATION_STATE=rotating run_rotation --rollback; then
    fail "rollback accepted ${bad} proof resource identity"
  fi
  assert_no_mutation
done
if MOCK_LEFTOVER_MODE=delete_fail MOCK_GENERATION_STATE=rotating run_rotation --rollback; then
  fail 'rollback accepted proof NetworkPolicy deletion failure'
fi
assert_rollback_stopped_before_workloads

printf 'staging NATS live ACL proof contract: OK\n'
