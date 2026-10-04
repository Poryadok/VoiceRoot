#!/usr/bin/env bash
# Regenerate Go pb/ stubs and fail if committed trees drift from protos/.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

# Exercise the drift detector without requiring Buf plugins or services.
if [[ "${BUF_GO_PB_CHECK_SKIP_SELF_TEST:-}" != "1" ]]; then
  bash "${ROOT}/scripts/ci/buf-go-pb-check_test.sh"
fi

if ! command -v buf >/dev/null 2>&1; then
  echo "buf CLI not found on PATH" >&2
  exit 1
fi

buf generate --template buf.gen.local-go.yaml
bash "${ROOT}/scripts/dev/sync-pb-from-gen.sh"

pb_pathspec=':(glob)src/backend/*/pb/**'
if git diff --exit-code HEAD -- "${pb_pathspec}" >/dev/null 2>&1 \
  && git diff --cached --exit-code HEAD -- "${pb_pathspec}" >/dev/null 2>&1 \
  && git diff --exit-code -- "${pb_pathspec}" >/dev/null 2>&1 \
  && [[ -z "$(git ls-files --others --exclude-standard -- "${pb_pathspec}")" ]]; then
  echo "Go pb/ trees in sync with protos/"
  exit 0
fi

echo "Go pb/ trees out of sync — run: make buf-generate-all" >&2
git diff --stat HEAD -- "${pb_pathspec}" >&2 || true
git diff --cached --stat HEAD -- "${pb_pathspec}" >&2 || true
untracked="$(git ls-files --others --exclude-standard -- "${pb_pathspec}")"
if [[ -n "${untracked}" ]]; then
  echo "Untracked generated Go pb/ files:" >&2
  printf '%s\n' "${untracked}" >&2
fi
exit 1
