#!/usr/bin/env bash
# Verify GHCR image manifests exist before staging deploy (stack lock or single tag).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REGISTRY="${VOICE_IMAGE_REGISTRY:?VOICE_IMAGE_REGISTRY required}"
EXPECTED_REGISTRY="${REGISTRY}"
LOCK_FILE="${STACK_LOCK_FILE:-}"
TAG="${VOICE_IMAGE_TAG:-}"
CATALOG="${ROOT}/scripts/ci/staging-image-catalog.json"

if [ ! -f "${CATALOG}" ]; then
  echo "ERROR: staging image catalog is missing: ${CATALOG}" >&2
  exit 1
fi
catalog_names="$(jq -r '.images[].name' "${CATALOG}")" || {
  echo 'ERROR: staging image catalog could not be read' >&2
  exit 1
}
if [ -z "${catalog_names}" ]; then
  echo 'ERROR: staging image catalog is empty' >&2
  exit 1
fi
CATALOG_NAMES=()
while IFS= read -r name; do
  [[ "${name}" =~ ^[a-z0-9-]+$ ]] || {
    echo 'ERROR: staging image catalog contains an invalid image name' >&2
    exit 1
  }
  CATALOG_NAMES+=("${name}")
done <<<"${catalog_names}"

set_output() {
  local key="$1"
  local value="$2"
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "${key}=${value}" >>"${GITHUB_OUTPUT}"
  fi
}

manifest_exists() {
  local ref="$1"
  local attempt
  for attempt in 1 2 3; do
    if docker manifest inspect "${ref}" >/dev/null 2>&1; then
      return 0
    fi
    if [ "${attempt}" -lt 3 ]; then
      echo "Retrying manifest lookup for ${ref} (${attempt}/3)" >&2
      sleep 5
    fi
  done
  return 1
}

check_image() {
  local name="$1"
  local ref="$2"
  if manifest_exists "${ref}"; then
    echo "ok: ${ref}"
    return 0
  fi
  echo "missing or unavailable after 3 attempts: ${ref}" >&2
  MISSING+=("${name}")
  return 1
}

MISSING=()

if [ -n "${LOCK_FILE}" ]; then
  if [ ! -f "${LOCK_FILE}" ]; then
    echo "ERROR: requested stack lock is missing: ${LOCK_FILE}" >&2
    exit 1
  fi
  REGISTRY="$(sed -n 's/^registry: //p' "${LOCK_FILE}")"
  lock_tag="$(sed -n 's/^tag: //p' "${LOCK_FILE}")"
  if [[ "${REGISTRY}" != "${EXPECTED_REGISTRY}" || "${lock_tag}" != "${TAG}" || ! "${lock_tag}" =~ ^[[:xdigit:]]{40}$ ]]; then
    echo 'ERROR: requested stack lock has invalid registry or tag' >&2
    exit 1
  fi
  expected_names="$(printf '%s\n' "${CATALOG_NAMES[@]}" nats-hub-config-renderer | sort)"
  actual_names="$(sed -n 's/^  \([a-z0-9-][a-z0-9-]*\): .*/\1/p' "${LOCK_FILE}" | sort)"
  if [[ "${actual_names}" != "${expected_names}" ]]; then
    echo 'ERROR: requested stack lock does not contain the complete staging image catalog' >&2
    exit 1
  fi
  TAG="${lock_tag}"
  echo "Verifying stack lock: ${LOCK_FILE} registry=${REGISTRY} tag=${TAG}"
  while IFS= read -r line; do
    [[ "${line}" =~ ^\ \ ([a-z0-9-][a-z0-9-]*):\ ([[:xdigit:]]{40})$ ]] || {
      echo 'ERROR: requested stack lock contains an invalid image row' >&2
      exit 1
    }
    name="${BASH_REMATCH[1]}"
    img_tag="${BASH_REMATCH[2]}"
    if [[ "${img_tag}" != "${lock_tag}" ]]; then
      echo 'ERROR: requested stack lock contains an image tag that differs from its source tag' >&2
      exit 1
    fi
    check_image "${name}" "${REGISTRY}/${name}:${img_tag}" || true
  done < <(sed -n '/^images:/,$p' "${LOCK_FILE}" | tail -n +2)
elif [ -n "${TAG}" ]; then
  if [[ ! "${TAG}" =~ ^[[:xdigit:]]{40}$ ]]; then
    echo 'ERROR: requested staging image tag must be a full Git SHA' >&2
    exit 1
  fi
  echo "Verifying staging images: ${REGISTRY} tag ${TAG}"
  for name in "${CATALOG_NAMES[@]}"; do
    check_image "${name}" "${REGISTRY}/${name}:${TAG}" || true
  done
  check_image "nats-hub-config-renderer" "${REGISTRY}/nats-hub-config-renderer:${TAG}" || true
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
