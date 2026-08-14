#!/usr/bin/env bash
# Verify staging-image-catalog.json deployments exist in referenced k8s manifests
# and that prod manifests include the same deployment names.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CATALOG="${ROOT}/scripts/ci/staging-image-catalog.json"

failures=0

check_manifest_contains_deployment() {
  local manifest_rel="$1"
  local deployment="$2"
  local manifest="${ROOT}/${manifest_rel}"
  if [ ! -f "${manifest}" ]; then
    echo "ERROR: missing manifest ${manifest_rel}" >&2
    failures=$((failures + 1))
    return 1
  fi
  if ! grep -q "name: ${deployment}" "${manifest}"; then
    echo "ERROR: deployment ${deployment} not found in ${manifest_rel}" >&2
    failures=$((failures + 1))
    return 1
  fi
  return 0
}

check_prod_contains_deployment() {
  local deployment="$1"
  local name="$2"
  if grep -rq "name: ${deployment}" "${ROOT}/deploy/prod/"; then
    return 0
  fi
  echo "ERROR: catalog ${name}: deployment ${deployment} missing from deploy/prod/" >&2
  failures=$((failures + 1))
  return 1
}

while IFS=$'\t' read -r name deployment manifest_rel; do
  [ -z "${name}" ] && continue
  [ -z "${deployment}" ] || [ "${deployment}" = "null" ] && continue
  check_manifest_contains_deployment "${manifest_rel}" "${deployment}"
  check_prod_contains_deployment "${deployment}" "${name}"
done < <(jq -r '.images[] | [.name, .deployment, .k8s_manifest] | @tsv' "${CATALOG}")

if [ "${failures}" -gt 0 ]; then
  echo "ERROR: ${failures} image-catalog drift issue(s)" >&2
  exit 1
fi

echo "image-catalog drift check OK (${CATALOG})"
