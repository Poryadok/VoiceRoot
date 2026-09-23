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
    if [[ "$*" == *go-template* ]]; then
      if grep -q 'MINIO_ROOT_USER' "${TEST_STATE}/secret.json" &&
         grep -q 'MINIO_ROOT_PASSWORD' "${TEST_STATE}/secret.json"; then
        printf 'ok'
      fi
    fi
    ;;
  create)
    [ ! -f "${TEST_STATE}/secret.json" ] || exit 1
    cat >"${TEST_STATE}/secret.json"
    echo 'secret/voice-minio-credentials created'
    ;;
  *) echo "unexpected kubectl call: $*" >&2; exit 1 ;;
esac
EOF
chmod +x "${TEST_DIR}/bin/kubectl"

export PATH="${TEST_DIR}/bin:${PATH}"
export TEST_STATE="${TEST_DIR}"
export VOICE_K8S_NAMESPACE=voice-staging

if STAGING_MINIO_ROOT_USER='' STAGING_MINIO_ROOT_PASSWORD='' \
  bash "${ROOT}/scripts/staging/ensure-minio-credentials.sh" >"${TEST_DIR}/output" 2>&1; then
  echo 'Expected missing GitHub secrets to fail' >&2
  exit 1
fi
[ ! -f "${TEST_DIR}/secret.json" ]

export STAGING_MINIO_ROOT_USER=test-user
export STAGING_MINIO_ROOT_PASSWORD=test-password
bash "${ROOT}/scripts/staging/ensure-minio-credentials.sh" >"${TEST_DIR}/output" 2>&1
user_b64="$(printf '%s' "${STAGING_MINIO_ROOT_USER}" | base64 | tr -d '\r\n')"
password_b64="$(printf '%s' "${STAGING_MINIO_ROOT_PASSWORD}" | base64 | tr -d '\r\n')"
grep -Fq "\"MINIO_ROOT_USER\":\"${user_b64}\"" "${TEST_DIR}/secret.json"
grep -Fq "\"MINIO_ROOT_PASSWORD\":\"${password_b64}\"" "${TEST_DIR}/secret.json"
if grep -Fq 'test-password' "${TEST_DIR}/output"; then
  echo 'Credential leaked to stdout' >&2
  exit 1
fi

export STAGING_MINIO_ROOT_USER=rotated-user
export STAGING_MINIO_ROOT_PASSWORD=rotated-password
bash "${ROOT}/scripts/staging/ensure-minio-credentials.sh" >"${TEST_DIR}/output" 2>&1
grep -Fq "\"MINIO_ROOT_USER\":\"${user_b64}\"" "${TEST_DIR}/secret.json"

printf '{"data":{"MINIO_ROOT_USER":"%s"}}' "${user_b64}" >"${TEST_DIR}/secret.json"
if bash "${ROOT}/scripts/staging/ensure-minio-credentials.sh" >"${TEST_DIR}/output" 2>&1; then
  echo 'Expected incomplete existing secret to fail' >&2
  exit 1
fi

echo 'ensure-minio-credentials tests passed'
