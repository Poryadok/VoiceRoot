#!/usr/bin/env bash
# Assert root docker-compose defines a NATS service with JetStream (-js).
# Used by CI and optionally locally (requires Docker). See docs/TESTING.md.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
rendered_config="$(mktemp)"
trap 'rm -f "$rendered_config"' EXIT

if ! docker compose config --format json >"$rendered_config"; then
  echo "FAIL: could not render Docker Compose configuration for the NATS JetStream check" >&2
  exit 1
fi

if ! docker run --rm -i -v "$ROOT:/w" ghcr.io/jqlang/jq:1.7 \
  -e -f /w/scripts/ci/compose-nats-jetstream.jq <"$rendered_config" >/dev/null; then
  echo 'FAIL: rendered Docker Compose must define services.nats.command with the "-js" JetStream flag.' >&2
  exit 1
fi

bash "$ROOT/scripts/ci/account-delete-nats-invariants-test.sh"
bash "$ROOT/scripts/ci/nats-realtime-bootstrap-contract-test.sh"
bash "$ROOT/scripts/ci/nats-notification-bootstrap-contract-test.sh"
if [[ "${VOICE_CI_COMPOSE_NATS_SMOKE:-1}" == "1" ]]; then
  bash "$ROOT/scripts/ci/nats-realtime-bootstrap-smoke.sh"
  bash "$ROOT/scripts/ci/nats-notification-bootstrap-smoke.sh"
fi
