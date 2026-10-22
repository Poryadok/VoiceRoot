#!/usr/bin/env bash
# Exercise production dispatch, its real Python preflight and the subset selector.
set -euo pipefail
command -v python3 >/dev/null
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "${fixture}"' EXIT
mkdir -p "${fixture}/scripts/"{ci,prod,staging,storage} "${fixture}/bin"
mkdir -p "${fixture}/scripts/staging/nats-rollout-preservation" "${fixture}/deploy/prod"
cp "${ROOT}/scripts/prod/render-and-apply-prod.sh" "${fixture}/scripts/prod/"
cp "${ROOT}/scripts/prod/production_image_refs.py" "${fixture}/scripts/prod/"
cp "${ROOT}/scripts/ci/staging-image-catalog.json" "${fixture}/scripts/ci/"
cp "${ROOT}/scripts/staging/nats-rollout-preservation/release_policy_wiring.py" "${fixture}/scripts/staging/nats-rollout-preservation/"
cp "${ROOT}/deploy/prod/services.yaml" "${fixture}/deploy/prod/services.base.yaml"
cp "${ROOT}/src/backend/pkg/releasepolicy/testdata/free-invite-alpha.json" "${fixture}/policy-fixture.json"
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
  # Supply the real mandatory preflight with explicitly synthetic alpha input.
  # Namespace forwarding is a fixture contract, not live TLS/image qualification.
  python3 - "${fixture}" "${VOICE_K8S_NAMESPACE}" <<'PY'
import hashlib, json, sys
from pathlib import Path
root, namespace = Path(sys.argv[1]), sys.argv[2]
raw = (root / 'policy-fixture.json').read_text()
policy = json.loads(raw)
descriptor = {'profile': 'free-invite-alpha', 'source_sha': policy['source_sha'],
    'policy_sha256': hashlib.sha256(raw.encode()).hexdigest(),
    'policy_object': {'apiVersion': 'v1', 'kind': 'ConfigMap',
        'metadata': {'name': 'voice-fixture-policy', 'namespace': namespace},
        'immutable': True, 'data': {'policy.json': raw}},
    'owners': {name: {'policy_directory': '/etc/fixture-' + name,
                      'snapshot_file': '/tmp/fixture-' + name + '.json'}
               for name in ('auth', 'user', 'notification', 'realtime')}}
(root / 'input.json').write_text(json.dumps(descriptor))
text = (root / 'deploy/prod/services.base.yaml').read_text(encoding='utf-8-sig')
(root / 'deploy/prod/services.yaml').write_text(text.replace('namespace: voice-prod', 'namespace: ' + namespace))
PY
  export VOICE_RELEASE_POLICY_INPUT="${fixture}/input.json"
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
python3 "${ROOT}/scripts/prod/policy-owner-images-only_test.py"
