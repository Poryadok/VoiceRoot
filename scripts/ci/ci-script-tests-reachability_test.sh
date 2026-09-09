#!/usr/bin/env bash
# Regression guard: the local CI-script suite must be reachable from Actions.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORKFLOW="${ROOT}/.github/workflows/ci.yml"
PATH_FILTERS="${ROOT}/.github/ci/path-filters.yml"
REQUIRED_JOBS="${ROOT}/.github/ci/verify-required-jobs.sh"
GO_DOWNLOAD_HELPER="src/backend/scripts/docker-go-mod-download.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

job_block="$(sed -n '/^  ci-script-tests:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
[[ -n "${job_block}" ]] || fail "CI workflow must define a ci-script-tests job"

global_paths="$(sed -n '/^global:$/,/^[[:alnum:]_]*:$/p' "${PATH_FILTERS}")"
echo "${global_paths}" | grep -Fxq "  - ${GO_DOWNLOAD_HELPER}" \
  || fail "Docker Go module download helper must be a global CI-policy path"

echo "${job_block}" | grep -Eq '^    needs: changes$' \
  || fail "ci-script-tests must depend on changes"
echo "${job_block}" | grep -Fq "needs.changes.outputs.global == 'true'" \
  || fail "ci-script-tests must run for existing global CI-policy paths"
echo "${job_block}" | grep -Eq '^      - name: CI script regression tests$' \
  || fail "ci-script-tests must name its regression-test step"
echo "${job_block}" | grep -Eq '^        run: make ci-script-tests$' \
  || fail "ci-script-tests must invoke make ci-script-tests"

gate_block="$(sed -n '/^  ci-gate:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
echo "${gate_block}" | grep -Eq '^      - ci-script-tests$' \
  || fail "ci-gate must require ci-script-tests"
grep -Fq 'check_if "${GLOBAL}" ci-script-tests' "${REQUIRED_JOBS}" \
  || fail "ci-gate must require ci-script-tests for global CI-policy paths"

compose_e2e_block="$(sed -n '/^  compose-e2e:$/,/^  [[:alnum:]_-]*:$/p' "${WORKFLOW}")"
[[ -n "${compose_e2e_block}" ]] || fail "CI workflow must define a compose-e2e job"
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
