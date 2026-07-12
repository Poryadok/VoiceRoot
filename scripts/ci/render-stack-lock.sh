#!/usr/bin/env bash
# Render deploy/staging/stack.lock.yaml from registry, head tag, and service lists.
# Usage:
#   REGISTRY=ghcr.io/org/repo HEAD_TAG=sha \
#   BUILT_SERVICES='["gateway"]' PROMOTED_SERVICES='["chat",...]' \
#   render-stack-lock.sh [output_path]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CATALOG="${ROOT}/scripts/ci/staging-image-catalog.json"
OUT="${1:-${STACK_LOCK_OUT:-${ROOT}/deploy/staging/stack.lock.yaml}}"

REGISTRY="${VOICE_IMAGE_REGISTRY:?VOICE_IMAGE_REGISTRY required}"
HEAD_TAG="${VOICE_IMAGE_HEAD_TAG:?VOICE_IMAGE_HEAD_TAG required}"
BUILT_JSON="${BUILT_SERVICES:-[]}"
PROMOTED_JSON="${PROMOTED_SERVICES:-[]}"

mkdir -p "$(dirname "${OUT}")"

mapfile -t ALL_NAMES < <(jq -r '.images[].name' "${CATALOG}")

{
  echo "version: 1"
  echo "registry: ${REGISTRY}"
  echo "tag: ${HEAD_TAG}"
  echo "images:"
  for name in "${ALL_NAMES[@]}"; do
  tag="${HEAD_TAG}"
  if echo "${BUILT_JSON}" | jq -e --arg n "${name}" 'index($n) != null' >/dev/null; then
    :
  elif echo "${PROMOTED_JSON}" | jq -e --arg n "${name}" 'index($n) != null' >/dev/null; then
    :
  else
    echo "WARN: ${name} not in built or promoted lists; using head tag" >&2
  fi
  echo "  ${name}: ${tag}"
  done
} >"${OUT}"

echo "Wrote stack lock: ${OUT}"
