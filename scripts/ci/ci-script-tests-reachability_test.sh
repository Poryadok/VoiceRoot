#!/usr/bin/env bash
# Regression guard: the local CI-script suite must be reachable from Actions.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORKFLOW="${ROOT}/.github/workflows/ci.yml"
PATH_FILTERS="${ROOT}/.github/ci/path-filters.yml"
REQUIRED_JOBS="${ROOT}/.github/ci/verify-required-jobs.sh"
GO_DOWNLOAD_HELPER="src/backend/scripts/docker-go-mod-download.sh"
VOICE_R22_BASE_RESOLVER="${ROOT}/scripts/ci/resolve-voice-r22-base.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

job_block="$(sed -n '/^  ci-script-tests:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
[[ -n "${job_block}" ]] || fail "CI workflow must define a ci-script-tests job"

global_paths="$(sed -n '/^global:$/,/^[[:alnum:]_]*:$/p' "${PATH_FILTERS}")"
echo "${global_paths}" | grep -Fxq "  - ${GO_DOWNLOAD_HELPER}" \
  || fail "Docker Go module download helper must be a global CI-policy path"

pr_trigger="$(sed -n '/^  pull_request:$/,/^  push:$/p' "${WORKFLOW}" | sed '$d')"
echo "${pr_trigger}" | grep -Fq 'master' \
  || fail "CI must run for PRs targeting master"
echo "${pr_trigger}" | grep -Fq 'develop' \
  || fail "CI must run for PRs targeting develop"
echo "${pr_trigger}" | grep -Fq 'codex/game-sdk-federation-docs' \
  || fail "CI must run for PRs targeting the game SDK feature base"
echo "${pr_trigger}" | grep -Fq 'codex/appearance-settings-view' \
  || fail "CI must run for PRs targeting the Appearance feature base"
echo "${pr_trigger}" | grep -Fq 'codex/settings-app-icon' \
  || fail "CI must run for PRs targeting the App Icon feature base"
pr_branches="$(echo "${pr_trigger}" | sed -n 's/.*branches: \[\(.*\)\].*/\1/p')"
[[ -n "${pr_branches}" && "${pr_branches}" != *'*'* && "${pr_branches}" != *'?'* ]] \
  || fail "CI pull-request branch filter must remain exact, without wildcard widening"
key_backup_job_block="$(tr -d '\r' < "${WORKFLOW}" | sed -n '/^  flutter-key-backup-goldens:$/,/^  [[:alnum:]_-]*:$/p')"
[[ -n "${key_backup_job_block}" ]] || fail "CI must define the cross-platform key-backup golden job"
echo "${key_backup_job_block}" | grep -Fq 'os: ubuntu-latest' \
  || fail "key-backup golden job must run on Linux"
echo "${key_backup_job_block}" | grep -Fq 'os: windows-latest' \
  || fail "key-backup golden job must run on Windows"
echo "${key_backup_job_block}" | grep -Fq 'flutter-version: ${{ env.FLUTTER_VERSION }}' \
  || fail "key-backup golden job must use the pinned Flutter version"
echo "${key_backup_job_block}" | grep -Fq 'flutter test test/e2e_key_backup_settings_test.dart' \
  || fail "key-backup golden job must run the targeted golden test"
echo "${key_backup_job_block}" | grep -Fq 'VOICE_E2E_KEY_BACKUP_CAPTURE_DIR' \
  || fail "key-backup golden job must preserve actual platform captures"
echo "${key_backup_job_block}" | grep -Fq 'flutter-linux-prefetch-sqlite3.sh host' \
  || fail "Linux key-backup golden job must prefetch SQLite native assets"
echo "${key_backup_job_block}" | grep -Fq 'flutter-windows-prefetch-sqlite3.ps1' \
  || fail "Windows key-backup golden job must prefetch SQLite native assets"
echo "${key_backup_job_block}" | grep -Fq 'e2e-key-backup-golden-${{ matrix.platform }}-${{ github.sha }}' \
  || fail "golden evidence artifact must identify platform and source SHA"
