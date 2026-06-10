#!/usr/bin/env bash
# Opt-in compose E2E: Phase-1 DM realtime, friends, auth, voice signaling (+ media on Linux),
# Phase-3 messaging features, and Flutter API-level live tests.
# Prerequisites: docker compose --profile app up (healthy gateway and dependencies).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export VOICE_RUN_LIVE_COMPOSE="${VOICE_RUN_LIVE_COMPOSE:-true}"
export VOICE_API_BASE_URL="${VOICE_API_BASE_URL:-http://127.0.0.1:18080}"
export VOICE_LIVEKIT_PUBLIC_URL="${VOICE_LIVEKIT_PUBLIC_URL:-ws://127.0.0.1:7880}"

cd "$ROOT/src/backend/gateway"
go test -count=1 -parallel 1 -timeout 15m -run 'TestCompose.*_live' ./...

cd "$ROOT/src/frontend"
flutter test --concurrency=1 \
  test/gateway_dm_ws_live_integration_test.dart \
  test/phase1_two_users_e2e_live_test.dart \
  test/phase1_friends_e2e_live_test.dart \
  test/phase1_auth_logout_e2e_live_test.dart \
  test/phase1_ws_resume_e2e_live_test.dart \
  test/phase2_voice_signaling_e2e_live_test.dart \
  test/phase3_typing_e2e_live_test.dart \
  test/phase3_edit_delete_e2e_live_test.dart \
  test/phase3_delivery_e2e_live_test.dart \
  test/phase3_dm_requests_e2e_live_test.dart \
  test/phase3_file_attachment_e2e_live_test.dart \
  test/phase4_groups_e2e_live_test.dart \
  test/phase4_group_roles_e2e_live_test.dart \
  test/phase4_group_voice_e2e_live_test.dart \
  test/phase4_forward_e2e_live_test.dart \
  test/phase4_reactions_e2e_live_test.dart \
  test/phase4_in_app_notifications_e2e_live_test.dart \
  --dart-define=VOICE_RUN_LIVE_INTEGRATION=true \
  --dart-define=VOICE_API_BASE_URL="${VOICE_API_BASE_URL}"
