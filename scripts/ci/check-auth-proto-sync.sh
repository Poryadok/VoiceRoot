#!/usr/bin/env bash
# Fail if the Auth Maven proto copy drifts from the canonical proto. Comments and formatting do not affect wire compatibility.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CANON="${ROOT}/protos/voice/auth/v1/auth.proto"
COPY="${ROOT}/src/backend/auth/src/main/proto/voice/auth/v1/auth.proto"
CANON_LABEL="protos/voice/auth/v1/auth.proto"
COPY_LABEL="src/backend/auth/src/main/proto/voice/auth/v1/auth.proto"

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
  echo "auth proto copy in sync with ${CANON_LABEL}"
  exit 0
fi

echo "Auth proto copy out of sync with ${CANON_LABEL}; update ${COPY_LABEL} from the canonical contract." >&2
diff -u --label "${CANON_LABEL}" --label "${COPY_LABEL}" \
  <(normalize "${CANON}") <(normalize "${COPY}") >&2 || true
exit 1
