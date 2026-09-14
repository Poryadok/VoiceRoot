#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
source "${ROOT}/scripts/ci/voice-r22-runtime-scope.sh"
expect_scope() {
  local expected="$1" paths="$2" actual=false
  if printf '%s\n' "$paths" | voice_r22_runtime_changed; then actual=true; fi
  if [[ "$actual" != "$expected" ]]; then
    printf 'Incorrect R22 runtime scope: expected %s for %s\n' "$expected" "$paths" >&2
    exit 1
  fi
}
expect_scope false $'src/backend/space/main.go\nsrc/backend/space/internal/grpcsvc/privacy_listener.go\nsrc/backend/user/main.go\ndocker-compose.yml'
expect_scope false $'protos/voice/space/v1/space.proto\nsrc/backend/voice/pb/voice/space/v1/space.pb.go'
expect_scope false $'src/backend/voice/main_test.go\nsrc/backend/voice/internal/roomlifecycle/ledger_test.go'
expect_scope false 'src/backend/voice-extra/main.go'
expect_scope true 'src/backend/voice/main.go'
expect_scope true 'src/backend/voice/database.go'
expect_scope true 'src/backend/voice/internal/roomlifecycle/postgres_store.go'
expect_scope true 'src/backend/voice/internal/grpcsvc/voice_room.go'
expect_scope true 'src/backend/voice/internal/new_adapter/adapter.go'
expect_scope true 'src/backend/voice/go.mod'
expect_scope true 'src/backend/voice/go.sum'
expect_scope true 'src/backend/voice/Dockerfile'
expect_scope true 'src/backend/migrations/voice_db/000001_room_lifecycle.up.sql'
expect_scope true $'src/backend/space/main.go\nsrc/backend/voice/database.go'
echo 'Voice R22 runtime scope regressions: PASS'
