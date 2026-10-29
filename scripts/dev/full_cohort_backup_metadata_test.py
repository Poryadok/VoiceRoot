import unittest
from unittest import mock
import json
import io
import hashlib
import os
from pathlib import Path
import secrets
import subprocess
import tarfile
import tempfile
import time
from types import SimpleNamespace

import media_backup_phase_test as fixture
import media_operation_journal
import resource_fencing

import full_cohort_backup_metadata as metadata


class RestorePlansTest(unittest.TestCase):
    def test_clean_go_ledger_is_restore_only(self):
        files = {'000001_init.up.sql': 'a' * 64, '000001_init.down.sql': 'b' * 64}
        self.assertEqual(metadata.restore_plan('chat_db', {'history': [{'version': 1, 'dirty': False}]}, files),
                         {'engine': 'golang-migrate', 'from': 1, 'to': 1, 'files': files})

    def test_dirty_or_multiple_ledger_rows_are_rejected(self):
        for history in ([{'version': 1, 'dirty': True}], [],
                        [{'version': 1, 'dirty': False}, {'version': 2, 'dirty': False}]):
            with self.subTest(history=history), self.assertRaises(metadata.MetadataError):
                metadata.restore_plan('chat_db', {'history': history}, {'000001_init.up.sql': 'a' * 64})

    def test_auth_requires_successful_source_matched_history(self):
        history = [{'version': '26', 'script': 'V26__ok.sql', 'checksum': 42, 'success': True}]
        result = metadata.restore_plan('auth_db', {'history': history}, {'V26__ok.sql': 'a' * 64})
        self.assertEqual((result['from'], result['to']), (26, 26))
        history[0]['script'] = 'V26__other.sql'
        with self.assertRaises(metadata.MetadataError):
            metadata.restore_plan('auth_db', {'history': history}, {'V26__ok.sql': 'a' * 64})

    def test_gateway_missing_ledger_and_extras_are_distinct(self):
        files = {'000001_init.up.sql': 'a' * 64}
        self.assertEqual(metadata.restore_plan('gateway_db', {'history': {'schema_migrations': False,
                                                                          'flyway_schema_history': False}}, files)['files'], files)
        with self.assertRaises(metadata.MetadataError):
            metadata.restore_plan('gateway_db', {'history': {'schema_migrations': True,
                                                              'flyway_schema_history': False}}, files)
        self.assertEqual(metadata.restore_plan('postgres', {}, {}),
                         {'engine': 'unmanaged', 'from': None, 'to': None, 'files': {}})

    def test_session_observation_rejects_wrong_database_role_engine_or_restart(self):
        expected = {'database': 'chat_db', 'database_oid': 42, 'role': 'postgres',
                    'server_start': 'start', 'read_only': 'on', 'data_directory': '/var/lib/postgresql/data', 'pg_major': 16}
        self.assertEqual(metadata.checked_session(expected | {'value': {'history': []}}, expected), {'history': []})
        for field, value in [('database', 'other'), ('database_oid', 43), ('role', 'other'),
                             ('server_start', 'restart'), ('read_only', 'off'), ('data_directory', '/other'), ('pg_major', 17)]:
            with self.subTest(field=field), self.assertRaises(metadata.MetadataError):
                metadata.checked_session(expected | {'value': {}, field: value}, expected)

    def test_real_query_wrapper_rejects_session_and_engine_drift(self):
        phase = object.__new__(metadata.BackupMetadataPhase)
        phase._guard = lambda: None
        session = {'database': 'chat_db', 'database_oid': 42, 'role': 'postgres',
                   'server_start': 'start', 'read_only': 'on', 'data_directory': '/var/lib/postgresql/data',
                   'pg_major': 16, 'value': {'history': [{'version': 1, 'dirty': False}]}}
        with mock.patch.object(metadata.schema, '_bounded_process', return_value=json.dumps(session).encode()) as transport:
            result = phase._read('chat_db', 42, {'uid': 'engine'}, 'start', 'postgres', lambda: {'uid': 'engine'})
            self.assertEqual(result, session['value'])
            self.assertIn('default_transaction_read_only=on', transport.call_args.args[0][15])
            with self.assertRaises(metadata.MetadataError):
                phase._read('chat_db', 42, {'uid': 'engine'}, 'start', 'postgres', lambda: {'uid': 'other'})
            session['database_oid'] = 43
            transport.return_value = json.dumps(session).encode()
            with self.assertRaises(metadata.MetadataError):
                phase._read('chat_db', 42, {'uid': 'engine'}, 'start', 'postgres', lambda: {'uid': 'engine'})

    def test_tool_receipt_tracks_create_and_removal_and_hashes_actual_bytes(self):
        phase = object.__new__(metadata.BackupMetadataPhase)
        phase._guard = lambda: None
        states = []
        phase._tool_state = lambda state: states.append(state)
        image_id, cid = 'sha256:' + 'a' * 64, 'b' * 64
        payload = io.BytesIO()
        with tarfile.open(fileobj=payload, mode='w') as archive:
            entry = tarfile.TarInfo('migrate'); entry.size = 4
            archive.addfile(entry, io.BytesIO(b'tool'))
        def transport(args, limit):
            if args[1:3] == ['image', 'inspect']:
                return json.dumps([{'Os': 'linux', 'Architecture': 'amd64', 'Id': image_id,
                                    'RepoDigests': [metadata.realtime_migration_job.MIGRATE_IMAGE]}]).encode()
            if args[1] == 'create': return cid.encode()
            if args[1] == 'inspect':
                return json.dumps([{'Id': cid, 'Image': image_id, 'Config': {'Labels': {'voice.test.owner': states[0]['name']}},
                                    'HostConfig': {'NetworkMode': 'none', 'PortBindings': None}, 'Mounts': []}]).encode()
            if args[1] == 'cp': return payload.getvalue()
            return b''
        with mock.patch.object(metadata.schema, '_bounded_process', side_effect=transport):
            digest = phase._tool_sha()
        self.assertEqual(digest, hashlib.sha256(b'tool').hexdigest())
        self.assertEqual([state['status'] for state in states], ['create-pending', 'created', 'remove-pending', 'removed'])


