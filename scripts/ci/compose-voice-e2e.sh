#!/usr/bin/env bash
# Deprecated alias — use scripts/ci/compose-e2e-live.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
exec "$(dirname "$0")/compose-e2e-live.sh" "$@"
