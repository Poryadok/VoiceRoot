#!/usr/bin/env bash
# Regenerate Go pb/ stubs and fail if committed trees drift from protos/.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

if ! command -v buf >/dev/null 2>&1; then
  echo "buf CLI not found on PATH" >&2
  exit 1
fi

buf generate --template buf.gen.local-go.yaml
bash "${ROOT}/scripts/dev/sync-pb-from-gen.sh"

if git diff --exit-code -- 'src/backend/*/pb/' >/dev/null 2>&1; then
  echo "Go pb/ trees in sync with protos/"
  exit 0
fi

echo "Go pb/ trees out of sync — run: make buf-generate-all" >&2
git diff --stat -- 'src/backend/*/pb/' >&2 || true
exit 1
