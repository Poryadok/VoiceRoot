#!/usr/bin/env bash
# Verify GHCR image manifests exist before staging deploy (stack lock or single tag).
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

if [ -n "${LOCK_FILE}" ] && [ -f "${LOCK_FILE}" ]; then
  REGISTRY="$(grep '^registry:' "${LOCK_FILE}" | awk '{print $2}')"
  TAG="$(grep '^tag:' "${LOCK_FILE}" | awk '{print $2}')"
  echo "Verifying stack lock: ${LOCK_FILE} registry=${REGISTRY} tag=${TAG}"
  while IFS= read -r line; do
    name="$(echo "${line}" | sed 's/:.*//' | xargs)"
    img_tag="$(echo "${line}" | awk -F: '{print $2}' | xargs)"
    [ -z "${name}" ] && continue
    [[ "${name}" == images ]] && continue
    check_image "${name}" "${REGISTRY}/${name}:${img_tag}" || true
  done < <(grep -E '^  [a-z].*:' "${LOCK_FILE}" || true)
elif [ -n "${TAG}" ]; then
  echo "Verifying staging images: ${REGISTRY} tag ${TAG}"
  while IFS= read -r name || [ -n "${name}" ]; do
    name="${name%%#*}"
    name="$(echo "${name}" | tr -d '[:space:]')"
    [ -z "${name}" ] && continue
    check_image "${name}" "${REGISTRY}/${name}:${TAG}" || true
  done < <(jq -r '.images[].name' "${ROOT}/scripts/ci/staging-image-catalog.json")
else
  echo "ERROR: set STACK_LOCK_FILE or VOICE_IMAGE_TAG" >&2
  exit 1
fi

if [ "${#MISSING[@]}" -eq 0 ]; then
  set_output "deploy_tag" "${TAG}"
  echo "All staging images present for tag ${TAG}"
  exit 0
fi

echo "ERROR: missing GHCR images:" >&2
printf '  - %s\n' "${MISSING[@]}" >&2
echo "Re-run CI on master or workflow_dispatch with a green git SHA." >&2
exit 1
