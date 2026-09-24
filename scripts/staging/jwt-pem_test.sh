#!/usr/bin/env bash
# Tests for scripts/staging/lib/jwt-pem.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/lib/jwt-pem.sh
source "${ROOT}/scripts/staging/lib/jwt-pem.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

JWT_FILE="${ROOT}/src/backend/auth/src/test/resources/jwt-test-private.pem"
if [ ! -f "${JWT_FILE}" ]; then
  fail "missing test JWT at ${JWT_FILE}"
fi
valid_pem="$(cat "${JWT_FILE}")"

jwt_pem_is_valid "${valid_pem}" || fail "repo test JWT should be valid"

# grep -q must not close a pipe while a valid, longer PEM is still being
# written: under pipefail that reports SIGPIPE as an invalid private key.
printf -v padding '%*s' 131072 ''
jwt_pem_is_valid "${valid_pem}${padding}" || fail "valid PEM with trailing whitespace should be valid under pipefail"

placeholder_pem="$(cat <<'EOF'
-----BEGIN PRIVATE KEY-----
REPLACE_WITH_PKCS8_PEM
-----END PRIVATE KEY-----
EOF
)"
if jwt_pem_is_valid "${placeholder_pem}"; then
  fail "placeholder PEM should be invalid"
fi

if jwt_pem_is_valid ""; then
  fail "empty PEM should be invalid"
fi

echo "OK: jwt-pem validation tests passed"
