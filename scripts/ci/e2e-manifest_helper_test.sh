#!/usr/bin/env bash
# Regression test for a missing Docker Go module download helper.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SOURCE_MANIFEST_TEST="${ROOT}/scripts/ci/e2e-manifest_test.sh"
SOURCE_MANIFEST_SCRIPT="${ROOT}/scripts/ci/e2e-manifest.sh"
SOURCE_MANIFEST="${ROOT}/.github/ci/e2e-features.yml"
SOURCE_ATTRIBUTES="${ROOT}/.gitattributes"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

fixture_root="$(mktemp -d)"
trap 'rm -rf "${fixture_root}"' EXIT

mkdir -p "${fixture_root}/scripts/ci" "${fixture_root}/.github/ci" "${fixture_root}/src/backend/scripts"
cp "${SOURCE_MANIFEST_TEST}" "${fixture_root}/scripts/ci/e2e-manifest_test.sh"
cp "${SOURCE_MANIFEST_SCRIPT}" "${fixture_root}/scripts/ci/e2e-manifest.sh"
cp "${SOURCE_MANIFEST}" "${fixture_root}/.github/ci/e2e-features.yml"
cp "${SOURCE_ATTRIBUTES}" "${fixture_root}/.gitattributes"

git -C "${fixture_root}" init -q
git -C "${fixture_root}" add .gitattributes

missing_helper="${fixture_root}/src/backend/scripts/docker-go-mod-download.sh"
set +e
output="$(bash "${fixture_root}/scripts/ci/e2e-manifest_test.sh" 2>&1)"
rc=$?
set -e

[[ "${rc}" -ne 0 ]] || fail "expected missing helper to fail, got exit 0"
grep -Fq "FAIL: Docker Go module download helper is missing: ${missing_helper}" <<<"${output}" \
  || { printf '%s\n' "${output}" >&2; fail "missing helper did not report the explicit fail message"; }

echo "Missing Docker Go module download helper test passed."
