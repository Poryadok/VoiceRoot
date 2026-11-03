#!/bin/sh
# Disposable PostgreSQL fixture only; never invoke against a real database.
set -eu
fixture=${GIS_REV32_FIXTURE:?set GIS_REV32_FIXTURE to the copied fixture directory}
export PGUSER=postgres PGDATABASE=game_integration_db GAME_INTEGRATION_DB_PASSWORD=fixture-password
# Exercise the shipped32 clock before forward33 can change any schema or data.
# This is an intermediate migration proof, not runtime admission at revision32.
check_runtime_authority() {
  test "$(psql -X -qAt -v ON_ERROR_STOP=1 -c 'SELECT version::text || chr(124) || dirty::text FROM schema_migrations')" = "$1|false"
  before=$(psql -X -qAt -v ON_ERROR_STOP=1 -c 'SELECT revision FROM gis_authority_revision WHERE singleton')
  psql -X -q -v ON_ERROR_STOP=1 <<'SQL'
BEGIN;
SET ROLE gameintegration_runtime;
DO $$ DECLARE previous bigint; actual bigint; BEGIN
 SELECT revision INTO STRICT previous FROM gis_authority_revision WHERE singleton;
 INSERT INTO applications(id,owner_account_id,name,status) VALUES
  ('11111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222','fixture','draft');
 SELECT revision INTO STRICT actual FROM gis_authority_revision WHERE singleton;
 IF actual IS DISTINCT FROM previous+1 THEN RAISE EXCEPTION 'runtime insert did not advance authority'; END IF;
 previous := actual;
 UPDATE applications SET name='updated fixture' WHERE id='11111111-1111-1111-1111-111111111111';
 SELECT revision INTO STRICT actual FROM gis_authority_revision WHERE singleton;
 IF actual IS DISTINCT FROM previous+1 THEN RAISE EXCEPTION 'runtime update did not advance authority'; END IF;
 previous := actual;
 DELETE FROM applications WHERE id='11111111-1111-1111-1111-111111111111';
 SELECT revision INTO STRICT actual FROM gis_authority_revision WHERE singleton;
 IF actual IS DISTINCT FROM previous+1 THEN RAISE EXCEPTION 'runtime delete did not advance authority'; END IF;
END $$;
ROLLBACK;
SQL
  test "$(psql -X -qAt -v ON_ERROR_STOP=1 -c 'SELECT revision FROM gis_authority_revision WHERE singleton')" = "$before"
  for mutation in 'DELETE FROM gis_authority_revision' \
      'UPDATE gis_authority_revision SET revision=revision-1' \
      'TRUNCATE gis_authority_revision' 'TRUNCATE applications'; do
    if printf 'BEGIN; SET ROLE gameintegration_runtime; %s; ROLLBACK;\n' "$mutation" |
         psql -X -q -v ON_ERROR_STOP=1 >/dev/null 2>&1; then
      printf '%s\n' 'Runtime changed or truncated the authority floor' >&2
      exit 1
    fi
  done
  printf 'GIS revision %s runtime authority fixture PASS\n' "$1"
}
psql -X -q -d postgres -v ON_ERROR_STOP=1 -c 'CREATE DATABASE game_integration_db'
psql -X -q -v ON_ERROR_STOP=1 -f "$fixture/provision.sql" >/dev/null
for file in "$fixture"/migrations/*.up.sql; do
  case "$file" in *000032_*|*000033_*) continue;; esac
  psql -X -q -v ON_ERROR_STOP=1 -f "$file" >/dev/null
done
psql -X -q -v ON_ERROR_STOP=1 -c 'CREATE TABLE schema_migrations(version bigint PRIMARY KEY, dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(31,false)'
psql -X -q -v ON_ERROR_STOP=1 -f "$fixture/migrations/000032_authority_source_revision.up.sql" >/dev/null
psql -X -q -v ON_ERROR_STOP=1 -c 'UPDATE schema_migrations SET version=32,dirty=false'
check_runtime_authority 32
psql -X -q -v ON_ERROR_STOP=1 -f "$fixture/migrations/000033_restore_session_community_schema.up.sql" >/dev/null
psql -X -q -v ON_ERROR_STOP=1 -c 'UPDATE schema_migrations SET version=33,dirty=false'
psql -X -qAt -v ON_ERROR_STOP=1 -f "$fixture/verify.sql" > "$fixture/result.json"
grep -q 'voice-gis-verifier-v3' "$fixture/result.json"
for field in authority_singleton_verified authority_triggers_verified authority_functions_verified; do
  grep -Eq '"'"$field"'"[[:space:]]*:[[:space:]]*true' "$fixture/result.json"
done
expect_refusal() {
  if printf 'BEGIN;\n%s\n\\i %s/verify.sql\nROLLBACK;\n' "$1" "$fixture" |
       psql -X -q -v ON_ERROR_STOP=1 >/dev/null 2>&1; then
    printf '%s\n' 'Expected verifier refusal did not occur' >&2
    exit 1
  fi
}
expect_refusal 'UPDATE schema_migrations SET version=32;'
expect_refusal 'ALTER TABLE applications DISABLE TRIGGER gis_authority_changed;'
expect_refusal 'ALTER TABLE gis_authority_revision DISABLE TRIGGER gis_authority_floor_change; DELETE FROM gis_authority_revision;'
expect_refusal 'CREATE OR REPLACE FUNCTION gis_authority_changed() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$;'
expect_refusal 'ALTER TABLE environments DISABLE TRIGGER gis_authority_no_truncate;'
expect_refusal 'ALTER TABLE applications ENABLE ROW LEVEL SECURITY;'
expect_refusal 'ALTER TABLE gis_authority_revision DROP CONSTRAINT gis_authority_revision_pkey;'
expect_refusal 'ALTER FUNCTION gis_authority_changed() SECURITY DEFINER;'
expect_refusal 'CREATE OR REPLACE FUNCTION revoke_credentials_on_application_lifecycle_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$;'
psql -X -q -v ON_ERROR_STOP=1 <<'SQL'
BEGIN;
CREATE FUNCTION public.fixture_suppress_floor() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN OLD; END $$;
CREATE TRIGGER z_suppress_floor BEFORE UPDATE ON gis_authority_revision FOR EACH ROW EXECUTE FUNCTION public.fixture_suppress_floor();
SET ROLE gameintegration_runtime;
DO $$ DECLARE previous bigint; actual bigint; BEGIN
 SELECT revision INTO previous FROM gis_authority_revision WHERE singleton;
 INSERT INTO applications(id,owner_account_id,name,status) VALUES
  ('33333333-3333-3333-3333-333333333333','44444444-4444-4444-4444-444444444444','suppressed','draft');
 SELECT revision INTO actual FROM gis_authority_revision WHERE singleton;
 IF actual<>previous THEN RAISE EXCEPTION 'fixture did not suppress authority revision'; END IF;
END $$;
ROLLBACK;
SQL
expect_refusal 'CREATE FUNCTION public.fixture_suppress_floor() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN OLD; END $$; CREATE TRIGGER z_suppress_floor BEFORE UPDATE ON gis_authority_revision FOR EACH ROW EXECUTE FUNCTION public.fixture_suppress_floor();'
check_runtime_authority 33
printf '%s\n' 'GIS revision 33 verifier fixture PASS'
