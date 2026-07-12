#!/usr/bin/env bash
# Emit staging-go-services.txt from staging-image-catalog.json (Go images only, gateway included).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CATALOG="${ROOT}/scripts/ci/staging-image-catalog.json"
OUT="${ROOT}/scripts/ci/staging-go-services.txt"

jq -r '.images[] | select(.language == "go") | .name' "${CATALOG}" | sort >"${OUT}"
echo "Wrote $(wc -l <"${OUT}" | tr -d ' ') lines to ${OUT}"
