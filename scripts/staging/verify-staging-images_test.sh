#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -f "${TEST_DIR}/docker" "${TEST_DIR}/jq" "${TEST_DIR}/sleep" "${TEST_DIR}/lock.yml" "${TEST_DIR}/valid-lock.yml" "${TEST_DIR}/tag-mismatch.yml" "${TEST_DIR}/row-mismatch.yml" "${TEST_DIR}/calls" "${TEST_DIR}/refs" "${TEST_DIR}/refs.sorted" "${TEST_DIR}/output"; rm -rf -- "${TEST_DIR}/repo"; rmdir "${TEST_DIR}"' EXIT

cat >"${TEST_DIR}/docker" <<'EOF'
#!/usr/bin/env bash
count=0
[ ! -f "${TEST_CALLS}" ] || count="$(cat "${TEST_CALLS}")"
count=$((count + 1))
echo "${count}" >"${TEST_CALLS}"
printf '%s\n' "$3" >>"${TEST_REFS}"
if [[ -n "${TEST_FAIL_REF:-}" && "$3" == "${TEST_FAIL_REF}" ]]; then exit 1; fi
[ "${count}" -gt "${TEST_FAIL_COUNT}" ]
EOF
cat >"${TEST_DIR}/sleep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat >"${TEST_DIR}/jq" <<'EOF'
#!/usr/bin/env bash
[[ "${TEST_JQ_FAIL:-}" != true ]] || exit 2
[[ "$1" == -r && "$2" == '.images[].name' && -f "$3" ]] || exit 2
sed -n 's/^[[:space:]]*"name":[[:space:]]*"\([a-z0-9-][a-z0-9-]*\)".*/\1/p' "$3"
EOF
chmod +x "${TEST_DIR}/docker" "${TEST_DIR}/jq" "${TEST_DIR}/sleep"

export PATH="${TEST_DIR}:${PATH}"
export TEST_CALLS="${TEST_DIR}/calls"
export TEST_REFS="${TEST_DIR}/refs"
export VOICE_IMAGE_REGISTRY=ghcr.io/test
export VOICE_IMAGE_TAG=0123456789abcdef0123456789abcdef01234567
export STACK_LOCK_FILE="${TEST_DIR}/lock.yml"

{
  printf 'version: 1\nregistry: %s\ntag: %s\nimages:\n' "${VOICE_IMAGE_REGISTRY}" "${VOICE_IMAGE_TAG}"
  jq -r '.images[].name' "${ROOT}/scripts/ci/staging-image-catalog.json" |
    while IFS= read -r name; do printf '  %s: %s\n' "${name}" "${VOICE_IMAGE_TAG}"; done
  printf '  nats-hub-config-renderer: %s\n' "${VOICE_IMAGE_TAG}"
} >"${STACK_LOCK_FILE}"
cp "${STACK_LOCK_FILE}" "${TEST_DIR}/valid-lock.yml"
expected_refs="$(jq -r '.images[].name' "${ROOT}/scripts/ci/staging-image-catalog.json" |
  sed "s|^|${VOICE_IMAGE_REGISTRY}/|; s|$|:${VOICE_IMAGE_TAG}|")"
expected_refs="$(printf '%s\n%s/%s:%s' "${expected_refs}" "${VOICE_IMAGE_REGISTRY}" nats-hub-config-renderer "${VOICE_IMAGE_TAG}" | sort)"
expected_count="$(printf '%s\n' "${expected_refs}" | wc -l | tr -d '[:space:]')"

export TEST_FAIL_COUNT=2
bash "${ROOT}/scripts/staging/verify-staging-images.sh" >"${TEST_DIR}/output" 2>&1
[ "$(cat "${TEST_CALLS}")" -eq "$((expected_count + 2))" ]
grep -q 'All staging images present' "${TEST_DIR}/output"
sort -u "${TEST_REFS}" >"${TEST_DIR}/refs.sorted"
diff -u <(printf '%s\n' "${expected_refs}") "${TEST_DIR}/refs.sorted"

rm -f "${TEST_CALLS}" "${TEST_REFS}"
unset STACK_LOCK_FILE
export TEST_FAIL_COUNT=0
bash "${ROOT}/scripts/staging/verify-staging-images.sh" >"${TEST_DIR}/output" 2>&1
[ "$(cat "${TEST_CALLS}")" -eq "${expected_count}" ]
grep -q 'Verifying staging images:' "${TEST_DIR}/output"
sort -u "${TEST_REFS}" >"${TEST_DIR}/refs.sorted"
diff -u <(printf '%s\n' "${expected_refs}") "${TEST_DIR}/refs.sorted"