echo "${key_backup_job_block}" | grep -Fq 'if: always()' \
  || fail "golden evidence must be collected on success and failure"
push_trigger="$(sed -n '/^  push:$/,/^  schedule:$/p' "${WORKFLOW}" | sed '$d')"
echo "${push_trigger}" | grep -Fq 'branches: [master, develop]' \
  || fail "CI push trigger must target only master and develop"
deploy_block="$(sed -n '/^  deploy-staging:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
[[ -n "${deploy_block}" ]] || fail "CI workflow must define deploy-staging"
echo "${deploy_block}" | grep -Fq "github.event_name == 'push'" \
  || fail "staging deployment must remain push-only"
echo "${deploy_block}" | grep -Fq "github.ref == 'refs/heads/master'" \
  || fail "staging deployment must remain master-only"
echo "${deploy_block}" | grep -Fq "vars.STAGING_DEPLOY_ENABLED == 'true'" \
  || fail "staging deployment must retain its explicit enable gate"
echo "${deploy_block}" | grep -Fq "needs.staging-stack-lock.result == 'success'" \
  || fail "staging deployment must retain the successful stack-lock gate"
echo "${deploy_block}" | grep -Fq '      - staging-stack-lock' \
  || fail "staging deployment must depend on the stack-lock job"
echo "${deploy_block}" | grep -Fq "needs.changes.outputs.ci_only_change != 'true'" \
  || fail "CI-only changes must never enter automatic staging deploy"
lock_block="$(sed -n '/^  staging-stack-lock:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
echo "${lock_block}" | grep -Fq "needs.changes.outputs.ci_only_change != 'true'" \
  || fail "CI-only changes must not publish a deployable stack lock"
! grep -Fq 'github.event.pull_request.draft' "${WORKFLOW}" \
  || fail "draft PRs must use the same path-filtered CI selection"
changes_block="$(sed -n '/^  changes:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
echo "${changes_block}" | grep -Fq "github.event_name == 'pull_request'" \
  || fail "draft feature-base PRs must enter the normal changes job"
echo "${changes_block}" | grep -Fq 'uses: dorny/paths-filter@v4' \
  || fail "feature-base PRs must use the existing path-filtered job selection"
echo "${changes_block}" | grep -Fq 'filters: .github/ci/path-filters.yml' \
  || fail "feature-base PRs must use the repository path filter definitions"
grep -Fq 'ci_only_change: ${{ steps.stagingmatrix.outputs.ci_only_change }}' "${WORKFLOW}" \
  || fail "changes must expose the CI-only no-publication classification"
grep -Fq 'dev_web_publish: ${{ steps.filter.outputs.dev_web_publish }}' "${WORKFLOW}" \
  || fail "changes must expose the narrow DEV web publisher selector"
grep -Fq 'dev_web_publish:' "${ROOT}/.github/ci/path-filters.yml" \
  || fail "DEV web publisher changes must have a dedicated path selector"
dev_web_filter_block="$(sed -n '/^dev_web_publish:$/,/^[[:alnum:]_-]*:$/p' "${ROOT}/.github/ci/path-filters.yml")"
echo "${dev_web_filter_block}" | grep -Fq '  - .github/workflows/ci.yml' \
  || fail "DEV web publisher selector must include its workflow contract"
echo "${dev_web_filter_block}" | grep -Fq '  - .github/ci/path-filters.yml' \
  || fail "DEV web publisher selector must include its own filter definition"
! echo "${dev_web_filter_block}" | grep -Fq '  - **' \
  || fail "DEV web publisher selector must remain narrow"
web_block="$(sed -n '/^  web:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
echo "${web_block}" | grep -Fq 'Build and push DEV web image' \
  || fail "master web job must publish the DEV-configured frontend"
echo "${web_block}" | grep -Fq '${{ steps.image.outputs.base }}/dev-web:${{ github.sha }}' \
  || fail "DEV web image must use an immutable source-SHA tag"
echo "${web_block}" | grep -Fq 'VOICE_RUNTIME_CONFIG_REQUIRED=true' \
  || fail "promotable web image must require explicit runtime binding"
