#!/usr/bin/env bash
# Public CLI for the YAML E2E feature catalog (Python3 + requirements-e2e.txt).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
exec python3 "${ROOT}/scripts/ci/e2e_manifest.py" "${1:?manifest path}" "${2:?section}"
