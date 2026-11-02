"""Disposable PostgreSQL reproduction of the Realtime migration role boundary."""
import pathlib
import copy
import hashlib
import json
import subprocess
import tempfile
import time
import unittest
import uuid
from unittest import mock

import realtime_db_provision as provision


class PostgreSQLBoundary(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.name = 'voice-realtime-provision-' + uuid.uuid4().hex[:16]
        cls.started = False
        result = subprocess.run(['docker', 'run', '-d', '--name', cls.name, '--network', 'none',
                                 '--label', 'voice.test.owner=realtime-db-provision',
                                 '-e', 'POSTGRES_HOST_AUTH_METHOD=trust', 'postgres:16-alpine'],
                                capture_output=True, timeout=60)
        if result.returncode:
            raise RuntimeError('private PostgreSQL fixture unavailable')
        cls.started = True
        for _ in range(100):
            ready = cls.sql('postgres', 'postgres', 'SHOW listen_addresses', check=False)
            if ready.returncode == 0 and ready.stdout.strip() == b'*':
                break
            time.sleep(.2)
        else:
            raise RuntimeError('private PostgreSQL fixture not ready')

    @classmethod
    def tearDownClass(cls):
        if cls.started:
            subprocess.run(['docker', 'rm', '-f', '-v', cls.name], capture_output=True, timeout=30)

    @classmethod
    def sql(cls, db, user, statement, check=True):
        result = subprocess.run(['docker', 'exec', '-i', cls.name, 'psql', '-X', '-qAt',
                                 '-v', 'ON_ERROR_STOP=1', '-U', user, '-d', db],
                                input=statement.encode(), capture_output=True, timeout=30)
        if check and result.returncode:
            detail = result.stderr.decode(errors='replace').replace('fixtureCredential_123456789012345678901234', '[redacted]')
            raise RuntimeError('fixture SQL rejected: ' + detail[-400:])
        return result

    def test_restricted_runtime_cannot_perform_canonical_migration(self):
        self.sql('postgres', 'postgres', 'CREATE DATABASE realtime_db;\nCREATE ROLE realtime LOGIN;\n')
        self.sql('realtime_db', 'postgres', 'REVOKE CREATE ON SCHEMA public FROM PUBLIC;')
        migration = (pathlib.Path(__file__).resolve().parents[2] / 'src/backend/migrations/realtime_db/000001_admission.up.sql').read_text(encoding='utf-8')
        denied = self.sql('realtime_db', 'realtime', migration, check=False)
        self.assertNotEqual(denied.returncode, 0, 'restricted runtime must not have migration DDL')
        self.sql('realtime_db', 'postgres', migration)
        self.sql('realtime_db', 'postgres', 'GRANT USAGE ON SCHEMA public TO realtime; GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO realtime;')
        result = self.sql('realtime_db', 'realtime', "SELECT count(*) FROM realtime_incarnations; INSERT INTO realtime_incarnations(id,runtime_identity,state) VALUES('00000000-0000-0000-0000-000000000001','fixture','OPEN'); UPDATE realtime_incarnations SET state='CLOSING';")
        self.assertEqual(result.returncode, 0)
        self.assertNotEqual(self.sql('realtime_db', 'realtime', 'CREATE TABLE forbidden(id int)', check=False).returncode, 0)
        self.assertNotEqual(self.sql('realtime_db', 'realtime', 'DROP TABLE realtime_incarnations', check=False).returncode, 0)

    def test_shadow_schema_cannot_hide_existing_realtime_role(self):
        self.sql('postgres','postgres',"DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='realtime') THEN CREATE ROLE realtime LOGIN; END IF; END $$; CREATE SCHEMA shadow_fixture; CREATE VIEW shadow_fixture.pg_roles AS SELECT * FROM pg_catalog.pg_roles WHERE rolname <> 'realtime'; ALTER ROLE postgres SET search_path = shadow_fixture, pg_catalog;")
        try:
            result=json.loads(self.sql('postgres','postgres',provision.CATALOG_SQL).stdout)
            self.assertIsInstance(result['role'],dict)
            self.assertGreater(result['role']['oid'],0)
        finally:
            self.sql('postgres','postgres','ALTER ROLE postgres RESET search_path; DROP SCHEMA shadow_fixture CASCADE;')

    def test_source_backed_single_login_and_create_collisions(self):
        # Separate names from the restricted-role characterization above.
        self.sql('postgres', 'postgres', 'CREATE DATABASE foreign_fixture;')
        self.sql('foreign_fixture', 'postgres', "CREATE TABLE sentinel(v text); INSERT INTO sentinel VALUES('preserved');")
        self.sql('postgres', 'postgres', 'DROP DATABASE realtime_db;\nDROP ROLE realtime;\n')
        password = b'fixtureCredential_123456789012345678901234'
        context = {'operation_id': 'media-c1-' + 'a' * 32}
        secret = {'name': 'voice-realtime-db-fixture', 'uid': 'u', 'resourceVersion': '4', 'password_sha256': hashlib.sha256(password).hexdigest()}
        intent = {'resource_context': context, 'namespace_uid': 'ns', 'postgres': {'pod_uid': 'p'}, 'realtime_credential': secret}
        journal = mock.Mock(poisoned=False, intent=copy.deepcopy(intent), state={'intent': copy.deepcopy(intent), 'steps': {}})
        fence = mock.Mock(context=context)
        fence.owned.return_value = True
        row = {'status': 'active'}
        fence.ledger.read.side_effect = lambda: (copy.deepcopy(row), ('uid', '1'))
        def write(_, value): row.clear(); row.update(copy.deepcopy(value))
        fence.ledger.write.side_effect = write
        fixture = self
        class PG:
            def snapshot(self): return {'pod_uid': 'p'}
            def catalog(self): return json.loads(fixture.sql('postgres', 'postgres', provision.CATALOG_SQL).stdout)
            def execute(self, sql): fixture.sql('postgres', 'postgres', sql)
        with mock.patch.object(provision, 'guard'), mock.patch.object(provision.Provision, '_secret'):
            worker = provision.Provision(journal, fence, mock.Mock(), {}, 'ns', PG(), password)
            receipt = worker.run()
        self.assertFalse(receipt['migration_pass'])
        catalog = json.loads(self.sql('postgres', 'postgres', provision.CATALOG_SQL).stdout)
        self.assertTrue(provision._valid_role(catalog['role']))
        self.assertEqual(catalog['database']['owner_oid'], catalog['role']['oid'])
        self.assertNotEqual(self.sql('postgres', 'postgres', provision.role_sql(b'fixtureCredential_123456789012345678901234'), check=False).returncode, 0)
        self.assertNotEqual(self.sql('postgres', 'postgres', provision.database_sql(), check=False).returncode, 0)
        self.assertEqual(self.sql('foreign_fixture', 'postgres', 'SELECT v FROM sentinel').stdout.strip(), b'preserved')
        self.assertEqual(receipt['role_oid'], catalog['role']['oid'])
        self.assertEqual(receipt['database_oid'], catalog['database']['oid'])
        # Enable SCRAM only on this fixture's loopback TCP; credentials travel
        # in the owned exec's stdin, not command arguments or output.
        subprocess.run(['docker', 'exec', self.name, 'sh', '-ec',
                        'sed -i "1ihost all all 127.0.0.1/32 scram-sha-256" "$PGDATA/pg_hba.conf"'],
                       capture_output=True, check=True, timeout=30)
        self.sql('postgres', 'postgres', 'SELECT pg_reload_conf();')
        def authenticate(credential):
            script = ('export PGPASSWORD=' + credential + '\n'
                      "exec psql -h 127.0.0.1 -X -qAt -U realtime -d realtime_db -c 'SELECT 1'\n")
            return subprocess.run(['docker', 'exec', '-i', self.name, 'sh', '-s'], input=script.encode(),
                                  capture_output=True, timeout=30)
        for _ in range(40):
            rejected = authenticate('wrongFixtureCredential')
            if rejected.returncode != 0 and b'password authentication failed' in rejected.stderr: break
            time.sleep(.1)
        else: raise RuntimeError('fixture SCRAM policy not active')
        self.assertEqual(authenticate(password.decode()).returncode, 0)
        other = mock.Mock(poisoned=False, intent=copy.deepcopy(intent), state={'intent': copy.deepcopy(intent), 'steps': {}})
        collision_pg = PG()
        collision_pg.execute = mock.Mock(side_effect=AssertionError('unexpected effect'))
        with mock.patch.object(provision, 'guard'), mock.patch.object(provision.Provision, '_secret'):
            collision = provision.Provision(other, fence, mock.Mock(), {}, 'ns', collision_pg, password)
            with self.assertRaises(provision.RecoveryRequired): collision.run()
            collision_pg.execute.assert_not_called()
        # Use the real repository-pinned migrate image binary on the private PG
        # socket. No password is used in this isolated trust-only fixture.
        image_container = subprocess.run(['docker', 'create', 'migrate/migrate:v4.18.1'], capture_output=True, check=True, timeout=30).stdout.decode().strip()
        try:
            with tempfile.TemporaryDirectory(prefix='realtime-migrate-fixture-') as tmp:
                tool = pathlib.Path(tmp) / 'migrate'
                subprocess.run(['docker', 'cp', image_container + ':/usr/local/bin/migrate', str(tool)], capture_output=True, check=True, timeout=30)
                subprocess.run(['docker', 'cp', str(tool), self.name + ':/tmp/migrate'], capture_output=True, check=True, timeout=30)
                source = pathlib.Path(__file__).resolve().parents[2] / 'src/backend/migrations/realtime_db'
                subprocess.run(['docker', 'cp', str(source), self.name + ':/tmp/migrations'], capture_output=True, check=True, timeout=30)
                cmd = ['docker', 'exec', self.name, '/tmp/migrate', '-path=/tmp/migrations',
                       '-database=postgres://realtime@localhost:5432/realtime_db?host=/var/run/postgresql&sslmode=disable', 'up']
                self.assertEqual(subprocess.run(cmd, capture_output=True, timeout=30).returncode, 0)
        finally:
            subprocess.run(['docker', 'rm', image_container], capture_output=True, timeout=30)
        self.assertEqual(self.sql('realtime_db', 'realtime', 'SELECT version,dirty FROM schema_migrations').stdout.strip(), b'1|f')
        self.assertEqual(self.sql('realtime_db', 'realtime', "SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tablename LIKE 'realtime_%'").stdout.strip(), b'6')
        self.assertEqual(self.sql('foreign_fixture', 'postgres', 'SELECT v FROM sentinel').stdout.strip(), b'preserved')
        self.sql('realtime_db', 'realtime', "INSERT INTO realtime_incarnations(id,runtime_identity,state) VALUES('00000000-0000-0000-0000-000000000001','restore-fixture','OPEN');")
        self.sql('realtime_db', 'realtime', "INSERT INTO realtime_chat_gates(chat_id,scope_known,generation,state) VALUES('00000000-0000-0000-0000-000000000002',true,1,'PURGED'); INSERT INTO realtime_admissions(id,incarnation_id,chat_id,generation,state) VALUES('00000000-0000-0000-0000-000000000003','00000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000002',1,'PENDING');")
        archive = subprocess.run(['docker', 'exec', self.name, 'pg_dump', '-Fc', '-U', 'realtime', '-d', 'realtime_db'],
                                 capture_output=True, check=True, timeout=30).stdout
        restored = 'voice-realtime-restore-' + uuid.uuid4().hex[:16]
        created = False
        try:
            subprocess.run(['docker', 'run', '-d', '--name', restored, '--network', 'none',
                            '--label', 'voice.test.owner=realtime-db-provision', '-e', 'POSTGRES_HOST_AUTH_METHOD=trust',
                            'postgres:16-alpine'], capture_output=True, check=True, timeout=60)
            created = True
            command = ['docker', 'exec', '-i', restored, 'psql', '-X', '-qAt', '-v', 'ON_ERROR_STOP=1', '-U', 'postgres', '-d', 'postgres']
            for _ in range(100):
                ready = subprocess.run(command, input=b'SHOW listen_addresses', capture_output=True, timeout=15)
                if ready.returncode == 0 and ready.stdout.strip() == b'*': break
                time.sleep(.2)
            else: raise RuntimeError('restore fixture not ready')
            subprocess.run(command, input=b'CREATE DATABASE restore_fixture;', capture_output=True, check=True, timeout=30)
            subprocess.run(['docker', 'exec', '-i', restored, 'pg_restore', '-U', 'postgres', '--no-owner',
                            '--no-privileges', '-d', 'restore_fixture'], input=archive, capture_output=True, check=True, timeout=30)
            verify = ['docker', 'exec', '-i', restored, 'psql', '-X', '-qAt', '-v', 'ON_ERROR_STOP=1', '-U', 'postgres', '-d', 'restore_fixture']
            actual = subprocess.run(verify, input=b'SELECT version,dirty FROM schema_migrations; SELECT runtime_identity FROM realtime_incarnations; SELECT state FROM realtime_admissions; SELECT state FROM realtime_chat_gates;',
                                    capture_output=True, check=True, timeout=30)
            self.assertEqual(actual.stdout.strip(), b'1|f\nrestore-fixture\nPENDING\nPURGED')
        finally:
            if created:
                subprocess.run(['docker', 'rm', '-f', '-v', restored], capture_output=True, timeout=30)
        # Only this owned isolated fixture is destroyed to reproduce a lost
        # create-role response. The real operator never executes this cleanup.
        self.sql('postgres', 'postgres', 'DROP DATABASE realtime_db;\nDROP ROLE realtime;\n')
        row.clear(); row.update({'status': 'active'})
        journal.state['steps'].clear()
        class LostPG(PG):
            def execute(self, sql):
                super().execute(sql)
                raise TimeoutError('synthetic response loss after commit')
        with mock.patch.object(provision, 'guard'), mock.patch.object(provision.Provision, '_secret'):
            worker = provision.Provision(journal, fence, mock.Mock(), {}, 'ns', LostPG(), password)
            with self.assertRaises(provision.RecoveryRequired): worker.run()
            self.assertIsNotNone(PG().catalog()['role'])
            self.assertIsNone(PG().catalog()['database'])
            self.assertEqual(journal.state['steps']['realtime-db-role']['status'], 'pending')
            with self.assertRaises(provision.RecoveryRequired): worker.run()
        self.assertEqual(self.sql('foreign_fixture', 'postgres', 'SELECT v FROM sentinel').stdout.strip(), b'preserved')


if __name__ == '__main__':
    unittest.main()
