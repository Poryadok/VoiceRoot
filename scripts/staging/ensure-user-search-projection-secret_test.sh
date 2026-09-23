#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "${TEST_DIR}"' EXIT
mkdir -p "${TEST_DIR}/bin"

cat >"${TEST_DIR}/bin/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "$1" in
  get)
    [ -f "${TEST_STATE}/secret.json" ] || exit 1
    if [[ "$*" == *go-template* ]] && grep -q 'cursor-hmac-key' "${TEST_STATE}/secret.json"; then
      printf 'ok'
    fi
    ;;
  create)
    [ ! -f "${TEST_STATE}/secret.json" ] || exit 1
    cat >"${TEST_STATE}/secret.json"
    ;;
  *) exit 1 ;;
esac
EOF
chmod +x "${TEST_DIR}/bin/kubectl"
export PATH="${TEST_DIR}/bin:${PATH}"
export TEST_STATE="${TEST_DIR}"
export VOICE_K8S_NAMESPACE=voice-staging

if STAGING_USER_SEARCH_CURSOR_HMAC_KEY='' \
  bash "${ROOT}/scripts/staging/ensure-user-search-projection-secret.sh" >"${TEST_DIR}/output" 2>&1; then
  echo 'Expected missing cursor key to fail' >&2
  exit 1
fi
[ ! -f "${TEST_DIR}/secret.json" ]

export STAGING_USER_SEARCH_CURSOR_HMAC_KEY=0123456789abcdef0123456789abcdef
bash "${ROOT}/scripts/staging/ensure-user-search-projection-secret.sh" >"${TEST_DIR}/output" 2>&1
key_b64="$(printf '%s' "${STAGING_USER_SEARCH_CURSOR_HMAC_KEY}" | base64 | tr -d '\r\n')"
grep -Fq "\"cursor-hmac-key\":\"${key_b64}\"" "${TEST_DIR}/secret.json"

export STAGING_USER_SEARCH_CURSOR_HMAC_KEY=rotated0123456789abcdef0123456789abcdef
bash "${ROOT}/scripts/staging/ensure-user-search-projection-secret.sh" >"${TEST_DIR}/output" 2>&1
grep -Fq "\"cursor-hmac-key\":\"${key_b64}\"" "${TEST_DIR}/secret.json"

echo 'ensure-user-search-projection-secret tests passed'
