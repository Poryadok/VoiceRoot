#!/usr/bin/env bash
# Promote unchanged GHCR images: copy manifest from BASE_TAG to HEAD_TAG.
# Usage: REGISTRY=ghcr.io/org/repo BASE_TAG=abc HEAD_TAG=def promote-ghcr-images.sh gateway chat ...
set -euo pipefail

REGISTRY="${VOICE_IMAGE_REGISTRY:?VOICE_IMAGE_REGISTRY required}"
BASE_TAG="${VOICE_IMAGE_BASE_TAG:?VOICE_IMAGE_BASE_TAG required}"
HEAD_TAG="${VOICE_IMAGE_HEAD_TAG:?VOICE_IMAGE_HEAD_TAG required}"

if [[ "${BASE_TAG}" == "${HEAD_TAG}" ]]; then
  echo "BASE_TAG equals HEAD_TAG (${HEAD_TAG}); nothing to promote"
  exit 0
fi

if (("$#" == 0)); then
  echo "No image names to promote" >&2
  exit 0
fi

promote_one() {
  local name="$1"
  local src="${REGISTRY}/${name}:${BASE_TAG}"
  local dst="${REGISTRY}/${name}:${HEAD_TAG}"
  if docker buildx imagetools create -t "${dst}" "${src}"; then
    echo "promoted: ${src} -> ${dst}"
    return 0
  fi
  echo "ERROR: failed to promote ${src} -> ${dst}" >&2
  return 1
}

failed=0
for name in "$@"; do
  promote_one "${name}" || failed=1
done

exit "${failed}"
