import unittest
import copy
import contextlib
import io
import json
from types import SimpleNamespace
from unittest.mock import Mock, patch
from pathlib import Path
import story_media_backup_cohort_finalize_test as fixture
import media_backup_phase_test as phase_fixture
import media_backup_phase as media
import media_operation_journal as journal
import cohort_backup_phases as phases
import story_media_backup_cohort_capture as real_capture
import story_media_backup_cohort as cohort
import cohort_backup_operation as operation
import cohort_upload_phase as upload


class OperationTest(unittest.TestCase):
    def test_bad_source_timestamp_or_restart_refused_before_invocation(self):
        base = fixture.FinalizeCohortTest('test_exact_receipts_are_bound_without_mutating_inputs')
        base.setUp(); self.addCleanup(base.doCleanups)
        for fault in ('timestamp', 'restart'):
            with self.subTest(fault=fault):
                binding = copy.deepcopy(base.binding)
                for source in binding['sources'].values(): source['server_start'] = '2026-10-28T20:00:00Z'
                if fault == 'timestamp':
                    for source in binding['sources'].values(): source['server_start'] = 'bad'
                if fault == 'restart':
                    for source in binding['sources'].values(): source['engine']['restart_count'] = 1
                engine = binding['sources']['roles']['engine']
                expected = dict(engine, operation=binding['operation_id'], snapshot_id=binding['snapshot_id'],
                    pg_major=16, statefulset_uid='statefulset', pv_name='pv', container='postgres',
                    mount_path='/var/lib/postgresql/data', data_directory='/var/lib/postgresql/data')
                if fault == 'restart': expected['restart_count'] = True
                p = Mock(); phase = Mock(); phase.phases = p; phase.binding = binding
                p.fence.context = {'operation_id':binding['operation_id'], **{k:binding[k] for k in
                    ('project','resource','actor','claim','window','generation','revision')}}
                p.intent = {'cohort_source': copy.deepcopy(expected), 'cohort_admin_role':'voice'}
                root = Path('/fixed') / binding['operation_id'] / 'cohort-backup'
                with patch.object(operation, 'ROOT', Path('/fixed')), patch.object(operation.cohort, 'export') as export:
                    with self.assertRaises(operation.CohortOperationError):
                        operation.run(phase, root, b'x'*32, expected, 'voice', lambda: None)
                p._start.assert_not_called(); p.fence.uncertain.assert_not_called(); export.assert_not_called()

    def test_wrong_source_or_path_never_reads_data(self):
        base = fixture.FinalizeCohortTest('test_exact_receipts_are_bound_without_mutating_inputs')
        base.setUp(); self.addCleanup(base.doCleanups)
        for fault in ('source', 'path', 'digest', 'image', 'major'):
            with self.subTest(fault=fault):
                binding = copy.deepcopy(base.binding)
                expected = dict(binding['sources']['roles']['engine'], operation=binding['operation_id'],
                                snapshot_id=binding['snapshot_id'], pg_major=16)
                p = Mock(); phase = Mock(); phase.phases = p; phase.binding = binding
                p.fence.context = {'operation_id':binding['operation_id'], **{k:binding[k] for k in
                    ('project','resource','actor','claim','window','generation','revision')}}
                p.intent = {'cohort_source': copy.deepcopy(expected), 'cohort_admin_role':'voice'}
                root = Path('/fixed') / binding['operation_id'] / 'cohort-backup'
                if fault == 'source': expected['pod_uid'] = 'wrong'
                if fault == 'path': root = Path('/wrong')
                if fault == 'digest': binding['restore_inputs']['catalog_sha256'] = 'wrong'
                if fault == 'image': binding['restore_inputs']['postgres_image'] = 'wrong'
                if fault == 'major': binding['restore_inputs']['postgres_major'] = 15
                with patch.object(operation, 'ROOT', Path('/fixed')), \
                     patch.object(operation.capture, 'Producer') as producer, \
                     patch.object(operation.cohort, 'export') as export:
                    with self.assertRaises(operation.CohortOperationError):
                        operation.run(phase, root, b'x'*32, expected, 'voice', lambda: None)
                producer.assert_not_called(); export.assert_not_called()
                p._start.assert_not_called(); p.fence.uncertain.assert_not_called()

    def test_real_guarded_producer_encrypts_distinct_members(self):
        self.run_real(False)

    def test_post_dump_encryption_failure_is_uncertain(self):
        self.run_real('encryption')

    def test_changed_ledger_prevents_bundle(self):
        self.run_real('ledger')

    def test_lost_bundle_marker_response_is_uncertain(self):
        self.run_real('marker')

    def test_drift_during_terminal_save_is_uncertain(self):
        self.run_real('terminal')

    def run_real(self, fail):
        base = phase_fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        base.setUp(); self.addCleanup(base.doCleanups)
        template = fixture.FinalizeCohortTest('test_exact_receipts_are_bound_without_mutating_inputs')
        template.setUp(); self.addCleanup(template.doCleanups)
        binding = copy.deepcopy(template.binding)
        binding.update(operation_id=base.context['operation_id'], source_sha=base.context['commit'],
                       snapshot_id=base.capture['snapshot_id'])
        for k in ('project','resource','actor','claim','window','generation','revision'): binding[k] = base.context[k]
        for source in binding['sources'].values(): source['server_start'] = '2026-10-28T20:00:00Z'
        engine = binding['sources']['roles']['engine']
        capture_id = {'snapshot_id':binding['snapshot_id'], **{k:engine[k] for k in
                      ('pod_uid','pvc_uid','pv_uid','container_id','restart_count')}}
        expected = dict(engine, operation=binding['operation_id'], snapshot_id=binding['snapshot_id'],
            pg_major=16, statefulset_uid='sts', pv_name='pv', container='postgres',
            mount_path='/var/lib/postgresql/data', data_directory='/var/lib/postgresql/data/pgdata')
        intent = base.intent | {'namespace_uid':engine['namespace_uid'], 'backup_capture':capture_id,
            'cohort_binding':binding, 'cohort_source':expected, 'cohort_admin_role':'source_admin',
            'upload_workflow_sha':base.context['commit'], 'kube_lock':base.lock}
        root = Path(base.temp.name)/binding['operation_id']/'cohort-backup'
        root.parent.mkdir(mode=0o700); root.mkdir(mode=0o700)
        j = journal.Journal(root.parent/'journal', intent); self.addCleanup(j.close)
        client = phase_fixture.Client(base.lock); original_call = client.call
        def call(*args):
            result = original_call(*args)
            if args[1] == 'namespace': result['metadata']['uid'] = engine['namespace_uid']
            return result
        client.call = call
        p = media.BackupPhases(j, base.fence, client, base.lock, engine['namespace_uid'])
        phase = phases.CohortPhases(p)
        mount = '42 1 0:42 / /var/lib/postgresql/data rw - ext4 /dev/x rw\n'
        objects = {'namespace': {'metadata': {'uid':engine['namespace_uid']}},
            'pod': {'metadata': {'uid':engine['pod_uid'], 'namespace':'voice-dev',
                     'ownerReferences':[{'uid':'sts','kind':'StatefulSet','controller':True}]},
                    'spec': {'containers':[{'name':'postgres','volumeMounts':[{'name':'data','mountPath':expected['mount_path']}]}],
                             'volumes':[{'name':'data','persistentVolumeClaim':{'claimName':'pgdata-voice-postgres-0'}}]},
                    'status': {'containerStatuses':[{'name':'postgres','containerID':engine['container_id'],
                        'restartCount':engine['restart_count'],'ready':True,'state':{'running':{}}}]}},
            'pvc': {'metadata':{'uid':engine['pvc_uid'],'namespace':'voice-dev'},'spec':{'volumeName':'pv'},'status':{'phase':'Bound'}},
            'pv': {'metadata':{'uid':engine['pv_uid']},'spec':{'claimRef':{'uid':engine['pvc_uid'],
                    'namespace':'voice-dev','name':'pgdata-voice-postgres-0'}}}}
        seen = []
        def runner(command, **kwargs):
            text = ' '.join(command)
            if 'get' in command:
                return SimpleNamespace(returncode=0, stdout=json.dumps(objects[command[command.index('get')+1]]).encode())
            if 'pg_postmaster_start_time' in text:
                db = command[-2]; oid = {'auth_db':1,'voice_db':2,'postgres':3}[db]
                return SimpleNamespace(returncode=0, stdout=json.dumps({'database':db,'database_oid':oid,
                    'role':'source_admin','server_start':'2026-10-28T20:00:00Z'}).encode())
            if 'pg_read_file' in text:
                return SimpleNamespace(returncode=0, stdout=json.dumps({'data_directory':expected['data_directory'],
                    'version':'160014','mountinfo':mount}).encode())
            if 'readlink' in text:
                return SimpleNamespace(returncode=0, stdout=(expected['data_directory']+'\n'+expected['mount_path']+'\n'+mount).encode())
            member = command[-1]; seen.append(member); kwargs['stdout'].write(('synthetic-'+member).encode())
            return SimpleNamespace(returncode=0, stdout=b'')
        producer_class = real_capture.Producer
        fault = patch.object(cohort.backup, 'stream_identity', side_effect=OSError('synthetic')) if fail == 'encryption' else contextlib.nullcontext()
        original_finalize = operation.finalize.finalize
        def change_ledger(*args):
            result = original_finalize(*args)
            row, rev = base.ledger.read()
            row['cohort_members']['voice_db']['receipt_sha256'] = '0'*64
            base.ledger.write(rev, row)
            return result
        drift = patch.object(operation.finalize, 'finalize', side_effect=change_ledger) if fail == 'ledger' else contextlib.nullcontext()
        original_write = base.ledger.write
        def lost_marker(revision, row):
            original_write(revision, row)
            if 'cohort_bundle' in row: raise OSError('synthetic lost reply')
        lost = patch.object(base.ledger, 'write', side_effect=lost_marker) if fail == 'marker' else contextlib.nullcontext()
        original_save = j.save
        def drift_terminal():
            original_save()
            if j.state['steps'].get('cohort-bundle', {}).get('status') == 'done':
                row, rev = base.ledger.read()
                row['cohort_bundle']['bundle_sha256'] = '0'*64
                base.ledger.write(rev, row)
        save_fault = patch.object(j, 'save', side_effect=drift_terminal) if fail == 'terminal' else contextlib.nullcontext()
        with patch.object(operation, 'ROOT', Path(base.temp.name)), \
             patch.object(operation.capture, 'Producer', side_effect=lambda b,d,e,a,g,dr,j: producer_class(b,d,e,a,g,dr,j,runner)), fault, drift, lost, save_fault:
            if fail:
                with self.assertRaises(operation.CohortOperationError):
                    operation.run(phase, root, b'x'*32, expected, 'source_admin', lambda: None)
                self.assertTrue(p.poisoned)
                self.assertEqual(base.ledger.row['status'], 'uncertain')
                self.assertEqual(len(base.ledger.row['cohort_members']), 1 if fail == 'encryption' else 3)
                if fail in ('encryption','ledger'): self.assertFalse((root/'cohort-backup.tar').exists())
                with patch.object(upload, 'ROOT', Path(base.temp.name)):
                    with self.assertRaises(upload.CohortUploadError): upload.prepare(phase,b'x'*32,root/'cohort-backup.tar')
                return
            bundle, final_binding, manifest = operation.run(phase, root, b'x'*32, expected, 'source_admin', lambda: None)
        self.assertEqual(set(seen), {'auth_db','voice_db','roles'}); self.assertEqual(len(seen), 3)
        self.assertTrue(bundle.is_file())
        with patch.object(upload, 'ROOT', Path(base.temp.name)):
            planned = upload.prepare(phase, b'x'*32, bundle)
        self.assertEqual(planned['status'], 'dispatch-pending')
        for member in seen:
            output = io.BytesIO()
            cohort.decrypt(root, b'x'*32, final_binding, binding['databases'], manifest, member, output)
            self.assertEqual(output.getvalue(), ('synthetic-'+member).encode())
        return {'base':base,'journal':j,'phase':phase,'binding':final_binding,'bundle':bundle,
                'root':root,'planned':planned,'key':b'x'*32}


if __name__ == '__main__': unittest.main()
