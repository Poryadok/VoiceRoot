import copy
import types
import unittest
from unittest import mock

import writer_cut
import writer_cut_migration as migration
import writer_cut_test as fixture


class MigrationCutTest(unittest.TestCase):
    def setUp(self):
        fixture.WriterCutTest.setUp(self)
        self.cut.drain()
        self.state = {'job': {'authored': {'metadata': {'name': 'migration'}}, 'state': 'absent',
                              'uid': None, 'resourceVersion': None},
                      'configmap': {'uid': 'config-uid'},
                      'stable': {'operation_id': self.fence.context['operation_id'],
                                 'source_sha': self.fence.context['commit'], 'nonce': 'a' * 32},
                      'execution': {'database': 'auth_db'}, 'terminal': None}
        self.pod = None
        self.phase = types.SimpleNamespace(journal=types.SimpleNamespace(state={'steps': {}}))
        self.normal = mock.patch.object(migration, 'normalized', side_effect=lambda *args: (None, copy.deepcopy(self.state)))
        self.observer = mock.patch.object(migration, 'observe', side_effect=lambda *args, **kwargs: (copy.deepcopy(self.state), copy.deepcopy(self.pod)))
        self.session_patch = mock.patch.object(migration, 'sessions', side_effect=lambda *args: {
            'scope': copy.deepcopy(self.admission['session_scope']), 'active_sessions': 0,
            'active_transactions': 0, 'attributed_sessions': []})
        for patcher in (self.normal, self.observer, self.session_patch):
            patcher.start(); self.addCleanup(patcher.stop)

    def bind(self):
        self.cut.bind_migration('realtime', self.phase, '.')

    def test_registered_job_terminal_receipt_and_effect_map(self):
        self.bind()
        with self.assertRaises(writer_cut.RecoveryRequired):
            self.cut.current_cut_proof()

    def test_uid_replacement_after_owned_holds(self):
        self.bind()
        self.state['job'].update(state='owned', uid='first', resourceVersion='1')
        self.cut.verify_migration_running()
        self.state['job']['uid'] = 'replacement'
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.verify_migration_running()
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_terminal_binds_observed_pod_and_journal(self):
        self.bind()
        self.state['job'].update(state='done', uid='first', resourceVersion='1')
        self.state['terminal'] = 'f' * 64
        self.pod = {'uid': 'pod-uid', 'ip': '10.1.2.3'}
        self.phase.journal.state['steps']['job'] = {'receipt': {'job_uid': 'first', 'pod_uid': 'pod-uid'}}
        self.cut.verify_migration_running()
        receipt = self.cut.finish_migration()
        self.assertEqual(receipt['job_uid'], 'first')
        self.assertTrue(self.cut.migration['finished'])
        self.assertEqual(self.ledger.row['writer_cut_effects']['writer-cut-migration-finish-' + 'a' * 32]['status'], 'done')

    def test_wrong_terminal_pod_holds(self):
        self.bind()
        self.state['job'].update(state='done', uid='first', resourceVersion='1')
        self.state['terminal'] = 'f' * 64
        self.pod = {'uid': 'pod-uid', 'ip': '10.1.2.3'}
        self.phase.journal.state['steps']['job'] = {'receipt': {'job_uid': 'first', 'pod_uid': 'other'}}
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.finish_migration()
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_resume_cannot_restore_writers_during_migration(self):
        self.bind()
        self.state['job'].update(state='owned', uid='first', resourceVersion='1')
        before = len(self.client.patches)
        targets = [copy.deepcopy(obj) for obj in self.client.workloads.values()]
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.resume(targets)
        self.assertEqual(len(self.client.patches), before)
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_guard_rejects_replaced_terminal_pod(self):
        self.normal.stop(); self.observer.stop()
        job = {'metadata': {'name': 'migration', 'namespace': 'voice-dev', 'uid': 'job-uid', 'resourceVersion': '1'},
               'spec': {}, 'status': {'conditions': [{'type': 'Complete', 'status': 'True'}]}}
        current_phase = ['Succeeded']
        def pod(uid):
            return {'metadata': {'uid': uid, 'ownerReferences': [{'controller': True, 'kind': 'Job', 'uid': 'job-uid'}]},
                    'status': {'podIP': '10.1.2.3', 'phase': current_phase[0], 'containerStatuses': [
                        {'state': {'terminated': {'exitCode': 0}}}]}}
        self.state['job'].update(authored={'metadata': {'name': 'migration'}}, state='done', uid='job-uid')
        self.state['terminal'] = 'f' * 64
        self.state['execution'] = {'node_name': 'node', 'node_uid': 'node-uid'}
        self.state['configmap'] = {'uid': 'config-uid', 'receipt': {'uid': 'config-uid'}}
        module = types.SimpleNamespace(verify_writer_job=lambda *args: {},
            verify_writer_pod=lambda authored, obj, p, execution: {'pod_uid': p['metadata']['uid'], 'pod_ip': '10.1.2.3'})
        binding = {'kind': 'realtime', 'phase': self.phase, 'root': '.', 'finished': True,
                   'job_uid': 'job-uid', 'pod_uid': 'original', 'config_uid': 'config-uid',
                   'plan': {'authored': self.state['job']['authored'], 'config': self.state['configmap']['receipt'],
                            'execution': self.state['execution'], 'stable': self.state['stable']}}
        current_uid = ['original']
        def call(*args):
            return {'items': {'jobs': [job], 'pods': [pod(current_uid[0])], 'nodes': [
                {'metadata': {'name': 'node', 'uid': 'node-uid'}, 'status': {'conditions': [
                    {'type': 'Ready', 'status': 'True'}]}}]}[args[1]]}
        with mock.patch.object(migration, 'normalized', return_value=(module, self.state)), \
             mock.patch.object(self.client, 'call', side_effect=call):
            migration.observe(self.cut, binding)
            current_phase[0] = 'Running'
            with self.assertRaises(ValueError): migration.observe(self.cut, binding)
            current_phase[0] = 'Succeeded'
            current_uid[0] = 'replacement'
            with self.assertRaises(ValueError): migration.observe(self.cut, binding)

    def test_resumed_proof_contains_only_done_current_targets(self):
        source = copy.deepcopy(self.client.workloads[('Deployment', 'voice-a')])
        self.cut._effect('resume', source, 2)
        proof = self.cut.current_resumed_proof()
        self.assertEqual([(x['kind'], x['name'], x['replicas']) for x in proof['controllers']],
                         [('Deployment', 'voice-a', 2)])
        self.assertEqual(proof['controllers'][0]['uid'], 'uid-a')
        self.client.workloads[('Deployment', 'voice-a')]['spec']['template']['spec']['containers'][0]['image'] = 'foreign'
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.current_resumed_proof()


if __name__ == '__main__': unittest.main()
