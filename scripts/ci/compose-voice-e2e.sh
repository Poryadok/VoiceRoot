#!/usr/bin/env bash
# Phase-2 voice call E2E via API Gateway (production REST + Realtime WS path).
# Prerequisites: docker compose --profile app up (healthy gateway, voice, livekit, realtime).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export VOICE_RUN_LIVE_COMPOSE="${VOICE_RUN_LIVE_COMPOSE:-true}"
export VOICE_API_BASE_URL="${VOICE_API_BASE_URL:-http://127.0.0.1:18080}"
export VOICE_LIVEKIT_PUBLIC_URL="${VOICE_LIVEKIT_PUBLIC_URL:-ws://127.0.0.1:7880}"

cd "$ROOT/src/backend/gateway"
go test -count=1 -timeout 5m -run 'TestComposeVoiceCall' ./...
