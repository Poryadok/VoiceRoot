#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_DIR="${ROOT}/.local"
OUT_FILE="${OUT_DIR}/compose.log"
SERVICES=(gateway messaging chat realtime user social voice file auth)

mkdir -p "${OUT_DIR}"
cd "${ROOT}"

docker compose logs --no-color --timestamps "${SERVICES[@]}" > "${OUT_FILE}"

echo "Wrote ${OUT_FILE}"
