#!/usr/bin/env bash
# Tests for e2e-manifest.sh section parsing.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MANIFEST="${ROOT}/.github/ci/e2e-features.yml"
SCRIPT="${ROOT}/scripts/ci/e2e-manifest.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

count_gateway="$(bash "${SCRIPT}" "${MANIFEST}" smoke_gateway | wc -l | tr -d '[:space:]')"
count_flutter="$(bash "${SCRIPT}" "${MANIFEST}" smoke_flutter | wc -l | tr -d '[:space:]')"

[[ "${count_gateway}" -ge 10 ]] || fail "expected >=10 smoke_gateway entries, got ${count_gateway}"
[[ "${count_flutter}" -ge 10 ]] || fail "expected >=10 smoke_flutter entries, got ${count_flutter}"

if bash "${SCRIPT}" "${MANIFEST}" smoke_gateway | grep -q 'TestComposeAuthLifecycle_live'; then
  :
else
  fail "expected TestComposeAuthLifecycle_live in smoke_gateway"
fi

if bash "${SCRIPT}" "${MANIFEST}" missing_section >/dev/null 2>&1; then
  fail "expected failure for missing section"
fi

echo "All e2e-manifest tests passed."
