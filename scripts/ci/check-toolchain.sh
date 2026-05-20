#!/usr/bin/env bash
# Prerequisites for make build-all (host Go/Maven tests; Docker for buf/compose/images/testcontainers).
set -euo pipefail

fail() {
  echo "check-toolchain: $*" >&2
  exit 1
}

warn() {
  echo "check-toolchain: warning: $*" >&2
}

require_cmd() {
  local name="$1"
  command -v "$name" >/dev/null 2>&1 || fail "missing $name in PATH (required for make build-all)"
}

require_cmd go
require_cmd docker
require_cmd mvn
require_cmd java

go_minor="$(go env GOVERSION 2>/dev/null || go version | awk '{print $3}')"
go_minor="${go_minor#go}"
case "$go_minor" in
  1.26*) ;;
  *)
    fail "Go 1.26.x required (got go${go_minor}); see README toolchain table"
    ;;
esac

if ! docker info >/dev/null 2>&1; then
  fail "Docker daemon not reachable (start Docker Desktop or dockerd)"
fi

if ! command -v golangci-lint >/dev/null 2>&1; then
  warn "golangci-lint not in PATH; make golangci-ci will run: go install ${GOLANGCI_LINT_MOD:-github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.1}"
fi

echo "check-toolchain: ok (go ${go_minor}, docker, mvn, java)"
