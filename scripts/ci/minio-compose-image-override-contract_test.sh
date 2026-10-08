#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORKFLOW="$ROOT/.github/workflows/ci.yml"
COMPOSE="$ROOT/docker-compose.yml"
STAGING="$ROOT/scripts/staging/apply-infra.sh"
FILTERS="$ROOT/.github/ci/path-filters.yml"
SERVER_REF='ghcr.io/poryadok/voiceroot/minio:86b2017f06d0d471e8b43abc78031e86756defe3@sha256:ab7687bc47a84c3aec0d9706dabd47b4719b081683cde745f8a1b84c6c7681e0'
MC_REF='ghcr.io/poryadok/voiceroot/minio-mc:86b2017f06d0d471e8b43abc78031e86756defe3@sha256:66a55c322fed37a3fefa0b815d195b01e7903d1bbffccd80d5a8af3cedf343f2'
VERIFY='bash scripts/ci/verify-compose-minio-images.sh'
VERIFY_SCRIPT="$ROOT/scripts/ci/verify-compose-minio-images.sh"
fail() { echo "FAIL: $*" >&2; exit 1; }

compose_invocation_is_valid() {
  local block="$1" target="$2" login_line verify_line compose_line
  grep -Fq "run: $target" <<<"$block" || return 1
  login_line="$(grep -nF 'uses: docker/login-action@v4' <<<"$block" | head -n1 | cut -d: -f1)"
  verify_line="$(grep -nF "run: $VERIFY" <<<"$block" | head -n1 | cut -d: -f1)"
  compose_line="$(grep -nE 'run: (COMPOSE_PARALLEL_LIMIT=[0-9]+ )?(docker compose|make compose-)' <<<"$block" | head -n1 | cut -d: -f1)"
  [[ -n "$compose_line" ]] || return 1
  (( login_line < verify_line && verify_line < compose_line ))
}

job_block() {
  local name="$1"
  awk -v name="$name" '
    $0 == "  " name ":" { in_job = 1; next }
    in_job && /^  [[:alnum:]_-]+:$/ { exit }
    in_job { print }
  ' "$WORKFLOW"
}

assert_job() {
  local name="$1" target="$2" filter="$3" block
  block="$(job_block "$name")"
  [[ -n "$block" ]] || fail "workflow job missing: $name"
  grep -Fq "VOICE_MINIO_IMAGE: $SERVER_REF" <<<"$block" || fail "$name lacks the exact immutable MinIO server ref"
  grep -Fq "VOICE_MINIO_MC_IMAGE: $MC_REF" <<<"$block" || fail "$name lacks the exact immutable MinIO mc ref"
  grep -Fq 'permissions:' <<<"$block" || fail "$name must declare least-privilege registry permissions"
  grep -Fq 'contents: read' <<<"$block" || fail "$name must retain source read permission"
  grep -Fq 'packages: read' <<<"$block" || fail "$name must grant only registry pull permission"
  grep -Fq 'uses: docker/login-action@v4' <<<"$block" || fail "$name must authenticate to GHCR before pulling the private mirror"
  grep -Fq 'registry: ghcr.io' <<<"$block" || fail "$name GHCR login must target ghcr.io"
  grep -Fq 'username: ${{ github.actor }}' <<<"$block" || fail "$name GHCR login must use the workflow actor"
  grep -Fq 'password: ${{ secrets.GITHUB_TOKEN }}' <<<"$block" || fail "$name GHCR login must use the existing workflow token"
  grep -Fq "run: $VERIFY" <<<"$block" || fail "$name must validate effective Compose image refs before running"
  compose_invocation_is_valid "$block" "$target" \
    || fail "$name must invoke its exact Compose target after login and image verification"
  grep -Fq "github.event_name == 'pull_request'" <<<"$block" || fail "$name must run on the affected pull request"
  grep -Fq "$filter" <<<"$block" || fail "$name pull request gate must follow its scoped path filter"
}

assert_job compose-e2e 'COMPOSE_PARALLEL_LIMIT=4 docker compose -f docker-compose.yml -f docker-compose.voice-space-media.yml --profile app up -d --build' "needs.changes.outputs.global == 'true'"
assert_job a1-e2e 'make compose-a1-multi-account-proof' "needs.changes.outputs.global == 'true'"
assert_job a1-flutter-profile-handoff 'make compose-a1-flutter-profile-handoff' "needs.changes.outputs.global == 'true'"
expected_compose='COMPOSE_PARALLEL_LIMIT=4 docker compose -f docker-compose.yml -f docker-compose.voice-space-media.yml --profile app up -d --build'
valid_compose_fixture="$(printf '%s\n' 'uses: docker/login-action@v4' "run: $VERIFY" "run: $expected_compose")"
compose_invocation_is_valid "$valid_compose_fixture" "$expected_compose" \
  || fail 'Compose contract rejected the expected verified app invocation'
