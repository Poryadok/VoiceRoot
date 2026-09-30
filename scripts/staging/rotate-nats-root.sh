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
    def exact($base; $wanted):
      .apiVersion == "v1" and .kind == "Secret" and .type == "Opaque" and
      .immutable == true and (keys | sort) == ["apiVersion","data","immutable","kind","metadata","type"] and
      (.metadata | keys | sort) == ["name","namespace"] and
      .metadata.name == ($base + "-" + $gen) and .metadata.namespace == "voice-staging" and
      (.data | type == "object" and (keys | sort) == ($wanted | sort) and all(.[]; encoded));
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
  local json="$1" name="$2" service="$3" generation="$4" expected_replicas="${5:-1}"
  check_identity "$json" Deployment "$name"
  jq -e --arg service "$service" --argjson replicas "$expected_replicas" '.spec.replicas == $replicas and any(.spec.template.spec.containers[]?; .name == "nats-leaf") and
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

prepare_realtime_image() {
  local realtime image
  realtime="$(read_required deployment voice-realtime)"
  image="$(jq -er '[.spec.template.spec.containers[]? | select(.name == "realtime") | .image] | if length == 1 then .[0] else empty end' <<<"$realtime")" || fail 'cannot resolve deployed Realtime image'
  [[ "$image" =~ ^ghcr\.io/[a-z0-9._/-]+/realtime:[a-f0-9]{40}$ ]] || fail 'deployed Realtime image is not an immutable GHCR SHA tag'
  ROTATION_IMAGE_REGISTRY="${image%/realtime:*}"
  ROTATION_IMAGE_TAG="${image##*:}"
}

check_proof_image_pull_secret() (
  local secret_json="$1" scratch image
  command -v docker >/dev/null 2>&1 || fail 'Docker is required to verify private proof image access'
  scratch="$(mktemp -d)" || fail 'cannot create protected image pull scratch directory'
  trap 'rm -f -- "$scratch/config.json"; rmdir -- "$scratch"' EXIT
  jq -er '.data[".dockerconfigjson"]' <<<"$secret_json" | base64 -d >"$scratch/config.json" 2>/dev/null ||
    fail 'invalid proof image pull Secret encoding'
  chmod 600 "$scratch/config.json"
  jq -e '(.auths["ghcr.io"] // .auths["https://ghcr.io"]) | type == "object"' \
    "$scratch/config.json" >/dev/null || fail 'proof image pull Secret has no GHCR credentials'
  image="${VOICE_NATS_PROOF_IMAGE_REGISTRY}/realtime:${VOICE_NATS_PROOF_IMAGE_TAG}"
  docker --config "$scratch" manifest inspect "$image" >/dev/null 2>&1 ||
    fail 'Kubernetes proof image pull Secret cannot access exact-master image'
)