! echo "${web_block}" | grep -Eq 'VOICE_API_BASE_URL=|VOICE_LIVEKIT_URL=' \
  || fail "promotable web image cannot embed environment endpoints"
echo "${web_block}" | grep -Fq 'dev-web@${{ steps.build-dev-web.outputs.digest }}' \
  || fail "DEV web receipt must bind the source-SHA tag to Buildx manifest digest"
echo "${web_block}" | grep -Fq 'IMAGE_REF: ${{ steps.image.outputs.base }}/web@${{ steps.build-dev-web.outputs.digest }}' \
  || fail "DEV receipt must use the same canonical web digest as STAGING"
echo "${web_block}" | grep -Fq '${{ steps.image.outputs.base }}/web:${{ github.sha }}' \
  || fail "staging web image publication must retain its source-SHA tag"

staging_deploy_workflow="${ROOT}/.github/workflows/staging-deploy.yml"
image_verify_line="$(grep -n '^      - name: Verify every exact staging image before cluster access$' "${staging_deploy_workflow}" | cut -d: -f1)"
kubectl_setup_line="$(grep -n '^      - uses: azure/setup-kubectl@v5$' "${staging_deploy_workflow}" | head -n1 | cut -d: -f1)"
[[ -n "${image_verify_line}" && -n "${kubectl_setup_line}" && "${image_verify_line}" -lt "${kubectl_setup_line}" ]] \
  || fail "staging image manifests must be verified before kubectl setup or cluster access"
grep -A7 '^      - name: Verify every exact staging image before cluster access$' "${staging_deploy_workflow}" \
  | grep -Fq 'VOICE_IMAGE_REGISTRY: ghcr.io/poryadok/voiceroot' \
  || fail "staging image preflight must use the trusted registry"
grep -Fq 'VOICE_IMAGE_TAG: ${{ inputs.image_tag }}' "${staging_deploy_workflow}" \
  || fail "all staging deploys must verify the exact requested image tag"

integration_pr_block="$(sed -n '/^  backend-go-integration-pr:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
backend_go_block="$(sed -n '/^  backend-go:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
grep -Fq 'integration_go_services: ${{ steps.gomatrix.outputs.integration_go_services }}' "${WORKFLOW}" \
  || fail "changes must publish a separate changed-service integration matrix"
echo "${integration_pr_block}" | grep -Fq 'fromJSON(needs.changes.outputs.integration_go_services)' \
  || fail "PR integration tests must use the changed-service matrix"
echo "${backend_go_block}" | grep -Fq 'fromJSON(needs.changes.outputs.go_services)' \
  || fail "normal backend Go CI must retain its broad Go test matrix"
grep -Fq "needs.changes.outputs.integration_go_services != '[]'" "${WORKFLOW}" \
  || fail "PR integration matrix must skip when no Go service path changed"
grep -Fq 'RUN_GO_INTEGRATION: ${{ (github.event_name == '\''pull_request'\'' || needs.changes.outputs.full_validation == '\''true'\'') && needs.changes.outputs.integration_go_services != '\''[]'\'' }}' "${WORKFLOW}" \
  || fail "ci-gate must receive whether the PR or full validation integration matrix is scheduled"
grep -Fq 'check_if "${RUN_GO_INTEGRATION}" backend-go-integration-pr' "${REQUIRED_JOBS}" \
  || fail "ci-gate must require PR integration only when its matrix is nonempty"
grep -Fq 'if [[ "${svc}" == "controlledgame" ]]; then' "${ROOT}/scripts/ci/resolve-go-matrix.sh" \
  && grep -Fq 'add_integration_unique gameintegration' "${ROOT}/scripts/ci/resolve-go-matrix.sh" \
  || fail "controlledgame changes must include the Game Integration consumer in PR integration tests"

for minio_job in minio-server-image-publish minio-mc-image-publish; do
  minio_block="$(sed -n "/^  ${minio_job}:$/,/^  [[:alnum:]_-]*:$/p" "${WORKFLOW}")"
  echo "${minio_block}" | grep -Fq "needs.changes.outputs.global == 'true'" \
    || fail "${minio_job} must remain limited to deployment-global changes"
  if echo "${minio_block}" | grep -Fq "needs.changes.outputs.ci_global"; then
    fail "${minio_job} must not publish images for CI-only changes"
  fi