wrong_app_fixture="$(printf '%s\n' 'uses: docker/login-action@v4' "run: $VERIFY" 'run: make compose-app-up')"
if compose_invocation_is_valid "$wrong_app_fixture" "$expected_compose"; then
  fail 'Compose contract accepted the obsolete app target'
fi
wrong_overlay_fixture="$(printf '%s\n' 'uses: docker/login-action@v4' "run: $VERIFY" 'run: COMPOSE_PARALLEL_LIMIT=4 docker compose -f docker-compose.yml --profile app up -d --build')"
if compose_invocation_is_valid "$wrong_overlay_fixture" "$expected_compose"; then
  fail 'Compose contract accepted an app invocation without the voice media overlay'
fi
wrong_order_fixture="$(printf '%s\n' 'uses: docker/login-action@v4' "run: $expected_compose" "run: $VERIFY")"
if compose_invocation_is_valid "$wrong_order_fixture" "$expected_compose"; then
  fail 'Compose contract accepted image verification after Compose started'
fi
grep -Fq '.github/workflows/**' "$FILTERS" \
  || fail 'global path filter must exercise the Compose E2E PR gate for workflow changes'
grep -Fq '.github/workflows/ci.yml' "$FILTERS" \
  || fail 'global path filter must exercise both A1 PR gates for CI workflow changes'

grep -Fq 'quay.io/minio/minio:RELEASE.2024-12-18T13-15-44Z@sha256:1dce27c494a16bae114774f1cec295493f3613142713130c2d22dd5696be6ad3' "$COMPOSE" \
  || fail 'ordinary local Compose server default must remain unchanged'
grep -Fq 'quay.io/minio/mc:RELEASE.2025-08-13T08-35-41Z@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727' "$COMPOSE" \
  || fail 'ordinary local Compose mc default must remain unchanged'
grep -Fq "MINIO_IMAGE=\"\${VOICE_MINIO_IMAGE:-$SERVER_REF}\"" "$STAGING" \
  || fail 'staging MinIO pin must remain unchanged'
grep -Fq "MINIO_MC_IMAGE=\"\${VOICE_MINIO_MC_IMAGE:-$MC_REF}\"" "$STAGING" \
  || fail 'staging mc pin must remain unchanged'

test_bin="$(mktemp -d)"
trap 'rm -rf -- "$test_bin"' EXIT
cat >"$test_bin/docker" <<'DOCKER_STUB'
#!/usr/bin/env bash
set -euo pipefail
[[ "$*" == 'compose --profile app config --format json' ]] || exit 90
[[ "${DOCKER_CONFIG_FAIL:-false}" != true ]] || exit 1
printf '%s' "${DOCKER_CONFIG_JSON:-}"
DOCKER_STUB
chmod +x "$test_bin/docker"
correct_config="$(printf '{"services":{"minio":{"image":"%s"},"minio-init":{"image":"%s"}}}' "$SERVER_REF" "$MC_REF")"
output="$(PATH="$test_bin:$PATH" VOICE_MINIO_IMAGE="$SERVER_REF" VOICE_MINIO_MC_IMAGE="$MC_REF" \
  DOCKER_CONFIG_JSON="$correct_config" bash "$VERIFY_SCRIPT")" \
  || fail 'effective image verifier rejected the correctly pinned Compose config'
[[ "$output" == 'COMPOSE_MINIO_IMAGES=PASS' ]] || fail 'image verifier must emit only the pass classification'
wrong_config="$(printf '{"services":{"minio":{"image":"other"},"minio-init":{"image":"%s"}}}' "$MC_REF")"
if PATH="$test_bin:$PATH" VOICE_MINIO_IMAGE="$SERVER_REF" VOICE_MINIO_MC_IMAGE="$MC_REF" \
  DOCKER_CONFIG_JSON="$wrong_config" bash "$VERIFY_SCRIPT" >"$test_bin/out"; then
  fail 'effective image verifier accepted an unexpected server image'
fi
grep -Fq 'COMPOSE_MINIO_IMAGES=FAIL_EFFECTIVE_IMAGE' "$test_bin/out" \
  || fail 'image mismatch must fail with a sanitized classification'
if PATH="$test_bin:$PATH" VOICE_MINIO_IMAGE="$SERVER_REF" VOICE_MINIO_MC_IMAGE="$MC_REF" \
  DOCKER_CONFIG_FAIL=true bash "$VERIFY_SCRIPT" >"$test_bin/out"; then
  fail 'effective image verifier accepted a failed Compose config command'
fi
grep -Fq 'COMPOSE_MINIO_IMAGES=FAIL_CONFIG' "$test_bin/out" \
  || fail 'Compose config failure must be sanitized'

echo 'CI MinIO Compose image override contract passed'
