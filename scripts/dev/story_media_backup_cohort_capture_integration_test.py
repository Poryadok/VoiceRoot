"""Real PostgreSQL dump with synthetic Kubernetes and authority callbacks."""
import json
import subprocess
import tempfile
from types import SimpleNamespace
import unittest

import story_media_backup_cohort_capture as capture
import story_media_backup_integration_test as fixtures
from story_media_backup_capture_integration_test import cleanup_fixture
from story_media_backup_capture_test import Journal


class CohortCaptureIntegrationTest(unittest.TestCase):
    def test_populated_database_and_roles_dump(self):
        fixture = fixtures.PopulatedRestoreTest()
        self.addCleanup(cleanup_fixture, fixture); fixture.setUp()
        fixture.sql(fixture.source, 'source_admin', 'postgres', 'CREATE DATABASE voice_db')
        fixture.sql(fixture.source, 'source_admin', 'voice_db',
                    'CREATE TABLE fixture_marker(id integer); INSERT INTO fixture_marker VALUES (1)')
        session = json.loads(fixture.sql(fixture.source, 'source_admin', 'voice_db',
            "SELECT json_build_object('database_oid',(SELECT oid::bigint FROM pg_database WHERE datname=current_database()),"
            "'server_start',pg_postmaster_start_time())"))
        mount = '/var/lib/postgresql/data'
        expected = {'operation': 'media-c2-' + 'a' * 32, 'snapshot_id': 'capture-' + 'b' * 32,
            'namespace_uid': 'ns', 'pod_uid': 'pod', 'statefulset_uid': 'sts', 'pvc_uid': 'pvc',
            'pv_name': 'pv', 'pv_uid': 'pvu', 'container': 'postgres', 'container_id': fixture.source,
            'restart_count': 0, 'mount_path': mount,
            'data_directory': fixture.sql(fixture.source, 'source_admin', 'postgres', 'SHOW data_directory'), 'pg_major': 16}
        engine = {k: expected[k] for k in ('namespace_uid', 'pod_uid', 'pvc_uid', 'pv_uid', 'container_id', 'restart_count')}
        source = {'engine': engine, 'server_start': session['server_start']}
        binding = {'operation_id': expected['operation'], 'snapshot_id': expected['snapshot_id'],
                   'source_sha': 'c' * 40, 'databases': ['voice_db'], 'sources': {
                       'voice_db': {**source, 'database_oid': session['database_oid']}, 'roles': source}}
        objects = {
            'namespace': {'metadata': {'uid': 'ns'}},
            'pod': {'metadata': {'uid': 'pod', 'namespace': 'voice-dev',
                    'ownerReferences': [{'uid': 'sts', 'kind': 'StatefulSet', 'controller': True}]},
                    'spec': {'containers': [{'name': 'postgres', 'volumeMounts': [{'name': 'data', 'mountPath': mount}]}],
                             'volumes': [{'name': 'data', 'persistentVolumeClaim': {'claimName': 'pgdata-voice-postgres-0'}}]},
                    'status': {'containerStatuses': [{'name': 'postgres', 'containerID': fixture.source,
                        'restartCount': 0, 'ready': True, 'state': {'running': {}}}]}},
            'pvc': {'metadata': {'uid': 'pvc', 'namespace': 'voice-dev'}, 'spec': {'volumeName': 'pv'},
                    'status': {'phase': 'Bound'}},
            'pv': {'metadata': {'uid': 'pvu'}, 'spec': {'claimRef': {'uid': 'pvc', 'namespace': 'voice-dev',
                                                                     'name': 'pgdata-voice-postgres-0'}}}}
        def runner(command, **kwargs):
            if 'get' in command:
                return SimpleNamespace(returncode=0, stdout=json.dumps(objects[command[command.index('get') + 1]]).encode())
            index = command.index('--', command.index('exec'))
            return subprocess.run(['docker', 'exec', fixture.source, *command[index + 1:]], **kwargs)
        journal = Journal()
        producer = capture.Producer(binding, ['voice_db'], expected, 'source_admin', lambda: None, lambda: None,
                                    journal, runner)
        with tempfile.TemporaryFile() as output:
            producer('voice_db', output); output.seek(0); self.assertEqual(output.read(5), b'PGDMP')
        with tempfile.TemporaryFile() as output:
            producer('roles', output); output.seek(0); self.assertIn(b'source_admin', output.read())
        self.assertEqual([e[0] for e in journal.events], ['begin', 'complete', 'begin', 'complete'])
        self.assertEqual(journal.events[1][2]['database_oid'], session['database_oid'])


if __name__ == '__main__': unittest.main()
