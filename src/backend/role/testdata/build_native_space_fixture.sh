#!/bin/sh
set -eu
# The fixture's implementation and source are independently pinned. No peer
# worktree, generated fake DTO, old catalog relabel, or mutable branch is used.
case "${ROLE_NATIVE_SPACE_COHORT:-39}" in
  39) source_pin=5e0fc4f551dd9dd4659d5a6da1b7393608cf0cff ;;
  44) source_pin=d410305b2e74993966e07aca5a486c16766d9e9a ;;
  *) echo "unsupported immutable Space fixture cohort" >&2; exit 1 ;;
esac
source_root="$(mktemp -d /tmp/role-native-space.XXXXXX)"
trap 'rm -rf "$source_root"' EXIT
# Windows worktree .git pointers are intentionally not resolved inside Linux.
# Host preparation: rtk git archive --format=tar -o
# tmp/swarm/native-space-$source_pin.tar $source_pin src/backend
tar -xf "/workspace/tmp/swarm/native-space-$source_pin.tar" -C "$source_root"
mkdir -p "$source_root/src/backend/space/cmd/role-native-fixture"
cp /workspace/src/backend/role/testdata/native_space_fixture/main.go "$source_root/src/backend/space/cmd/role-native-fixture/main.go"
cd "$source_root/src/backend/space"
go build -o /tmp/role-native-space-fixture ./cmd/role-native-fixture
# The actual migrator fixture needs the same immutable migration set, copied
# outside the temporary build tree for the lifetime of the Role test process.
mkdir -p /tmp/role-native-space-migrations
cp "$source_root"/src/backend/migrations/space_db/*.sql /tmp/role-native-space-migrations/
