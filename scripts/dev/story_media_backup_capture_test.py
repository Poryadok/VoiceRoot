import io
import json
import subprocess
import unittest
from types import SimpleNamespace

import story_media_backup_capture as capture


class Journal:
    def __init__(self):
        self.events = []
        self.pending = set()

    def begin(self, member):
        if member in self.pending:
            raise RuntimeError('replay refused')
        self.pending.add(member)
        self.events.append(('begin', member))

    def uncertain(self, member):
        self.events.append(('uncertain', member))

    def complete(self, member, receipt):
        self.events.append(('complete', member, receipt))


class CaptureTest(unittest.TestCase):
    def setUp(self):
        self.expected = {'operation': 'op', 'snapshot_id': 'snap', 'namespace_uid': 'ns', 'pod_uid': 'pod',
                         'statefulset_uid': 'sts', 'pvc_uid': 'pvc', 'pv_name': 'pv', 'pv_uid': 'pvu',
                         'container': 'postgres', 'container_id': 'containerd://abc', 'restart_count': 0,
                         'mount_path': '/var/lib/postgresql/data', 'data_directory': '/var/lib/postgresql/data/pgdata',
                         'pg_major': 16}
        self.journal = Journal()
        self.guards = []
        self.execs = 0
        self.fault = None
        self.reads = 0
        self.lost = False

    def guard(self):
        self.guards.append('guard')
        if self.lost: raise RuntimeError('authority lost')
    def drain(self): self.guards.append('drain')

    def runner(self, command, **kwargs):
        if 'exec' in command:
            if 'pg_read_file' in ' '.join(command):
                info = {'data_directory': '/var/lib/postgresql/data/pgdata', 'version': '160014',
                        'mountinfo': '42 1 0:42 / /var/lib/postgresql/data rw - ext4 /dev/x rw\n'}
                if self.fault == 'wrong_dir': info['data_directory'] = '/elsewhere'
                if self.fault == 'stacked': info['mountinfo'] *= 2
                return SimpleNamespace(returncode=0, stdout=json.dumps(info).encode())
            if 'readlink' in ' '.join(command):
                value = ('/var/lib/postgresql/data/pgdata\n/var/lib/postgresql/data\n'
                         '42 1 0:42 / /var/lib/postgresql/data rw - ext4 /dev/x rw\n')
                if self.fault == 'wrong_mount': value = value.replace('0:42', '0:43')
                if self.fault == 'stacked': value += '42 1 0:42 / /var/lib/postgresql/data rw - ext4 /dev/x rw\n'
                return SimpleNamespace(returncode=0, stdout=value.encode())
            self.execs += 1
            if self.fault == 'timeout': raise subprocess.TimeoutExpired(command, 90)
            if self.fault == 'exit':
                kwargs['stderr'].write(b'private database diagnostic')
                return SimpleNamespace(returncode=1, stdout=b'')
            kwargs['stdout'].write(b'private fixture')
            return SimpleNamespace(returncode=0, stdout=b'')
        self.reads += 1
        kind = command[command.index('get') + 1]
        if kind == 'pv' and ((self.fault == 'authority_pre' and not self.execs)
                             or (self.fault == 'authority_post' and self.execs)):
            self.lost = True
        objects = {
            'namespace': {'metadata': {'uid': 'ns'}},
            'pod': {'metadata': {'uid': 'changed' if self.fault == 'changed' and self.execs else 'pod',
                                 'namespace': 'voice-dev', 'ownerReferences': [{'uid': 'sts', 'kind': 'StatefulSet', 'controller': True}]},
                    'spec': {'containers': [{'name': 'postgres', 'volumeMounts': [{'name': 'data', 'mountPath': '/var/lib/postgresql/data'}]}], 'volumes': [{'name': 'data', 'persistentVolumeClaim': {'claimName': 'pgdata-voice-postgres-0'}}]},
                    'status': {'containerStatuses': [{'name': 'postgres', 'containerID': 'containerd://abc',
                                                       'restartCount': 1 if self.fault == 'restart' or (self.fault == 'restart_post' and self.execs) else 0,
                                                       'ready': True, 'state': {'running': {}}}]}},
            'pvc': {'metadata': {'uid': 'pvc', 'namespace': 'voice-dev'},
                    'spec': {'volumeName': 'pv'}, 'status': {'phase': 'Bound'}},
            'pv': {'metadata': {'uid': 'pvu'}, 'spec': {'claimRef': {'uid': 'pvc', 'namespace': 'voice-dev', 'name': 'pgdata-voice-postgres-0'}}},
        }
        return SimpleNamespace(returncode=0, stdout=json.dumps(objects[kind]).encode())

    def make(self):
        return capture.Producer(self.expected, self.guard, self.drain, self.journal, runner=self.runner)

    def test_stream_success_requires_guard_drain_and_identity_before_and_after(self):
        output = io.BytesIO()
        self.make()('file_db', output)
        self.assertEqual(output.getvalue(), b'private fixture')
        self.assertEqual(self.execs, 1)
        self.assertEqual(self.reads, 15)
        self.assertEqual(self.guards, ['guard', 'drain'] * 6)
        self.assertEqual(self.journal.events[0], ('begin', 'file_db'))
        self.assertEqual(self.journal.events[-1][0], 'complete')
        self.assertEqual(self.journal.events[-1][2], {'operation_id': 'op', 'member': 'file_db',
                         'snapshot_id': 'snap', 'pod_uid': 'pod', 'pvc_uid': 'pvc', 'pv_uid': 'pvu',
                         'container_id': 'containerd://abc', 'restart_count': 0, 'exit': 0,
                         'scope': 'dump-terminal-only'})
        self.assertNotIn('private fixture', repr(self.journal.events))

    def test_unknown_exit_or_changed_identity_remains_uncertain_and_no_replay(self):
        for fault in ('timeout', 'exit', 'changed', 'restart_post'):
            with self.subTest(fault=fault):
                self.setUp(); self.fault = fault
                with self.assertRaises(capture.CaptureUncertain) as error: self.make()('roles', io.BytesIO())
                self.assertNotIn('private database diagnostic', str(error.exception))
                self.assertEqual(self.journal.events[-1], ('uncertain', 'roles'))
                self.fault = None
                with self.assertRaises(capture.CaptureUncertain): self.make()('roles', io.BytesIO())
                self.assertEqual(self.execs, 1)

    def test_no_unrecognised_member_or_missing_authority(self):
        with self.assertRaises(capture.CaptureError): self.make()('auth_db', io.BytesIO())
        with self.assertRaises(capture.CaptureError):
            capture.Producer(self.expected, self.guard, None, self.journal, runner=self.runner)
        self.assertEqual(self.execs, 0)

    def test_authority_loss_during_last_identity_read_fails_closed(self):
        self.fault = 'authority_pre'
        with self.assertRaises(capture.CaptureError): self.make()('file_db', io.BytesIO())
        self.assertEqual(self.execs, 0)
        self.setUp(); self.fault = 'authority_post'
        with self.assertRaises(capture.CaptureUncertain): self.make()('file_db', io.BytesIO())
        self.assertEqual(self.execs, 1)
        self.assertEqual(self.journal.events[-1], ('uncertain', 'file_db'))
        self.assertNotIn('complete', [event[0] for event in self.journal.events])

    def test_physical_source_identity_must_match_before_capture(self):
        for fault in ('restart', 'wrong_dir', 'wrong_mount', 'stacked'):
            with self.subTest(fault=fault):
                self.setUp(); self.fault = fault
                with self.assertRaises(capture.CaptureError): self.make()('file_db', io.BytesIO())
                self.assertEqual(self.execs, 0)


if __name__ == '__main__': unittest.main()
