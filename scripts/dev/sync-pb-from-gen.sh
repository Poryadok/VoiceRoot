#!/usr/bin/env bash
# Copy generated Go protobuf stubs from gen/go/voice/ into committed src/backend/*/pb/voice/ trees.
# Preserves per-service go.mod / go.sum in pb/ (not overwritten).
# Prerequisite: make buf-generate (outputs to gen/go/voice/).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GEN_ROOT="${ROOT}/gen/go/voice"

if [[ ! -d "${GEN_ROOT}" ]]; then
  echo "gen/go/voice/ not found — run: make buf-generate" >&2
  exit 1
fi

# Services with committed pb/voice subtrees (see docs/REPOSITORIES.md).
SERVICES=(analytics chat file messaging role user voice)

synced=0
for svc in "${SERVICES[@]}"; do
  pb_root="${ROOT}/src/backend/${svc}/pb/voice"
  [[ -d "${pb_root}" ]] || continue

  while IFS= read -r -d '' pkg_dir; do
    pkg="$(basename "${pkg_dir}")"
    src="${GEN_ROOT}/${pkg}"
    dst="${pb_root}/${pkg}"
    if [[ ! -d "${src}" ]]; then
      echo "skip ${svc}/pb/voice/${pkg}: no gen/go/voice/${pkg}" >&2
      continue
    fi
    mkdir -p "${dst}"
    if command -v rsync >/dev/null 2>&1; then
      rsync -a --delete \
        --exclude 'go.mod' --exclude 'go.sum' \
        "${src}/" "${dst}/"
    else
      find "${dst}" -mindepth 1 -maxdepth 1 ! -name 'go.mod' ! -name 'go.sum' -exec rm -rf {} +
      for f in "${src}"/*; do
        base="$(basename "${f}")"
        [[ "${base}" == go.mod || "${base}" == go.sum ]] && continue
        cp -a "${f}" "${dst}/"
      done
    fi
    echo "synced ${svc}/pb/voice/${pkg}"
    synced=$((synced + 1))
  done < <(find "${pb_root}" -mindepth 1 -maxdepth 1 -type d -print0)
done

if [[ "${synced}" -eq 0 ]]; then
  echo "no pb/voice package trees found under src/backend/*/pb/voice" >&2
  exit 1
fi

echo "sync-pb-from-gen: ${synced} package tree(s) updated"
