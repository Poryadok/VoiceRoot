#!/usr/bin/env bash
# Remove leftover testcontainers containers (label org.testcontainers).
# Does not stop voice-* compose stacks. Safe to run after backend integration tests.
set -euo pipefail

ids="$(docker ps -aq --filter "label=org.testcontainers" 2>/dev/null || true)"
if [[ -z "${ids// }" ]]; then
  echo "testcontainers-prune: no containers to remove"
  exit 0
fi

echo "testcontainers-prune: removing $(echo "$ids" | wc -w | tr -d ' ') container(s)"
# shellcheck disable=SC2086
docker rm -f $ids
