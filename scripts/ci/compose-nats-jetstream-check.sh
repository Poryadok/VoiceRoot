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
rendered_config="$(mktemp)"
trap 'rm -f "$rendered_config"' EXIT

if ! docker compose config --format json >"$rendered_config"; then
  echo "FAIL: could not render Docker Compose configuration for the NATS JetStream check" >&2
  exit 1
fi

if ! jq -e -f "$FILTER" "$rendered_config" >/dev/null; then
  echo 'FAIL: rendered Docker Compose must define services.nats.command with the "-js" JetStream flag.' >&2
  exit 1
fi

bash "$ROOT/scripts/ci/account-delete-nats-invariants-test.sh"
