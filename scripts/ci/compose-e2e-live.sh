#!/usr/bin/env bash
# Opt-in compose E2E: Phase-1 DM realtime, friends, auth, voice signaling (+ media on Linux).
# Prerequisites: docker compose --profile app up (healthy gateway and dependencies).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export VOICE_RUN_LIVE_COMPOSE="${VOICE_RUN_LIVE_COMPOSE:-true}"
export VOICE_API_BASE_URL="${VOICE_API_BASE_URL:-http://127.0.0.1:18080}"
export VOICE_LIVEKIT_PUBLIC_URL="${VOICE_LIVEKIT_PUBLIC_URL:-ws://127.0.0.1:7880}"

cd "$ROOT/src/backend/gateway"
go test -count=1 -timeout 10m -run 'TestCompose.*_live' ./...