done

echo "${job_block}" | grep -Eq '^    needs: changes$' \
  || fail "ci-script-tests must depend on changes"
echo "${job_block}" | grep -Fq "needs.changes.outputs.global == 'true'" \
  || fail "ci-script-tests must run for existing global CI-policy paths"
ci_global_paths="$(sed -n '/^ci_global:$/,/^[[:alnum:]_]*:$/p' "${PATH_FILTERS}")"
for compatibility_path in \
  'scripts/release/client_compatibility.py' \
  'scripts/release/client_compatibility_test.py'; do
  echo "${ci_global_paths}" | grep -Fxq "  - ${compatibility_path}" \
    || fail "ci_global must select ${compatibility_path} so its changes run ci-script-tests"
done
echo "${job_block}" | grep -Eq '^      - name: CI script regression tests$' \
  || fail "ci-script-tests must name its regression-test step"
echo "${job_block}" | grep -Fxq '        run: make ci-script-tests-${{ matrix.shard }}' \
  || fail "ci-script-tests must invoke every explicit Make shard"
grep -Fq 'scripts/staging/nats-live-acl-proof-contract_test.sh' "${ROOT}/Makefile" \
  || fail "ci-script-tests must include the live NATS ACL proof contract"
echo "${job_block}" | grep -Eq '^          fetch-depth: 0$' \
  || fail "ci-script-tests must fetch master history for scope checks"
echo "${job_block}" | grep -Fq 'VOICE_CI_EVENT_NAME: ${{ github.event_name }}' \
  || fail "ci-script-tests must pass the event name to the Voice scope-base resolver"
echo "${job_block}" | grep -Fq 'bash scripts/ci/resolve-voice-r22-base.sh' \
  || fail "ci-script-tests must use the Voice scope-base resolver"
echo "${job_block}" | grep -Fq 'VOICE_R22_BASE_SHA=${base_sha}' \
  || fail "ci-script-tests must export the resolved Voice scope base"

pr_base='1111111111111111111111111111111111111111'
push_before='2222222222222222222222222222222222222222'
zero_sha='0000000000000000000000000000000000000000'
resolve_voice_base() {
  VOICE_CI_EVENT_NAME="$1" \
    VOICE_CI_PR_BASE_SHA="$2" \
    VOICE_CI_PUSH_BEFORE_SHA="$3" \
    bash "${VOICE_R22_BASE_RESOLVER}"
}
[[ "$(resolve_voice_base pull_request "${pr_base}" "${push_before}")" == "${pr_base}" ]] \
  || fail "Voice scope base must use the pull request base SHA"
[[ "$(resolve_voice_base push "${pr_base}" "${push_before}")" == "${push_before}" ]] \
  || fail "Voice scope base must use a nonzero push before SHA"
[[ -z "$(resolve_voice_base workflow_dispatch "${pr_base}" "${push_before}")" ]] \
  || fail "Voice scope base must fall back for workflow dispatch"
[[ -z "$(resolve_voice_base push "${pr_base}" "${zero_sha}")" ]] \
  || fail "Voice scope base must fall back for a zero push before SHA"
if resolve_voice_base pull_request "${zero_sha}" "${push_before}" >/dev/null 2>&1; then
  fail "Voice scope base must reject an invalid pull request base SHA"
fi

windows_job_block="$(sed -n '/^  flutter-windows:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
[[ -n "${windows_job_block}" ]] || fail "CI workflow must define flutter-windows"
echo "${windows_job_block}" | grep -Fq "github.event_name == 'pull_request'" \
  || fail "Windows desktop build must be selected for native-target PR changes"
echo "${windows_job_block}" | grep -Fq "needs.changes.outputs.windows_desktop == 'true'" \
  || fail "Windows desktop PR selection must use its dedicated path filter"
echo "${windows_job_block}" | grep -Fq "github.event_name == 'workflow_dispatch' && inputs.profile == 'full'" \
  || fail "Windows desktop build must preserve manual full-profile selection"
