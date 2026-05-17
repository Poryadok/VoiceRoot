#!/usr/bin/env bash
# Assert root docker-compose defines a NATS service with JetStream (-js).
# Used by CI and optionally locally (requires jq). See docs/TESTING.md.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
if ! command -v jq >/dev/null 2>&1; then
  echo "compose-nats-jetstream-check: jq is required on PATH" >&2
  exit 1
fi
FILTER="$ROOT/scripts/ci/compose-nats-jetstream.jq"
docker compose config --format json | jq -e -f "$FILTER" >/dev/null
