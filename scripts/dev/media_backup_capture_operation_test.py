import unittest
from unittest.mock import Mock, patch
from pathlib import Path
import copy
import contextlib
import io
import json
import tarfile
from types import SimpleNamespace

import media_backup_capture_operation as operation
import media_backup_phase as phase
import media_backup_phase_test as fixture
import media_operation_journal as journal
import story_media_backup as backup
import story_media_backup_capture as capture


class CaptureOperationTest(unittest.TestCase):
    def fixture(self):
        phases = Mock()
        capture = {'snapshot_id': 'capture-1', 'pod_uid': 'pod', 'pvc_uid': 'pvc',
                   'pv_uid': 'pv', 'container_id': 'container', 'restart_count': 0}
        expected = dict(capture, operation='media-c1-fixture', namespace_uid='ns', pg_major=16)
        phases.fence.context = {'operation_id': 'media-c1-fixture', 'commit': 'source',
            'project': 'voice', 'resource': 'DEV', 'actor': 'scheduled_devops',
            'claim': 'claim', 'window': 'window', 'generation': 1, 'revision': 2}
        phases.namespace_uid = 'ns'
        phases.intent = {'backup_capture': capture, 'backup_source': expected,
                         'backup_binding': dict(capture, operation_id='media-c1-fixture', source_sha='source',
                                                project='voice', resource='DEV', actor='scheduled_devops',
                                                claim='claim', window='window', generation=1, revision=2,
                                                namespace_uid='ns', restore_inputs={'postgres_major': 16})}
        return phases, expected, Path('/fixed/media-c1-fixture/backup')

    def test_same_producer_and_receipts_flow_to_upload(self):
        phases, expected, root = self.fixture()
        phases.journal.state = {'steps': {'dump-' + m: {'status': 'done', 'receipt': {'member': m}}
                                          for m in operation.MEMBERS}}
        producer = Mock()
        with patch.object(operation.upload, 'ROOT', Path('/fixed')), \
             patch.object(operation.capture, 'Producer', return_value=producer), \
             patch.object(operation.backup, 'export', return_value={'manifest': 1}) as export, \
             patch.object(operation.finalize, 'finalize', return_value=({'final': 1}, {'manifest': 2})) as finalize, \
             patch.object(operation.custody, 'build_bundle', return_value='/bundle') as bundle, \
             patch.object(operation.upload, 'prepare', return_value={'pending': 1}) as prepare:
            result = operation.run(root, b'k' * 32, expected, phases, Mock())
        self.assertEqual(result, {'pending': 1})
        self.assertIs(export.call_args.args[3], producer)
        self.assertEqual(set(finalize.call_args.args[4]), operation.MEMBERS)
        bundle.assert_called_once_with(root, b'k' * 32, {'final': 1}, {'manifest': 2})
        prepare.assert_called_once_with(phases, b'k' * 32, '/bundle')

    def test_failure_retains_fence_and_never_prepares_upload(self):
        phases, expected, root = self.fixture()
        with patch.object(operation.upload, 'ROOT', Path('/fixed')), patch.object(operation.capture, 'Producer'), \
             patch.object(operation.backup, 'export', side_effect=RuntimeError('private')), \
             patch.object(operation.upload, 'prepare') as prepare:
            with self.assertRaises(operation.CaptureOperationError):
                operation.run(root, b'k' * 32, expected, phases, Mock())
        phases.fence.uncertain.assert_called_once()
        self.assertTrue(phases.poisoned)
        prepare.assert_not_called()

    def test_mismatched_source_or_root_never_starts_capture(self):
        for bad_source in (True, False):
            phases, expected, root = self.fixture()
            if bad_source:
                expected = dict(expected, pod_uid='other')
            else:
                root = Path('/wrong')
            with patch.object(operation.upload, 'ROOT', Path('/fixed')), \
                 patch.object(operation.capture, 'Producer') as producer, \
                 patch.object(operation.backup, 'export') as export:
                with self.assertRaises(operation.CaptureOperationError):
                    operation.run(root, b'k' * 32, expected, phases, Mock())
            producer.assert_not_called()
            export.assert_not_called()


