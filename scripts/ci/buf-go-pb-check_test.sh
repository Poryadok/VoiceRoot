#!/usr/bin/env bash
# Regression fixture for the Go protobuf generated-tree drift check. No Buf plugins or services required.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

make_fixture() {
  local name="$1"
  local fixture_root="${TMP_DIR}/${name}"
  mkdir -p \
    "${fixture_root}/bin" \
    "${fixture_root}/fixtures" \
    "${fixture_root}/scripts/ci" \
    "${fixture_root}/scripts/dev" \
    "${fixture_root}/src/backend/role/pb/voice/example"

  cp "${ROOT}/scripts/ci/buf-go-pb-check.sh" "${fixture_root}/scripts/ci/"
  cp "${ROOT}/scripts/dev/sync-pb-from-gen.sh" "${fixture_root}/scripts/dev/sync-pb-from-gen.real.sh"
  cat >"${fixture_root}/scripts/dev/sync-pb-from-gen.sh" <<'SYNC'
#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bash "${script_dir}/sync-pb-from-gen.real.sh"
case "${PB_CHECK_FIXTURE_MODE:-unchanged}" in
  staged-change) git add src/backend/role/pb/voice/example/existing.pb.go ;;
  staged-deletion) git add -u src/backend/role/pb/voice/example/existing.pb.go ;;
  staged-addition) git add src/backend/role/pb/voice/example/new.pb.go ;;
esac
case "${PB_CHECK_FIXTURE_MODE:-unchanged}" in
  staged-change) git show HEAD:src/backend/role/pb/voice/example/existing.pb.go >src/backend/role/pb/voice/example/existing.pb.go ;;
  staged-deletion) git show HEAD:src/backend/role/pb/voice/example/existing.pb.go >src/backend/role/pb/voice/example/existing.pb.go ;;
  staged-addition) rm src/backend/role/pb/voice/example/new.pb.go ;;
esac
SYNC
  printf '%s\n' 'package example' '// committed generated file' >"${fixture_root}/src/backend/role/pb/voice/example/existing.pb.go"
  printf '%s\n' 'package example' '// committed generated file' >"${fixture_root}/fixtures/existing.pb.go"
  printf '%s\n' 'package example' '// changed generated file' >"${fixture_root}/fixtures/changed.pb.go"
  printf '%s\n' 'package example' '// newly generated file' >"${fixture_root}/fixtures/new.pb.go"

  cat >"${fixture_root}/bin/buf" <<'BUF'
#!/usr/bin/env bash
set -euo pipefail
out="${PWD}/gen/go/voice/example"
mkdir -p "${out}"
case "${PB_CHECK_FIXTURE_MODE:-unchanged}" in
  unchanged)
    cp fixtures/existing.pb.go "${out}/existing.pb.go"
    ;;
  changed)
    cp fixtures/changed.pb.go "${out}/existing.pb.go"
    ;;
  staged-change)
    cp fixtures/changed.pb.go "${out}/existing.pb.go"
    ;;
  deleted)
    ;;
  additive)
    cp fixtures/existing.pb.go "${out}/existing.pb.go"
    cp fixtures/new.pb.go "${out}/new.pb.go"
    ;;
  staged-addition)
    cp fixtures/existing.pb.go "${out}/existing.pb.go"
    cp fixtures/new.pb.go "${out}/new.pb.go"
    ;;
  *)
    echo "unknown fixture mode: ${PB_CHECK_FIXTURE_MODE}" >&2
    exit 2
    ;;
esac
BUF
  chmod +x "${fixture_root}/bin/buf"

  git -C "${fixture_root}" init -q
  git -C "${fixture_root}" config user.name 'Voice CI fixture'
  git -C "${fixture_root}" config user.email 'voice-ci-fixture@example.invalid'
  git -C "${fixture_root}" add .
  git -C "${fixture_root}" commit -qm 'Create generated output baseline'
  printf '%s\n' 'unrelated local work' >"${fixture_root}/unrelated.txt"
  printf '%s\n' "${fixture_root}"
}

run_checker() {
  local fixture_root="$1"
  local mode="$2"
  local output_file="${3:-/dev/null}"
  PATH="${fixture_root}/bin:${PATH}" PB_CHECK_FIXTURE_MODE="${mode}" \
    BUF_GO_PB_CHECK_SKIP_SELF_TEST=1 bash "${fixture_root}/scripts/ci/buf-go-pb-check.sh" >"${output_file}" 2>&1
}

expect_pass() {
  local mode="$1"
  local fixture_root
  fixture_root="$(make_fixture "${mode}")"
  if ! run_checker "${fixture_root}" "${mode}"; then
    fail "expected ${mode} generated output to pass"
  fi
}

expect_drift_failure() {
  local mode="$1"
  local fixture_root output
  fixture_root="$(make_fixture "${mode}")"
  output="${TMP_DIR}/${mode}.log"
  if run_checker "${fixture_root}" "${mode}" "${output}"; then
    echo "unexpectedly passing output for ${mode}:" >&2
    cat "${output}" >&2
    git -C "${fixture_root}" status --short >&2
    fail "expected ${mode} generated output to fail the drift check"
  fi
}

expect_staged_change_failure() {
  local fixture_root
  fixture_root="$(make_fixture staged-change)"
  printf '%s\n' 'staged generated file' >"${fixture_root}/fixtures/changed.pb.go"
  if run_checker "${fixture_root}" staged-change; then
    fail 'expected staged pb changes to fail the drift check'
  fi
}

expect_staged_deletion_failure() {
  local fixture_root
  fixture_root="$(make_fixture staged-deletion)"
  if run_checker "${fixture_root}" deleted; then
    fail 'expected staged pb deletions to fail the drift check'
  fi
}

expect_staged_addition_failure() {
  local fixture_root
  fixture_root="$(make_fixture staged-addition)"
  if run_checker "${fixture_root}" additive; then
    fail 'expected staged pb additions to fail the drift check'
  fi
}

expect_pass unchanged
echo '== unchanged generated outputs pass =='
expect_drift_failure additive
echo '== newly generated untracked outputs fail =='
expect_drift_failure changed
echo '== changed tracked generated outputs fail =='
expect_drift_failure deleted
echo '== deleted tracked generated outputs fail =='
expect_staged_change_failure
echo '== staged tracked changes fail =='
expect_staged_deletion_failure
echo '== staged tracked deletions fail =='
expect_staged_addition_failure
echo '== staged generated additions fail =='
echo 'Go pb drift check regression fixture passed.'
