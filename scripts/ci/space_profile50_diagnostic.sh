#!/usr/bin/env bash
# Diagnostic only: separate legacy34 cohort, qualified through 45/50.
# This script owns the job's isolated PostgreSQL service and is its only
# administrative database creator/deleter while it runs. The OID check and DROP
# are separate commands; they rely on that exclusive-admin precondition.
# A hard kill can prevent the EXIT trap and artifact upload from running.
set -euo pipefail

query="SELECT COALESCE((SELECT oid FROM pg_database WHERE datname='space_profile50'),0)"
admin_system="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c 'SELECT system_identifier FROM pg_control_system()')"
[[ "$admin_system" =~ ^[0-9]+$ ]]
[[ "$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "$query")" == 0 ]]
owned_oid=""
recovery="$RUNNER_TEMP/space-profile50-cleanup.txt"
printf 'database=space_profile50\nstate=creating\n' > "$recovery"
cleanup() {
  local actual current_system
  if ! current_system="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c 'SELECT system_identifier FROM pg_control_system()')" || [[ "$current_system" != "$admin_system" ]]; then
    printf 'database=space_profile50\nstate=uncertain\nreason=cluster_identity_unverified\n' > "$recovery"
    return 1
  fi
  if ! actual="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "$query")"; then
    printf 'database=space_profile50\nstate=uncertain\nreason=inspection_failed\n' > "$recovery"
    return 1
  fi
  if [[ "$actual" == 0 ]]; then
    printf 'database=space_profile50\nstate=absent\n' > "$recovery"
    return 0
  fi
  if [[ -z "$owned_oid" || "$actual" != "$owned_oid" ]]; then
    printf 'database=space_profile50\nstate=uncertain\nreason=identity_unverified\nobserved_oid=%s\n' "$actual" > "$recovery"
    return 1
  fi
  if ! psql -X -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c 'DROP DATABASE space_profile50'; then
    printf 'database=space_profile50\nstate=uncertain\nreason=drop_failed\nowned_oid=%s\n' "$owned_oid" > "$recovery"
    return 1
  fi
  if ! current_system="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c 'SELECT system_identifier FROM pg_control_system()')" || [[ "$current_system" != "$admin_system" ]] || ! actual="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "$query")" || [[ "$actual" != 0 ]]; then
    printf 'database=space_profile50\nstate=uncertain\nreason=post_drop_unverified\nowned_oid=%s\n' "$owned_oid" > "$recovery"
    return 1
  fi
  printf 'database=space_profile50\nstate=removed\nowned_oid=%s\n' "$owned_oid" > "$recovery"
}
trap cleanup EXIT
psql -X -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c 'CREATE DATABASE space_profile50'
owned_oid="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "$query")"
[[ "$owned_oid" =~ ^[1-9][0-9]*$ ]]
printf 'database=space_profile50\nstate=owned\nowned_oid=%s\n' "$owned_oid" > "$recovery"
target_identity="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" -c "SELECT (SELECT system_identifier FROM pg_control_system())::text || ':' || oid::text FROM pg_database WHERE datname=current_database()")"
[[ "$admin_system" =~ ^[0-9]+$ && "$target_identity" == "$admin_system:$owned_oid" ]]
psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "SELECT 'postgres_server_version_num=' || current_setting('server_version_num')" >> "$RUNNER_TEMP/space-profile50-identities.txt"
psql -X -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "ALTER DATABASE space_profile50 SET track_functions = 'all'"
psql -X -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "ALTER DATABASE space_profile50 SET pg_stat_statements.track = 'all'"
psql -X -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" -c 'CREATE SCHEMA diagnostic_jit; CREATE EXTENSION pg_stat_statements WITH SCHEMA diagnostic_jit'

receipt() {
  local name="$1" inventory="$2" test_name="$3" result
  set +e
  go test -json -count=1 -timeout 5m -tags=spacebanintegration \
    -run "^${test_name}$" ./internal/store > "$RUNNER_TEMP/space-profile50-${name}-events.jsonl"
  result=$?
  set -e
  python3 -B ../../../scripts/ci/go_integration_matrix.py produce \
    --inventory "../../../scripts/ci/${inventory}" --service space \
    --candidate "$CANDIDATE_SHA" --run "$GITHUB_RUN_ID" --attempt "$GITHUB_RUN_ATTEMPT" \
    --events "$RUNNER_TEMP/space-profile50-${name}-events.jsonl" --exit-code "$result" \
    --output "$RUNNER_TEMP/space-profile50-${name}-receipt.json" --reject-skips
}

