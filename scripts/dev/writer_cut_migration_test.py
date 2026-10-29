import copy
import sys
import types
import unittest
from unittest import mock

import writer_cut
import writer_cut_migration as migration
import writer_cut_test as fixture
import full_cohort_inventory_phase
import writer_cut_pg_probe
import auth_live_migration_phase
import media_operation_journal
from pathlib import Path


class MigrationCutTest(unittest.TestCase):
    def setUp(self):
        fixture.WriterCutTest.setUp(self)
        self.cut.drain()
        self.state = {'job': {'authored': {'metadata': {'name': 'migration'}}, 'state': 'absent',
                              'uid': None, 'resourceVersion': None},
                      'configmap': {'uid': 'config-uid'},
                      'stable': {'operation_id': self.fence.context['operation_id'],
                                 'source_sha': self.fence.context['commit'], 'nonce': 'a' * 32},
                      'execution': {'database': 'auth_db', 'database_oid': 10}, 'terminal': None}
        self.pod = None
        self.phase = types.SimpleNamespace(journal=types.SimpleNamespace(state={'steps': {}}))
        self.scope_match_patch = mock.patch.object(self.cut, '_effective_scope_matches', return_value=True)
        self.scope_match_patch.start(); self.addCleanup(self.scope_match_patch.stop)
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
        for obj in self.client.workloads.values():
            obj['spec']['template']['spec']['containers'][0]['image'] = 'new-target-image'
            obj['metadata']['generation'] += 1
            obj['status']['observedGeneration'] = obj['metadata']['generation']
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

    def test_resumed_proof_accepts_admitted_replacement_controller(self):
        current = self.client.workloads[('Deployment', 'voice-a')]
        current['metadata']['uid'] = 'new-target-uid'
        self.cut._effect('resume', copy.deepcopy(current), 2)
        self.assertEqual(self.cut.current_resumed_proof()['controllers'][0]['uid'], 'new-target-uid')

    def test_bound_inventory_and_fixed_observer_do_not_persist_token(self):
        class Inventory:
            def __init__(inner):
                inner.fence, inner.client, inner.lock = self.fence, self.client, copy.deepcopy(self.lock)
                inner.namespace_uid = 'ns'
                receipt = {'operation_id': self.context['operation_id'], 'qualified': True}
                inner.journal = types.SimpleNamespace(poisoned=False, dirfd=1, lockfd=1,
                    state={'steps': {full_cohort_inventory_phase.STEP: {'status': 'done', 'receipt': receipt}}})
                self.ledger.row['full_cohort_inventory'] = {'status': 'done'}
            def verify_receipt(inner, token):
                self.assertEqual(token, 'private-token-marker')
                return {'operation_id': self.context['operation_id'], 'qualified': True}
        phase = Inventory()
        empty = {'scope': self.admission['session_scope'], 'active_sessions': 0,
                 'active_transactions': 0, 'attributed_sessions': []}
        with mock.patch.object(full_cohort_inventory_phase, 'InventoryPhase', Inventory), \
             mock.patch.object(writer_cut_pg_probe, 'inspect_resumed', return_value=empty, create=True):
            self.cut.bind_resume_inventory(phase, 'private-token-marker')
        self.assertNotIn('private-token-marker', repr(self.journal.state))
        source = copy.deepcopy(self.client.workloads[('Deployment', 'voice-a')])
        self.cut._effect('resume', source, 2)
        def observed(cut, actual_phase, token):
            self.assertIs(cut, self.cut)
            self.assertIs(actual_phase, phase)
            self.assertEqual(token, 'private-token-marker')
            self.assertEqual(cut.current_resumed_proof()['controllers'][0]['name'], 'voice-a')
            return {'scope': self.admission['session_scope'], 'active_sessions': 0,
                    'active_transactions': 0, 'attributed_sessions': [{'fixture': 'writer'}]}
        with mock.patch.object(writer_cut_pg_probe, 'inspect_resumed', side_effect=observed, create=True):
            self.assertEqual(len(self.cut._resumed_sessions()['attributed_sessions']), 1)
        for field in ('active_sessions', 'active_transactions'):
            bad = observed(self.cut, phase, 'private-token-marker'); bad[field] = 1
            with mock.patch.object(writer_cut_pg_probe, 'inspect_resumed', return_value=bad, create=True):
                with self.assertRaises(writer_cut.RecoveryRequired): self.cut._resumed_sessions()

    def resume_with_observer(self, unknown, drift=False):
        self.bind()
        self.state['job'].update(state='done', uid='first', resourceVersion='1')
        self.state['terminal'] = 'f' * 64
        self.pod = {'uid': 'pod-uid', 'ip': '10.1.2.3'}
        self.phase.journal.state['steps']['job'] = {'receipt': {'job_uid': 'first', 'pod_uid': 'pod-uid'}}
        self.cut.verify_migration_running()
        self.cut.finish_migration()
        class Inventory:
            def __init__(inner):
                inner.fence, inner.client, inner.lock = self.fence, self.client, copy.deepcopy(self.lock)
                inner.namespace_uid = 'ns'
                inner.journal = types.SimpleNamespace(poisoned=False, dirfd=1, lockfd=1,
                    state={'steps': {full_cohort_inventory_phase.STEP: {'status': 'done', 'receipt': {'qualified': True}}}})
                self.ledger.row['full_cohort_inventory'] = {'status': 'done'}
            def verify_receipt(inner, token): return {'qualified': True}
        empty = {'scope': self.admission['session_scope'], 'active_sessions': 0,
                 'active_transactions': 0, 'attributed_sessions': []}
        with mock.patch.object(full_cohort_inventory_phase, 'InventoryPhase', Inventory), \
             mock.patch.object(writer_cut_pg_probe, 'inspect_resumed', return_value=empty, create=True):
            self.cut.bind_resume_inventory(Inventory(), 'token')
        counts = []
        def observer(cut, phase, token):
            counts.append(len(cut.current_resumed_proof()['controllers']))
            return {'scope': self.admission['session_scope'], 'active_sessions': int(unknown),
                    'active_transactions': 0, 'attributed_sessions': [{'fixture': 'first-writer'}]}
        for obj in self.client.workloads.values():
            obj['spec']['template']['spec']['containers'][0]['image'] = 'new-target-image'
            obj['metadata']['generation'] += 1
            obj['status']['observedGeneration'] = obj['metadata']['generation']
        targets = [copy.deepcopy(obj) for obj in self.client.workloads.values()]
        guard = self.cut._migration_resume_guard
        def after_guard():
            guard()
            if drift and self.cut._has_resumed_effect():
                self.ledger.row['full_cohort_inventory'] = {'status': 'foreign'}
                self.ledger.version += 1
        with mock.patch.object(writer_cut_pg_probe, 'inspect_resumed', side_effect=observer, create=True), \
             mock.patch.object(self.cut, '_migration_resume_guard', side_effect=after_guard):
            if unknown or drift:
                with self.assertRaises(writer_cut.RecoveryRequired): self.cut.resume(targets)
            else:
                self.cut.resume(targets)
        self.assertTrue(counts)
        self.assertTrue(all(count == 1 for count in counts))
        self.assertEqual(self.client.workloads[('StatefulSet', 'voice-b')]['spec']['replicas'], 0 if unknown or drift else 1)
        if unknown or drift: self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_resume_uses_qualified_observer_after_first_cas(self):
        self.resume_with_observer(False)

    def test_unknown_session_blocks_second_resume_cas(self):
        self.resume_with_observer(True)

    def test_inventory_marker_drift_after_observer_blocks_next_cas(self):
        self.resume_with_observer(False, drift=True)

    def test_missing_fixed_observer_holds_before_resume_mutation(self):
        class Inventory:
            def __init__(inner):
                inner.fence, inner.client, inner.lock = self.fence, self.client, copy.deepcopy(self.lock)
                inner.namespace_uid = 'ns'
                inner.journal = types.SimpleNamespace(poisoned=False, dirfd=1, lockfd=1,
                    state={'steps': {full_cohort_inventory_phase.STEP: {'status': 'done', 'receipt': {'qualified': True}}}})
                self.ledger.row['full_cohort_inventory'] = {'status': 'done'}
            def verify_receipt(inner, token): return {'qualified': True}
        before = len(self.client.patches)
        with mock.patch.object(full_cohort_inventory_phase, 'InventoryPhase', Inventory), \
             mock.patch.object(writer_cut_pg_probe, 'inspect_resumed', side_effect=RuntimeError('not qualified'), create=True):
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.bind_resume_inventory(Inventory(), 'token')
        self.assertEqual(len(self.client.patches), before)
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_previous_migration_does_not_reject_current_owned_session(self):
        scope = self.admission['session_scope'] | {'database_oids': {'auth_db': 10}}
        self.admission['session_scope'] = copy.deepcopy(scope)
        self.cut.admission['session_scope'] = copy.deepcopy(scope)
        self.cut.intent['writer_cut']['session_scope'] = copy.deepcopy(scope)
        self.journal.intent['writer_cut']['session_scope'] = copy.deepcopy(scope)
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.bind()
        self.cut.migration_history.append({'kind': 'auth', 'phase': object(), 'root': '.'})
        with mock.patch.object(migration, 'verify_cleaned'), \
             mock.patch.object(migration, 'sessions', return_value={
                 'scope': scope, 'active_sessions': 0, 'active_transactions': 0,
                 'attributed_sessions': [{'fixture': 'current-job'}]}) as observer:
            self.cut._guards()
            self.assertEqual(observer.call_count, 1)
            self.state['execution']['database_oid'] = 11
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut._guards()

    def test_completed_auth_successor_rechecks_source_journal_and_ledger(self):
        source = media_operation_journal.Journal(Path(self.temp.name) / 'auth-successor', {
            'resource_context': self.context, 'namespace_uid': 'ns'})
        self.addCleanup(source.close)
        phase = types.SimpleNamespace(phases=types.SimpleNamespace(journal=source))
        self.ledger.row['auth_live_c2'] = {'status': 'done', 'nonce': 'a' * 32}
        with mock.patch.object(auth_live_migration_phase, 'verify_completed', return_value={'status': 'no-op'}):
            bound = self.cut.bind_auth_successor(phase, '.')
            self.assertEqual(bound['receipt_sha256'], writer_cut.digest({'status': 'no-op'}))
            self.cut._guards()
            self.ledger.row['auth_live_c2']['nonce'] = 'b' * 32
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut._guards()


