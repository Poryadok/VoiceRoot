#!/usr/bin/env bash
# Install Voice git safety hooks (repo + optional Cursor beforeShellExecution).
# Run from repo root: ./scripts/git/install-hooks.sh

set -euo pipefail

SKIP_CURSOR=false
if [[ "${1:-}" == "--skip-cursor" ]]; then
  SKIP_CURSOR=true
fi

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GIT_HOOKS="$ROOT/.git/hooks"
SRC_HOOKS="$(cd "$(dirname "$0")/hooks" && pwd)"

if [[ ! -d "$GIT_HOOKS" ]]; then
  echo "Not a git repository: $GIT_HOOKS missing" >&2
  exit 1
fi

for name in pre-rebase pre-push; do
  cp "$SRC_HOOKS/$name" "$GIT_HOOKS/$name"
  chmod +x "$GIT_HOOKS/$name"
  echo "Installed git hook: $GIT_HOOKS/$name"
done

if $SKIP_CURSOR; then
  echo "Skipped Cursor hook (--skip-cursor)."
  exit 0
fi

CURSOR_HOOKS="${HOME}/.cursor/hooks"
CURSOR_DST="$CURSOR_HOOKS/block-history-rewrite.js"
CURSOR_SRC="$(dirname "$0")/cursor-hook-block-history-rewrite.js"
HOOKS_JSON="${HOME}/.cursor/hooks.json"

mkdir -p "$CURSOR_HOOKS"
cp "$CURSOR_SRC" "$CURSOR_DST"
echo "Installed Cursor hook: $CURSOR_DST"

HOOK_PATH="${CURSOR_DST//\\//}"
ENTRY=$(cat <<EOF
{
  "command": "node \\"$HOOK_PATH\\"",
  "matcher": "\\\\bgit(\\\\.exe)?\\\\s",
  "failClosed": true
}
EOF
)

if [[ -f "$HOOKS_JSON" ]] && command -v node >/dev/null 2>&1; then
  node <<NODE
const fs = require('fs');
const path = '$HOOKS_JSON';
const entry = $ENTRY;
let cfg = { version: 1, hooks: { beforeShellExecution: [] } };
if (fs.existsSync(path)) {
  cfg = JSON.parse(fs.readFileSync(path, 'utf8'));
  cfg.hooks = cfg.hooks || {};
  cfg.hooks.beforeShellExecution = cfg.hooks.beforeShellExecution || [];
}
cfg.hooks.beforeShellExecution = cfg.hooks.beforeShellExecution.filter(
  (h) => !(h.command || '').includes('block-history-rewrite.js')
);
cfg.hooks.beforeShellExecution.push(entry);
fs.writeFileSync(path, JSON.stringify(cfg, null, 2) + '\n');
console.log('Updated Cursor hooks.json:', path);
NODE
else
  mkdir -p "$(dirname "$HOOKS_JSON")"
  cat > "$HOOKS_JSON" <<EOF
{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      $ENTRY
    ]
  }
}
EOF
  echo "Created Cursor hooks.json: $HOOKS_JSON"
fi

echo "Done. Self-test: node scripts/git/selftest-history-rewrite.js"
