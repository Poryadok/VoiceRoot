"""Synthetic authority and transport; real encrypted dump and isolated restore."""
from datetime import datetime, timedelta, timezone
import hashlib
import io
import json
from pathlib import Path
import secrets
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch
import zipfile

import story_media_backup_cohort as cohort
import story_media_backup_cohort_capture as capture
import story_media_backup_cohort_custody as custody
import story_media_backup_cohort_finalize as finalize
import story_media_backup_cohort_restore as restore
import story_media_backup_restore as media_restore
import story_media_backup_integration_test as fixtures
from story_media_backup_capture_integration_test import cleanup_fixture
from story_media_backup_restore_integration_test import Journal, POSTGRES, MIGRATE


class CaptureJournal:
    def __init__(self): self.receipts = {}
    def begin(self, member): return '%032x' % (len(self.receipts) + 1)
    def complete(self, member, receipt): self.receipts[member] = receipt
    def uncertain(self, member): raise RuntimeError('synthetic capture uncertain')


class CohortChainTest(unittest.TestCase):
    def test_cycle_two_populated_backup_mock_readback_and_restore(self):
        fixture = fixtures.PopulatedRestoreTest()
        self.addCleanup(cleanup_fixture, fixture); fixture.setUp()
        fixture.sql(fixture.source, 'source_admin', 'postgres', 'CREATE DATABASE voice_db')
        folder = Path('/repo/src/backend/migrations/voice_db')
        for path in sorted(folder.glob('*.up.sql')):
            if int(path.name[:6]) <= 15: fixture.sql(fixture.source, 'source_admin', 'voice_db', path.read_text())
        fixture.sql(fixture.source, 'source_admin', 'voice_db',
            'CREATE TABLE schema_migrations(version bigint NOT NULL, dirty boolean NOT NULL);'
            'INSERT INTO schema_migrations VALUES(15,false);'
            'CREATE TABLE fixture_marker(id integer PRIMARY KEY); INSERT INTO fixture_marker VALUES(1)')
        session = json.loads(fixture.sql(fixture.source, 'source_admin', 'voice_db',
            "SELECT json_build_object('database_oid',(SELECT oid::bigint FROM pg_database WHERE datname=current_database()),"
            "'server_start',pg_postmaster_start_time())"))
        catalog = {'voice_db': json.loads(fixture.sql(fixture.source, 'source_admin', 'voice_db', media_restore.CATALOG))}
        roles = json.loads(fixture.sql(fixture.source, 'source_admin', 'postgres', media_restore.role_catalog_sql()))
        files = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in folder.glob('*.sql')}
        plans = {'voice_db': {'engine': 'golang-migrate', 'from': 15, 'to': 15, 'files': files}}
        metadata = {'catalog': catalog, 'roles': roles, 'migrations': plans}
        def digest(value):
            return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode()).hexdigest()
        engine = {'namespace_uid': '33333333-3333-3333-3333-333333333333',
                  'pod_uid': '44444444-4444-4444-4444-444444444444',
                  'pvc_uid': '55555555-5555-5555-5555-555555555555',
                  'pv_uid': '66666666-6666-6666-6666-666666666666',
                  'container_id': 'containerd://' + 'd' * 64, 'restart_count': 7}
        source = {'engine': engine, 'server_start': session['server_start']}
        binding = {'operation_id': 'media-c2-' + 'a' * 32, 'snapshot_id': 'capture-' + 'b' * 32,
            'source_sha': 'ec214b28747a4552eb89f4e431fc018aa82f023a', 'project': 'voice', 'resource': 'DEV',
            'actor': 'scheduled_devops', 'claim': '11111111-1111-1111-1111-111111111111',
            'window': '22222222-2222-2222-2222-222222222222', 'generation': 3, 'revision': 4,
            'databases': ['voice_db'], 'sources': {'voice_db': {**source, 'database_oid': session['database_oid']}, 'roles': source},
            'restore_inputs': {'catalog_sha256': digest(catalog), 'roles_catalog_sha256': digest(roles),
                'migration_catalog_sha256': digest(plans), 'postgres_image': POSTGRES, 'migrate_image': MIGRATE,
                'migrate_binary_sha256': 'e' * 64, 'postgres_major': 16}}
        mount = '/var/lib/postgresql/data'
        expected = {**engine, 'operation': binding['operation_id'], 'snapshot_id': binding['snapshot_id'],
            'statefulset_uid': 'sts', 'pv_name': 'pv', 'container': 'postgres', 'mount_path': mount,
            'data_directory': fixture.sql(fixture.source, 'source_admin', 'postgres', 'SHOW data_directory'), 'pg_major': 16}
        objects = {'namespace': {'metadata': {'uid': engine['namespace_uid']}},
            'pod': {'metadata': {'uid': engine['pod_uid'], 'namespace': 'voice-dev',
                    'ownerReferences': [{'uid': 'sts', 'kind': 'StatefulSet', 'controller': True}]},
                    'spec': {'containers': [{'name': 'postgres', 'volumeMounts': [{'name': 'data', 'mountPath': mount}]}],
                             'volumes': [{'name': 'data', 'persistentVolumeClaim': {'claimName': 'pgdata-voice-postgres-0'}}]},
                    'status': {'containerStatuses': [{'name': 'postgres', 'containerID': engine['container_id'],
                        'restartCount': 7, 'ready': True, 'state': {'running': {}}}]}},
            'pvc': {'metadata': {'uid': engine['pvc_uid'], 'namespace': 'voice-dev'},
                    'spec': {'volumeName': 'pv'}, 'status': {'phase': 'Bound'}},
            'pv': {'metadata': {'uid': engine['pv_uid']}, 'spec': {'claimRef': {'uid': engine['pvc_uid'],
                'namespace': 'voice-dev', 'name': 'pgdata-voice-postgres-0'}}}}
        def runner(command, **kwargs):
            if 'get' in command:
                return SimpleNamespace(returncode=0, stdout=json.dumps(objects[command[command.index('get') + 1]]).encode())
            index = command.index('--', command.index('exec'))
            return subprocess.run(['docker', 'exec', fixture.source, *command[index + 1:]], **kwargs)
        journal = CaptureJournal()
        producer = capture.Producer(binding, ['voice_db'], expected, 'source_admin', lambda: None, lambda: None, journal, runner)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); source_dir = root / 'source'; dest = root / 'readback'
            source_dir.mkdir(mode=0o700); dest.mkdir(mode=0o700); key = secrets.token_bytes(32)
            manifest = cohort.export(source_dir, key, binding, ['voice_db'], producer)
            bound, manifest = finalize.finalize(source_dir, key, binding, ['voice_db'], manifest, journal.receipts)
            bundle = custody.build_bundle(source_dir, key, bound, ['voice_db'], manifest)
            raw = bundle.read_bytes(); wire = io.BytesIO()
            with zipfile.ZipFile(wire, 'w', compression=zipfile.ZIP_STORED) as zipped: zipped.writestr('cohort-backup.tar', raw)
            wire = wire.getvalue(); now = datetime.now(timezone.utc)
            artifact = {'operation_id': binding['operation_id'], 'snapshot_id': binding['snapshot_id'], 'challenge': 'e' * 32,
                'run_id': 42, 'head_sha': 'f' * 40, 'created_at': (now-timedelta(seconds=1)).isoformat(),
                'bundle_bytes': len(raw), 'bundle_sha256': hashlib.sha256(raw).hexdigest()}
            remote = {'id': 23, 'url': custody.API + '23', 'name': 'voice-cohort-backup-' + binding['operation_id'] + '-' + 'e' * 32,
                'expired': False, 'created_at': now.isoformat(), 'expires_at': (now+timedelta(days=1)).isoformat(),
                'workflow_run': {'id': 42, 'head_sha': 'f' * 40}, 'size_in_bytes': len(wire)}
            def download(url, headers, target, limit, deadline, redirects=False):
                data = wire if redirects else json.dumps(remote).encode(); target.write(data); return len(data)
            with patch.object(custody.transport, '_download', side_effect=download):
                _, verified_manifest = custody.readback_extract('fixture-token', artifact, 23, bundle, key, bound, ['voice_db'], dest)
            observed = []; original = media_restore._psql
            def psql(cid, admin, db, sql, timeout=120):
                result = original(cid, admin, db, sql, timeout=timeout)
                if sql == media_restore.CATALOG:
                    self.assertEqual(original(cid, admin, db, 'SELECT count(*) FROM fixture_marker').strip(), b'1')
                    observed.append(db)
                return result
            with patch.object(media_restore, '_psql', side_effect=psql), patch.object(media_restore, '_tool_bytes', side_effect=AssertionError('no migration')):
                receipt = restore.qualify(dest, key, bound, ['voice_db'], verified_manifest, metadata, Path('/repo'), POSTGRES, MIGRATE, Journal())
            self.assertEqual(receipt['databases']['voice_db']['mode'], 'restore-only')
            self.assertEqual(receipt['operation_id'], binding['operation_id'])
            self.assertEqual(receipt['cleanup'], 'verified'); self.assertEqual(observed, ['voice_db', 'voice_db'])


if __name__ == '__main__': unittest.main()