class EffectiveScopeTest(unittest.TestCase):
    def setUp(self):
        fixture.WriterCutTest.setUp(self)
        scope = {'engine': {'uid': 'pg'}, 'databases': ['auth_db'], 'database_oids': {'auth_db': 10}, 'server_start': 'start'}
        self.admission['session_scope'] = copy.deepcopy(scope)
        self.cut.admission = copy.deepcopy(self.admission)
        self.journal.intent['writer_cut'] = copy.deepcopy(self.admission)
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.cut.intent = copy.deepcopy(self.journal.intent)
        self.cut.drain()
        source = media_operation_journal.Journal(Path(self.temp.name) / 'realtime-source', {
            'resource_context': self.context, 'namespace_uid': 'ns'})
        self.addCleanup(source.close)
        phase = types.SimpleNamespace(invocation_status='done', journal=source,
            intent={'realtime_migration': {'engine': {'uid': 'pg'}}})
        self.state = {'job': {'state': 'done', 'uid': 'job'}, 'terminal': 'c' * 64,
                      'execution': {'database': 'realtime_db', 'database_oid': 13, 'role': 'realtime'}}
        self.cut.migration = {'kind': 'realtime', 'phase': phase, 'root': '.', 'finished': True,
                              'terminal': self.state['terminal']}
        self.ledger.row['realtime_migration'] = {'status': 'done'}
        patches = [mock.patch.object(migration, 'normalized', side_effect=lambda *args: (None, copy.deepcopy(self.state))),
                   mock.patch.object(migration, 'observe', side_effect=lambda *args, **kwargs: (copy.deepcopy(self.state), None)),
                   mock.patch.object(migration, 'sessions', return_value={'attributed_sessions': []})]
        for patcher in patches: patcher.start(); self.addCleanup(patcher.stop)

    def test_realtime_scope_adds_only_source_oid_and_detects_drift(self):
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.current_effective_session_scope()
        self.cut.bind_realtime_scope()
        scope = self.cut.current_effective_session_scope()
        self.assertEqual(scope['database_oids'], {'auth_db': 10, 'realtime_db': 13})
        self.assertEqual(scope['databases'], ['auth_db', 'realtime_db'])
        for mutation in ('database', 'oid', 'server_start'):
            self.cut.effective_session['scope'] = copy.deepcopy(scope)
            changed = self.cut.effective_session['scope']
            if mutation == 'database': changed['databases'].remove('auth_db')
            if mutation == 'oid': changed['database_oids']['auth_db'] = 99
            if mutation == 'server_start': changed['server_start'] = 'foreign'
            with self.subTest(mutation=mutation), self.assertRaises(writer_cut.RecoveryRequired):
                self.cut.current_effective_session_scope()
        self.cut.effective_session['scope'] = copy.deepcopy(scope)
        self.ledger.row['realtime_migration']['status'] = 'foreign'
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.current_effective_session_scope()

    def test_colliding_source_oid_never_binds(self):
        self.state['execution']['database_oid'] = 10
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.bind_realtime_scope()
        self.assertEqual(self.ledger.row['status'], 'uncertain')


