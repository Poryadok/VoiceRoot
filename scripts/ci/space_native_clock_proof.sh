#!/usr/bin/env bash
# Run clock-sensitive Space proofs in a private PostgreSQL container.
set -euo pipefail
: "${RUNNER_TEMP:?}" "${GITHUB_RUN_ID:?}" "${GITHUB_RUN_ATTEMPT:?}" "${CANDIDATE_SHA:?}"
[[ "$GITHUB_RUN_ID" =~ ^[0-9]+$ && "$GITHUB_RUN_ATTEMPT" =~ ^[0-9]+$ && "$CANDIDATE_SHA" =~ ^[0-9a-f]{40}$ ]]
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root/src/backend/space"
clock_dir="$RUNNER_TEMP/space-clock-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"
container="voice-ci-space-clock-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"
compiler="${container}-compiler"
owner="${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"
mkdir "$clock_dir"
chmod 777 "$clock_dir"
cleanup() {
  local result=$? id label remaining name
  trap - EXIT
  for name in "$container" "$compiler"; do
    if ! id="$(docker container ls -a --no-trunc --filter "name=^/${name}$" --format '{{.ID}}')"; then
      echo 'clock fixture cleanup inventory failed' >&2
      result=1
      continue
    fi
    if [[ -n "$id" ]]; then
      if [[ ! "$id" =~ ^[0-9a-f]{64}$ ]]; then result=1; continue; fi
      label="$(docker container inspect "$id" --format '{{index .Config.Labels "voice.ci.fixture"}}')" || result=1
      if [[ "$label" != "$owner" ]] || ! docker container rm -f "$id" >/dev/null; then result=1; fi
      if ! remaining="$(docker container ls -a --no-trunc --filter "id=$id" --format '{{.ID}}')" || [[ -n "$remaining" ]]; then result=1; fi
    fi
  done
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
for name in "$container" "$compiler"; do
  existing="$(docker container ls -a --no-trunc --filter "name=^/${name}$" --format '{{.ID}}')"
  if [[ -n "$existing" ]]; then
    echo 'clock fixture name already exists' >&2
    trap - EXIT
    exit 1
  fi
done
docker create --name "$compiler" --network none --label "voice.ci.fixture=$owner" \
  --mount "type=bind,source=$clock_dir,target=/fixture-clock" \
  --mount "type=bind,source=$root/src/backend/space/internal/store/testdata/account_ban_wall_clock.c,target=/fixture-clock.c,readonly" \
  golang@sha256:23ce71337b726608afb1bc00986e630a05034a87a87ea1fc2cd1ae5cf33387eb \
  sh -ec 'cc -shared -fPIC -O2 -o /fixture-clock/fixture-clock.so /fixture-clock.c -ldl; chmod 755 /fixture-clock/fixture-clock.so' >/dev/null
docker start -a "$compiler"
clock_sha="$(sha256sum "$clock_dir/fixture-clock.so" | cut -d' ' -f1)"
printf '%s\n' 0 > "$clock_dir/walltime-us"
chmod 666 "$clock_dir/walltime-us"
docker create --name "$container" --label "voice.ci.fixture=$owner" \
  --mount "type=bind,source=$clock_dir,target=/fixture-clock" \
  -e LD_PRELOAD=/fixture-clock/fixture-clock.so -e POSTGRES_USER=voice \
  -e POSTGRES_PASSWORD=voice-clock-fixture -e POSTGRES_DB=postgres \
  -p 127.0.0.1::5432 postgres@sha256:efedf3595f1d6f415c08568ba171029bf54052e754cc9f030e3f2412b21f3d67 >/dev/null
docker start "$container" >/dev/null
ready=false
for _ in $(seq 1 90); do
  if docker exec "$container" pg_isready -U voice -d postgres >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
if [[ "$ready" != true ]]; then
  docker logs "$container" >&2 || true
  echo 'private PostgreSQL clock fixture did not become ready' >&2
  exit 1
fi
port="$(docker inspect "$container" --format '{{(index (index .NetworkSettings.Ports "5432/tcp") 0).HostPort}}')"
[[ "$port" =~ ^[0-9]+$ ]]
admin="postgres://voice:voice-clock-fixture@127.0.0.1:${port}/postgres?sslmode=disable"
export SPACE_BAN_TEST_DATABASE_URL="postgres://voice:voice-clock-fixture@127.0.0.1:${port}/space_db?sslmode=disable"
export CGO_ENABLED=0
psql -X -v ON_ERROR_STOP=1 "$admin" -c 'CREATE DATABASE space_db'
oid="$(psql -XAt -v ON_ERROR_STOP=1 "$admin" -c "SELECT oid FROM pg_database WHERE datname='space_db'")"
[[ "$oid" =~ ^[1-9][0-9]*$ ]]
migrate() { "$RUNNER_TEMP/native-migrate-driver" -path ../migrations/space_db -database "$SPACE_BAN_TEST_DATABASE_URL" goto "$1"; }
receipt() {
  local name="$1" inventory="$2" expression="$3" result
  set +e
  go test -json -count=1 -timeout 5m -tags=spacebanintegration -run "$expression" ./internal/store > "$RUNNER_TEMP/space-clock-${name}-events.jsonl"
  result=$?
  set -e
  python3 ../../../scripts/ci/go_integration_matrix.py produce \
    --inventory "../../../scripts/ci/$inventory" --service space --candidate "$CANDIDATE_SHA" \
    --run "$GITHUB_RUN_ID" --attempt "$GITHUB_RUN_ATTEMPT" \
    --events "$RUNNER_TEMP/space-clock-${name}-events.jsonl" --exit-code "$result" \
    --output "$RUNNER_TEMP/space-clock-${name}-receipt.json" --reject-skips
}
migrate 34
psql -X -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" <<'SQL'
INSERT INTO spaces(id,name,owner_profile_id,member_count) VALUES
  ('30000000-0000-0000-0000-000000000013','native observer fixture','10000000-0000-0000-0000-000000000001',3);
INSERT INTO space_members(space_id,profile_id) VALUES
  ('30000000-0000-0000-0000-000000000013','10000000-0000-0000-0000-000000000001'),
  ('30000000-0000-0000-0000-000000000013','10000000-0000-0000-0000-000000000003');
INSERT INTO community_owner_authority(space_id,application_id,environment_id,corporation_key,owner_account_id,owner_profile_id,owner_generation,status,roster_source_revision,roster_sha256,roster_lease_expires_at) VALUES
  ('30000000-0000-0000-0000-000000000013','40000000-0000-0000-0000-000000000001','40000000-0000-0000-0000-000000000002','fixture','20000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001',1,'active',1,decode(repeat('aa',32),'hex'),now()+interval '1 day');
INSERT INTO community_roster_members(space_id,profile_id,source_rank,source_reasons,source_revision,owner_generation,lease_expires_at) VALUES
  ('30000000-0000-0000-0000-000000000013','10000000-0000-0000-0000-000000000005','member',ARRAY['membership'],1,1,now()+interval '1 day');
INSERT INTO spaces(id,name,owner_profile_id,member_count) VALUES
  ('30000000-0000-0000-0000-000000000008','roster fixture','10000000-0000-0000-0000-000000000001',5);
INSERT INTO space_members(space_id,profile_id)
  SELECT '30000000-0000-0000-0000-000000000008'::uuid,
         format('10000000-0000-0000-0000-%s',lpad(to_hex(p),12,'0'))::uuid
  FROM (VALUES (1),(2),(3),(6)) AS profiles(p);
INSERT INTO community_owner_authority(space_id,application_id,environment_id,corporation_key,owner_account_id,owner_profile_id,owner_generation,status,roster_source_revision,roster_sha256,roster_lease_expires_at) VALUES
  ('30000000-0000-0000-0000-000000000008','40000000-0000-0000-0000-000000000001','40000000-0000-0000-0000-000000000002','native7-fixture-8','20000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001',1,'active',1,decode(repeat('aa',32),'hex'),now()+interval '1 day');
INSERT INTO community_roster_members(space_id,profile_id,source_rank,source_reasons,source_revision,owner_generation,lease_expires_at)
  SELECT '30000000-0000-0000-0000-000000000008'::uuid,
         format('10000000-0000-0000-0000-%s',lpad(to_hex(p),12,'0'))::uuid,
         'member',ARRAY['membership'],1,1,now()+interval '1 day' FROM (VALUES (2),(5)) AS profiles(p);
SQL
migrate 44
migrate 45
receipt transition45 space_transition45_required.json '^TestAccountBanFixtureTrusted45Transition$'
migrate 50
receipt transition50 space_transition50_required.json '^TestAccountBanFixtureTrusted50Transition$'
receipt enrollment space_seeded50_drift_required.json '^TestNativeCatalogConcurrentRowDrift$'
receipt observer space_clock50_observer_required.json '^(TestNativeObserverOtherProfileCommunityGap|TestNativeObserverNoopLeaseGapTuple)$'
receipt roster-enrollment space_roster_enrollment_required.json '^TestAccountBanFixtureRosterEnrollment50$'
receipt roster space_gis50_required.json '^TestAccountBanGIS50$'
receipt roster-expiry space_gis_expiry_required.json '^TestAccountBanGISExpiry50$'
[[ "$(sha256sum "$clock_dir/fixture-clock.so" | cut -d' ' -f1)" == "$clock_sha" ]]
[[ "$(tr -d '\r\n' < "$clock_dir/walltime-us")" == 0 ]]
[[ "$(psql -XAt -v ON_ERROR_STOP=1 "$admin" -c "SELECT oid FROM pg_database WHERE datname='space_db'")" == "$oid" ]]
psql -X -v ON_ERROR_STOP=1 "$admin" -c 'DROP DATABASE space_db'

# The native ban scenario mutates several spaces and intentionally leaves a
# CAPTURING operation. Give its ordered ledger/expiry proofs a fresh database
# so they cannot alter the observer fixture or depend on its prior state.
psql -X -v ON_ERROR_STOP=1 "$admin" -c 'CREATE DATABASE space_ban_db'
ban_oid="$(psql -XAt -v ON_ERROR_STOP=1 "$admin" -c "SELECT oid FROM pg_database WHERE datname='space_ban_db'")"
[[ "$ban_oid" =~ ^[1-9][0-9]*$ ]]
export SPACE_BAN_TEST_DATABASE_URL="postgres://voice:voice-clock-fixture@127.0.0.1:${port}/space_ban_db?sslmode=disable"
migrate 34
psql -X -v ON_ERROR_STOP=1 "$SPACE_BAN_TEST_DATABASE_URL" <<'SQL'
INSERT INTO spaces(id,name,owner_profile_id,member_count,visibility)
 SELECT format('30000000-0000-0000-0000-%s',lpad(to_hex(n),12,'0'))::uuid,
        'account-ban-fixture','10000000-0000-0000-0000-000000000001',5,'public'
 FROM generate_series(1,24) n;
INSERT INTO space_members(space_id,profile_id)
 SELECT format('30000000-0000-0000-0000-%s',lpad(to_hex(n),12,'0'))::uuid,
        format('10000000-0000-0000-0000-%s',lpad(to_hex(p),12,'0'))::uuid
 FROM generate_series(1,24) n CROSS JOIN (VALUES (1),(2),(3),(6)) AS profiles(p);
INSERT INTO community_owner_authority(space_id,application_id,environment_id,corporation_key,owner_account_id,owner_profile_id,owner_generation,status,roster_source_revision,roster_sha256,roster_lease_expires_at)
 SELECT format('30000000-0000-0000-0000-%s',lpad(to_hex(n),12,'0'))::uuid,
        '40000000-0000-0000-0000-000000000001','40000000-0000-0000-0000-000000000002',
        'account-ban-fixture-' || n,'20000000-0000-0000-0000-000000000001',
        '10000000-0000-0000-0000-000000000001',1,'active',1,decode(repeat('aa',32),'hex'),now()+interval '1 day'
 FROM generate_series(1,24) n;
INSERT INTO community_roster_members(space_id,profile_id,source_rank,source_reasons,source_revision,owner_generation,lease_expires_at)
 SELECT format('30000000-0000-0000-0000-%s',lpad(to_hex(n),12,'0'))::uuid,
        format('10000000-0000-0000-0000-%s',lpad(to_hex(p),12,'0'))::uuid,
        'member',ARRAY['membership'],1,1,now()+interval '1 day'
 FROM generate_series(1,24) n CROSS JOIN (VALUES (2),(5)) AS profiles(p);
INSERT INTO space_bans(space_id,account_id,banned_by_profile_id)
 VALUES ('30000000-0000-0000-0000-000000000005','20000000-0000-0000-0000-000000000002',
         '10000000-0000-0000-0000-000000000001');
SQL
migrate 44
migrate 45
receipt account-ban-transition45 space_transition45_required.json '^TestAccountBanFixtureTrusted45Transition$'
migrate 50
receipt account-ban-transition50 space_transition50_required.json '^TestAccountBanFixtureTrusted50Transition$'
receipt account-ban-native space_account_ban_native50_required.json '^TestAccountBanNative50$'
receipt account-ban-ledger space_account_ban_ledger50_required.json '^TestAccountBanLedgerGuards50$'
receipt account-ban-expiry space_account_ban_expiry_required.json '^TestAccountBanFixtureClockExpiry$'
[[ "$(sha256sum "$clock_dir/fixture-clock.so" | cut -d' ' -f1)" == "$clock_sha" ]]
[[ "$(tr -d '\r\n' < "$clock_dir/walltime-us")" == 0 ]]
[[ "$(psql -XAt -v ON_ERROR_STOP=1 "$admin" -c "SELECT oid FROM pg_database WHERE datname='space_ban_db'")" == "$ban_oid" ]]
psql -X -v ON_ERROR_STOP=1 "$admin" -c 'DROP DATABASE space_ban_db'
