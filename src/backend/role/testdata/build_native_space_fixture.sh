#!/bin/sh
set -eu
role_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
repo_root="$(CDPATH= cd -- "$role_root/../../.." && pwd)"
# The fixture's implementation and source are independently pinned. No peer
# worktree, generated fake DTO, old catalog relabel, or mutable branch is used.
case "${ROLE_NATIVE_SPACE_COHORT:-39}" in
  39) source_pin=5e0fc4f551dd9dd4659d5a6da1b7393608cf0cff ;;
  44) source_pin=a36cc5e504fab412f6d48dd5233ed199b9b5d243 ;;
  *) echo "unsupported immutable Space fixture cohort" >&2; exit 1 ;;
esac
source_root="$(mktemp -d /tmp/role-native-space.XXXXXX)"
trap 'rm -rf "$source_root"' EXIT
# The calling test owns this fresh directory through its complete lifetime.
# Binary and SQL must move together; no persistent cross-cohort /tmp outputs.
: "${ROLE_NATIVE_SPACE_OUTPUT_DIR:?fresh caller-owned output directory required}"
case "$ROLE_NATIVE_SPACE_OUTPUT_DIR" in /*) ;; *) echo "absolute output directory required" >&2; exit 1 ;; esac
test -d "$ROLE_NATIVE_SPACE_OUTPUT_DIR"
test -z "$(find "$ROLE_NATIVE_SPACE_OUTPUT_DIR" -mindepth 1 -maxdepth 1 -print -quit)"
# Windows worktree .git pointers are intentionally not resolved inside Linux.
# Host preparation: rtk git archive --format=tar -o
# tmp/swarm/native-space-$source_pin.tar $source_pin src/backend
tar -xf "${ROLE_NATIVE_SPACE_ARCHIVE_DIR:-$repo_root/tmp/swarm}/native-space-$source_pin.tar" -C "$source_root"
mkdir -p "$source_root/src/backend/space/cmd/role-native-fixture"
cp "$role_root/testdata/native_space_fixture/main.go" "$source_root/src/backend/space/cmd/role-native-fixture/main.go"
cp "$role_root/testdata/native_space_fixture/dsn_test.go" "$source_root/src/backend/space/cmd/role-native-fixture/dsn_test.go"
cd "$source_root/src/backend/space"
go test ./cmd/role-native-fixture -run '^TestFixtureDSNCredentialRoundTrip$' -count=1
go build -o "$ROLE_NATIVE_SPACE_OUTPUT_DIR/fixture" ./cmd/role-native-fixture
# The actual migrator fixture needs the same immutable migration set, copied
# outside the temporary build tree for the lifetime of the Role test process.
mkdir "$ROLE_NATIVE_SPACE_OUTPUT_DIR/migrations"
cp "$source_root"/src/backend/migrations/space_db/*.sql "$ROLE_NATIVE_SPACE_OUTPUT_DIR/migrations/"
