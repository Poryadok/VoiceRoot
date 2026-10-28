"""Exercise the source probe against owned Linux PostgreSQL, not Kubernetes."""
import json
import subprocess
import tempfile
import unittest
from types import SimpleNamespace

import story_media_backup_capture as capture
import story_media_backup_integration_test as fixtures
from story_media_backup_capture_test import Journal


def cleanup_fixture(fixture):
    if not fixture.doCleanups():
        raise RuntimeError('fixture cleanup failed')


class FixtureCleanupTest(unittest.TestCase):
    def test_failure_is_reported(self):
        with self.assertRaises(RuntimeError):
            cleanup_fixture(SimpleNamespace(doCleanups=lambda: False))


class PhysicalProbeTest(unittest.TestCase):
    def test_probe_and_dump_against_disposable_postgres(self):
        fixture = fixtures.PopulatedRestoreTest()
        self.addCleanup(cleanup_fixture, fixture)
        fixture.setUp()
        fixture.sql(fixture.source, 'source_admin', 'postgres', 'CREATE DATABASE file_db')
        data_dir = fixture.sql(fixture.source, 'source_admin', 'postgres', 'SHOW data_directory')
        mount_path = '/var/lib/postgresql/data'
        expected = {'operation': 'synthetic', 'snapshot_id': 'synthetic', 'namespace_uid': 'ns',
                    'pod_uid': 'pod', 'statefulset_uid': 'sts', 'pvc_uid': 'pvc', 'pv_name': 'pv',
                    'pv_uid': 'pvu', 'container': 'postgres', 'container_id': fixture.source,
                    'restart_count': 0, 'mount_path': mount_path, 'data_directory': data_dir, 'pg_major': 16}
        objects = {
            'namespace': {'metadata': {'uid': 'ns'}},
            'pod': {'metadata': {'uid': 'pod', 'namespace': 'voice-dev',
                                 'ownerReferences': [{'uid': 'sts', 'kind': 'StatefulSet', 'controller': True}]},
                    'spec': {'containers': [{'name': 'postgres', 'volumeMounts': [{'name': 'data', 'mountPath': mount_path}]}],
                             'volumes': [{'name': 'data', 'persistentVolumeClaim': {'claimName': 'pgdata-voice-postgres-0'}}]},
                    'status': {'containerStatuses': [{'name': 'postgres', 'containerID': fixture.source,
                                                       'restartCount': 0, 'ready': True, 'state': {'running': {}}}]}},
            'pvc': {'metadata': {'uid': 'pvc', 'namespace': 'voice-dev'},
                    'spec': {'volumeName': 'pv'}, 'status': {'phase': 'Bound'}},
            'pv': {'metadata': {'uid': 'pvu'}, 'spec': {'claimRef': {'uid': 'pvc', 'namespace': 'voice-dev', 'name': 'pgdata-voice-postgres-0'}}},
        }
        def runner(command, **kwargs):
            if 'get' in command:
                return SimpleNamespace(returncode=0, stdout=json.dumps(objects[command[command.index('get') + 1]]).encode())
            index = command.index('--', command.index('exec'))
            return subprocess.run(['docker', 'exec', fixture.source, *command[index + 1:]], **kwargs)
        journal = Journal()
        # Synthetic authority callbacks only; this test cannot authorize live capture.
        producer = capture.Producer(expected, lambda: None, lambda: None, journal, runner=runner)
        with tempfile.TemporaryFile() as output:
            producer('file_db', output)
            output.seek(0)
            self.assertEqual(output.read(5), b'PGDMP')
        self.assertEqual(journal.events[-1][0], 'complete')


if __name__ == '__main__': unittest.main()
