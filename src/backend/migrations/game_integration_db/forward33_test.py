"""Private real-driver GIS33 proof. Run with Python and Docker; no shared DB.

python forward33_test.py --migrations <complete candidate directory>
The directory must contain the reconciled15–30 restoration and forward33.
"""
import argparse
import pathlib
import re
import shutil
import subprocess
import tempfile
import unittest
import uuid


def command(*args, input=None, ok=True):
    result = subprocess.run(args, input=input, text=True, encoding="utf-8",
                            capture_output=True, timeout=90)
    if ok and result.returncode:
        raise AssertionError(result.stderr[:2500] + result.stderr[-700:] + result.stdout[-700:])
    return result


class Forward33(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.container = "gis-forward33-" + uuid.uuid4().hex[:12]
        cls.temp = tempfile.TemporaryDirectory(prefix="gis-forward33-")
        cls.root = pathlib.Path(cls.temp.name)
        cls.complete = cls.root / "complete"
        cls.legacy = cls.root / "legacy"
        cls.complete.mkdir()
        cls.legacy.mkdir()
        for path in OPTIONS.migrations.glob("*.sql"):
            shutil.copyfile(path, cls.complete / path.name)
            if int(path.name[:6]) <= 14 or int(path.name[:6]) in (31, 32):
                shutil.copyfile(path, cls.legacy / path.name)
        command("docker", "run", "-d", "--name", cls.container,
                "--memory", "512m", "--cpus", "1",
                "-e", "POSTGRES_PASSWORD=private_fixture", OPTIONS.postgres_image)
        try:
            command("docker", "exec", cls.container, "sh", "-c",
                    "for n in $(seq 1 60); do pg_isready -h 127.0.0.1 -U postgres && exit 0; sleep 1; done; exit 1")
            cls.sql("postgres", "CREATE ROLE gameintegration_runtime NOLOGIN")
            cls.sql("postgres", "CREATE DATABASE game_integration_db")
        except BaseException:
            cls.tearDownClass()
            raise

    @classmethod
    def tearDownClass(cls):
        command("docker", "rm", "-f", cls.container)
        cls.temp.cleanup()

    @classmethod
    def sql(cls, database, sql, ok=True):
        return command("docker", "exec", "-i", cls.container, "psql", "-X", "-qAt",
                       "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", database,
                       input=sql, ok=ok)

    def database(self, legacy=False):
        database = "proof_" + uuid.uuid4().hex
        self.sql("postgres", "CREATE DATABASE " + database)
        self.migrate(database, self.legacy if legacy else self.complete, "goto", "32")
        self.assertEqual("32|f", self.sql(database, "SELECT version,dirty FROM schema_migrations").stdout.strip())
        return database

    def migrate(self, database, source, *args, ok=True):
        result = command("docker", "run", "--rm", "--network", "container:" + self.container,
                         "-v", str(source.resolve()) + ":/migrations:ro",
                         "migrate/migrate:v4.18.1", "-path", "/migrations", "-database",
                         "postgres://postgres:private_fixture@127.0.0.1:5432/" + database + "?sslmode=disable",
                         *args, ok=ok)
        if not ok:
            self.assertNotEqual(0, result.returncode, "migration must HOLD")
        return result

    def schema(self, database):
        result = command("docker", "exec", self.container, "pg_dump", "-U", "postgres",
                         "--schema-only", "--no-owner", "--no-privileges", database).stdout
        # PostgreSQL18 randomizes psql restrict tokens; they are not schema.
        return re.sub(r"^\\(?:un)?restrict .*\n", "", result, flags=re.M)

    def seed(self, database, status="closed"):
        self.sql(database, """
INSERT INTO applications(id,owner_account_id,name,status) VALUES
 ('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000002','retained','sandbox');
INSERT INTO environments(id,application_id,kind,status) VALUES
 ('00000000-0000-4000-8000-000000000003','00000000-0000-4000-8000-000000000001','sandbox','active');
INSERT INTO gis_sessions(id,application_id,environment_id,kind,external_key,session_status,stage,roster_revision,roster_complete,members)
 VALUES('00000000-0000-4000-8000-000000000004','00000000-0000-4000-8000-000000000001',
 '00000000-0000-4000-8000-000000000003','match','retained-session','""" + status + """','closed',1,true,'{}');
INSERT INTO gis_session_operations(operation_id,session_id,application_id,environment_id,operation_kind,request_hash,status,stage,error_code)
 VALUES('00000000-0000-4000-8000-000000000005','00000000-0000-4000-8000-000000000004',
 '00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000003','close',decode(repeat('ab',32),'hex'),'succeeded','closed','retained_error');
INSERT INTO gis_session_owner_receipts(operation_id,application_id,environment_id,stage,owner_operation_id,owner_request_hash,resource_id,receipt_id,receipt_bytes)
 VALUES('00000000-0000-4000-8000-000000000005','00000000-0000-4000-8000-000000000001',
 '00000000-0000-4000-8000-000000000003','close','00000000-0000-4000-8000-000000000006',decode(repeat('cd',32),'hex'),
 '00000000-0000-4000-8000-000000000007','00000000-0000-4000-8000-000000000008',decode('001122ff','hex'));
""")

    def retained(self, database):
        return self.sql(database, """
SELECT jsonb_build_object('apps',(SELECT jsonb_agg(to_jsonb(a)) FROM applications a),
 'envs',(SELECT jsonb_agg(to_jsonb(e)) FROM environments e),
 'receipts',(SELECT jsonb_agg(to_jsonb(r)) FROM gis_session_owner_receipts r),
 'sessions',(SELECT jsonb_agg(to_jsonb(s)-ARRAY['host_profile_id','roster_hash','lease_expires_at','roster_receipt_id','closed_at','terminalized_at','history_expires_at']) FROM gis_sessions s),
 'operations',(SELECT jsonb_agg(to_jsonb(o)-ARRAY['terminalized_at','error_outcome']) FROM gis_session_operations o));
""").stdout.strip()

    def test_fresh_and_real_legacy_rows_reach_same_schema(self):
        fresh, legacy = self.database(), self.database(legacy=True)
        self.seed(legacy)
        rows = self.retained(legacy)
        floor = int(self.sql(legacy, "SELECT revision FROM gis_authority_revision").stdout)
        self.migrate(fresh, self.complete, "up")
        self.migrate(legacy, self.complete, "up")
        self.assertEqual("33|f", self.sql(legacy, "SELECT version,dirty FROM schema_migrations").stdout.strip())
        self.assertEqual(self.schema(fresh), self.schema(legacy))
        self.assertEqual(rows, self.retained(legacy))
        self.assertGreaterEqual(int(self.sql(legacy, "SELECT revision FROM gis_authority_revision").stdout), floor)
        self.assertEqual("t", self.sql(legacy, "SELECT terminalized_at=updated_at AND history_expires_at=updated_at+interval '30 days' AND host_profile_id IS NULL AND roster_hash IS NULL AND lease_expires_at IS NULL FROM gis_sessions").stdout.strip())
        self.assertEqual("unknown", self.sql(legacy, "SELECT error_outcome FROM gis_session_operations WHERE error_code='retained_error'").stdout.strip())

    def test_unknown_partial_and_tampered_layouts_hold_atomically(self):
        for alteration in ["CREATE TABLE unexpected(id int)",
                           "ALTER TABLE gis_sessions ADD COLUMN host_profile_id uuid",
                           "ALTER TABLE applications ALTER COLUMN name DROP NOT NULL",
                           "GRANT TRUNCATE ON applications TO gameintegration_runtime",
                           "GRANT REFERENCES(name) ON applications TO gameintegration_runtime",
                           "ALTER TABLE applications ENABLE ROW LEVEL SECURITY",
                           "ALTER TABLE gis_sessions DISABLE TRIGGER gis_authority_changed",
                           "CREATE OR REPLACE FUNCTION gis_authority_changed() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NULL; END'"]:
            with self.subTest(alteration=alteration):
                database = self.database(legacy=True)
                self.seed(database)
                self.sql(database, alteration)
                before, rows = self.schema(database), self.retained(database)
                self.migrate(database, self.complete, "up", ok=False)
                self.assertEqual(before, self.schema(database))
                self.assertEqual(rows, self.retained(database))
                self.assertEqual("33|t", self.sql(database, "SELECT version,dirty FROM schema_migrations").stdout.strip())

    def test_dirty32_holds_before_body(self):
        database = self.database(legacy=True)
        self.sql(database, "UPDATE schema_migrations SET dirty=true")
        before = self.schema(database)
        self.migrate(database, self.complete, "up", ok=False)
        self.assertEqual(before, self.schema(database))
        self.assertEqual("32|t", self.sql(database, "SELECT version,dirty FROM schema_migrations").stdout.strip())

    def test_schema_owner_and_acl_changes_hold_atomically(self):
        for legacy in (True, False):
            for alteration in ["GRANT CREATE ON SCHEMA public TO gameintegration_runtime",
                               "ALTER SCHEMA public OWNER TO gameintegration_runtime",
                               "GRANT USAGE ON SCHEMA public TO gameintegration_runtime WITH GRANT OPTION",
                               "GRANT CREATE ON SCHEMA public TO PUBLIC",
                               "REVOKE USAGE ON SCHEMA public FROM PUBLIC"]:
                with self.subTest(legacy=legacy, alteration=alteration):
                    database = self.database(legacy=legacy)
                    self.seed(database)
                    self.sql(database, alteration)
                    before, rows = self.schema(database), self.retained(database)
                    acl = self.sql(database, "SELECT nspowner::regrole::text,nspacl::text FROM pg_namespace WHERE nspname='public'").stdout
                    self.migrate(database, self.complete, "up", ok=False)
                    self.assertEqual(before, self.schema(database))
                    self.assertEqual(rows, self.retained(database))
                    self.assertEqual(acl, self.sql(database, "SELECT nspowner::regrole::text,nspacl::text FROM pg_namespace WHERE nspname='public'").stdout)
                    self.assertEqual("33|t", self.sql(database, "SELECT version,dirty FROM schema_migrations").stdout.strip())

    def test_missing_floor_holds_without_reset(self):
        database = self.database(legacy=True)
        self.sql(database, "ALTER TABLE gis_authority_revision DISABLE TRIGGER gis_authority_floor_change; DELETE FROM gis_authority_revision; ALTER TABLE gis_authority_revision ENABLE TRIGGER gis_authority_floor_change")
        self.migrate(database, self.complete, "up", ok=False)
        self.assertEqual("0", self.sql(database, "SELECT count(*) FROM gis_authority_revision").stdout.strip())

    def test_internal_fk_and_unknown_operator_hold(self):
        for alteration in [
                "DO $$ DECLARE name text; BEGIN SELECT tgname INTO STRICT name FROM pg_trigger WHERE tgrelid='gis_sessions'::regclass AND tgisinternal ORDER BY tgname LIMIT 1; EXECUTE format('ALTER TABLE gis_sessions DISABLE TRIGGER %I',name); END $$",
                "CREATE OPERATOR public.=== (FUNCTION=pg_catalog.int4eq,LEFTARG=integer,RIGHTARG=integer)"]:
            with self.subTest(alteration=alteration):
                database = self.database(legacy=True)
                self.sql(database, alteration)
                before = self.schema(database)
                self.migrate(database, self.complete, "up", ok=False)
                self.assertEqual(before, self.schema(database))

    def test_active_legacy_has_no_invented_qualification(self):
        database = self.database(legacy=True)
        self.seed(database, "active")
        before, rows = self.schema(database), self.retained(database)
        self.migrate(database, self.complete, "up", ok=False)
        self.assertEqual(before, self.schema(database))
        self.assertEqual(rows, self.retained(database))

    def test_canonical_tampering_and_downgrade_hold(self):
        database = self.database()
        self.sql(database, "ALTER TABLE gis_session_roster_updates DROP CONSTRAINT gis_session_roster_updates_pkey")
        before = self.schema(database)
        self.migrate(database, self.complete, "up", ok=False)
        self.assertEqual(before, self.schema(database))
        database = self.database()
        self.migrate(database, self.complete, "up")
        before = self.schema(database)
        self.migrate(database, self.complete, "down", "1", ok=False)
        self.assertEqual(before, self.schema(database))
        # migrate marks the attempted DOWN target dirty before invoking33.down.
        self.assertEqual("32|t", self.sql(database, "SELECT version,dirty FROM schema_migrations").stdout.strip())


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--migrations", type=pathlib.Path, default=pathlib.Path(__file__).parent)
    parser.add_argument("--postgres-image", default="postgres:16-alpine")
    OPTIONS, remaining = parser.parse_known_args()
    unittest.main(argv=[__file__, *remaining])
