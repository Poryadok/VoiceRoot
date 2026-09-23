#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "${TEST_DIR}"' EXIT
mkdir -p "${TEST_DIR}/bin" "${TEST_DIR}/present"

cat >"${TEST_DIR}/bin/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "$1" in
  get) [ -f "${TEST_STATE}/present/$3" ] ;;
  create)
    cat >"${TEST_STATE}/created.json"
    grep -Fq '"kind":"List"' "${TEST_STATE}/created.json"
    for name in social-principal-signing file-principal-signing search-principal-signing principal-ca \
                social-principal-tls user-principal-tls space-principal-tls file-principal-tls \
                user-file-principal-tls search-principal-tls; do
      touch "${TEST_STATE}/present/voice-${name}"
    done
    ;;
  *) echo "unexpected kubectl call: $*" >&2; exit 1 ;;
esac
EOF
chmod +x "${TEST_DIR}/bin/kubectl"
export PATH="${TEST_DIR}/bin:${PATH}"
export TEST_STATE="${TEST_DIR}"
export VOICE_K8S_NAMESPACE=voice-staging
export STAGING_PRINCIPAL_SECRETS_B64="$(printf '%s' '{"kind":"List","items":[]}' | gzip | base64 | tr -d '\r\n')"

bash "${ROOT}/scripts/staging/restore-principal-secrets.sh" >"${TEST_DIR}/output" 2>&1
[ -f "${TEST_DIR}/created.json" ]

rm "${TEST_DIR}/created.json"
unset STAGING_PRINCIPAL_SECRETS_B64
bash "${ROOT}/scripts/staging/restore-principal-secrets.sh" >"${TEST_DIR}/output" 2>&1
[ ! -f "${TEST_DIR}/created.json" ]

rm "${TEST_DIR}/present/voice-search-principal-signing"
if bash "${ROOT}/scripts/staging/restore-principal-secrets.sh" >"${TEST_DIR}/output" 2>&1; then
  echo 'Expected a partial secret set to fail' >&2
  exit 1
fi
grep -q 'partial principal secret set' "${TEST_DIR}/output"

echo 'restore-principal-secrets tests passed'