driver_before="$(sha256sum "$RUNNER_TEMP/native-migrate-driver" | cut -d' ' -f1)"
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
set +e
go test -c -p 2 -tags=spacebanintegration -o "$RUNNER_TEMP/space-profile50.test" ./internal/store
result=$?
set -e
if [[ "$result" != 0 ]]; then exit "$result"; fi
sha256sum "$RUNNER_TEMP/space-profile50.test" >> "$RUNNER_TEMP/space-profile50-identities.txt"
binary_before="$(sha256sum "$RUNNER_TEMP/space-profile50.test" | cut -d' ' -f1)"
# Verify the PostgreSQL 16 view really exposes all counters used below.
psql -XAt -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" -c "SELECT count(*), coalesce(sum(jit_functions),0), coalesce(sum(jit_generation_time),0), coalesce(sum(jit_inlining_count),0), coalesce(sum(jit_optimization_count),0) FROM diagnostic_jit.pg_stat_statements WHERE dbid=(SELECT oid FROM pg_database WHERE datname=current_database())" > "$RUNNER_TEMP/space-profile50-jit-schema.txt"
# Force one separate query through JIT to check that counters are operational.
jit_count_sql="SELECT coalesce(sum(jit_functions),0)::bigint FROM diagnostic_jit.pg_stat_statements WHERE dbid=(SELECT oid FROM pg_database WHERE datname=current_database())"
jit_canary_before="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" -c "$jit_count_sql")"
psql -XAt -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" -c "SET jit=on; SET jit_above_cost=0; SELECT sum(i) FROM generate_series(1,1000) i" > /dev/null
jit_canary_after="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" -c "$jit_count_sql")"
[[ "$jit_canary_before" =~ ^[0-9]+$ && "$jit_canary_after" =~ ^[0-9]+$ && "$jit_canary_after" -gt "$jit_canary_before" ]]
# Database-wide statement counters are diagnostic only and are never reset.
jit_snapshot() {
  local phase="$1"
  psql -XAt -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" -c "SELECT json_build_object('phase', '$phase', 'jit', current_setting('jit'), 'jit_above_cost', current_setting('jit_above_cost'), 'track', current_setting('pg_stat_statements.track'), 'jit_functions', coalesce(sum(jit_functions),0), 'jit_generation_time', coalesce(sum(jit_generation_time),0), 'jit_inlining_count', coalesce(sum(jit_inlining_count),0), 'jit_optimization_count', coalesce(sum(jit_optimization_count),0)) FROM diagnostic_jit.pg_stat_statements WHERE dbid=(SELECT oid FROM pg_database WHERE datname=current_database())" >> "$RUNNER_TEMP/space-profile50-jit.jsonl"
}
jit_result=0
jit_snapshot before || jit_result=1
set +e
go tool test2json -t -p voice/backend/space/internal/store "$RUNNER_TEMP/space-profile50.test" \
  -test.v -test.count=1 -test.timeout=5m -test.run '^TestNative50Over100ProfileCatalog$' > "$RUNNER_TEMP/space-profile50-diagnostic-events.jsonl"
result=$?
set -e
jit_snapshot after || jit_result=1
source_status=unavailable
head_status=unavailable
tracked_status=unavailable
untracked_status=unavailable
driver_status=unavailable
binary_status=unavailable
if post_head="$(git rev-parse HEAD)"; then
  head_status=mismatch
  if [[ "$post_head" == "$CANDIDATE_SHA" ]]; then head_status=verified; fi
fi
if post_tracked="$(git status --porcelain --untracked-files=no)"; then
  tracked_status=mismatch
  if [[ -z "$post_tracked" ]]; then tracked_status=verified; fi
fi
if post_untracked="$(git ls-files --others --exclude-standard)"; then
  untracked_status=mismatch
  if [[ -z "$post_untracked" ]]; then untracked_status=verified; fi
fi
if post_tree="$(git status --porcelain --untracked-files=all)" && [[ "$head_status" != unavailable ]]; then
  source_status=mismatch
  if [[ "$head_status" == verified && -z "$post_tree" ]]; then source_status=verified; fi
fi
if post_driver="$(sha256sum "$RUNNER_TEMP/native-migrate-driver" | cut -d' ' -f1)" && [[ "$post_driver" =~ ^[0-9a-f]{64}$ ]]; then
  driver_status=mismatch
  if [[ "$post_driver" == "$driver_before" ]]; then driver_status=verified; fi
fi
if post_binary="$(sha256sum "$RUNNER_TEMP/space-profile50.test" | cut -d' ' -f1)" && [[ "$post_binary" =~ ^[0-9a-f]{64}$ ]]; then
  binary_status=mismatch
  if [[ "$post_binary" == "$binary_before" ]]; then binary_status=verified; fi
fi
printf 'profile_exit_code=%s\npost_run_source=%s\npost_run_head=%s\npost_run_tracked=%s\npost_run_untracked=%s\npost_run_driver=%s\npost_run_binary=%s\nbackend_inventory=endpoint_only\n' \
  "$result" "$source_status" "$head_status" "$tracked_status" "$untracked_status" "$driver_status" "$binary_status" >> "$RUNNER_TEMP/space-profile50-identities.txt"
identity_result=1
if [[ "$source_status" == verified && "$head_status" == verified && "$tracked_status" == verified && "$untracked_status" == verified && "$driver_status" == verified && "$binary_status" == verified ]]; then
  identity_result=0
  printf 'post_run_source_and_binaries=verified\n' >> "$RUNNER_TEMP/space-profile50-identities.txt"
fi
validation_result=0
python3 -B ../../../scripts/ci/space_profile50_validate.py "$RUNNER_TEMP/space-profile50-diagnostic-events.jsonl" "$result" || validation_result=$?
printf 'profile_validation_exit_code=%s\n' "$validation_result" >> "$RUNNER_TEMP/space-profile50-identities.txt"
if [[ "$result" != 0 ]]; then exit "$result"; fi
if [[ "$identity_result" != 0 ]]; then exit "$identity_result"; fi
if [[ "$jit_result" != 0 ]]; then exit "$jit_result"; fi
exit "$validation_result"
