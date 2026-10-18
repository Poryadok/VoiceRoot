#!/usr/bin/env bash
# Bounded isolated Linux proof. Caller supplies disposable PostgreSQL and NATS.
set -euo pipefail
[[ "$(uname -s)" == Linux ]] || { echo 'Linux runner required' >&2; exit 1; }
: "${REPORT_INTAKE_TEST_DSN:?disposable isolated PostgreSQL required}"
: "${REPORT_INTAKE_TEST_NATS_URL:?disposable isolated JetStream required}"
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REPORT_PROOF_DIR="$(mktemp -d)"
trap 'rm -rf -- "$REPORT_PROOF_DIR"' EXIT
export REPORT_TARGET_PROOF_BINARY_DIR="$REPORT_PROOF_DIR"
export REPORT_TARGET_RUNTIME_FIXTURE="$REPORT_PROOF_DIR/pki"
export REPORT_TARGET_PROOF_OS=linux
export REPORT_TARGET_TEST_DSN="$REPORT_INTAKE_TEST_DSN"
python3 "$ROOT/scripts/dev/generate-report-target-fixture.py" "$REPORT_TARGET_RUNTIME_FIXTURE"
for owner in user messaging space story; do
  (cd "$ROOT/src/backend/$owner" && go test -mod=readonly -p=1 -c -o "$REPORT_PROOF_DIR/$owner" ./internal/grpcsvc)
  (cd "$ROOT/src/backend/$owner" && go test -mod=readonly -p=1 -count=1 -run '^TestReportTargetOwnerPostgres$' ./internal/store)
done
(cd "$ROOT/src/backend/pkg" && go test -mod=readonly -p=1 -count=1 ./reporttargetprincipal)
(cd "$ROOT/src/backend/user" && go test -mod=readonly -p=1 -count=1 -run '^TestReportTargetCurrentAuth$' ./internal/grpcsvc)
cd "$ROOT/src/backend/moderation"
go test -mod=readonly -p=1 -count=1 -run '^TestReportTargetRuntimeFixture$' .
go test -mod=readonly -p=1 -count=1 ./internal/reporttargetclient ./internal/reportintake
go test -mod=readonly -p=1 -count=1 -v -run '^TestReportIntakeFourOwnerRuntime$' ./internal/grpcsvc
