#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_DIR="${ROOT}/.local"
OUT_FILE="${OUT_DIR}/compose.log"
NDJSON_FILE="${OUT_DIR}/dev.ndjson"
SERVICES=(gateway messaging chat realtime user social voice file auth)

mkdir -p "${OUT_DIR}"
cd "${ROOT}"

docker compose logs --no-color --timestamps "${SERVICES[@]}" > "${OUT_FILE}"

: > "${NDJSON_FILE}"
while IFS= read -r line; do
  case "${line}" in
    *'| {'*)
      json="${line#*| }"
      printf '%s\n' "${json}" >> "${NDJSON_FILE}"
      ;;
  esac
done < "${OUT_FILE}"

echo "Wrote ${OUT_FILE}"
echo "Wrote ${NDJSON_FILE}"
