import copy
import json
import os
import stat
import threading
import unittest
from pathlib import Path
from unittest import mock

import media_backup_phase_test as fixture
import media_operation_journal as journal
import resource_fencing
import update_images
import writer_cut


class Client:
    PREFIX = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
              '--namespace', 'voice-dev', '--request-timeout=15s']
    def __init__(self, lock, journal):
        self.lock, self.journal = lock, journal
        self.workloads = {}
        self.guard_rows = {'jobs': [], 'cronjobs': [], 'horizontalpodautoscalers': []}
        self.rs, self.pods = [], []
        self.patches = []
        self.lost = False

    def call(self, *args):
        if args[1] == 'namespace':
            return {'metadata': {'name': 'voice-dev', 'uid': 'ns', 'labels': update_images.LABELS}}
        if args[1] == 'configmap': return copy.deepcopy(self.lock)
        if args[1] in self.guard_rows: return {'items': copy.deepcopy(self.guard_rows[args[1]])}
        if args[1] == 'replicasets': return {'items': copy.deepcopy(self.rs)}
        if args[1] == 'pods': return {'items': copy.deepcopy(self.pods)}
        kind = {'deployment': 'Deployment', 'statefulset': 'StatefulSet'}[args[1]]
        return copy.deepcopy(self.workloads[(kind, args[2])])

    def patch(self, kind, name, patch):
        current = self.workloads[(kind, name)]
        self.patches.append(copy.deepcopy(patch))
        if (patch[0]['value'] != current['metadata']['uid'] or patch[1]['value'] != current['metadata']['resourceVersion']
                or patch[2]['value'] != current['spec']):
            raise RuntimeError('CAS conflict')
        changed = current['spec']['replicas'] != patch[3]['value']
        current['spec']['replicas'] = patch[3]['value']
        current['metadata']['resourceVersion'] = str(int(current['metadata']['resourceVersion']) + 1)
        current['metadata']['generation'] += int(changed)
        current['status'] = {'observedGeneration': current['metadata']['generation'],
                             'replicas': current['spec']['replicas'], 'readyReplicas': current['spec']['replicas']}
        if self.lost: raise OSError('response lost after commit')
        return copy.deepcopy(current)


def workload(kind, name, uid, replicas, image):
    return {'apiVersion': 'apps/v1', 'kind': kind, 'metadata': {'name': name, 'namespace': 'voice-dev',
             'uid': uid, 'resourceVersion': '1', 'generation': 1},
            'spec': {'replicas': replicas, 'template': {'spec': {'containers': [{'name': 'app', 'image': image}]}}},
            'status': {'observedGeneration': 1, 'replicas': replicas, 'readyReplicas': replicas}}


