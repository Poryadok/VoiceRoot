#!/usr/bin/env bash
# Fail if Auth Maven proto copy drifts from canonical protos (wire-equivalent; // comments ignored).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CANON="${ROOT}/protos/voice/auth/v1/auth.proto"
COPY="${ROOT}/src/backend/auth/src/main/proto/voice/auth/v1/auth.proto"

if [[ ! -f "${CANON}" ]]; then
  echo "missing canonical auth proto: ${CANON}" >&2
  exit 1
fi
if [[ ! -f "${COPY}" ]]; then
  echo "missing Auth copy proto: ${COPY}" >&2
  exit 1
fi

normalize() {
  sed 's|//.*||' "$1" | tr -d ' \t\r' | grep -v '^$' || true
}

if diff -q <(normalize "${CANON}") <(normalize "${COPY}") >/dev/null 2>&1; then
  echo "auth proto copy in sync with protos/voice/auth/v1/auth.proto"
  exit 0
fi

echo "Auth proto copy out of sync with protos/voice/auth/v1/auth.proto (wire fields must match)" >&2
diff <(normalize "${CANON}") <(normalize "${COPY}") >&2 || true
exit 1
