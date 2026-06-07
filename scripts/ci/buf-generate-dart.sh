#!/usr/bin/env bash
# Generate committed Dart protobuf stubs for Flutter (src/frontend/lib/gen).
# Requires: buf on PATH, `dart pub global activate protoc_plugin`.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

if ! command -v protoc-gen-dart >/dev/null 2>&1; then
  echo "protoc-gen-dart not found; run: dart pub global activate protoc_plugin" >&2
  exit 1
fi

buf generate --template buf.gen.dart.yaml

echo "Dart protobuf codegen OK: src/frontend/lib/gen"