prepare_proof() {
  local generation="$1" bundle="$2" credential="$3" mode size run_id attempt
  [[ -f "$credential" && ! -L "$credential" ]] || fail 'proof credential must be a regular file'
  mode="$(stat -c %a -- "$credential")" || fail 'cannot inspect proof credential permissions'
  [[ "$mode" == 400 || "$mode" == 600 ]] || fail 'proof credential must be mode 0400 or 0600'
  size="$(wc -c <"$credential")"
  [[ "$size" =~ ^[0-9]+$ ]] && ((size > 0 && size <= 65536)) || fail 'proof credential size is invalid'
  [[ "${VOICE_NATS_PROOF_IMAGE_REGISTRY:-}" == ghcr.io/poryadok/voiceroot ]] || fail 'exact-master proof image registry is required'
  [[ "${VOICE_NATS_PROOF_IMAGE_TAG:-}" =~ ^[a-f0-9]{40}$ ]] || fail 'exact-master proof image SHA is required'
  if [[ -n "${GITHUB_SHA:-}" ]]; then
    [[ "$VOICE_NATS_PROOF_IMAGE_TAG" == "${GITHUB_SHA,,}" ]] || fail 'proof image differs from workflow master SHA'
  fi
  if [[ -n "${VOICE_NATS_PROOF_IMAGE_PULL_SECRET:-}" ]]; then
    [[ "$VOICE_NATS_PROOF_IMAGE_PULL_SECRET" =~ ^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$ ]] ||
      fail 'invalid proof image pull Secret name'
    local pull_secret
    pull_secret="$(read_required secret "$VOICE_NATS_PROOF_IMAGE_PULL_SECRET")"
    check_identity "$pull_secret" Secret "$VOICE_NATS_PROOF_IMAGE_PULL_SECRET"
    jq -e '.type == "kubernetes.io/dockerconfigjson" and
      (.data[".dockerconfigjson"] | type == "string" and length > 0)' \
      <<<"$pull_secret" >/dev/null || fail 'proof image pull Secret has invalid type or data'
    check_proof_image_pull_secret "$pull_secret"
  else
    [[ "${VOICE_NATS_PROOF_IMAGE_PUBLIC:-}" == true ]] ||
      fail 'anonymous GHCR pull proof or reviewed image pull Secret is required'
  fi
  command -v go >/dev/null 2>&1 || fail 'Go is required for proof credential validation'
  local credential_abs bundle_abs
  credential_abs="$(cd -- "$(dirname -- "$credential")" && pwd -P)/$(basename -- "$credential")"
  bundle_abs="$(cd -- "$(dirname -- "$bundle")" && pwd -P)/$(basename -- "$bundle")"
  (cd "$ROOT/src/backend/pkg" && go run ./cmd/nats-proof-credential \
    --check-min-validity 30m --credential "$credential_abs" --bundle "$bundle_abs" \
    --generation "$generation" --namespace "$NS") >/dev/null 2>&1 ||
    fail 'proof credential is invalid or expires within 30 minutes'
  run_id="${GITHUB_RUN_ID:-$(openssl rand -hex 6)}"
  attempt="${GITHUB_RUN_ATTEMPT:-1}"
  [[ "$run_id" =~ ^[a-z0-9]{1,18}$ && "$attempt" =~ ^[0-9]{1,3}$ ]] || fail 'invalid proof run identity'
  PROOF_NAME="voice-nats-acl-proof-${generation}-${run_id}-${attempt}"
  (("${#PROOF_NAME}" <= 63)) || fail 'proof resource name is too long'
  local kind
  for kind in secret networkpolicy job; do
    if read_optional "$kind" "$PROOF_NAME" >/dev/null; then
      fail 'proof resource from this run already exists'
    fi
  done
  PROOF_ACL_SHA="$(sha256sum "$ROOT/deploy/nats/acl-intent.yaml" | cut -d' ' -f1)"
  [[ "$PROOF_ACL_SHA" =~ ^[a-f0-9]{64}$ ]] || fail 'invalid reviewed ACL digest'
}

