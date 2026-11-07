#!/usr/bin/env bash
# Real legacy34 cohort, qualified through 45/50, consumed by the native Ban.
# This script owns the job's isolated PostgreSQL service and is its only
# administrative database creator/deleter while it runs. The OID check and DROP
# are separate commands; they rely on that exclusive-admin precondition.
# A hard kill can prevent the EXIT trap and artifact upload from running.
set -euo pipefail

query="SELECT COALESCE((SELECT oid FROM pg_database WHERE datname='space_over100'),0)"
[[ "$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "$query")" == 0 ]]
owned_oid=""
recovery="$RUNNER_TEMP/space-over100-cleanup.txt"
printf 'database=space_over100\nstate=creating\n' > "$recovery"
cleanup() {
  local actual
  if ! actual="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "$query")"; then
    printf 'database=space_over100\nstate=uncertain\nreason=inspection_failed\n' > "$recovery"
    return 1
  fi
  if [[ "$actual" == 0 ]]; then
    printf 'database=space_over100\nstate=absent\n' > "$recovery"
    return 0
  fi
  if [[ -z "$owned_oid" || "$actual" != "$owned_oid" ]]; then
    printf 'database=space_over100\nstate=uncertain\nreason=identity_unverified\nobserved_oid=%s\n' "$actual" > "$recovery"
    return 1
  fi
  if ! psql -X -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c 'DROP DATABASE space_over100'; then
    printf 'database=space_over100\nstate=uncertain\nreason=drop_failed\nowned_oid=%s\n' "$owned_oid" > "$recovery"
    return 1
  fi
  printf 'database=space_over100\nstate=removed\nowned_oid=%s\n' "$owned_oid" > "$recovery"
}
trap cleanup EXIT
psql -X -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c 'CREATE DATABASE space_over100'
owned_oid="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "$query")"
[[ "$owned_oid" =~ ^[1-9][0-9]*$ ]]
printf 'database=space_over100\nstate=owned\nowned_oid=%s\n' "$owned_oid" > "$recovery"

receipt() {
  local name="$1" inventory="$2" test_name="$3" result
  set +e
  go test -json -count=1 -timeout 5m -tags=spacebanintegration \
    -run "^${test_name}$" ./internal/store > "$RUNNER_TEMP/space-over100-${name}-events.jsonl"
  result=$?
  set -e
  python3 ../../../scripts/ci/go_integration_matrix.py produce \
    --inventory "../../../scripts/ci/${inventory}" --service space \
    --candidate "$CANDIDATE_SHA" --run "$GITHUB_RUN_ID" --attempt "$GITHUB_RUN_ATTEMPT" \
    --events "$RUNNER_TEMP/space-over100-${name}-events.jsonl" --exit-code "$result" \
    --output "$RUNNER_TEMP/space-over100-${name}-receipt.json" --reject-skips
}

"$RUNNER_TEMP/native-migrate-driver" -path ../migrations/space_db -database "$SPACE_BAN_TEST_DATABASE_URL" goto 34
psql -X -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" <<'SQL'
INSERT INTO spaces(id,name,owner_profile_id,member_count) VALUES
 ('30000000-0000-0000-0000-00000000000d','legacy over100 fixture','10000000-0000-0000-0000-000000000001',5);
INSERT INTO space_members(space_id,profile_id)
 SELECT '30000000-0000-0000-0000-00000000000d'::uuid,
        format('10000000-0000-0000-0000-%s',lpad(to_hex(p),12,'0'))::uuid
 FROM (VALUES (1),(2),(3),(6)) AS profiles(p);
INSERT INTO community_owner_authority(space_id,application_id,environment_id,corporation_key,owner_account_id,owner_profile_id,owner_generation,status,roster_source_revision,roster_sha256,roster_lease_expires_at) VALUES
 ('30000000-0000-0000-0000-00000000000d','40000000-0000-0000-0000-000000000001','40000000-0000-0000-0000-000000000002','legacy13-fixture','20000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001',1,'active',1,decode(repeat('aa',32),'hex'),now()+interval '1 day');
INSERT INTO community_roster_members(space_id,profile_id,source_rank,source_reasons,source_revision,owner_generation,lease_expires_at)
 SELECT '30000000-0000-0000-0000-00000000000d'::uuid,
        format('10000000-0000-0000-0000-%s',lpad(to_hex(p),12,'0'))::uuid,
        'member',ARRAY['membership'],1,1,now()+interval '1 day' FROM (VALUES (2),(5)) AS profiles(p);
SQL
receipt fixture space_over100_fixture_required.json TestAccountBanFixtureOver100Cohort34
"$RUNNER_TEMP/native-migrate-driver" -path ../migrations/space_db -database "$SPACE_BAN_TEST_DATABASE_URL" goto 44
"$RUNNER_TEMP/native-migrate-driver" -path ../migrations/space_db -database "$SPACE_BAN_TEST_DATABASE_URL" goto 45
receipt transition45 space_transition45_required.json TestAccountBanFixtureTrusted45Transition
"$RUNNER_TEMP/native-migrate-driver" -path ../migrations/space_db -database "$SPACE_BAN_TEST_DATABASE_URL" goto 50
receipt transition50 space_transition50_required.json TestAccountBanFixtureTrusted50Transition
receipt consumer space_over100_consumer_required.json TestAccountBanOver100Cohort50
