#!/usr/bin/env bash
# Owned legacy25 populated-chain proof. The fixture creates only a private
# database under the job's isolated service and drops exactly what it created
# before exiting. A hard kill can prevent the recovery-state write and the
# artifact upload from running.
set -euo pipefail
root=../../../scripts/ci
events="$RUNNER_TEMP/space-legacy25-events.jsonl"
receipt="$RUNNER_TEMP/space-legacy25-receipt.json"
inventory="$root/space_legacy25_required.json"
recovery="$RUNNER_TEMP/space-legacy25-cleanup.txt"
query="SELECT COALESCE((SELECT oid FROM pg_database WHERE datname='space_db'),0)"
absent() {
  printf 'databases=space_db\nstate=absent\n' > "$recovery"
}
uncertain() {
  printf 'databases=space_db\nstate=uncertain\nreason=%s\n' "$1" > "$recovery"
}
cleanup() {
  local actual
  if ! actual="$(psql -XAt -v ON_ERROR_STOP=1 "$NATIVE_INSTALLER_TEST_ADMIN_URL" -c "SELECT COALESCE((SELECT oid FROM pg_database WHERE datname='space_db'),0)" 2>/dev/null)"; then
    uncertain inspection_failed
    return 1
  fi
  if [[ "$actual" != 0 ]]; then
    uncertain ownership_unverified
    return 1
  fi
  absent
}
[[ "$(psql -XAt -v ON_ERROR_STOP=1 "$NATIVE_INSTALLER_TEST_ADMIN_URL" -c "$query")" == 0 ]]
absent
trap cleanup EXIT
set +e
go test -json -count=1 -timeout 20m -tags=integration \
  -run '^TestProductionLegacy25PopulatedChainAndRetainedVerify$' \
  ./internal/nativeinstaller > "$events"
result=$?
set -e
python3 "$root/go_integration_matrix.py" produce \
  --inventory "$inventory" --service space \
  --candidate "$CANDIDATE_SHA" --run "$GITHUB_RUN_ID" --attempt "$GITHUB_RUN_ATTEMPT" \
  --events "$events" --exit-code "$result" \
  --output "$receipt" --reject-skips
trap - EXIT
cleanup
python3 "$root/go_integration_matrix.py" verify \
  --inventory "$inventory" --services '["space"]' \
  --candidate "$CANDIDATE_SHA" --run "$GITHUB_RUN_ID" --attempt "$GITHUB_RUN_ATTEMPT" \
  --receipt "$receipt" --events "$events" --require-no-skips
