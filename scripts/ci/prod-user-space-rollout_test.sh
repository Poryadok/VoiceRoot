#!/usr/bin/env bash
# Exercise production dispatch, its real Python preflight and the subset selector.
set -euo pipefail
command -v python3 >/dev/null
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "${fixture}"' EXIT
mkdir -p "${fixture}/scripts/"{ci,prod,staging,storage} "${fixture}/bin"
cp "${ROOT}/scripts/prod/render-and-apply-prod.sh" "${fixture}/scripts/prod/"
cp "${ROOT}/scripts/prod/production_image_refs.py" "${fixture}/scripts/prod/"
cp "${ROOT}/scripts/ci/staging-image-catalog.json" "${fixture}/scripts/ci/"
cp "${ROOT}/scripts/staging/rollout-subset.sh" "${fixture}/scripts/staging/"
for file in prod/load-prod-domains.sh prod/map-prod-env.sh staging/rollout-helpers.sh; do
  printf ':\n' > "${fixture}/scripts/${file}"
done
for file in prod/apply-infra.sh prod/apply-gateway-ingress.sh prod/apply-app-manifests.sh \
  prod/apply-livekit-ingress.sh staging/rollout-app-tier.sh staging/rollout-user-space-tier.sh \
  staging/deploy-changed.sh staging/apply-migrate-jobs.sh staging/repair-auth-flyway.sh \
  storage/apply-signing-env.sh storage/apply-minio-cors.sh; do
  cat > "${fixture}/scripts/${file}" <<'STUB'
set -euo pipefail
step="${0##*/}"
printf '%s %s\n' "${step}" "${VOICE_K8S_NAMESPACE}" >> "${CALLS}"
if [ "${FAIL_STEP:-}" = "${step}" ]; then exit 7; fi
STUB
done
cat > "${fixture}/bin/kubectl" <<'STUB'
#!/usr/bin/env bash
printf 'kubectl %s\n' "$*" >> "${CALLS}"
STUB
chmod 755 "${fixture}/bin/kubectl"
export PATH="${fixture}/bin:${PATH}"
export VOICE_IMAGE_REGISTRY=example.invalid/voice VOICE_IMAGE_TAG=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
export VOICE_K8S_NAMESPACE=voice-prod CHANGED_SERVICES=user,space
export VOICE_APPLY_OBSERVABILITY=false
export CALLS="${fixture}/calls"
run() {
  : > "${CALLS}"
  local status=0
  bash "${fixture}/scripts/prod/render-and-apply-prod.sh" > "${fixture}/output" 2>&1 || status=$?
  if [ "${status}" -ne 0 ]; then cat "${fixture}/output" >&2; fi
  return "${status}"
}
has() {
  if ! grep -qFx -- "$1" "${CALLS}"; then
    echo "FAIL missing expected call: $1" >&2
    cat "${CALLS}" >&2
    exit 1
  fi
}
absent() { if grep -qF -- "$1" "${CALLS}"; then cat "${CALLS}" >&2; exit 1; fi; }

DEPLOY_MODE=full NEEDS_FULL_ROLLOUT=false NEEDS_USER_SPACE_ROLLOUT=false run
has 'rollout-app-tier.sh voice-prod'
absent 'rollout-user-space-tier.sh'
echo 'PASS default full rollout'

DEPLOY_MODE=full NEEDS_FULL_ROLLOUT=false NEEDS_USER_SPACE_ROLLOUT=true run
has 'apply-infra.sh voice-prod'
has 'rollout-user-space-tier.sh voice-prod'
absent 'rollout-app-tier.sh'
echo 'PASS explicit user/space subset in full mode'

DEPLOY_MODE=full NEEDS_FULL_ROLLOUT=true NEEDS_USER_SPACE_ROLLOUT=true run
has 'rollout-app-tier.sh voice-prod'
absent 'rollout-user-space-tier.sh'
echo 'PASS full rollout takes precedence'

DEPLOY_MODE=app-only NEEDS_FULL_ROLLOUT=false NEEDS_USER_SPACE_ROLLOUT=true run
has 'rollout-user-space-tier.sh voice-prod'
absent 'apply-infra.sh'
echo 'PASS existing app-only subset'

VOICE_K8S_NAMESPACE=custom-prod DEPLOY_MODE=full NEEDS_FULL_ROLLOUT=false NEEDS_USER_SPACE_ROLLOUT=true run
has 'rollout-user-space-tier.sh custom-prod'
has 'kubectl rollout status deployment/voice-gateway -n custom-prod --timeout=300s'
absent 'voice-staging'
echo 'PASS explicit production namespace'

set +e
DEPLOY_MODE=full NEEDS_FULL_ROLLOUT=false NEEDS_USER_SPACE_ROLLOUT=true FAIL_STEP=rollout-user-space-tier.sh run
result=$?
set -e
test "${result}" -eq 7
absent 'apply-livekit-ingress.sh'
absent 'kubectl rollout status'
echo 'PASS subset failure stops production completion'

set +e
VOICE_IMAGE_TAG=fixture DEPLOY_MODE=full NEEDS_FULL_ROLLOUT=false NEEDS_USER_SPACE_ROLLOUT=true run
result=$?
set -e
test "${result}" -ne 0
test ! -s "${CALLS}"
echo 'PASS real image preflight rejects invalid tag before effects'
echo 'Production User/Space rollout: 7 cases passed (isolated shell fixture).'
