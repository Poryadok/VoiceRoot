#!/bin/sh
set -eu
# The runner explicitly mounts the current integration checkout read-only.
# Copy it into disposable storage; never build or generate into that checkout.
: "${ROLE_NATIVE_SPACE_SOURCE_ROOT:?explicit current integration source required}"
test -f "$ROLE_NATIVE_SPACE_SOURCE_ROOT/src/backend/space/go.mod"
source_root="$(mktemp -d /tmp/role-native-space.XXXXXX)"
trap 'rm -rf "$source_root"' EXIT
mkdir -p "$source_root/src/backend/migrations"
for module in space pkg role; do
    cp -R "$ROLE_NATIVE_SPACE_SOURCE_ROOT/src/backend/$module" "$source_root/src/backend/$module"
done
for service in user chat voice messaging file realtime analytics; do
    mkdir -p "$source_root/src/backend/$service"
    cp -R "$ROLE_NATIVE_SPACE_SOURCE_ROOT/src/backend/$service/pb" "$source_root/src/backend/$service/pb"
done
cp -R "$ROLE_NATIVE_SPACE_SOURCE_ROOT/src/backend/migrations/space_db" "$source_root/src/backend/migrations/space_db"
echo "Native Space fixture source: $ROLE_NATIVE_SPACE_SOURCE_ROOT (current checkout snapshot)" >&2
mkdir -p "$source_root/src/backend/space/cmd/role-native-fixture"
fixture_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
cp "$fixture_dir"/native_space_fixture/*.go "$source_root/src/backend/space/cmd/role-native-fixture/"
cd "$source_root/src/backend/space"
go build -o /tmp/role-native-space-fixture ./cmd/role-native-fixture
# The actual migrator fixture needs the same immutable migration set, copied
# outside the temporary build tree for the lifetime of the Role test process.
mkdir -p /tmp/role-native-space-migrations
cp "$source_root"/src/backend/migrations/space_db/*.sql /tmp/role-native-space-migrations/
