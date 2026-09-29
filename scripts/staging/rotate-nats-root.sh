#!/usr/bin/env bash
# Planned-outage, staging-only NATS root generation switch. Never prints data.
set +x
set -euo pipefail
umask 077

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
LEAVES=(auth social user role space chat file messaging voice matchmaking search notification realtime bot subscription moderation story analytics)
BOOTSTRAPS=(realtime notification search analytics-chat)
SERVICES=(analytics auth bot chat file gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice)
SECRET_BASES=(voice-nats-operator voice-nats-hub-tls voice-nats-bootstrap-credentials voice-nats-service-credentials)

fail() { printf 'ERROR: NATS root rotation: %s\n' "$1" >&2; exit 1; }
[[ "$NS" == voice-staging ]] || fail 'namespace must be voice-staging'
for command in kubectl jq base64 openssl cmp mktemp; do
  command -v "$command" >/dev/null 2>&1 || fail "required command missing: $command"
done

valid_generation() { [[ "${1:-}" =~ ^r[0-9]{8}[a-z0-9]{0,8}$ ]]; }
ref() { if [[ "$2" == legacy ]]; then printf '%s' "$1"; else printf '%s-%s' "$1" "$2"; fi; }

# A failed read is absence only when the API server explicitly says NotFound.
read_optional() {
  local output
  if output="$(kubectl get "$1" "$2" -n "$NS" -o json 2>&1)"; then
    [[ -n "$output" ]] || fail "empty Kubernetes response for $1/$2"
    printf '%s' "$output"
    return 0
  fi
  if [[ "$output" == *NotFound* || "$output" == *'not found'* ]]; then return 1; fi
  fail "unable to read $1/$2"
}

read_required() {
  local output
  output="$(read_optional "$1" "$2")" || fail "required $1/$2 is absent"
  printf '%s' "$output"
}

check_identity() {
  jq -e --arg kind "$2" --arg name "$3" --arg ns "$NS" \
    '.kind == $kind and .metadata.name == $name and .metadata.namespace == $ns' \
    <<<"$1" >/dev/null || fail "unexpected $2/$3 identity"
}

