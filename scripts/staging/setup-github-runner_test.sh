#!/usr/bin/env bash
# Offline contract test for the staging GitHub runner bootstrapper.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SCRIPT="${ROOT}/scripts/staging/setup-github-runner.sh"
TEST_TMP="$(mktemp -d "${TMPDIR:-/tmp}/setup-github-runner-tests.XXXXXXXX")"
trap 'rm -rf "${TEST_TMP}"' EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

make_fake_tools() {
  local case_dir="$1"
  mkdir -p "${case_dir}/bin" "${case_dir}/runner"

  cat >"${case_dir}/bin/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "curl $*" >>"${FAKE_LOG}"
touch "${PKG_NAME}"
EOF
  cat >"${case_dir}/bin/tar" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "tar $*" >>"${FAKE_LOG}"
cat > config.sh <<'CONFIG'
#!/usr/bin/env bash
set -euo pipefail
echo "config $*" >>"${FAKE_LOG}"
CONFIG
chmod +x config.sh
EOF
  cat >"${case_dir}/bin/jq" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "${case_dir}/bin/curl" "${case_dir}/bin/tar" "${case_dir}/bin/jq"
}

run_runner() {
  local case_dir="$1"
  local version="$2"
  local pkg="actions-runner-linux-x64-${version}.tar.gz"

  FAKE_LOG="${case_dir}/calls.log" \
  PKG_NAME="${pkg}" \
  PATH="${case_dir}/bin:${PATH}" \
  RUNNER_TOKEN=test-token \
  RUNNER_VERSION="${version}" \
  RUNNER_DIR="${case_dir}/runner" \
  RUNNER_NAME=test-runner \
  GITHUB_REPO=Poryadok/VoiceRoot \
  bash "${SCRIPT}"
}

unsupported_case="${TEST_TMP}/unsupported"
make_fake_tools "${unsupported_case}"
if run_runner "${unsupported_case}" '2.327.0'; then
  fail 'runner version below 2.327.1 must be rejected'
fi
[[ ! -e "${unsupported_case}/calls.log" ]] || \
  fail 'unsupported runner version must fail before download or runner setup'
[[ ! -e "${unsupported_case}/runner/config.sh" ]] || \
  fail 'unsupported runner version must not create runner setup files'

for version in '2.327.1' '2.400.0'; do
  case_dir="${TEST_TMP}/${version}"
  make_fake_tools "${case_dir}"
  run_runner "${case_dir}" "${version}"
  grep -Fqx "curl -fsSLO https://github.com/actions/runner/releases/download/v${version}/actions-runner-linux-x64-${version}.tar.gz" "${case_dir}/calls.log" || \
    fail "supported runner version ${version} must reach download"
  grep -Fq 'config --url https://github.com/Poryadok/VoiceRoot' "${case_dir}/calls.log" || \
    fail "supported runner version ${version} must reach runner configuration"
done

echo 'All setup-github-runner tests passed.'