echo "${windows_job_block}" | grep -Fq "needs.changes.outputs.run_flutter_tier2 == 'true'" \
  || fail "Windows desktop build must preserve its master/manual tier-2 predicate"
gate_block="$(sed -n '/^  ci-gate:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
echo "${gate_block}" | grep -Eq '^      - compose-e2e$' \
  || fail "ci-gate must wait for compose-e2e to finish"
echo "${gate_block}" | grep -Eq '^      - ci-script-tests$' \
  || fail "ci-gate must require ci-script-tests"
grep -Fq 'check_if "$(or_true "${GLOBAL}" "${RUN_CI_SCRIPTS:-false}")" ci-script-tests' "${REQUIRED_JOBS}" \
  || fail "ci-gate must require ci-script-tests for global and scoped CI-policy paths"
echo "${gate_block}" | grep -Eq '^      - flutter-windows$' \
  || fail "ci-gate must wait for the Windows desktop job"
echo "${gate_block}" | grep -Fq 'RUN_WINDOWS_DESKTOP:' \
  || fail "ci-gate must receive the conditional Windows selection"
echo "${gate_block}" | grep -Fq 'JOB_FLUTTER_WINDOWS: ${{ needs.flutter-windows.result }}' \
  || fail "ci-gate must receive the Windows job result"
grep -Fq 'check_if "${RUN_WINDOWS_DESKTOP}" flutter-windows' "${REQUIRED_JOBS}" \
  || fail "ci-gate must require Windows only when its job selector is true"
echo "${gate_block}" | grep -Fq 'JOB_COMPOSE_E2E: ${{ needs.compose-e2e.result }}' \
  || fail "ci-gate must receive the compose-e2e result"
echo "${gate_block}" | grep -Fq 'RUN_COMPOSE_E2E:' \
  || fail "ci-gate must receive whether compose-e2e is scheduled"
compose_gate_env_block="$(printf '%s\n' "${gate_block}" | sed -n '/RUN_COMPOSE_E2E:/,/JOB_PROTOBUF:/p')"
[[ -n "${compose_gate_env_block}" ]] || fail "ci-gate compose-e2e selector must be a complete expression"
grep -Fq 'require_job compose-e2e "${compose_result}"' "${REQUIRED_JOBS}" \
  || fail "ci-gate must require Compose success when its selector is true"

run_compose_required_check() {
  local code="$1" scheduled="$2" result="$3"
  if JOB_CURRENT_DIFF_CONTRACT=success RUN_COMPOSE_E2E="${scheduled}" JOB_COMPOSE_E2E="${result}" \
    bash "${REQUIRED_JOBS}" pull_request "${code}" auto >/dev/null 2>&1; then
    return 0
  fi
  return 1
}
run_compose_required_check true false skipped \
  || fail "ci-gate must allow the intentional compose-e2e path skip"
run_compose_required_check false false skipped \
  || fail "ci-gate must allow a docs-only intentional Compose skip"
run_compose_required_check true false success \
  || fail "ci-gate must tolerate an unneeded but successful compose-e2e run"
run_compose_required_check true true success \
  || fail "ci-gate must accept a successful scheduled compose-e2e run"
if run_compose_required_check true true skipped; then
  fail "ci-gate must reject an unexpectedly skipped scheduled compose-e2e run"
fi
if run_compose_required_check true true failure; then
  fail "ci-gate must reject a failed scheduled compose-e2e run"
fi
if run_compose_required_check true false failure; then
  fail "ci-gate must reject any failed compose-e2e run, including an unexpected run"
fi
run_compose_required_check false true success \
  || fail "ci-gate must check Compose when a PR global policy path schedules it despite code=false"
if run_compose_required_check false true skipped; then
  fail "ci-gate must reject a skipped PR global Compose run even when code=false"
fi
if run_compose_required_check false true failure; then
  fail "ci-gate must reject a failed PR global Compose run even when code=false"
fi

echo "== Windows path-filter restricted-pattern model (not dorny equivalence) =="
python - "${PATH_FILTERS}" <<'PY'
import fnmatch
import re
import sys
from pathlib import Path