check_bundle() {
  local bundle="$1" generation="$2" tmp san
  [[ -f "$bundle" && ! -L "$bundle" ]] || fail 'bundle must be a regular file'
  [[ "$(wc -c <"$bundle")" -le 1048576 ]] || fail 'bundle is too large'
  jq -e --arg gen "$generation" '
    def encoded: type == "string" and length > 0 and test("^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$");
    def exact($base; $keys):
      .apiVersion == "v1" and .kind == "Secret" and .type == "Opaque" and
      .immutable == true and (keys | sort) == ["apiVersion","data","immutable","kind","metadata","type"] and
      (.metadata | keys | sort) == ["name","namespace"] and
      .metadata.name == ($base + "-" + $gen) and .metadata.namespace == "voice-staging" and
      (.data | type == "object" and (keys | sort) == ($keys | sort) and all(.[]; encoded));
    .apiVersion == "v1" and .kind == "List" and (keys | sort) == ["apiVersion","items","kind"] and
    (.items | type == "array" and length == 4) and
    ([.items[].metadata.name] | sort) == (["voice-nats-operator","voice-nats-hub-tls","voice-nats-bootstrap-credentials","voice-nats-service-credentials"] | map(. + "-" + $gen) | sort) and
    all(.items[]; if .metadata.name == ("voice-nats-operator-" + $gen) then
      exact("voice-nats-operator"; ["operator.jwt","account.jwt","system-account.jwt","account.public","system-account.public"])
    elif .metadata.name == ("voice-nats-hub-tls-" + $gen) then
      exact("voice-nats-hub-tls"; ["tls.crt","tls.key","ca.crt"])
    elif .metadata.name == ("voice-nats-bootstrap-credentials-" + $gen) then
      exact("voice-nats-bootstrap-credentials"; ["bootstrap.creds"])
    else
      exact("voice-nats-service-credentials"; ["analytics.creds","auth.creds","bot.creds","chat.creds","file.creds","gateway.creds","matchmaking.creds","messaging.creds","moderation.creds","notification.creds","realtime.creds","role.creds","search.creds","social.creds","space.creds","story.creds","subscription.creds","user.creds","voice.creds"])
    end)
  ' "$bundle" >/dev/null 2>&1 || fail 'invalid four-Secret rotation bundle'

  tmp="$(mktemp -d)" || fail 'cannot create protected TLS scratch directory'
  trap 'rm -f -- "$tmp"/*; rmdir -- "$tmp"' EXIT
  jq -r --arg name "voice-nats-hub-tls-${generation}" '.items[] | select(.metadata.name == $name) | .data["tls.crt"]' "$bundle" | base64 -d >"$tmp/tls.crt" 2>/dev/null || fail 'invalid TLS certificate encoding'
  jq -r --arg name "voice-nats-hub-tls-${generation}" '.items[] | select(.metadata.name == $name) | .data["tls.key"]' "$bundle" | base64 -d >"$tmp/tls.key" 2>/dev/null || fail 'invalid TLS key encoding'
  jq -r --arg name "voice-nats-hub-tls-${generation}" '.items[] | select(.metadata.name == $name) | .data["ca.crt"]' "$bundle" | base64 -d >"$tmp/ca.crt" 2>/dev/null || fail 'invalid TLS CA encoding'
  openssl verify -CAfile "$tmp/ca.crt" "$tmp/tls.crt" >/dev/null 2>&1 || fail 'TLS certificate does not verify against CA'
  openssl x509 -in "$tmp/tls.crt" -noout -checkhost voice-nats 2>/dev/null | grep -q 'does match' || fail 'TLS certificate does not match voice-nats'
  san="$(openssl x509 -in "$tmp/tls.crt" -noout -ext subjectAltName 2>/dev/null)" || fail 'TLS certificate has no SAN'
  [[ "$san" =~ DNS:voice-nats([,[:space:]]|$) ]] || fail 'TLS certificate lacks exact voice-nats DNS SAN'
  openssl x509 -in "$tmp/tls.crt" -pubkey -noout >"$tmp/cert.pub" 2>/dev/null || fail 'invalid TLS certificate public key'
  openssl pkey -in "$tmp/tls.key" -pubout >"$tmp/key.pub" 2>/dev/null || fail 'invalid TLS private key'
  cmp -s "$tmp/cert.pub" "$tmp/key.pub" || fail 'TLS certificate and key differ'
  rm -f -- "$tmp"/*
  rmdir -- "$tmp"
  trap - EXIT
}

check_secret_set() {
  local generation="$1" mode="$2" base name result count=0
  for base in "${SECRET_BASES[@]}"; do
    name="$(ref "$base" "$generation")"
    if result="$(read_optional secret "$name")"; then
      ((count += 1))
      check_identity "$result" Secret "$name"
      if [[ "$mode" == source ]]; then
        jq -e --arg base "$base" '
          .type == "Opaque" and (.data | type == "object" and
            (keys | sort) == (if $base == "voice-nats-operator" then ["operator.jwt","account.jwt","system-account.jwt","account.public","system-account.public"]
              elif $base == "voice-nats-hub-tls" then ["tls.crt","tls.key","ca.crt"]
              elif $base == "voice-nats-bootstrap-credentials" then ["bootstrap.creds"]
              else ["analytics.creds","auth.creds","bot.creds","chat.creds","file.creds","gateway.creds","matchmaking.creds","messaging.creds","moderation.creds","notification.creds","realtime.creds","role.creds","search.creds","social.creds","space.creds","story.creds","subscription.creds","user.creds","voice.creds"] end | sort))
        ' <<<"$result" >/dev/null || fail "invalid source Secret $name"
      fi
    fi
  done
  if [[ "$mode" == target ]]; then
    ((count == 0)) || fail 'target Secret generation already exists or is partial'
  else
    ((count == 4)) || fail 'source Secret generation is incomplete'
  fi
}

volume_ref() {
  jq -er --arg name "$2" --arg type "$3" \
    '[.spec.template.spec.volumes[]? | select(.name == $name)] | if length == 1 then .[0][$type] | if $type == "secret" then .secretName else .claimName end else empty end' \
    <<<"$1" 2>/dev/null
}

check_hub() {
  local json="$1" generation="$2" expected
  check_identity "$json" Deployment voice-nats-pvc-candidate
  jq -e '.spec.replicas == 1 and .spec.selector.matchLabels.app == "voice-nats-pvc-candidate" and .spec.template.metadata.labels.app == "voice-nats-pvc-candidate"' <<<"$json" >/dev/null || fail 'candidate hub is not a singleton with the expected selector'
  expected="$(ref voice-nats-jsdata "$generation")"
  [[ "$(volume_ref "$json" jsdata persistentVolumeClaim)" == "$expected" ]] || fail 'candidate hub source PVC differs'
  expected="$(ref voice-nats-operator "$generation")"
  [[ "$(volume_ref "$json" nats-resolver-input secret)" == "$expected" && "$(volume_ref "$json" nats-operator-jwt secret)" == "$expected" ]] || fail 'candidate hub operator refs differ'
  expected="$(ref voice-nats-hub-tls "$generation")"
  [[ "$(volume_ref "$json" nats-hub-tls secret)" == "$expected" ]] || fail 'candidate hub TLS ref differs'
}

check_leaf() {
  local json="$1" name="$2" service="$3" generation="$4"
  check_identity "$json" Deployment "$name"
  jq -e --arg service "$service" '.spec.replicas == 1 and any(.spec.template.spec.containers[]?; .name == "nats-leaf") and
    ([.spec.template.spec.volumes[]? | select(.name == "nats-service-creds")] | length == 1) and
    ([.spec.template.spec.volumes[]? | select(.name == "nats-hub-tls")] | length == 1) and
    any(.spec.template.spec.volumes[]?; .name == "nats-service-creds" and .secret.items == [{"key":($service + ".creds"),"path":($service + ".creds")}]) and
    any(.spec.template.spec.volumes[]?; .name == "nats-hub-tls" and .secret.items == [{"key":"ca.crt","path":"ca.crt"}])' <<<"$json" >/dev/null || fail "unexpected leaf layout for $name"
  [[ "$(volume_ref "$json" nats-service-creds secret)" == "$(ref voice-nats-service-credentials "$generation")" ]] || fail "$name service credential ref differs"
  [[ "$(volume_ref "$json" nats-hub-tls secret)" == "$(ref voice-nats-hub-tls "$generation")" ]] || fail "$name TLS ref differs"
}

check_source_pvc() {
  local json="$1" name="$2" class="$3" size="$4"
  check_identity "$json" PersistentVolumeClaim "$name"
  jq -e --arg class "$class" --arg size "$size" '.spec.storageClassName == $class and .spec.resources.requests.storage == $size and .spec.accessModes == ["ReadWriteOnce"]' <<<"$json" >/dev/null || fail 'source PVC contract differs from requested storage'
}

check_workloads() {
  local generation="$1" hub service json
  hub="$(read_required deployment voice-nats-pvc-candidate)"
  check_hub "$hub" "$generation"
  for service in "${LEAVES[@]}"; do
    json="$(read_required deployment "voice-${service}")"
    check_leaf "$json" "voice-${service}" "$service" "$generation"
  done
}

marker_state() {
  local marker
  if marker="$(read_optional configmap voice-nats-generation)"; then
    check_identity "$marker" ConfigMap voice-nats-generation
    MARKER_PHASE="$(jq -er '.data.phase' <<<"$marker")" || fail 'invalid generation marker phase'
    MARKER_GENERATION="$(jq -er '.data.generation' <<<"$marker")" || fail 'invalid generation marker generation'
    MARKER_PREVIOUS="$(jq -er '.data.previousGeneration' <<<"$marker")" || fail 'invalid generation marker previous generation'
    [[ "$MARKER_GENERATION" == legacy ]] || valid_generation "$MARKER_GENERATION" || fail 'invalid generation marker'
    [[ "$MARKER_PREVIOUS" == legacy ]] || valid_generation "$MARKER_PREVIOUS" || fail 'invalid previous generation marker'
  else
    MARKER_PHASE=absent MARKER_GENERATION=legacy MARKER_PREVIOUS=''
  fi
}

check_selector() {
  local svc selector
  svc="$(read_required service voice-nats)"
  check_identity "$svc" Service voice-nats
  selector="$(jq -er '.spec.selector.app' <<<"$svc")" || fail 'voice-nats Service has no app selector'
  [[ "$selector" == voice-nats-pvc-candidate ]] || fail 'voice-nats Service does not select the candidate hub'
  jq -e '.spec.selector == {"app":"voice-nats-pvc-candidate"}' <<<"$svc" >/dev/null || fail 'voice-nats Service selector has unexpected terms'
}

check_no_second_hub() {
  local old
  if old="$(read_optional deployment voice-nats)"; then
    check_identity "$old" Deployment voice-nats
    jq -e '(.spec.replicas // 0) == 0' <<<"$old" >/dev/null || fail 'source NATS hub is still running'
  fi
}

prepare_pvc() {
  cat <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: voice-nats-jsdata-${1}
  namespace: voice-staging
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: ${2}
  resources:
    requests:
      storage: ${3}
EOF
}

patch_hub() {
  local generation="$1" json patch
  json="$(read_required deployment voice-nats-pvc-candidate)"
  patch="$(jq -cn --argjson pod "$json" --arg pvc "$(ref voice-nats-jsdata "$generation")" --arg op "$(ref voice-nats-operator "$generation")" --arg tls "$(ref voice-nats-hub-tls "$generation")" --arg gen "$generation" '
    [ {op:"add",path:"/spec/strategy",value:{type:"Recreate"}},
      {op:"add",path:"/spec/template/metadata/annotations",value: (($pod.spec.template.metadata.annotations // {}) + {"voice.dev/nats-generation":$gen})} ] +
    [ $pod.spec.template.spec.volumes | to_entries[] | select(.value.name == "jsdata" or .value.name == "nats-resolver-input" or .value.name == "nats-operator-jwt" or .value.name == "nats-hub-tls") |
      {op:"replace",path:("/spec/template/spec/volumes/" + (.key|tostring) + (if .value.name == "jsdata" then "/persistentVolumeClaim/claimName" else "/secret/secretName" end)),
       value:(if .value.name == "jsdata" then $pvc elif .value.name == "nats-hub-tls" then $tls else $op end)} ]
  ')" || fail 'cannot prepare hub patch'
  kubectl patch deployment voice-nats-pvc-candidate -n "$NS" --type=json -p "$patch" >/dev/null
}

patch_leaf() {
  local service="$1" generation="$2" json patch
  json="$(read_required deployment "voice-${service}")"
  patch="$(jq -cn --argjson pod "$json" --arg creds "$(ref voice-nats-service-credentials "$generation")" --arg tls "$(ref voice-nats-hub-tls "$generation")" --arg gen "$generation" '
    [ {op:"add",path:"/spec/strategy",value:{type:"Recreate"}},
      {op:"add",path:"/spec/template/metadata/annotations",value:(($pod.spec.template.metadata.annotations // {}) + {"voice.dev/nats-generation":$gen})} ] +
    [ $pod.spec.template.spec.volumes | to_entries[] | select(.value.name == "nats-service-creds" or .value.name == "nats-hub-tls") |
      {op:"replace",path:("/spec/template/spec/volumes/" + (.key|tostring) + "/secret/secretName"),value:(if .value.name == "nats-service-creds" then $creds else $tls end)} ]
  ')" || fail "cannot prepare leaf patch for ${service}"
  kubectl patch deployment "voice-${service}" -n "$NS" --type=json -p "$patch" >/dev/null
}

wait_gone() {
  local app="$1" pods attempt
  for ((attempt=0; attempt<60; attempt++)); do
    pods="$(kubectl get pods -n "$NS" -l "app=${app}" -o name)" || fail "cannot check stopped pods for $app"
    [[ -z "$pods" ]] && return 0
    sleep 3
  done
  fail "$app pods did not stop"
}

stop_leaves() {
  local service
  for service in "${LEAVES[@]}"; do
    kubectl scale "deployment/voice-${service}" -n "$NS" --replicas=0 >/dev/null
  done
  for service in "${LEAVES[@]}"; do wait_gone "voice-${service}"; done
}

stop_hub() {
  kubectl scale deployment/voice-nats-pvc-candidate -n "$NS" --replicas=0 >/dev/null
  wait_gone voice-nats-pvc-candidate
}

start_hub() {
  kubectl scale deployment/voice-nats-pvc-candidate -n "$NS" --replicas=1 >/dev/null
  kubectl rollout status deployment/voice-nats-pvc-candidate -n "$NS" --timeout=300s >/dev/null
}

start_leaves() {
  local service
  for service in "${LEAVES[@]}"; do
    kubectl scale "deployment/voice-${service}" -n "$NS" --replicas=1 >/dev/null
    kubectl rollout status "deployment/voice-${service}" -n "$NS" --timeout=300s >/dev/null
  done
}

set_marker() {
  local phase="$1" generation="$2" previous="$3"
  if [[ "$MARKER_PHASE" == absent ]]; then
    cat <<EOF | kubectl create -f - >/dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: voice-nats-generation
  namespace: voice-staging
data:
  phase: ${phase}
  generation: ${generation}
  previousGeneration: ${previous}
EOF
    MARKER_PHASE="$phase"
  else
    kubectl patch configmap voice-nats-generation -n "$NS" --type=merge \
      -p "$(jq -cn --arg phase "$phase" --arg gen "$generation" --arg prev "$previous" '{data:{phase:$phase,generation:$gen,previousGeneration:$prev}}')" >/dev/null
  fi
}

render_job() {
  local file="$1" generation="$2" registry="${VOICE_IMAGE_REGISTRY:-}" tag="${VOICE_IMAGE_TAG:-}"
  [[ "$registry" =~ ^[A-Za-z0-9./:_-]+$ && "$tag" =~ ^[A-Za-z0-9._-]+$ ]] || fail 'VOICE_IMAGE_REGISTRY and VOICE_IMAGE_TAG are required and must be safe template tokens'
  sed -e "s|__NAMESPACE__|${NS}|g" \
      -e "s|__IMAGE_REGISTRY__|${registry}|g" -e "s|__IMAGE_TAG__|${tag}|g" \
      -e "s|voice-nats-bootstrap-credentials|$(ref voice-nats-bootstrap-credentials "$generation")|g" \
      -e "s|voice-nats-service-credentials|$(ref voice-nats-service-credentials "$generation")|g" \
      -e "s|voice-nats-hub-tls|$(ref voice-nats-hub-tls "$generation")|g" "$file" |
    awk -v generation="$generation" '
      /^kind: / {job=($2 == "Job")}
      job && /^  namespace: / {print; print "  annotations:"; print "    voice.dev/nats-generation: " generation; job=0; next}
      {print}
    '
}

run_jobs() {
  local generation="$1" bootstrap job
  for bootstrap in "${BOOTSTRAPS[@]}"; do
    job="voice-nats-${bootstrap}-bootstrap"
    kubectl delete job "$job" -n "$NS" --ignore-not-found >/dev/null
    render_job "$ROOT/deploy/templates/nats-${bootstrap}-bootstrap.yaml" "$generation" | kubectl apply -f - >/dev/null
    kubectl wait --for=condition=complete "job/${job}" -n "$NS" --timeout=120s >/dev/null
  done
  kubectl delete job voice-nats-realtime-permissions-preflight -n "$NS" --ignore-not-found >/dev/null
  render_job "$ROOT/deploy/templates/nats-realtime-permissions-preflight.yaml" "$generation" | kubectl apply -f - >/dev/null
  kubectl wait --for=condition=complete job/voice-nats-realtime-permissions-preflight -n "$NS" --timeout=210s >/dev/null
}

main() {
  local mode="${1:-}" target bundle source pvc class size selector old
  case "$mode" in
    --activate)
      [[ $# -eq 3 ]] || fail 'usage: rotate-nats-root.sh --activate GENERATION BUNDLE.json | --rollback'
      target="$2" bundle="$3"
      valid_generation "$target" || fail 'invalid generation token'
      check_bundle "$bundle" "$target"
      marker_state
      [[ "$MARKER_PHASE" == absent || "$MARKER_PHASE" == active ]] || fail 'generation marker is not active'
      source="$MARKER_GENERATION"
      [[ "$target" != "$source" ]] || fail 'target generation is already active'
      check_selector
      check_no_second_hub
      check_secret_set "$source" source
      check_secret_set "$target" target
      class="${VOICE_NATS_STORAGE_CLASS:-}" size="${VOICE_NATS_STORAGE_SIZE:-}"
      [[ "$class" =~ ^[a-z0-9][a-z0-9.-]*$ && "$size" =~ ^[1-9][0-9]*(Mi|Gi|Ti)$ ]] || fail 'approved NATS storage class and size are required'
      pvc="$(read_required pvc "$(ref voice-nats-jsdata "$source")")"
      check_source_pvc "$pvc" "$(ref voice-nats-jsdata "$source")" "$class" "$size"
      if read_optional pvc "$(ref voice-nats-jsdata "$target")" >/dev/null; then fail 'target PVC already exists'; fi
      check_workloads "$source"
      kubectl create -n "$NS" --dry-run=server -o name -f "$bundle" >/dev/null 2>&1 || fail 'target Secrets rejected by API dry-run'
      prepare_pvc "$target" "$class" "$size" | kubectl create -n "$NS" --dry-run=server -o name -f - >/dev/null 2>&1 || fail 'target PVC rejected by API dry-run'
      set_marker rotating "$target" "$source"
      kubectl create -n "$NS" -f "$bundle" >/dev/null
      prepare_pvc "$target" "$class" "$size" | kubectl create -n "$NS" -f - >/dev/null
      stop_leaves
      stop_hub
      patch_hub "$target"
      start_hub
      run_jobs "$target"
      for old in "${LEAVES[@]}"; do patch_leaf "$old" "$target"; done
      start_leaves
      set_marker active "$target" "$source"
      echo "NATS root generation ${target} active in voice-staging"
      ;;
    --rollback)
      [[ $# -eq 1 ]] || fail 'usage: rotate-nats-root.sh --activate GENERATION BUNDLE.json | --rollback'
      marker_state
      [[ "$MARKER_PHASE" == active || "$MARKER_PHASE" == rotating ]] || fail 'no active or interrupted rotation to roll back'
      target="$MARKER_GENERATION" source="$MARKER_PREVIOUS"
      valid_generation "$target" || fail 'legacy generation has no root rotation to roll back'
      [[ "$source" == legacy ]] || valid_generation "$source" || fail 'invalid retained generation'
      [[ "$source" != "$target" ]] || fail 'rollback would be a no-op'
      check_selector
      check_no_second_hub
      check_secret_set "$source" source
      read_required pvc "$(ref voice-nats-jsdata "$source")" >/dev/null
      check_secret_set "$target" source
      read_required pvc "$(ref voice-nats-jsdata "$target")" >/dev/null
      # The active marker proves the target is fully converged. Interrupted
      # activation may have mixed refs, so only the fixed workload identities
      # are required before forcing every reference back to the retained set.
      for old in voice-nats-pvc-candidate "${LEAVES[@]/#/voice-}"; do
        read_required deployment "$old" >/dev/null
      done
      set_marker rotating "$target" "$source"
      stop_leaves
      stop_hub
      patch_hub "$source"
      start_hub
      run_jobs "$source"
      for old in "${LEAVES[@]}"; do patch_leaf "$old" "$source"; done
      start_leaves
      set_marker active "$source" "$target"
      echo "NATS root generation ${source} restored in voice-staging"
      ;;
    *) fail 'usage: rotate-nats-root.sh --activate GENERATION BUNDLE.json | --rollback' ;;
  esac
}

main "$@"
