#!/usr/bin/env bash
# Historical fixture evidence only; this does not execute an account Ban.
set -euo pipefail
root=../../../scripts/ci
recovery="$RUNNER_TEMP/space-large34-cleanup.txt"
events="$RUNNER_TEMP/space-large34-events.jsonl"
receipt="$RUNNER_TEMP/space-large34-receipt.json"
inventory="$root/space_large34_required.json"
query="SELECT COALESCE((SELECT oid FROM pg_database WHERE datname='space_large34'),0)"
cluster="SELECT system_identifier::text || '|' || inet_server_addr()::text || '|' || inet_server_port()::text FROM pg_control_system()"
database="SELECT current_database() || '|' || oid::text FROM pg_database WHERE datname=current_database()"
owned_oid=''
bound=false
state() { printf 'database=space_large34\nstate=%s\n%s' "$1" "${2:-}" > "$recovery"; }
python3 "$root/space_large34_source.py" --candidate "$CANDIDATE_SHA"
[[ "${SPACE_BAN_TEST_DATABASE_URL%%\?*}" == */space_large34 ]]
[[ "$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "$query" 2>/dev/null)" == 0 ]]
admin_identity="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "$cluster" 2>/dev/null)"
[[ -n "$admin_identity" ]]
state creating
cleanup() {
  local actual current peer target
  if ! actual="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "$query" 2>/dev/null)"; then
    state uncertain $'reason=inspection_failed\n'; return 1
  fi
  if [[ -z "$owned_oid" || "$actual" != "$owned_oid" ]]; then
    state uncertain $'reason=identity_unverified\n'; return 1
  fi
  if ! current="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "$cluster" 2>/dev/null)" || [[ "$current" != "$admin_identity" ]]; then
    state uncertain $'reason=identity_unverified\n'; return 1
  fi
  if [[ "$bound" == true ]]; then
    if ! peer="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" -c "$cluster" 2>/dev/null)" || [[ "$peer" != "$admin_identity" ]] ||
       ! target="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" -c "$database" 2>/dev/null)" || [[ "$target" != "space_large34|$owned_oid" ]]; then
      state uncertain $'reason=identity_unverified\n'; return 1
    fi
  fi
  if ! psql -X -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c 'DROP DATABASE space_large34' >/dev/null 2>&1; then
    state uncertain $'reason=drop_failed\n'; return 1
  fi
  if ! actual="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "$query" 2>/dev/null)" || [[ "$actual" != 0 ]]; then
    state uncertain $'reason=post_drop_inspection_failed\n'; return 1
  fi
  state removed
}
trap cleanup EXIT
psql -X -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c 'CREATE DATABASE space_large34' >/dev/null 2>&1
owned_oid="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_NATIVE_ADMIN_URL" -c "$query" 2>/dev/null)"
[[ "$owned_oid" =~ ^[1-9][0-9]*$ ]]
state owned
peer="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" -c "$cluster" 2>/dev/null)"
target="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" -c "$database" 2>/dev/null)"
[[ "$peer" == "$admin_identity" && "$target" == "space_large34|$owned_oid" ]]
bound=true
"$RUNNER_TEMP/native-migrate-driver" -path ../migrations/space_db -database "$SPACE_BAN_TEST_DATABASE_URL" goto 34 >/dev/null 2>&1
version="$(psql -XAt -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" -c "SELECT CASE WHEN count(*) = 1 AND bool_and(version=34 AND NOT dirty) THEN '34' ELSE 'invalid' END FROM schema_migrations" 2>/dev/null)"
[[ "$version" == 34 ]]
psql -X -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" >/dev/null 2>&1 <<'SQL'
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
set +e
go test -json -count=1 -timeout 5m -tags=spacebanintegration -run '^TestAccountBanFixtureLargeCohort34$' ./internal/store > "$events"
result=$?
set -e
python3 "$root/go_integration_matrix.py" produce --inventory "$inventory" --service space --candidate "$CANDIDATE_SHA" --run "$GITHUB_RUN_ID" --attempt "$GITHUB_RUN_ATTEMPT" --events "$events" --exit-code "$result" --output "$receipt" --reject-skips
trap - EXIT
cleanup
python3 "$root/space_large34_source.py" --candidate "$CANDIDATE_SHA"
python3 "$root/go_integration_matrix.py" verify --inventory "$inventory" --services '["space"]' --candidate "$CANDIDATE_SHA" --run "$GITHUB_RUN_ID" --attempt "$GITHUB_RUN_ATTEMPT" --receipt "$receipt" --events "$events" --require-no-skips
export SPACE_LARGE34_OWNED_OID="$owned_oid"
python3 "$root/space_large34_manifest.py"
