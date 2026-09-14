#!/usr/bin/env bash
# Return success only when a delta changes Voice runtime/build/storage inputs.
# Cross-service scope restrictions belong to that delta, not every monorepo PR.
voice_r22_runtime_changed() {
  local file
  while IFS= read -r file; do
    case "$file" in
      src/backend/voice/pb/*|src/backend/voice/*_test.go) ;;
      src/backend/voice/*.go|src/backend/voice/go.mod|src/backend/voice/go.sum|src/backend/voice/Dockerfile|src/backend/migrations/voice_db/*)
        return 0 ;;
    esac
  done
  return 1
}
