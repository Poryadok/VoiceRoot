#!/usr/bin/env bash
# Hosted disposable credential bootstrap proof; never targets a developer stack.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
project="social-principal-${GITHUB_RUN_ID:?hosted run id required}-${GITHUB_RUN_ATTEMPT:-1}"
compose() { docker compose -p "$project" -f "${ROOT}/docker-compose.yml" --profile app "$@"; }
cleanup() { compose down --volumes --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT
compose run --build --rm --no-deps social-principal-init
# A second init proves reuse does not rotate keys underneath running consumers.
compose run --rm --no-deps social-principal-init
compose run --rm --no-deps --entrypoint sh social-principal-init -ec '
  test ! -f /ca/ca.key
  test "$(stat -c %a /signing/current.pem)" = 600
  test "$(stat -c %u /signing/current.pem)" = 65532
  openssl pkey -in /signing/current.pem -check -noout
  openssl pkey -in /signing/next.pem -check -noout
  current=$(openssl pkey -in /signing/current.pem -pubout 2>/dev/null | openssl dgst -sha256)
  next=$(openssl pkey -in /signing/next.pem -pubout 2>/dev/null | openssl dgst -sha256)
  test "$current" != "$next"
  for service in social user space; do
    openssl verify -CAfile /ca/ca.crt -verify_hostname "$service" "/$service/tls.crt"
  done
'
