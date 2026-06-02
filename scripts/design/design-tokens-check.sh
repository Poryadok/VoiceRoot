#!/usr/bin/env bash
# Compare canonical design tokens with Flutter asset copy.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CANON="$ROOT/design/tokens/voice.tokens.json"
ASSET="$ROOT/src/frontend/assets/design/voice.tokens.json"
if [[ ! -f "$CANON" ]]; then
  echo "missing canonical tokens: $CANON" >&2
  exit 1
fi
if [[ ! -f "$ASSET" ]]; then
  echo "missing Flutter asset tokens: $ASSET" >&2
  exit 1
fi
if ! cmp -s "$CANON" "$ASSET"; then
  echo "design tokens out of sync:" >&2
  echo "  canonical: $CANON" >&2
  echo "  asset:     $ASSET" >&2
  echo "Run: cp design/tokens/voice.tokens.json src/frontend/assets/design/voice.tokens.json" >&2
  exit 1
fi
echo "design tokens: canonical and asset match"
