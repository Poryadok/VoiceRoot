#!/usr/bin/env bash
# Verify GHCR image manifests exist before production deploy (stack lock or single tag).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REGISTRY="${VOICE_IMAGE_REGISTRY:?VOICE_IMAGE_REGISTRY required}"
LOCK_FILE="${STACK_LOCK_FILE:-}"
TAG="${VOICE_IMAGE_TAG:-}"

set_output() {
  local key="$1"
  local value="$2"
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "${key}=${value}" >>"${GITHUB_OUTPUT}"
  fi
}

manifest_exists() {
  docker manifest inspect "$1" >/dev/null 2>&1
}

check_image() {
  local name="$1"
  local ref="$2"
  if manifest_exists "${ref}"; then
    echo "ok: ${ref}"
    return 0
  fi
  echo "missing: ${ref}" >&2
  MISSING+=("${name}")
  return 1
}

MISSING=()

# Capture validation directly: process substitution would hide a parser failure
# and could turn an empty plan into a successful zero-image verification.
PLAN_ARGS=(--catalog "${ROOT}/scripts/ci/staging-image-catalog.json" --registry "${REGISTRY}" --tag "${TAG}")
PLAN_ARGS+=(--mode "${DEPLOY_MODE:-full}" --changed-services "${CHANGED_SERVICES:-}")
if [ "${CHANGED_SERVICES_JSON+x}" = x ]; then
  PLAN_ARGS+=(--changed-services-json "${CHANGED_SERVICES_JSON}")
fi
if [ -n "${LOCK_FILE}" ]; then
  PLAN_ARGS+=(--lock "${LOCK_FILE}")
fi
IMAGE_PLAN="$(python3 "${ROOT}/scripts/prod/production_image_refs.py" "${PLAN_ARGS[@]}")"
while IFS=$'\t' read -r name ref; do
  if [ "${name}" = "tag" ]; then
    TAG="${ref}"
    echo "Verifying production images: ${REGISTRY} tag ${TAG}"
  elif [ "${name}" = "scope" ]; then
    CHANGED="${ref}"
  else
    check_image "${name}" "${ref}" || true
  fi
done <<< "${IMAGE_PLAN}"

if [ "${#MISSING[@]}" -eq 0 ]; then
  if [ -n "${PROD_STACK_LOCK_OUT:-}" ]; then
    mkdir -p "$(dirname "${PROD_STACK_LOCK_OUT}")"
    # Serialize the already verified plan; do not reread mutable input files.
    {
      printf 'version: 1\nregistry: %s\ntag: %s\nimages:\n' "${REGISTRY}" "${TAG}"
      while IFS=$'\t' read -r name ref; do
        [ "${name}" = tag ] || [ "${name}" = scope ] || printf '  %s: %s\n' "${name}" "${TAG}"
      done <<< "${IMAGE_PLAN}"
    } > "${PROD_STACK_LOCK_OUT}"
    set_output "stack_lock_file" "${PROD_STACK_LOCK_OUT}"
  fi
  set_output "deploy_tag" "${TAG}"
  set_output "changed_services" "${CHANGED}"
  echo "All production images present for tag ${TAG}"
  exit 0
fi

echo "ERROR: missing GHCR images:" >&2
printf '  - %s\n' "${MISSING[@]}" >&2
echo "Re-run CI on master or workflow_dispatch with a green git SHA." >&2
exit 1