class CatalogBracketTest(unittest.TestCase):
    def test_catalog_change_during_migration_sample_holds(self):
        scope = {'database_oids': {'auth_db': 10}}
        cut = types.SimpleNamespace(admission={'session_scope': scope}, probe=object(), _guard=lambda: None)
        state = {'execution': {'database': 'auth_db', 'database_oid': 10, 'role': 'voice', 'application_name': 'auth'}}
        records = [{'scope': scope, 'catalog': {'auth_db': 10}},
                   {'scope': scope, 'catalog': {'auth_db': 10, 'new_db': 11}}]
        affected = types.SimpleNamespace(verify_migration_catalog=mock.Mock(side_effect=records))
        receipt = {'scope': scope, 'active_sessions': 0, 'active_transactions': 0, 'attributed_sessions': []}
        with mock.patch.dict(sys.modules, {'full_cohort_affected_plan': affected}), \
             mock.patch.object(writer_cut_pg_probe, 'inspect_migration', return_value=receipt):
            with self.assertRaisesRegex(ValueError, 'catalog changed'):
                migration.sessions(cut, state, None)
            affected.verify_migration_catalog.side_effect = [records[0], records[0]]
            self.assertEqual(migration.sessions(cut, state, None), receipt)
        self.assertEqual(affected.verify_migration_catalog.call_count, 4)


if __name__ == '__main__': unittest.main()
