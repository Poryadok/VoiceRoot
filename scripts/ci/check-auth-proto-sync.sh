#!/usr/bin/env bash
# Fail if either Auth Maven proto copy drifts. Comments/formatting do not affect wire compatibility.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
normalize() {
  sed 's|//.*||' "$1" | tr -d ' \t\r' | grep -v '^$' || true
}

check_proto() {
  local name="$1"
  local CANON_LABEL="protos/voice/auth/v1/${name}"
  local COPY_LABEL="src/backend/auth/src/main/proto/voice/auth/v1/${name}"
  local CANON="${ROOT}/${CANON_LABEL}"
  local COPY="${ROOT}/${COPY_LABEL}"
  if [[ ! -f "${CANON}" || ! -f "${COPY}" ]]; then
    echo "missing canonical or Maven Auth proto: ${name}" >&2
    return 1
  fi

  if diff -q <(normalize "${CANON}") <(normalize "${COPY}") >/dev/null 2>&1; then
    echo "auth proto copy in sync with ${CANON_LABEL}"
    return 0
  fi
  echo "Auth proto copy out of sync with ${CANON_LABEL}; update ${COPY_LABEL} from the canonical contract." >&2
  diff -u --label "${CANON_LABEL}" --label "${COPY_LABEL}" \
    <(normalize "${CANON}") <(normalize "${COPY}") >&2 || true
  return 1
}

check_proto auth.proto
check_proto subscription_lifecycle.proto