class WriterCutTest(unittest.TestCase):
    def setUp(self):
        fixture.PhaseTest.setUp(self)
        self.sources = [workload('Deployment', 'voice-a', 'uid-a', 2, 'old-a'),
                        workload('StatefulSet', 'voice-b', 'uid-b', 1, 'old-b')]
        self.admission = {'controllers': copy.deepcopy(self.sources),
            'guards': {'jobs': [], 'cronjobs': [], 'hpas': []},
            'linked': {'jobs': [], 'cronjobs': []},
            'session_scope': {'engine': 'postgres', 'databases': ['a', 'b'], 'roles': ['writer']}}
        self.journal.intent['writer_cut'] = copy.deepcopy(self.admission)
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.client = Client(self.lock, self.journal)
        self.client.workloads = {(o['kind'], o['metadata']['name']): copy.deepcopy(o) for o in self.sources}
        self.ledger.client = self.client
        self.type_patch = mock.patch.object(writer_cut.frontend_runtime_executor, 'KubectlTransport', Client)
        self.type_patch.start()
        self.addCleanup(self.type_patch.stop)
        self.sessions = 0
        self.cut = writer_cut.WriterCut(self.journal, self.fence, self.client, self.lock, 'ns', self.admission,
            lambda scope: {'scope': scope, 'active_sessions': self.sessions, 'active_transactions': 0})
        self.effect_patch = mock.patch.object(self.cut, '_patch', side_effect=self.client.patch)
        self.effect_patch.start()
        self.addCleanup(self.effect_patch.stop)

    def test_space_guard_receives_inventory_after_owned_migration_job(self):
        import writer_cut_migration as migration
        import writer_cut_space as space
        owned={'metadata':{'name':'migration','uid':'owned','namespace':'voice-dev'},'spec':{},'status':{}}
        self.client.guard_rows['jobs']=[owned]
        self.cut.space_fresh={'finished':True};self.cut.migration={'finished':False}
        state={'job':{'state':'owned','uid':'owned'}}
        with mock.patch.object(migration,'observe',return_value=(state,None)), \
             mock.patch.object(migration,'sessions',return_value={'attributed_sessions':[]}), \
             mock.patch.object(space,'guard_jobs',side_effect=lambda _cut,rows:rows) as guard:
            self.cut._guards()
            guard.assert_called_once_with(self.cut,[])
            self.client.guard_rows['jobs'].append({'metadata':{'name':'foreign','uid':'foreign'},'spec':{},'status':{}})
            with self.assertRaises(writer_cut.RecoveryRequired):self.cut._guards()

    def test_drain_and_resume_use_current_template_not_old(self):
        self.assertEqual(self.cut.drain()['active_sessions'], 0)
        self.assertTrue(all(v['spec']['replicas'] == 0 for v in self.client.workloads.values()))
        targets = []
        for item in self.sources:
            current = self.client.workloads[(item['kind'], item['metadata']['name'])]
            current['spec']['template']['spec']['containers'][0]['image'] = 'new-candidate'
            current['metadata']['resourceVersion'] = '5'
            targets.append(copy.deepcopy(current))
        self.assertEqual(self.cut.resume(targets)['status'], 'replicas-restored')
        for item in self.sources:
            current = self.client.workloads[(item['kind'], item['metadata']['name'])]
            self.assertEqual(current['spec']['replicas'], item['spec']['replicas'])
            self.assertEqual(current['spec']['template']['spec']['containers'][0]['image'], 'new-candidate')
        self.assertTrue(all(p[-1]['path'] == '/spec/replicas' for p in self.client.patches))

    def test_active_sessions_never_produce_drain_receipt(self):
        self.sessions = 1
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.drain()
        self.assertEqual(self.ledger.row['status'], 'uncertain')
        self.assertEqual(self.ledger.row['writer_cut_effects']['writer-cut-drained']['status'], 'pending')

    def test_replaced_session_probe_after_drain_holds(self):
        self.cut.drain()
        self.cut.probe = lambda scope: {'scope': scope, 'active_sessions': 0, 'active_transactions': 0}
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.verify_drained()
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_lost_committed_patch_never_retries(self):
        self.client.lost = True
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.drain()
        self.assertEqual(self.client.workloads[('Deployment', 'voice-a')]['spec']['replicas'], 0)
        self.assertEqual(self.ledger.row['status'], 'uncertain')
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.drain()
        self.assertEqual(len(self.client.patches), 1)

    def test_stale_source_holds_before_any_patch(self):
        self.client.workloads[('StatefulSet', 'voice-b')]['metadata']['resourceVersion'] = '2'
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.drain()
        self.assertEqual(self.client.patches, [])

    def test_guard_drift_or_target_hpa_never_mutate(self):
        self.client.guard_rows['horizontalpodautoscalers'] = [{'metadata': {'name': 'h', 'uid': 'h', 'resourceVersion': '1',
            'namespace': 'voice-dev'}, 'spec': {'scaleTargetRef': {'kind': 'Deployment', 'name': 'voice-a'}}}]
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.drain()
        self.assertEqual(self.client.patches, [])

    def test_admitted_hpa_active_job_and_unsuspended_cronjob_hold(self):
        for kind in ('hpas', 'jobs', 'cronjobs'):
            with self.subTest(kind=kind):
                self.setUp()
                item = {'metadata': {'name': 'guard', 'uid': 'guard', 'resourceVersion': '1', 'namespace': 'voice-dev'},
                        'spec': {}, 'status': {}}
                if kind == 'hpas': item['spec'] = {'scaleTargetRef': {'kind': 'Deployment', 'name': 'voice-a'}}
                if kind == 'jobs': item['status'] = {'active': 1}
                if kind == 'cronjobs': item['spec'] = {'suspend': False}
                resource = 'horizontalpodautoscalers' if kind == 'hpas' else kind
                self.client.guard_rows[resource] = [copy.deepcopy(item)]
                self.admission['guards'][kind] = writer_cut.inventory([item])
                if kind != 'hpas': self.admission['linked'][kind] = ['guard']
                self.cut.admission = copy.deepcopy(self.admission)
                self.journal.intent['writer_cut'] = copy.deepcopy(self.admission)
                self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
                self.cut.intent = copy.deepcopy(self.journal.intent)
                with self.assertRaises(writer_cut.RecoveryRequired): self.cut.drain()
                self.assertEqual(self.client.patches, [])

    def test_private_patch_file_never_exposes_spec_in_argv(self):
        self.effect_patch.stop()
        private_patch = [{'op': 'test', 'path': '/spec', 'value': {'private-marker': 'secret-sentinel'}}]
        def run(args, **kwargs):
            self.assertNotIn('secret-sentinel', ' '.join(args))
            path = next(v.split('=', 1)[1] for v in args if v.startswith('--patch-file='))
            self.assertEqual(stat.S_IMODE(os.stat(path).st_mode), 0o600)
            with open(path, encoding='utf-8') as stream: self.assertEqual(json.load(stream), private_patch)
            self.assertEqual(args[:len(Client.PREFIX)], Client.PREFIX)
            return mock.Mock(stdout=b'{}')
        with mock.patch.object(writer_cut.subprocess, 'run', side_effect=run):
            self.assertEqual(self.cut._patch('Deployment', 'voice-a', private_patch), {})
        self.assertEqual(set(os.listdir(self.journal.dirfd)), {'lock', 'state.json'})

    def test_stale_resume_target_never_restores_any(self):
        self.cut.drain()
        targets = [copy.deepcopy(self.client.workloads[(o['kind'], o['metadata']['name'])]) for o in self.sources]
        self.client.workloads[('StatefulSet', 'voice-b')]['spec']['template']['spec']['containers'][0]['image'] = 'foreign'
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.resume(targets)
        self.assertEqual(len(self.client.patches), 2)

    def test_resume_rejects_unretired_new_target_before_any_restore(self):
        self.cut.drain()
        targets = [copy.deepcopy(self.client.workloads[(o['kind'], o['metadata']['name'])]) for o in self.sources]
        current = self.client.workloads[('Deployment', 'voice-a')]
        current['metadata']['uid'] = 'new-target'
        current['status']['replicas'] = 1
        targets[0] = copy.deepcopy(current)
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.resume(targets)
        self.assertEqual(len(self.client.patches), 2)

    def test_resume_accepts_retired_new_target_without_old_spec(self):
        self.cut.drain()
        targets = [copy.deepcopy(self.client.workloads[(o['kind'], o['metadata']['name'])]) for o in self.sources]
        current = self.client.workloads[('Deployment', 'voice-a')]
        current['metadata']['uid'] = 'new-target'
        current['spec']['template']['spec']['containers'][0]['image'] = 'target-image'
        targets[0] = copy.deepcopy(current)
        self.cut.resume(targets)
        self.assertEqual(current['spec']['replicas'], 2)
        self.assertEqual(current['spec']['template']['spec']['containers'][0]['image'], 'target-image')

    def test_resume_accepts_status_only_rv_and_returns_generation_receipts(self):
        self.cut.drain()
        targets = [copy.deepcopy(self.client.workloads[(o['kind'], o['metadata']['name'])]) for o in self.sources]
        self.client.workloads[('Deployment', 'voice-a')]['metadata']['resourceVersion'] = '18'
        result = self.cut.resume(targets)
        entry = result['effects']['writer-resume-deployment-voice-a']
        self.assertEqual(entry['replicas'], 2)
        self.assertEqual(entry['generation'], self.client.workloads[('Deployment', 'voice-a')]['metadata']['generation'])
        self.assertEqual(self.client.patches[2][1]['value'], '18')
        self.assertEqual(entry['receipt_sha256'], writer_cut.digest(
            self.journal.state['steps']['writer-resume-deployment-voice-a']['receipt']))

    def test_resume_rejects_generation_drift_before_mutation(self):
        self.cut.drain()
        targets = [copy.deepcopy(self.client.workloads[(o['kind'], o['metadata']['name'])]) for o in self.sources]
        self.client.workloads[('Deployment', 'voice-a')]['metadata']['generation'] += 1
        self.client.workloads[('Deployment', 'voice-a')]['status']['observedGeneration'] += 1
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.resume(targets)
        self.assertEqual(len(self.client.patches), 2)

    def test_resume_rejects_target_terminating_after_admission(self):
        self.cut.drain()
        targets = [copy.deepcopy(self.client.workloads[(o['kind'], o['metadata']['name'])]) for o in self.sources]
        current = self.client.workloads[('Deployment', 'voice-a')]
        current['metadata']['deletionTimestamp'] = '2026-10-29T00:00:00Z'
        current['metadata']['resourceVersion'] = '20'
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.resume(targets)
        self.assertEqual(len(self.client.patches), 2)

    def test_completed_effect_removed_from_ledger_blocks_next_patch(self):
        before = self.cut._complete
        def alter_after_first(name, record, receipt):
            before(name, record, receipt)
            if name == 'writer-drain-deployment-voice-a':
                del self.ledger.row['writer_cut_effects'][name]
                self.ledger.version += 1
        with mock.patch.object(self.cut, '_complete', side_effect=alter_after_first):
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.drain()
        self.assertEqual(len(self.client.patches), 1)

    def test_previous_effect_lost_after_final_guard_never_submits_next_patch(self):
        guard = self.cut._guard
        def drift_after_guard():
            guard()
            effects = self.ledger.row.get('writer_cut_effects', {})
            if (effects.get('writer-drain-statefulset-voice-b', {}).get('status') == 'pending'
                    and 'writer-drain-deployment-voice-a' in effects):
                del effects['writer-drain-deployment-voice-a']
                self.ledger.version += 1
        with mock.patch.object(self.cut, '_guard', side_effect=drift_after_guard):
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.drain()
        self.assertEqual(len(self.client.patches), 1)

    def test_status_only_reobservation_preserves_success_receipt(self):
        self.cut.drain()
        targets = [copy.deepcopy(self.client.workloads[(o['kind'], o['metadata']['name'])]) for o in self.sources]
        original = self.client.patch
        def status_update(kind, name, patch):
            result = original(kind, name, patch)
            obj = self.client.workloads[(kind, name)]
            obj['metadata']['resourceVersion'] = str(int(obj['metadata']['resourceVersion']) + 1)
            return result
        self.cut._patch = status_update
        entry = self.cut.resume(targets)['effects']['writer-resume-deployment-voice-a']
        self.assertNotEqual(entry['responseResourceVersion'], entry['resourceVersion'])
        self.assertEqual(entry['generation'], self.client.workloads[('Deployment', 'voice-a')]['metadata']['generation'])

    def test_generation_change_after_commit_holds(self):
        self.cut.drain()
        targets = [copy.deepcopy(self.client.workloads[(o['kind'], o['metadata']['name'])]) for o in self.sources]
        original = self.client.patch
        def changed(kind, name, patch):
            result = original(kind, name, patch)
            self.client.workloads[(kind, name)]['metadata']['generation'] += 1
            return result
        self.cut._patch = changed
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.resume(targets)
        self.assertEqual(len(self.client.patches), 3)
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_resource_version_changed_after_preflight_refuses_cas(self):
        self.cut.drain()
        targets = [copy.deepcopy(self.client.workloads[(o['kind'], o['metadata']['name'])]) for o in self.sources]
        original = self.client.patch
        def drift(kind, name, patch):
            self.client.workloads[(kind, name)]['metadata']['resourceVersion'] = '30'
            return original(kind, name, patch)
        self.cut._patch = drift
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.resume(targets)
        self.assertEqual(self.client.workloads[('Deployment', 'voice-a')]['spec']['replicas'], 0)
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_old_rs_pod_remains_detected_after_rs_deletion(self):
        self.client.rs = [{'metadata': {'uid': 'rs-old', 'ownerReferences': [
            {'kind': 'Deployment', 'uid': 'uid-a', 'controller': True}]}}]
        self.client.pods = [{'metadata': {'uid': 'pod-old', 'ownerReferences': [
            {'kind': 'ReplicaSet', 'uid': 'rs-old', 'controller': True}]}}]
        self.cut.retired_rs = {'rs-old'}
        self.client.rs = []
        self.assertFalse(self.cut._pods_drained())

    def test_live_pod_with_unresolvable_replicaset_is_not_drain_proof(self):
        self.client.rs = []
        self.client.pods = [{'metadata': {'uid': 'pod', 'ownerReferences': [
            {'kind': 'ReplicaSet', 'uid': 'never-seen', 'controller': True}]}, 'status': {'phase': 'Running'}}]
        self.assertFalse(self.cut._pods_drained())

    def test_unknown_active_controller_is_not_drain_proof(self):
        self.client.pods = [{'metadata': {'uid': 'unowned'}, 'status': {'phase': 'Running'}}]
        self.assertFalse(self.cut._pods_drained())

    def test_authority_changed_during_guard_inventory_prevents_patch(self):
        original = self.cut._guards
        count = 0
        def change_on_second():
            nonlocal count
            original()
            count += 1
            if count == 2:
                self.ledger.row['status'] = 'uncertain'
                self.ledger.version += 1
        with mock.patch.object(self.cut, '_guards', side_effect=change_on_second):
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.drain()
        self.assertEqual(self.client.patches, [])

    def test_two_private_journals_cannot_both_own_invocation(self):
        other = journal.Journal(Path(self.temp.name) / 'other', self.journal.intent)
        self.addCleanup(other.close)
        second_client = copy.copy(self.client)
        second_client.journal = other
        class LedgerView:
            client = second_client
            def read(_): return self.ledger.read()
            def write(_, revision, row): return self.ledger.write(revision, row)
        other_fence = resource_fencing.Fence(self.context, lambda: copy.deepcopy(self.card), LedgerView())
        second = writer_cut.WriterCut(other, other_fence, second_client, self.lock, 'ns', self.admission,
            lambda scope: {'scope': scope, 'active_sessions': 0, 'active_transactions': 0})
        second._patch = second_client.patch
        barrier = threading.Barrier(2, timeout=5)
        write = self.ledger.write
        def racing(rev, row):
            if 'writer_cut_invocation' in row and 'writer_cut_effects' not in row:
                barrier.wait()
            write(rev, row)
        outcomes = []
        def run(obj):
            try: obj.drain(); outcomes.append('done')
            except writer_cut.RecoveryRequired: outcomes.append('refused')
        with mock.patch.object(self.ledger, 'write', side_effect=racing):
            threads = [threading.Thread(target=run, args=(obj,)) for obj in (self.cut, second)]
            for thread in threads: thread.start()
            for thread in threads: thread.join(10)
        self.assertTrue(all(not t.is_alive() for t in threads))
        self.assertEqual(len(outcomes), 2)
        self.assertLessEqual(outcomes.count('done'), 1)


if __name__ == '__main__': unittest.main()
