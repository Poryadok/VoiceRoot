#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${VOICE_MINIO_IMAGE:-}" || -z "${VOICE_MINIO_MC_IMAGE:-}" ]]; then
  echo 'COMPOSE_MINIO_IMAGES=FAIL_MISSING_OVERRIDE'
  exit 1
fi

if ! config="$(docker compose --profile app config --format json 2>/dev/null)"; then
  echo 'COMPOSE_MINIO_IMAGES=FAIL_CONFIG'
  exit 1
fi

if ! python3 -c '
import json
import sys

try:
    services = json.load(sys.stdin).get("services", {})
    matches = (services.get("minio", {}).get("image") == sys.argv[1]
               and services.get("minio-init", {}).get("image") == sys.argv[2])
except (json.JSONDecodeError, AttributeError, TypeError):
    sys.exit(1)
sys.exit(0 if matches else 1)
' "$VOICE_MINIO_IMAGE" "$VOICE_MINIO_MC_IMAGE" <<<"$config" >/dev/null 2>&1; then
  echo 'COMPOSE_MINIO_IMAGES=FAIL_EFFECTIVE_IMAGE'
  exit 1
fi

echo 'COMPOSE_MINIO_IMAGES=PASS'
