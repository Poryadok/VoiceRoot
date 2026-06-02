#!/usr/bin/env bash
# Fail if forbidden color patterns appear outside lib/theme/.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
UI="$ROOT/src/frontend/lib/ui"
SHELL="$ROOT/src/frontend/lib/shell"
PATTERN='ColorScheme\.fromSeed|Colors\.indigo|Color\(0x'
found=0
for dir in "$UI" "$SHELL"; do
  if rg -q "$PATTERN" "$dir" 2>/dev/null; then
    echo "forbidden color usage under $dir:" >&2
    rg -n "$PATTERN" "$dir" >&2 || true
    found=1
  fi
done
if [[ "$found" -ne 0 ]]; then
  exit 1
fi
echo "flutter ui color gate: ok"