text = Path(sys.argv[1]).read_text(encoding="utf-8")
match = re.search(r"(?m)^windows_desktop:\s*\n((?:[ \t]+-\s+[^\n]+\n)+)", text)
if not match:
    raise SystemExit("FAIL: windows_desktop path filter is missing or empty")
patterns = [line.strip()[2:].strip().strip("'\"") for line in match.group(1).splitlines()]
cases = {
    "src/frontend/windows/runner/desktop_host.cpp": True,
    "src/frontend/pubspec.yaml": True,
    "src/frontend/assets/app_icons/voice_sky.png": True,
    "src/frontend/lib/services/windows_desktop_host.dart": False,
    "src/backend/windows/desktop_host.cpp": False,
    "src/frontend/assets/avatars/profile.png": False,
}
for path, expected in cases.items():
    actual = any(fnmatch.fnmatchcase(path, pattern) for pattern in patterns)
    if actual != expected:
        raise SystemExit(f"FAIL: Windows path-filter model for {path}: expected {expected}, got {actual}")
print("Windows path-filter model cases passed; actual dorny selection is verified by the PR job.")
PY

compose_e2e_block="$(sed -n '/^  compose-e2e:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
[[ -n "${compose_e2e_block}" ]] || fail "CI workflow must define a compose-e2e job"
printf '%s\n' "${compose_e2e_block}" | grep -Fq -- "TestComposeSpaceVoiceRoomBidirectionalAudio_live|TestRequestComposeSpaceVoiceRoomJoin.*|TestRequestComposeJoinToken.*" \
  || fail "compose-e2e must run the live bidirectional test and bounded Space media readiness helper regressions"
for selector in \
  "github.event_name == 'pull_request'" \
  "github.event_name == 'push'" \
  "github.ref == 'refs/heads/master'" \
  "needs.changes.outputs.code == 'true'" \
  "needs.changes.outputs.compose == 'true'" \
  "needs.changes.outputs.frontend == 'true'" \
  "needs.changes.outputs.global == 'true'" \
  "needs.changes.outputs.ci_global == 'true'" \
  "needs.changes.outputs.admin == 'true'" \
  "needs.changes.outputs.developer-portal == 'true'"; do
  printf '%s\n' "${compose_e2e_block}" | grep -Fq -- "${selector}" \
    || fail "compose-e2e scheduling predicate is missing ${selector}"
  printf '%s\n' "${compose_gate_env_block}" | grep -Fq -- "${selector}" \
    || fail "ci-gate compose-e2e selector must stay aligned with ${selector}"
done
echo "${compose_e2e_block}" | grep -Eq '^    needs: changes$' \
  || fail "compose-e2e must depend only on changes, keeping the ci-gate DAG acyclic"
compose_env_block="$(printf '%s\n' "${compose_e2e_block}" | sed -n '/^      - name: Prepare compose env$/, /^      - name: Start compose infra$/p')"
[[ -n "${compose_env_block}" ]] || fail "compose-e2e must prepare its Compose environment"
for required in \
  'USER_R2_ENDPOINT=http://host.docker.internal:9000' \
  'USER_R2_REGION=us-east-1' \
  'USER_R2_ACCESS_KEY_ID=voice-minio' \
  'USER_R2_SECRET_ACCESS_KEY=voice-minio-dev' \
  'USER_R2_BUCKET=voice-dev-avatars' \
  'USER_R2_PUBLIC_BASE_URL=http://127.0.0.1:9000/voice-dev-avatars' \
  'FILE_R2_ENDPOINT=http://host.docker.internal:9000' \
  'FILE_R2_REGION=us-east-1' \
  'FILE_R2_ACCESS_KEY_ID=voice-minio' \
  'FILE_R2_SECRET_ACCESS_KEY=voice-minio-dev' \
  'FILE_R2_BUCKET=voice-dev-files'; do
  printf '%s\n' "${compose_env_block}" | grep -Fxq "          ${required}" || \
    fail "compose-e2e MinIO fixture is missing ${required}"
done

echo "CI script tests are reachable from Actions."
