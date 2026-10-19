#!/bin/sh
set -eu
[ "$(uname -s)" = Linux ] || { echo "private Linux fixture runner required" >&2; exit 1; }
: "${ROLE_NATIVE_FIXTURE_EXPORT_DIR:?absolute fresh private directory required}"
: "${ROLE_NATIVE_FIXTURE_ROLE_SOURCE:?exact immutable Role source SHA required}"
role_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$role_root"
exec go test -tags role_space_native . -run '^TestMemberAdmissionRole25Space44ProtectedReceipt$' -race -count=1 -timeout=11m -v