rm -f "${TEST_CALLS}" "${TEST_REFS}"
export TEST_JQ_FAIL=true
if bash "${ROOT}/scripts/staging/verify-staging-images.sh" >"${TEST_DIR}/output" 2>&1; then
  echo 'Expected an unreadable staging catalog to fail closed' >&2
  exit 1
fi
[ ! -f "${TEST_CALLS}" ] || [ "$(cat "${TEST_CALLS}")" -eq 0 ]
grep -q 'staging image catalog could not be read' "${TEST_DIR}/output"
unset TEST_JQ_FAIL

mkdir -p "${TEST_DIR}/repo/scripts/staging"
cp "${ROOT}/scripts/staging/verify-staging-images.sh" "${TEST_DIR}/repo/scripts/staging/verify-staging-images.sh"
if bash "${TEST_DIR}/repo/scripts/staging/verify-staging-images.sh" >"${TEST_DIR}/output" 2>&1; then
  echo 'Expected a missing staging catalog to fail closed' >&2
  exit 1
fi
grep -q 'staging image catalog is missing' "${TEST_DIR}/output"

export STACK_LOCK_FILE="${TEST_DIR}/valid-lock.yml"
rm -f "${TEST_CALLS}"
sed "s/^tag: .*/tag: fedcba9876543210fedcba9876543210fedcba98/" \
  "${TEST_DIR}/valid-lock.yml" >"${TEST_DIR}/tag-mismatch.yml"
export STACK_LOCK_FILE="${TEST_DIR}/tag-mismatch.yml"
if bash "${ROOT}/scripts/staging/verify-staging-images.sh" >"${TEST_DIR}/output" 2>&1; then
  echo 'Expected a lock from a different source tag to fail closed' >&2
  exit 1
fi
[ ! -f "${TEST_CALLS}" ] || [ "$(cat "${TEST_CALLS}")" -eq 0 ]
grep -q 'invalid registry or tag' "${TEST_DIR}/output"

awk -v tag="${VOICE_IMAGE_TAG}" 'BEGIN { changed = 0 } /^  [a-z0-9-]+: / && !changed { sub(/[[:xdigit:]]{40}$/, "fedcba9876543210fedcba9876543210fedcba98"); changed = 1 } { print }' \
  "${TEST_DIR}/valid-lock.yml" >"${TEST_DIR}/row-mismatch.yml"
export STACK_LOCK_FILE="${TEST_DIR}/row-mismatch.yml"
if bash "${ROOT}/scripts/staging/verify-staging-images.sh" >"${TEST_DIR}/output" 2>&1; then
  echo 'Expected a mixed-tag stack lock to fail closed' >&2
  exit 1
fi
[ ! -f "${TEST_CALLS}" ] || [ "$(cat "${TEST_CALLS}")" -eq 0 ]
grep -q 'image tag that differs from its source tag' "${TEST_DIR}/output"

rm -f "${TEST_CALLS}" "${TEST_REFS}"
export STACK_LOCK_FILE="${TEST_DIR}/valid-lock.yml"
export TEST_FAIL_COUNT=0
export TEST_FAIL_REF="${VOICE_IMAGE_REGISTRY}/nats-hub-config-renderer:${VOICE_IMAGE_TAG}"
if bash "${ROOT}/scripts/staging/verify-staging-images.sh" >"${TEST_DIR}/output" 2>&1; then
  echo 'Expected a persistent lookup failure' >&2
  exit 1
fi
[ "$(wc -l <"${TEST_REFS}" | tr -d '[:space:]')" -eq "$((expected_count + 2))" ]
grep -q 'missing or unavailable after 3 attempts' "${TEST_DIR}/output"
grep -Fq "${TEST_FAIL_REF}" "${TEST_DIR}/output"

rm "${TEST_CALLS}"
rm "${TEST_REFS}"
export STACK_LOCK_FILE="${TEST_DIR}/missing-lock.yml"
if bash "${ROOT}/scripts/staging/verify-staging-images.sh" >"${TEST_DIR}/output" 2>&1; then
  echo 'Expected a missing requested lock to fail closed' >&2
  exit 1
fi
[ ! -f "${TEST_CALLS}" ] || [ "$(cat "${TEST_CALLS}")" -eq 0 ]
grep -q 'requested stack lock is missing' "${TEST_DIR}/output"

echo 'verify-staging-images retry tests passed'
