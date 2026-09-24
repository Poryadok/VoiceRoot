#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RESET="${ROOT}/scripts/staging/reset-voice-staging-namespace.sh"
fail() { echo "FAIL: $*" >&2; exit 1; }
test -x "$RESET" || fail 'reset helper must be executable'

tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT
mkdir -p "${tmp}/bin"
cat >"${tmp}/bin/kubectl" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${KUBECTL_LOG:?}"
EOF
chmod +x "${tmp}/bin/kubectl"

if PATH="${tmp}/bin:${PATH}" KUBECTL_LOG="${tmp}/denied.log" VOICE_K8S_NAMESPACE=voice-staging bash "$RESET" >/dev/null 2>&1; then
  fail 'reset must reject a missing explicit approval'
fi
[ ! -f "${tmp}/denied.log" ] || fail 'approval must be checked before any cluster command'

if PATH="${tmp}/bin:${PATH}" KUBECTL_LOG="${tmp}/wrong-namespace.log" VOICE_STAGING_WIPE_APPROVED=true VOICE_K8S_NAMESPACE=voice-prod VOICE_NATS_STORAGE_CLASS=local-path VOICE_NATS_STORAGE_SIZE=20Gi bash "$RESET" >/dev/null 2>&1; then
  fail 'reset must reject every namespace except voice-staging'
fi
[ ! -f "${tmp}/wrong-namespace.log" ] || fail 'namespace guard must run before any cluster command'

PATH="${tmp}/bin:${PATH}" \
KUBECTL_LOG="${tmp}/allowed.log" \
VOICE_STAGING_WIPE_APPROVED=true \
VOICE_K8S_NAMESPACE=voice-staging \
VOICE_NATS_STORAGE_CLASS=local-path \
VOICE_NATS_STORAGE_SIZE=20Gi \
  bash "$RESET" >/dev/null

grep -Fxq 'get namespace voice-staging' "${tmp}/allowed.log" || fail 'reset must confirm the dedicated namespace exists'
grep -Fxq 'delete namespace voice-staging --wait=true --timeout=300s' "${tmp}/allowed.log" || fail 'reset must delete only the dedicated namespace'
grep -Fxq "apply -f ${ROOT}/deploy/staging/namespace.yaml" "${tmp}/allowed.log" || fail 'reset must recreate the same namespace'
grep -Fxq 'wait --for=jsonpath={.status.phase}=Active namespace/voice-staging --timeout=120s' "${tmp}/allowed.log" || fail 'reset must wait for the namespace before continuing'
grep -Fxq 'create configmap voice-nats-clean-install-state --namespace voice-staging --from-literal=mode=clean-install --from-literal=storageClass=local-path --from-literal=storageSize=20Gi' "${tmp}/allowed.log" || fail 'reset must record the clean-install mode and verified storage request'
[ "$(wc -l <"${tmp}/allowed.log" | tr -d ' ')" -eq 5 ] || fail 'reset must issue only the five expected namespace-scoped commands'

echo 'staging Voice clean-install reset contract: OK'