@unittest.skipUnless(os.environ.get('COHORT_METADATA_PG_TEST') == '1', 'explicit disposable PostgreSQL fixture')
class PostgreSQLTest(unittest.TestCase):
    def test_catalog_sql_and_session_on_real_pg16(self):
        name = 'voice-cohort-metadata-test-' + secrets.token_hex(8)
        subprocess.run(['docker', 'run', '-d', '--name', name, '--network', 'none',
                        '-e', 'POSTGRES_PASSWORD=fixture-only', 'postgres:16-alpine'],
                       check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.addCleanup(subprocess.run, ['docker', 'rm', '-f', '-v', name], check=True,
                        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        for _ in range(60):
            logs = subprocess.run(['docker', 'logs', name], stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                  check=True).stdout
            ready = subprocess.run(['docker', 'exec', name, 'pg_isready', '-U', 'postgres'],
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            if b'PostgreSQL init process complete; ready for start up.' in logs and ready.returncode == 0:
                break
            time.sleep(0.5)
        else:
            self.fail('disposable PostgreSQL failed to start')
        def sql(query):
            result = subprocess.run(['docker', 'exec', '-e', 'PGOPTIONS=-c default_transaction_read_only=on -c search_path=pg_catalog,public',
                                     name, 'psql', '-X', '-qAt', '-v', 'ON_ERROR_STOP=1', '-U', 'postgres',
                                     '-d', 'postgres', '-c', query], check=True, stdout=subprocess.PIPE,
                                    stderr=subprocess.DEVNULL)
            return json.loads(result.stdout)
        roles = sql(metadata._sql(metadata._query_for('roles')))
        self.assertEqual(roles['pg_major'], 16)
        self.assertEqual(roles['read_only'], 'on')
        self.assertEqual(set(roles['value']), {'roles', 'memberships'})
        catalog = sql(metadata._sql(metadata._query_for('postgres')))
        self.assertEqual(catalog['database_oid'], roles['database_oid'])
        self.assertEqual(catalog['server_start'], roles['server_start'])
        self.assertIn('relations', catalog['value'])
        gateway = sql(metadata._sql(metadata._query_for('gateway_db')))
        self.assertEqual(gateway['value']['history'], {'schema_migrations': False, 'flyway_schema_history': False})
        subprocess.run(['docker', 'exec', name, 'psql', '-X', '-q', '-v', 'ON_ERROR_STOP=1', '-U', 'postgres',
                        '-d', 'postgres', '-c', 'CREATE TABLE public.schema_migrations(version bigint, dirty boolean); '
                        'INSERT INTO public.schema_migrations VALUES (25, false); CREATE SCHEMA shadow; '
                        'CREATE TABLE shadow.schema_migrations(version bigint, dirty boolean); '
                        'INSERT INTO shadow.schema_migrations VALUES (99, false); '
                        'ALTER ROLE postgres SET search_path TO shadow,pg_catalog,public;'], check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        go = sql(metadata._sql(metadata._query_for('chat_db')))
        self.assertEqual(go['value']['history'], [{'version': 25, 'dirty': False}])
        self.assertEqual(metadata.restore_plan('chat_db', go['value'], {'000025_init.up.sql': 'a' * 64})['to'], 25)


class JournalBoundaryTest(unittest.TestCase):
    def setUp(self):
        self.context = {'project': 'voice', 'resource': 'DEV', 'actor': 'scheduled_devops',
                        'claim': '11111111-1111-4111-8111-111111111111',
                        'window': '22222222-2222-4222-8222-222222222222', 'revision': 1, 'generation': 1,
                        'operation_id': 'media-c1-' + 'a' * 32, 'commit': 'a' * 40,
                        'images': {'voice-file/file': 'ghcr.io/x@sha256:' + 'b' * 64}}
        self.ledger = fixture.Ledger(self.context)
        self.fence = resource_fencing.Fence(self.context, lambda: {}, self.ledger)
        self.tmp = tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.journal = media_operation_journal.Journal(Path(self.tmp.name) / 'journal',
                          {'resource_context': self.context, 'namespace_uid': 'ns'})
        self.addCleanup(self.journal.close)
        self.phase = object.__new__(metadata.BackupMetadataPhase)
        self.phase.journal, self.phase.fence = self.journal, self.fence
        other = SimpleNamespace(fence=self.fence)
        self.phase.package = self.phase.inventory = self.phase.catalog = self.phase.cut = other
        self.phase._guard = lambda: None

    def test_actual_journal_and_fence_reservation_rejects_duplicate(self):
        first = {'status': 'pending', 'nonce': '1' * 32}
        self.journal.state['steps'][metadata.STEP] = first.copy(); self.journal.save()
        self.phase._record(first)
        self.assertEqual(self.ledger.row[metadata.LEDGER], first)
        with self.assertRaises(metadata.MetadataError):
            self.phase._record(first)

    def test_tool_state_rejects_changed_private_journal(self):
        first = {'status': 'pending', 'nonce': '1' * 32}
        self.journal.state['steps'][metadata.STEP] = first.copy(); self.journal.save()
        self.phase._record(first)
        self.journal.state['steps'][metadata.STEP]['nonce'] = '2' * 32
        with self.assertRaises(metadata.MetadataError):
            self.phase._tool_state({'status': 'create-pending'})
        self.assertEqual(self.ledger.row[metadata.LEDGER], first)

    def test_lost_reservation_reply_marks_real_fence_uncertain(self):
        catalog = {'databases': {'postgres': 1}}
        self.ledger.row['full_cohort_db_catalog'] = {'receipt_sha256': metadata._digest(catalog)}
        self.phase.package.verify_current = lambda token: {'package': 'fixture'}
        self.phase.inventory.verify_receipt = lambda token: {}
        self.phase.catalog.verify_current = lambda token: catalog
        self.phase.cut.verify_drained = lambda: None
        self.phase._source_files = lambda receipt, token: {'files': {}}
        self.phase.poisoned = False
        original = self.ledger.write
        def lost(revision, row):
            original(revision, row)
            if row.get(metadata.LEDGER, {}).get('status') == 'pending':
                raise RuntimeError('lost reply')
        with mock.patch.object(self.ledger, 'write', side_effect=lost):
            with self.assertRaises(metadata.MetadataError):
                self.phase.run('token')
        self.assertTrue(self.phase.poisoned)
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_typed_phase_run_and_private_readback_with_synthetic_observers(self):
        client = object.__new__(metadata.frontend_runtime_executor.KubectlTransport)
        client.journal = self.journal
        self.ledger.client = client
        package = object.__new__(metadata.package.PackagePhase)
        inventory = object.__new__(metadata.inventory.InventoryPhase)
        cut = object.__new__(metadata.writer_cut.WriterCut)
        for obj in (package, inventory, cut):
            obj.fence = self.fence; obj.lock = {}; obj.namespace_uid = 'ns'
        inventory.package = package
        candidate = {'package_sha256': 'c' * 64}
        package.verify_current = lambda token: candidate
        report = {'database_observations': {'workloads': {'fixture': [{'status': 'observed', 'result': {
            'metadata': {'engine': {'namespace_uid': 'ns'}, 'session': {'server_start': 'start'}}}}]}}}
        inventory.verify_receipt = lambda token: report
        inventory.verify_current = lambda token: report
        inventory._admin_role = lambda: 'postgres'
        cut.verify_drained = lambda: None
        projected = {'all_databases': {'postgres': 1}, 'databases': {'postgres': 1}, 'excluded': {},
                     'properties': {'postgres': {'allowconn': True}}}
        catalog_phase = metadata.affected.FullCatalogPhase(self.journal, self.fence, client, {}, 'ns', inventory)
        operation = Path(self.tmp.name) / self.context['operation_id']
        operation.mkdir(mode=0o700)
        phase = metadata.BackupMetadataPhase(self.journal, self.fence, client, {}, 'ns', package,
                                            inventory, catalog_phase, cut, metadata.canonical.ROOT)
        def tool():
            phase._tool_state({'status': 'removed'})
            return 'd' * 64
        def read(database, *args):
            return {'roles': [], 'memberships': []} if database == 'roles' else {'database_owner': 'postgres'}
        with mock.patch.object(metadata.media_operation_guard, 'verify'), \
             mock.patch.object(metadata.affected, '_full_catalog', return_value=projected), \
             mock.patch.object(metadata.media_backup_upload_phase, 'ROOT', Path(self.tmp.name)), \
             mock.patch.object(metadata.schema, 'kube_engine_snapshot', return_value=lambda: {'namespace_uid': 'ns'}), \
             mock.patch.object(metadata.unmanaged, 'validate_source_catalog'), \
             mock.patch.object(phase, '_source_files', return_value={'files': {}}), \
             mock.patch.object(phase, '_tool_sha', side_effect=tool), mock.patch.object(phase, '_read', side_effect=read) as observer:
            catalog_phase.run('token')
            result = phase.run('token')
            self.assertEqual(result['databases'], ['postgres'])
            self.assertEqual(result['metadata']['migrations']['postgres']['engine'], 'unmanaged')
            self.assertEqual(metadata.verify_completed(phase, 'token'), result)
            self.assertEqual(metadata.verify_current(phase, 'token'), result)
            values = iter([{'roles': [], 'memberships': []}, {'database_owner': 'postgres'},
                           {'roles': [], 'memberships': []}, {'database_owner': 'changed'}])
            observer.side_effect = lambda *args: next(values)
            with self.assertRaises(metadata.MetadataError): metadata.verify_current(phase, 'token')
            observer.side_effect = lambda database, *args: {'roles': [], 'memberships': []} if database == 'roles' else {}
            with self.assertRaises(metadata.MetadataError): metadata.verify_current(phase, 'token')
            with self.assertRaises(metadata.MetadataError): phase.run('token')
            path = operation / metadata.FILENAME
            path.write_bytes(b'{}')
            with self.assertRaises(metadata.MetadataError): metadata.verify_completed(phase, 'token')
