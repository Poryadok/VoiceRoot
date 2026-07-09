#!/usr/bin/env bash
# Fails until Makefile buf-generate-ci uses local template.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
if grep -A3 '^buf-generate-ci:' "$ROOT/Makefile" | grep -q 'buf.gen.local-go.yaml'; then
  exit 0
fi
echo "buf-generate-ci must use local template buf.gen.local-go.yaml" >&2
exit 1
