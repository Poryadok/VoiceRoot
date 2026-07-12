#!/usr/bin/env bash
# Verify GHCR image manifests exist before staging deploy.
# Writes deploy_tag to GITHUB_OUTPUT when GITHUB_OUTPUT is set.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REGISTRY="${VOICE_IMAGE_REGISTRY:?VOICE_IMAGE_REGISTRY required}"
TAG="${VOICE_IMAGE_TAG:?VOICE_IMAGE_TAG required}"
ALLOW_LATEST_FALLBACK="${STAGING_IMAGE_ALLOW_LATEST_FALLBACK:-false}"

if [ -n "${GITHUB_OUTPUT:-}" ]; then
  : >"${GITHUB_OUTPUT}.verify" 2>/dev/null || true
fi

set_output() {
  local key="$1"
  local value="$2"
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "${key}=${value}" >>"${GITHUB_OUTPUT}"
  fi
}

manifest_exists() {
  local ref="$1"
  docker manifest inspect "${ref}" >/dev/null 2>&1
}

check_image() {
  local name="$1"
  local ref="${REGISTRY}/${name}:${TAG}"
  if manifest_exists "${ref}"; then
    echo "ok: ${ref}"
    return 0
  fi
  echo "missing: ${ref}" >&2
  MISSING+=("${name}")
  return 1
}

MISSING=()
FRONTEND_IMAGES=(auth web developer-portal)
if [ -f "${ROOT}/deploy/staging/admin.yaml" ]; then
  FRONTEND_IMAGES+=(admin)
fi

echo "Verifying staging images: ${REGISTRY} tag ${TAG}"

while IFS= read -r svc || [ -n "${svc}" ]; do
  svc="${svc%%#*}"
  svc="$(echo "${svc}" | tr -d '[:space:]')"
  [ -z "${svc}" ] && continue
  check_image "${svc}" || true
done <"${ROOT}/scripts/ci/staging-go-services.txt"

for img in "${FRONTEND_IMAGES[@]}"; do
  check_image "${img}" || true
done

if [ "${#MISSING[@]}" -eq 0 ]; then
  set_output "deploy_tag" "${TAG}"
  echo "All staging images present for tag ${TAG}"
  exit 0
fi

if [ "${ALLOW_LATEST_FALLBACK}" = "true" ] && [ "${TAG}" != "latest" ]; then
  echo "Trying :latest fallback for missing images (workflow_dispatch only)..." >&2
  STILL_MISSING=()
  for name in "${MISSING[@]}"; do
    if manifest_exists "${REGISTRY}/${name}:latest"; then
      echo "fallback ok: ${REGISTRY}/${name}:latest" >&2
    else
      STILL_MISSING+=("${name}")
    fi
  done
  if [ "${#STILL_MISSING[@]}" -eq 0 ]; then
    set_output "deploy_tag" "latest"
    echo "Using :latest for deploy (all missing images have :latest)" >&2
    exit 0
  fi
  MISSING=("${STILL_MISSING[@]}")
fi

echo "ERROR: missing GHCR images for tag ${TAG}:" >&2
printf '  - %s\n' "${MISSING[@]}" >&2
if [ "${ALLOW_LATEST_FALLBACK}" != "true" ]; then
  echo "Auto-deploy requires exact CI SHA tags. Re-run CI on master or use workflow_dispatch with a green SHA." >&2
else
  echo "For manual deploy use git SHA from a green CI run on master." >&2
fi
exit 1
