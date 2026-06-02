#!/usr/bin/env bash
# Run golangci-lint on all backend Go modules (parity with Makefile GO_MODULES_LINT / CI job golangci).
set -euo pipefail

ROOT="${1:-}"
if [[ -z "${ROOT}" ]]; then
  ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
fi

GOLANGCI_LINT="${GOLANGCI_LINT:-golangci-lint}"
GOLANGCI_LINT_MOD="${GOLANGCI_LINT_MOD:-github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.1}"

gopath_bin="$(go env GOPATH)/bin"
export PATH="${gopath_bin}:${PATH}"

if ! command -v "${GOLANGCI_LINT}" >/dev/null 2>&1; then
  echo "golangci-ci: installing ${GOLANGCI_LINT_MOD}"
  go install "${GOLANGCI_LINT_MOD}"
fi

# Keep in sync with Makefile GO_MODULES_LINT
modules=(
  pkg analytics bot chat federation file gateway matchmaking messaging moderation
  notification realtime role search social space story subscription user voice
)

for m in "${modules[@]}"; do
  echo "== ${m} =="
  (cd "${ROOT}/src/backend/${m}" && "${GOLANGCI_LINT}" run ./...) || exit 1
done

echo "golangci-ci: ok"
