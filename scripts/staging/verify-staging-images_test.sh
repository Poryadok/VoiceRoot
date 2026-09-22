#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -f "${TEST_DIR}/docker" "${TEST_DIR}/sleep" "${TEST_DIR}/lock.yml" "${TEST_DIR}/calls" "${TEST_DIR}/output"; rmdir "${TEST_DIR}"' EXIT

cat >"${TEST_DIR}/lock.yml" <<'EOF'
registry: ghcr.io/test
tag: test
images:
  developer-portal: test
EOF

cat >"${TEST_DIR}/docker" <<'EOF'
#!/usr/bin/env bash
count=0
[ ! -f "${TEST_CALLS}" ] || count="$(cat "${TEST_CALLS}")"
count=$((count + 1))
echo "${count}" >"${TEST_CALLS}"
[ "${count}" -gt "${TEST_FAIL_COUNT}" ]
EOF
cat >"${TEST_DIR}/sleep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "${TEST_DIR}/docker" "${TEST_DIR}/sleep"

export PATH="${TEST_DIR}:${PATH}"
export TEST_CALLS="${TEST_DIR}/calls"
export VOICE_IMAGE_REGISTRY=ghcr.io/test
export STACK_LOCK_FILE="${TEST_DIR}/lock.yml"

export TEST_FAIL_COUNT=2
bash "${ROOT}/scripts/staging/verify-staging-images.sh" >"${TEST_DIR}/output" 2>&1
[ "$(cat "${TEST_CALLS}")" -eq 3 ]
grep -q 'All staging images present' "${TEST_DIR}/output"

rm "${TEST_CALLS}"
export TEST_FAIL_COUNT=3
if bash "${ROOT}/scripts/staging/verify-staging-images.sh" >"${TEST_DIR}/output" 2>&1; then
  echo 'Expected a persistent lookup failure' >&2
  exit 1
fi
[ "$(cat "${TEST_CALLS}")" -eq 3 ]
grep -q 'missing or unavailable after 3 attempts' "${TEST_DIR}/output"

echo 'verify-staging-images retry tests passed'