render_proof_secret() {
  local name="$1" generation="$2" credential="$3"
  kubectl create secret generic "$name" -n "$NS" --from-file="proof.creds=$credential" \
    --dry-run=client -o json |
    jq -c --arg gen "$generation" --arg name "$name" '
      .immutable = true |
      .metadata.annotations = ((.metadata.annotations // {}) + {"voice.io/nats-generation":$gen}) |
      .metadata.labels = ((.metadata.labels // {}) +
        {"voice.io/nats-proof":"true", "voice.io/nats-proof-generation":$gen,
         "voice.io/nats-proof-run":$name})
    '
}

render_proof_manifest() {
  local file="$1" name="$2" generation="$3" pull_secrets='[]'
  if [[ -n "${VOICE_NATS_PROOF_IMAGE_PULL_SECRET:-}" ]]; then
    pull_secrets="[{name: ${VOICE_NATS_PROOF_IMAGE_PULL_SECRET}}]"
  fi
  sed -e "s|__PROOF_NAME__|${name}|g" \
      -e "s|__GENERATION__|${generation}|g" \
      -e "s|__ACL_SHA__|${PROOF_ACL_SHA}|g" \
      -e "s|__IMAGE_REGISTRY__|${VOICE_NATS_PROOF_IMAGE_REGISTRY}|g" \
      -e "s|__IMAGE_TAG__|${VOICE_NATS_PROOF_IMAGE_TAG}|g" \
      -e "s|__IMAGE_PULL_SECRETS__|${pull_secrets}|g" "$file"
}

# A lost runner bypasses the EXIT trap. Rollback reclaims only proof resources
# whose selector, name, generation annotation, and namespace all agree.
recover_proof_resources() {
  local generation="$1" selector listing remaining kind resource name
  selector="voice.io/nats-proof=true,voice.io/nats-proof-generation=${generation}"
  listing="$(kubectl get jobs,networkpolicies,secrets -n "$NS" -l "$selector" -o json 2>/dev/null)" ||
    fail 'cannot list interrupted live ACL proof resources'
  jq -e --arg gen "$generation" --arg ns "$NS" '
    .apiVersion == "v1" and .kind == "List" and (.items | type == "array") and
    (.items | all(.[];
      (.kind == "Job" or .kind == "NetworkPolicy" or .kind == "Secret") and
      .metadata.namespace == $ns and
      (.metadata.name | type == "string" and test("^voice-nats-acl-proof-" + $gen + "-[a-z0-9]{1,18}-[0-9]{1,3}$")) and
      .metadata.labels["voice.io/nats-proof"] == "true" and
      .metadata.labels["voice.io/nats-proof-generation"] == $gen and
      .metadata.labels["voice.io/nats-proof-run"] == .metadata.name and
      .metadata.annotations["voice.io/nats-generation"] == $gen)) and
    ([.items[] | .kind + "/" + .metadata.name] as $ids |
      ($ids | length) == ($ids | unique | length))
  ' <<<"$listing" >/dev/null || fail 'interrupted live ACL proof resources have ambiguous identity'
  for kind in Job NetworkPolicy Secret; do
    case "$kind" in
      Job) resource=job ;;
      NetworkPolicy) resource=networkpolicy ;;
      Secret) resource=secret ;;
    esac
    while IFS= read -r name; do
      [[ -n "$name" ]] || continue
      if [[ "$kind" == Job ]]; then
        kubectl delete "$resource" "$name" -n "$NS" --cascade=foreground --ignore-not-found=true --wait=true >/dev/null 2>&1 ||
          fail 'interrupted live ACL proof Job cleanup failed'
      else
        kubectl delete "$resource" "$name" -n "$NS" --ignore-not-found=true --wait=true >/dev/null 2>&1 ||
          fail 'interrupted live ACL proof resource cleanup failed'
      fi
      if read_optional "$resource" "$name" >/dev/null; then
        fail 'interrupted live ACL proof resource remains after cleanup'
      fi
    done < <(jq -r --arg kind "$kind" '.items[] | select(.kind == $kind) | .metadata.name' <<<"$listing")
  done
  remaining="$(kubectl get jobs,networkpolicies,secrets -n "$NS" -l "$selector" -o json 2>/dev/null)" ||
    fail 'cannot verify interrupted live ACL proof cleanup'
  jq -e '.apiVersion == "v1" and .kind == "List" and .items == []' \
    <<<"$remaining" >/dev/null || fail 'interrupted live ACL proof resources remain'
}

preflight_proof_resources() {
  local name="$1" generation="$2" credential="$3"
  render_proof_secret "$name" "$generation" "$credential" |
    kubectl create -n "$NS" --dry-run=server -f - >/dev/null 2>&1 ||
    fail 'proof Secret rejected by API dry-run'
  render_proof_manifest "$ROOT/deploy/templates/network-policy-nats-live-acl-proof.yaml" "$name" "$generation" |
    kubectl create -n "$NS" --dry-run=server -f - >/dev/null 2>&1 ||
    fail 'proof NetworkPolicy rejected by API dry-run'
  render_proof_manifest "$ROOT/deploy/templates/nats-realtime-acl-proof.yaml" "$name" "$generation" |
    kubectl create -n "$NS" --dry-run=server -f - >/dev/null 2>&1 ||
    fail 'proof Job rejected by API dry-run'
}

cleanup_proof_resources() {
  local result="$1" name="$2" kind response cleanup_failed=0
  trap - EXIT
  # Foreground deletion waits for the proof Pod to disappear before its
  # credential Secret or network exception can be removed.
  kubectl delete job "$name" -n "$NS" --cascade=foreground --ignore-not-found=true --wait=true >/dev/null 2>&1 ||
    cleanup_failed=1
  for kind in networkpolicy secret; do
    kubectl delete "$kind" "$name" -n "$NS" --ignore-not-found=true --wait=true >/dev/null 2>&1 ||
      cleanup_failed=1
  done
  for kind in job networkpolicy secret; do
    if response="$(kubectl get "$kind" "$name" -n "$NS" -o json 2>&1)"; then
      cleanup_failed=1
    elif [[ "$response" != *NotFound* && "$response" != *'not found'* ]]; then
      cleanup_failed=1
    fi
  done
  if ((cleanup_failed)); then
    printf 'ERROR: NATS root rotation: live ACL proof cleanup failed\n' >&2
    exit 1
  fi
  exit "$result"
}

