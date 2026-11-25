#!/usr/bin/env bash
# Owned legacy24 proofs. Every fixture here creates only private databases under
# the job's isolated service and drops exactly what it created before exiting.
# A hard kill can prevent the recovery-state write and artifact upload.
set -euo pipefail
root=../../../scripts/ci
events="$RUNNER_TEMP/space-legacy24-events.jsonl"
receipt="$RUNNER_TEMP/space-legacy24-receipt.json"
inventory="$root/space_legacy24_proofs_required.json"
recovery="$RUNNER_TEMP/space-legacy24-cleanup.txt"
query="SELECT COALESCE((SELECT oid FROM pg_database WHERE datname='space_db'),0)"
absent() {
  printf 'databases=space_db,space_original24,space_original25,space_restored25,space_clone,space_upgraded,space_other,space_downgraded\nstate=absent\n' > "$recovery"
}
uncertain() {
  printf 'databases=space_db,space_original24,space_original25,space_restored25,space_clone,space_upgraded,space_other,space_downgraded\nstate=uncertain\nreason=%s\n' "$1" > "$recovery"
}
cleanup() {
  local name actual
  for name in space_db space_original24 space_original25 space_restored25 space_clone space_upgraded space_other space_downgraded; do
    if ! actual="$(psql -XAt -v ON_ERROR_STOP=1 "$NATIVE_INSTALLER_TEST_ADMIN_URL" -c "SELECT COALESCE((SELECT oid FROM pg_database WHERE datname='$name'),0)" 2>/dev/null)"; then
      uncertain inspection_failed
      return 1
    fi
    if [[ "$actual" != 0 ]]; then
      uncertain ownership_unverified
      return 1
    fi
  done
  absent
}
[[ "$(psql -XAt -v ON_ERROR_STOP=1 "$NATIVE_INSTALLER_TEST_ADMIN_URL" -c "$query")" == 0 ]]
absent
trap cleanup EXIT
set +e
go test -json -count=1 -timeout 20m -tags=integration \
  -run '^(TestProductionLegacy24InspectReadOnlyWithActiveClient|TestLegacy24LogicalRestoreCanonicalEquivalence|TestProductionLegacy24OriginalVerifyAndNoAdoption|TestProductionLegacy24FixedSearchPath|TestProductionLegacy24LostEventNeverReplaysCommittedPrefix|TestLegacy24Exact25TransitionPreservesSourceRows|TestProductionLegacy24PopulatedChain)$' \
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