class RealComponentsTest(unittest.TestCase):
    def test_real_producer_ciphertext_and_reservation(self):
        self.run_real(False)

    def test_encryption_failure_after_dump_prevents_reservation(self):
        self.run_real(True)

    def run_real(self, fail):
        base = fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        base.setUp()
        self.addCleanup(base.doCleanups)
        uid = '33333333-3333-4333-8333-333333333333'
        source = dict(base.capture, pod_uid=uid, pvc_uid=uid, pv_uid=uid)
        expected = dict(source, operation=base.context['operation_id'], namespace_uid=uid,
                        statefulset_uid=uid, pv_name='pv', container='postgres',
                        mount_path='/var/lib/postgresql/data', data_directory='/var/lib/postgresql/data/pgdata', pg_major=16)
        binding = dict(source, operation_id=base.context['operation_id'], source_sha=base.context['commit'],
                       namespace_uid=uid, **{k: base.context[k] for k in
                       ('project', 'resource', 'actor', 'claim', 'window', 'generation', 'revision')},
                       restore_inputs={'catalog_sha256': '1'*64, 'roles_catalog_sha256': '2'*64,
                           'migration_catalog_sha256': '3'*64, 'migrate_binary_sha256': '4'*64,
                           'postgres_image': 'example/postgres@sha256:'+'5'*64,
                           'migrate_image': 'example/migrate@sha256:'+'6'*64, 'postgres_major': 16})
        intent = dict(base.intent, namespace_uid=uid, backup_capture=source, backup_source=expected,
                      backup_binding=binding, upload_workflow_sha=base.context['commit'])
        j = journal.Journal(Path(base.temp.name) / 'real-journal', intent)
        self.addCleanup(j.close)
        client = fixture.Client(base.lock)
        original_call = client.call
        def call(*args):
            result = original_call(*args)
            if args[1] == 'namespace': result['metadata']['uid'] = uid
            return result
        client.call = call
        phases = phase.BackupPhases(j, base.fence, client, base.lock, uid)
        root = Path(base.temp.name) / base.context['operation_id'] / 'backup'
        root.mkdir(parents=True, mode=0o700)
        objects = {
          'namespace': {'metadata': {'uid': uid}},
          'pod': {'metadata': {'uid': uid, 'namespace': 'voice-dev', 'ownerReferences': [{'uid': uid, 'kind':'StatefulSet', 'controller':True}]},
            'spec': {'containers':[{'name':'postgres','volumeMounts':[{'name':'data','mountPath':expected['mount_path']}]}],
                     'volumes':[{'name':'data','persistentVolumeClaim':{'claimName':'pgdata-voice-postgres-0'}}]},
            'status': {'containerStatuses':[{'name':'postgres','containerID':source['container_id'],
                'restartCount':1,'ready':True,'state':{'running':{}}}]}},
          'pvc': {'metadata':{'uid':uid,'namespace':'voice-dev'},'spec':{'volumeName':'pv'},'status':{'phase':'Bound'}},
          'pv': {'metadata':{'uid':uid},'spec':{'claimRef':{'uid':uid,'namespace':'voice-dev','name':'pgdata-voice-postgres-0'}}}}
        mount = '42 1 0:42 / /var/lib/postgresql/data rw - ext4 /dev/x rw\n'
        def runner(command, **kwargs):
            text = ' '.join(command)
            if 'get' in command:
                return SimpleNamespace(returncode=0, stdout=json.dumps(objects[command[command.index('get')+1]]).encode())
            if 'pg_read_file' in text:
                return SimpleNamespace(returncode=0, stdout=json.dumps({'data_directory':expected['data_directory'], 'version':'160014','mountinfo':mount}).encode())
            if 'readlink' in text:
                return SimpleNamespace(returncode=0, stdout=(expected['data_directory']+'\n'+expected['mount_path']+'\n'+mount).encode())
            kwargs['stdout'].write(b'synthetic source bytes')
            return SimpleNamespace(returncode=0, stdout=b'')
        real_producer = capture.Producer
        key = bytes(range(32))
        fault = patch.object(backup, 'stream_identity', side_effect=OSError('synthetic')) if fail else contextlib.nullcontext()
        with patch.object(operation.upload, 'ROOT', Path(base.temp.name)), \
             patch.object(operation.capture, 'Producer', side_effect=lambda e,g,d,p: real_producer(e,g,d,p,runner=runner)), fault:
            if fail:
                with self.assertRaises(operation.CaptureOperationError):
                    operation.run(root, key, expected, phases, lambda: None)
                self.assertTrue(phases.poisoned)
                self.assertEqual(base.ledger.row['status'], 'uncertain')
                self.assertNotIn('backup_upload', base.ledger.row)
                self.assertEqual(len(base.ledger.row['backup_members']), 1)
                return
            planned = operation.run(root, key, expected, phases, lambda: None)
        self.assertEqual(planned['status'], 'dispatch-pending')
        final_binding = binding | {'dump_receipts': {m: base.ledger.row['backup_members'][m]['receipt_sha256'] for m in operation.MEMBERS}}
        # The authenticated ciphertext decrypts to the bytes produced by the real adapter.
        with tarfile.open(root / 'media-backup.tar') as archive:
            manifest = json.load(archive.extractfile('manifest.json'))
        for member in operation.MEMBERS:
            output = io.BytesIO()
            backup.decrypt(root, key, final_binding, manifest, member, output)
            self.assertEqual(output.getvalue(), b'synthetic source bytes')


if __name__ == '__main__':
    unittest.main()