run_live_acl_proof() (
  local name="$1" generation="$2" credential="$3" logs expected
  trap 'cleanup_proof_resources "$?" "$name"' EXIT
  trap 'exit 1' INT TERM
  render_proof_secret "$name" "$generation" "$credential" |
    kubectl create -n "$NS" -f - >/dev/null 2>&1 ||
    fail 'cannot create proof Secret'
  render_proof_manifest "$ROOT/deploy/templates/network-policy-nats-live-acl-proof.yaml" "$name" "$generation" |
    kubectl create -n "$NS" -f - >/dev/null 2>&1 ||
    fail 'cannot create proof NetworkPolicy'
  render_proof_manifest "$ROOT/deploy/templates/nats-realtime-acl-proof.yaml" "$name" "$generation" |
    kubectl create -n "$NS" -f - >/dev/null 2>&1 ||
    fail 'cannot create live ACL proof Job'
  kubectl wait --for=condition=complete "job/$name" -n "$NS" --timeout=300s >/dev/null 2>&1 ||
    fail 'live ACL proof Job did not complete'
  logs="$(kubectl logs "job/$name" -n "$NS" -c realtime-proof --tail=5 --limit-bytes=512 2>/dev/null)" ||
    fail 'cannot read live ACL proof result'
  expected="NATS_LIVE_ACL_PROOF=PASS generation=${generation} acl_sha=${PROOF_ACL_SHA}"
  [[ "$logs" == "$expected" ]] || fail 'live ACL proof did not return the exact PASS token'
)

marker_state() {
  local marker
  if marker="$(read_optional configmap voice-nats-generation)"; then
    check_identity "$marker" ConfigMap voice-nats-generation
    MARKER_PHASE="$(jq -er '.data.phase' <<<"$marker")" || fail 'invalid generation marker phase'
    MARKER_GENERATION="$(jq -er '.data.generation' <<<"$marker")" || fail 'invalid generation marker generation'
    MARKER_PREVIOUS="$(jq -er '.data.previousGeneration' <<<"$marker")" || fail 'invalid generation marker previous generation'
    MARKER_RESOURCE_VERSION="$(jq -r '.metadata.resourceVersion // empty' <<<"$marker")"
    [[ "$MARKER_GENERATION" == legacy ]] || valid_generation "$MARKER_GENERATION" || fail 'invalid generation marker'
    [[ "$MARKER_PREVIOUS" == legacy ]] || valid_generation "$MARKER_PREVIOUS" || fail 'invalid previous generation marker'
  else
    MARKER_PHASE=absent MARKER_GENERATION=legacy MARKER_PREVIOUS=''
    MARKER_RESOURCE_VERSION=''
  fi
}

recover_legacy() {
  local target="$1" pvc hub pods svc service leaf patch marker
  [[ "$target" == r20260930a1 ]] || fail 'legacy recovery is scoped to r20260930a1'
  marker_state
  [[ "$MARKER_PHASE" == rotating && "$MARKER_GENERATION" == "$target" && "$MARKER_PREVIOUS" == legacy ]] ||
    fail 'legacy recovery marker does not match the interrupted rotation'
  [[ "$MARKER_RESOURCE_VERSION" =~ ^[0-9]+$ ]] || fail 'legacy recovery marker has no resourceVersion'
  check_selector
  check_no_second_hub
  svc="$(read_required service voice-nats)"
  jq -e 'any(.spec.ports[]?; .name == "client" and .port == 4222 and .targetPort == 4222)' <<<"$svc" >/dev/null ||
    fail 'voice-nats client Service port differs'
  check_secret_set legacy source
  pvc="$(read_required pvc voice-nats-jsdata)"
  check_identity "$pvc" PersistentVolumeClaim voice-nats-jsdata
  jq -e '.status.phase == "Bound"' <<<"$pvc" >/dev/null || fail 'legacy NATS PVC is not Bound'
  hub="$(read_required deployment voice-nats-pvc-candidate)"
  check_hub "$hub" legacy
  jq -e '.status.readyReplicas == 1 and .status.observedGeneration >= .metadata.generation' <<<"$hub" >/dev/null ||
    fail 'legacy NATS hub is not ready'
  pods="$(kubectl get pods -n "$NS" -l app=voice-nats-pvc-candidate -o json)" || fail 'cannot inspect hub Pods'
  jq -e '(.items | length) == 1 and .items[0].metadata.deletionTimestamp == null and
    .items[0].metadata.annotations["voice.io/nats-generation"] == "legacy" and
    .items[0].status.phase == "Running" and
    (.items[0].status.containerStatuses | length) > 0 and
    all(.items[0].status.containerStatuses[]; .ready == true)' <<<"$pods" >/dev/null ||
    fail 'legacy hub Pod is not the sole ready candidate'
  for service in "${LEAVES[@]}"; do
    leaf="$(read_required deployment "voice-${service}")"
    check_leaf "$leaf" "voice-${service}" "$service" legacy 0
    jq -e --arg app "voice-${service}" '.spec.selector.matchLabels.app == $app and .spec.template.metadata.labels.app == $app' <<<"$leaf" >/dev/null ||
      fail "unexpected leaf selector for voice-${service}"
    pods="$(kubectl get pods -n "$NS" -l "app=voice-${service}" -o json)" || fail "cannot inspect voice-${service} Pods"
    jq -e '(.items | length) == 0' <<<"$pods" >/dev/null || fail "voice-${service} still has Pods"
  done
  marker="$(read_required configmap voice-nats-generation)"
  jq -e --arg rv "$MARKER_RESOURCE_VERSION" --arg target "$target" '
    .metadata.resourceVersion == $rv and .data.phase == "rotating" and
    .data.generation == $target and .data.previousGeneration == "legacy"
  ' <<<"$marker" >/dev/null || fail 'generation marker changed during recovery preflight'

  start_leaves
  patch="$(jq -cn --arg rv "$MARKER_RESOURCE_VERSION" --arg target "$target" '
    [{op:"test",path:"/metadata/resourceVersion",value:$rv},
     {op:"test",path:"/data/phase",value:"rotating"},
     {op:"test",path:"/data/generation",value:$target},
     {op:"test",path:"/data/previousGeneration",value:"legacy"},
     {op:"replace",path:"/data/phase",value:"active"},
     {op:"replace",path:"/data/generation",value:"legacy"},
     {op:"replace",path:"/data/previousGeneration",value:$target}]
  ')" || fail 'cannot prepare legacy recovery marker CAS'
  kubectl patch configmap voice-nats-generation -n "$NS" --type=json -p "$patch" >/dev/null ||
    fail 'generation marker changed during legacy recovery'
  echo 'NATS_RECOVERY=LEGACY_ACTIVE'
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
  local generation="$1" json patch rv
  json="$(read_required deployment voice-nats-pvc-candidate)"
  rv="$(jq -er '.metadata.resourceVersion | select(type == "string" and test("^[0-9]+$"))' <<<"$json")" || fail 'candidate hub has no resourceVersion'
  patch="$(jq -cn --argjson pod "$json" --arg rv "$rv" --arg pvc "$(ref voice-nats-jsdata "$generation")" --arg op "$(ref voice-nats-operator "$generation")" --arg tls "$(ref voice-nats-hub-tls "$generation")" --arg gen "$generation" '
    [ {op:"test",path:"/metadata/resourceVersion",value:$rv},
      {op:"add",path:"/spec/strategy",value:{type:"Recreate"}},
      {op:"add",path:"/spec/template/metadata/annotations",value: (($pod.spec.template.metadata.annotations // {}) + {"voice.io/nats-generation":$gen})} ] +
    [ $pod.spec.template.spec.volumes | to_entries[] | select(.value.name == "jsdata" or .value.name == "nats-resolver-input" or .value.name == "nats-operator-jwt" or .value.name == "nats-hub-tls") |
      {op:"replace",path:("/spec/template/spec/volumes/" + (.key|tostring) + (if .value.name == "jsdata" then "/persistentVolumeClaim/claimName" else "/secret/secretName" end)),
       value:(if .value.name == "jsdata" then $pvc elif .value.name == "nats-hub-tls" then $tls else $op end)} ]
  ')" || fail 'cannot prepare hub patch'
  kubectl patch deployment voice-nats-pvc-candidate -n "$NS" --type=json -p "$patch" >/dev/null
}

patch_leaf() {
  local service="$1" generation="$2" json patch rv
  json="$(read_required deployment "voice-${service}")"
  rv="$(jq -er '.metadata.resourceVersion | select(type == "string" and test("^[0-9]+$"))' <<<"$json")" || fail "${service} leaf has no resourceVersion"
  patch="$(jq -cn --argjson pod "$json" --arg rv "$rv" --arg creds "$(ref voice-nats-service-credentials "$generation")" --arg tls "$(ref voice-nats-hub-tls "$generation")" --arg gen "$generation" '
    [ {op:"test",path:"/metadata/resourceVersion",value:$rv},
      {op:"add",path:"/spec/strategy",value:{type:"Recreate"}},
      {op:"add",path:"/spec/template/metadata/annotations",value:(($pod.spec.template.metadata.annotations // {}) + {"voice.io/nats-generation":$gen})} ] +
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
  local phase="$1" generation="$2" previous="$3" patch
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
  else
    # Compare-and-swap the state observed by this process. A concurrent
    # operator cannot silently overwrite an activation or rollback marker.
    patch="$(jq -cn --arg oldPhase "$MARKER_PHASE" --arg oldGen "$MARKER_GENERATION" --arg oldPrev "$MARKER_PREVIOUS" --arg phase "$phase" --arg gen "$generation" --arg prev "$previous" '
      [{op:"test",path:"/data/phase",value:$oldPhase},
       {op:"test",path:"/data/generation",value:$oldGen},
       {op:"test",path:"/data/previousGeneration",value:$oldPrev},
       {op:"replace",path:"/data/phase",value:$phase},
       {op:"replace",path:"/data/generation",value:$gen},
       {op:"replace",path:"/data/previousGeneration",value:$prev}]
    ')" || fail 'cannot prepare generation marker transition'
    kubectl patch configmap voice-nats-generation -n "$NS" --type=json -p "$patch" >/dev/null || fail 'generation marker changed concurrently'
  fi
  MARKER_PHASE="$phase"
  MARKER_GENERATION="$generation"
  MARKER_PREVIOUS="$previous"
}

render_job() {
  local file="$1" generation="$2" registry="${ROTATION_IMAGE_REGISTRY:-}" tag="${ROTATION_IMAGE_TAG:-}"
  [[ "$registry" =~ ^[A-Za-z0-9./:_-]+$ && "$tag" =~ ^[A-Za-z0-9._-]+$ ]] || fail 'VOICE_IMAGE_REGISTRY and VOICE_IMAGE_TAG are required and must be safe template tokens'
  sed -e "s|__NAMESPACE__|${NS}|g" \
      -e "s|__IMAGE_REGISTRY__|${registry}|g" -e "s|__IMAGE_TAG__|${tag}|g" \
      -e "s|voice-nats-bootstrap-credentials|$(ref voice-nats-bootstrap-credentials "$generation")|g" \
      -e "s|voice-nats-service-credentials|$(ref voice-nats-service-credentials "$generation")|g" \
      -e "s|voice-nats-hub-tls|$(ref voice-nats-hub-tls "$generation")|g" "$file" |
    awk -v generation="$generation" '
      /^kind: / {job=($2 == "Job")}
      job && /^  namespace: / {print; print "  annotations:"; print "    voice.io/nats-generation: " generation; job=0; next}
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
  local mode="${1:-}" target bundle proof_credential source pvc class size selector old
  case "$mode" in
    --recover-legacy)
      [[ $# -eq 2 ]] || fail 'usage: rotate-nats-root.sh --recover-legacy r20260930a1'
      recover_legacy "$2"
      ;;
    --activate)
      [[ $# -eq 4 ]] || fail 'usage: rotate-nats-root.sh --activate GENERATION BUNDLE.json PROOF.creds | --rollback'
      target="$2" bundle="$3" proof_credential="$4"
      valid_generation "$target" || fail 'invalid generation token'
      check_bundle "$bundle" "$target"
      prepare_proof "$target" "$bundle" "$proof_credential"
      marker_state
      [[ "$MARKER_PHASE" == absent || "$MARKER_PHASE" == active ]] || fail 'generation marker is not active'
      source="$MARKER_GENERATION"
      [[ "$target" != "$source" ]] || fail 'target generation is already active'
      check_selector
      check_no_second_hub
      check_secret_set "$source" source
      check_secret_set "$target" target
      pvc="$(read_required pvc "$(ref voice-nats-jsdata "$source")")"
      class="$(jq -er '.spec.storageClassName' <<<"$pvc")" || fail 'source PVC has no storage class'
      size="$(jq -er '.spec.resources.requests.storage' <<<"$pvc")" || fail 'source PVC has no storage size'
      [[ "$class" =~ ^[a-z0-9][a-z0-9.-]*$ && "$size" =~ ^[1-9][0-9]*(Mi|Gi|Ti)$ ]] || fail 'source PVC storage class or size is unsafe'
      [[ -z "${VOICE_NATS_STORAGE_CLASS:-}" || "$class" == "$VOICE_NATS_STORAGE_CLASS" ]] || fail 'source PVC class differs from approved input'
      [[ -z "${VOICE_NATS_STORAGE_SIZE:-}" || "$size" == "$VOICE_NATS_STORAGE_SIZE" ]] || fail 'source PVC size differs from approved input'
      check_source_pvc "$pvc" "$(ref voice-nats-jsdata "$source")" "$class" "$size"
      if read_optional pvc "$(ref voice-nats-jsdata "$target")" >/dev/null; then fail 'target PVC already exists'; fi
      check_workloads "$source"
      prepare_realtime_image
      kubectl create -n "$NS" --dry-run=server -o name -f "$bundle" >/dev/null 2>&1 || fail 'target Secrets rejected by API dry-run'
      prepare_pvc "$target" "$class" "$size" | kubectl create -n "$NS" --dry-run=server -o name -f - >/dev/null 2>&1 || fail 'target PVC rejected by API dry-run'
      preflight_proof_resources "$PROOF_NAME" "$target" "$proof_credential"
      set_marker rotating "$target" "$source"
      kubectl create -n "$NS" -f "$bundle" >/dev/null
      prepare_pvc "$target" "$class" "$size" | kubectl create -n "$NS" -f - >/dev/null
      stop_leaves
      stop_hub
      patch_hub "$target"
      start_hub
      run_jobs "$target"
      run_live_acl_proof "$PROOF_NAME" "$target" "$proof_credential"
      printf 'NATS_LIVE_ACL_PROOF=PASS generation=%s acl_sha=%s\n' "$target" "$PROOF_ACL_SHA"
      for old in "${LEAVES[@]}"; do patch_leaf "$old" "$target"; done
      start_leaves
      set_marker active "$target" "$source"
      echo "NATS root generation ${target} active in voice-staging"
      ;;
    --rollback)
      [[ $# -eq 1 ]] || fail 'usage: rotate-nats-root.sh --activate GENERATION BUNDLE.json PROOF.creds | --rollback'
      marker_state
      [[ "$MARKER_PHASE" == active || "$MARKER_PHASE" == rotating ]] || fail 'no active or interrupted rotation to roll back'
      target="$MARKER_GENERATION" source="$MARKER_PREVIOUS"
      valid_generation "$target" || fail 'legacy generation has no root rotation to roll back'
      [[ "$source" == legacy ]] || valid_generation "$source" || fail 'invalid retained generation'
      [[ "$source" != "$target" ]] || fail 'rollback would be a no-op'
      recover_proof_resources "$target"
      check_selector
      check_no_second_hub
      check_secret_set "$source" source
      pvc="$(read_required pvc "$(ref voice-nats-jsdata "$source")")"
      class="$(jq -er '.spec.storageClassName' <<<"$pvc")" || fail 'retained PVC has no storage class'
      size="$(jq -er '.spec.resources.requests.storage' <<<"$pvc")" || fail 'retained PVC has no storage size'
      [[ "$class" =~ ^[a-z0-9][a-z0-9.-]*$ && "$size" =~ ^[1-9][0-9]*(Mi|Gi|Ti)$ ]] || fail 'retained PVC storage contract is unsafe'
      check_source_pvc "$pvc" "$(ref voice-nats-jsdata "$source")" "$class" "$size"
      # A failed activation may stop immediately after writing the rotating
      # marker, before either target resource exists. Retained source assets
      # alone are sufficient to restore the previous generation.
      # The active marker proves the target is fully converged. Interrupted
      # activation may have mixed refs, so only the fixed workload identities
      # are required before forcing every reference back to the retained set.
      for old in voice-nats-pvc-candidate "${LEAVES[@]/#/voice-}"; do
        read_required deployment "$old" >/dev/null
      done
      prepare_realtime_image
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
    *) fail 'usage: rotate-nats-root.sh --activate GENERATION BUNDLE.json PROOF.creds | --rollback' ;;
  esac
}

main "$@"
