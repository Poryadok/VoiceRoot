import copy
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import full_cohort_image_apply as cohort
import media_backup_phase_test as fixture
import media_operation_journal as journal
import resource_fencing
import update_images


class Cluster:
    def __init__(self, lock, journal):
        self.lock, self.journal = lock, journal
        self.deployments = {}
        self.replicasets = []
        self.pods = []
        self.patches = 0
        self.lost = False
        self.accounts = {'default': {'kind': 'ServiceAccount', 'metadata': {'name': 'default',
            'namespace': 'voice-dev', 'uid': 'sa', 'resourceVersion': '1'}}}

    def call(self, *args, body=None):
        if args[1] == 'namespace':
            return {'metadata': {'name': 'voice-dev', 'uid': 'ns', 'labels': update_images.LABELS}}
        if args[1] == 'configmap':
            return copy.deepcopy(self.lock)
        if args[1] == 'serviceaccount':
            return copy.deepcopy(self.accounts[args[2]])
        if args[1] == 'deployment':
            return copy.deepcopy(self.deployments[args[2]])
        if args[1] == 'replicasets':
            return {'items': copy.deepcopy(self.replicasets)}
        if args[1] == 'pods':
            return {'items': copy.deepcopy(self.pods)}
        raise AssertionError(args)

    def patch(self, name, patch):
        obj = self.deployments[name]
        if (patch[0]['value'] != obj['metadata']['uid'] or patch[1]['value'] != obj['metadata']['resourceVersion']
                or patch[2]['value'] != obj['metadata']['generation'] or patch[3]['value'] != obj['spec']):
            raise RuntimeError('CAS failed')
        self.patches += 1
        if obj['spec'] != patch[4]['value']:
            obj['metadata']['generation'] += 1
        obj['spec'] = copy.deepcopy(patch[4]['value'])
        obj['metadata']['resourceVersion'] = str(int(obj['metadata']['resourceVersion']) + 1)
        if self.lost:
            raise OSError('committed, response lost')
        return copy.deepcopy(obj)

    def resume(self, journal, ledger):
        self.replicasets, self.pods = [], []
        for name, obj in self.deployments.items():
            source = copy.deepcopy(obj)
            obj['spec']['replicas'] = 1
            obj['metadata'].update(resourceVersion='30', generation=source['metadata']['generation'] + 1)
            obj['status'] = {'observedGeneration': obj['metadata']['generation'], 'updatedReplicas': 1, 'availableReplicas': 1}
            step = 'writer-resume-deployment-' + name
            details = {'source': source, 'target_spec': copy.deepcopy(obj['spec'])}
            receipt = {'uid': obj['metadata']['uid'], 'spec_sha256': cohort.digest(obj['spec']),
                       'responseResourceVersion': '30', 'resourceVersion': '30', 'generation': obj['metadata']['generation']}
            item = {'status': 'done', 'nonce': 'e' * 32, 'details_sha256': cohort.digest(details),
                    'details': details, 'receipt': receipt}
            journal.state['steps'][step] = item
            ledger.row['writer_cut_effects'][step] = {'status': 'done', 'nonce': item['nonce'],
                'details_sha256': item['details_sha256'], 'receipt_sha256': cohort.digest(receipt)}
            template = copy.deepcopy(obj['spec']['template'])
            template.setdefault('metadata', {}).setdefault('labels', {})['pod-template-hash'] = 'abc'
            rsuid = name + '-rs'
            self.replicasets.append({'metadata': {'name': name + '-rs', 'namespace': 'voice-dev', 'uid': rsuid, 'resourceVersion': '29', 'ownerReferences': [
                {'kind': 'Deployment', 'controller': True, 'uid': obj['metadata']['uid']}]}, 'spec': {'template': template}})
            c = obj['spec']['template']['spec']['containers'][0]
            self.pods.append({'metadata': {'name': name + '-pod', 'namespace': 'voice-dev', 'uid': name + '-pod-uid', 'labels': {'app': name},
                'resourceVersion': '31', 'ownerReferences': [{'kind': 'ReplicaSet', 'controller': True, 'uid': rsuid}]},
                'spec': copy.deepcopy(obj['spec']['template']['spec']), 'status': {'phase': 'Running', 'conditions': [{'type': 'Ready', 'status': 'True'}],
                'containerStatuses': [{'name': c['name'], 'ready': True, 'imageID': 'containerd://sha256:' + 'c' * 64}]}})
        journal.save()


class FakeWriterCut:
    def __init__(self, journal, fence, client, lock):
        self.journal, self.fence, self.client, self.lock = journal, fence, client, lock
        self.namespace_uid = 'ns'
        self.intent = copy.deepcopy(journal.intent)
        self.admission = copy.deepcopy(journal.intent['writer_cut'])
        self.invocation = 'd' * 32
        self.retired_rs = set()
        self.poisoned = False
        self.config_journal = None
        self.config_snapshot = None
        self.successors = {}
        self.proof_calls = 0
        proof = journal.state['steps']['fixture-auth-proof']
        self.auth_successor = {'proof': copy.deepcopy(proof), 'receipt_sha256': cohort.digest(proof)}
        self.probe = lambda scope: {'scope': scope, 'active_sessions': 0, 'active_transactions': 0}

    def _guards(self):
        pass

    def _guard(self):
        pass

    def _auth_successor_matches(self, row):
        return row.get('fixture_auth_proof') == self.auth_successor['receipt_sha256']

    def _recheck_config_successors(self):
        pass

    def _pods_drained(self):
        return True

    def _session_observation(self):
        return self.probe(copy.deepcopy(self.admission['session_scope']))

    def current_cut_proof(self):
        self.proof_calls += 1
        writers = []
        for original in self.journal.intent['writer_cut']['controllers']:
            name = original['metadata']['name']
            step = 'writer-drain-deployment-' + name
            item = self.journal.state['steps'][step]
            successor = self.successors.get(name)
            accepted = self.journal.state['steps'].get('writer-cut-config-successor-' + name.removeprefix('voice-')) if successor else None
            writers.append({'kind': 'Deployment', 'name': name, 'uid': item['receipt']['uid'],
                'generation': successor['receipt']['generation'] if successor else item['receipt']['generation'],
                'spec': copy.deepcopy(successor['target_spec'] if successor else item['details']['target_spec']),
                'drain_step': step, 'drain_nonce': item['nonce'],
                'drain_receipt_sha256': cohort.digest(item['receipt']), 'successor':
                    {'step': 'writer-cut-config-successor-' + name.removeprefix('voice-'), 'nonce': accepted['nonce'],
                     'receipt_sha256': cohort.digest(accepted['receipt'])} if successor else None})
        return {'operation_id': self.fence.context['operation_id'], 'namespace_uid': 'ns', 'invocation_nonce': 'd' * 32,
                'writers': writers, 'session': self.probe(self.journal.intent['writer_cut']['session_scope'])}


class FullCohortTest(unittest.TestCase):
    def setUp(self):
        self.context = {'project': 'voice', 'resource': 'DEV', 'actor': 'scheduled_devops',
            'claim': '11111111-1111-4111-8111-111111111111', 'window': '22222222-2222-4222-8222-222222222222',
            'revision': 4, 'generation': 3, 'operation_id': 'full-cohort-test', 'commit': 'a' * 40,
            'images': {'voice-' + name + '/' + container: 'ghcr.io/x/' + name + '@sha256:' + 'b' * 64
                       for name, container in update_images.SERVICES.items()}}
        self.card = {'code': 'DEV', 'environment': 'dev', 'owner': 'scheduled_devops', 'claim': self.context['claim'],
            'generation': 3, 'window': self.context['window'], 'revision': 4, 'state': 'held', 'phase': 'deploy',
            'candidate': {'state': 'known', 'commit': self.context['commit'], 'images': self.context['images']}}
        self.ledger = fixture.Ledger(self.context)
        self.fence = resource_fencing.Fence(self.context, lambda: copy.deepcopy(self.card), self.ledger)
        self.lock = {'metadata': {'name': update_images.LOCK, 'namespace': 'voice-dev', 'uid': 'lock',
            'resourceVersion': '8', 'labels': update_images.LABELS}, 'data': {'operation': 'token',
            'resource_operation': self.context['operation_id'], 'resource_context': json.dumps(self.context, sort_keys=True, separators=(',', ':'))}}
        workloads = {}
        for name, container in update_images.SERVICES.items():
            deployment = 'voice-' + name
            spec = {'replicas': 0, 'template': {'metadata': {'labels': {'app': deployment}},
                'spec': {'containers': [{'name': container, 'image': 'old', 'env': [{'name': 'X', 'value': 'preserve'}]}]}}}
            workloads[deployment] = {'uid': deployment + '-uid', 'resourceVersion': '1', 'generation': 2, 'spec': spec, 'resume_replicas': 1}
        self.intent = {'resource_context': self.context, 'namespace_uid': 'ns', 'full_cohort': {
            'images': copy.deepcopy(self.context['images']),
            'platform_digests': {k: 'sha256:' + 'c' * 64 for k in self.context['images']},
            'workloads': workloads, 'cut': sorted(workloads),
            'service_accounts': {'default': {'kind': 'ServiceAccount', 'metadata': {'name': 'default',
                'namespace': 'voice-dev', 'uid': 'sa', 'resourceVersion': '1'}}}}}
        cut_intent = {'resource_context': self.context, 'namespace_uid': 'ns', 'writer_cut': {'session_scope': {'role': 'writer'}, 'controllers': [{'kind': 'Deployment', 'metadata': {'name': name,
            'namespace': 'voice-dev', 'uid': snap['uid'], 'resourceVersion': '1', 'generation': 1},
            'spec': copy.deepcopy(snap['spec']) | {'replicas': snap['resume_replicas']}}
            for name, snap in workloads.items()]}}
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.journal = journal.Journal(Path(self.temp.name) / 'journal', self.intent)
        self.addCleanup(self.journal.close)
        self.cut_journal = journal.Journal(Path(self.temp.name) / 'cut-journal', cut_intent)
        self.addCleanup(self.cut_journal.close)
        self.cut_journal.state['steps']['writer-cut-invocation'] = {'status': 'owned', 'nonce': 'd' * 32}
        self.ledger.row['writer_cut_invocation'] = {'nonce': 'd' * 32, 'operation_id': self.context['operation_id']}
        self.ledger.row['writer_cut_effects'] = {}
        def record(step, details, receipt):
            item = {'status': 'done', 'nonce': 'e' * 32, 'details_sha256': cohort.digest(details),
                    'details': details, 'receipt': receipt}
            self.cut_journal.state['steps'][step] = item
            self.ledger.row['writer_cut_effects'][step] = {k: item[k] for k in ('status', 'nonce', 'details_sha256')}
            self.ledger.row['writer_cut_effects'][step]['receipt_sha256'] = cohort.digest(receipt)
        for source in cut_intent['writer_cut']['controllers']:
            target = copy.deepcopy(source['spec'])
            target['replicas'] = 0
            record('writer-drain-deployment-' + source['metadata']['name'], {'source': source, 'target_spec': target},
                   {'uid': source['metadata']['uid'], 'spec_sha256': cohort.digest(target),
                    'responseResourceVersion': '2', 'resourceVersion': '2', 'generation': 2})
        record('writer-cut-drained', {'session_scope': {'role': 'writer'}},
               {'scope': {'role': 'writer'}, 'active_sessions': 0, 'active_transactions': 0})
        auth_target = copy.deepcopy(workloads['voice-auth']['spec'])
        auth_target['template']['spec']['containers'][0]['image'] = self.context['images']['voice-auth/auth']
        proof = {'candidate': {'target_spec_sha256': cohort.digest(auth_target),
                               'image': self.context['images']['voice-auth/auth'],
                               'platform_digest': self.intent['full_cohort']['platform_digests']['voice-auth/auth']}}
        self.cut_journal.state['steps']['fixture-auth-proof'] = copy.deepcopy(proof)
        self.ledger.row['fixture_auth_proof'] = cohort.digest(proof)
        details = {'operation_id': self.context['operation_id'], 'cut_invocation': 'd' * 32,
                   'receipt_sha256': cohort.digest(proof)}
        record('writer-cut-auth-successor', details, copy.deepcopy(details))
        self.cut_journal.save()
        self.client = Cluster(self.lock, self.journal)
        self.ledger.client = self.client
        for name, snap in workloads.items():
            self.client.deployments[name] = {'kind': 'Deployment', 'metadata': {'name': name, 'namespace': 'voice-dev',
                'uid': snap['uid'], 'resourceVersion': snap['resourceVersion'], 'generation': snap['generation']}, 'spec': copy.deepcopy(snap['spec'])}
        self.client_patch = mock.patch.object(cohort, 'KubectlTransport', Cluster)
        self.client_patch.start()
        self.addCleanup(self.client_patch.stop)
        self.cut_patch = mock.patch.object(cohort, 'WriterCut', FakeWriterCut)
        self.cut_patch.start()
        self.addCleanup(self.cut_patch.stop)
        self.image_number = 0
        self.fake_config = None
        self.fake_successors = {}

    def new(self):
        cut_client = copy.copy(self.client)
        cut_client.journal = self.cut_journal
        backing = self.ledger
        class LedgerView:
            client = cut_client
            def read(_): return backing.read()
            def write(_, revision, row): return backing.write(revision, row)
        cut_fence = resource_fencing.Fence(self.context, lambda: copy.deepcopy(self.card), LedgerView())
        cut = FakeWriterCut(self.cut_journal, cut_fence, cut_client, self.lock)
        if self.fake_config is not None:
            cut.config_journal = self.fake_config
            cut.config_snapshot = {'fixture': True}
            cut.successors = copy.deepcopy(self.fake_successors)
        capture = cohort.prepare_cut(cut)
        intent = copy.deepcopy(self.journal.intent)
        intent['full_cohort']['cut_proof'] = copy.deepcopy(capture.proof)
        self.journal.close()
        self.image_number += 1
        self.journal = journal.Journal(Path(self.temp.name) / ('image-' + str(self.image_number)), intent)
        self.addCleanup(self.journal.close)
        self.client.journal = self.journal
        return cohort.FullCohortImageApply(self.journal, self.fence, self.client, self.lock, 'ns', capture)

    def test_all_23_apply_preserves_cut_and_full_spec(self):
        phase = self.new()
        result = phase.apply()
        self.assertEqual(phase.admission.cut.proof_calls, 1)
        self.assertEqual(len(result), 23)
        self.assertEqual(self.client.patches, 23)

    def test_auth_target_mismatch_holds_before_any_image_patch(self):
        phase = self.new()
        binding = phase.admission.cut.auth_successor
        binding['proof']['candidate']['target_spec_sha256'] = 'f' * 64
        binding['receipt_sha256'] = cohort.digest(binding['proof'])
        self.ledger.row['fixture_auth_proof'] = binding['receipt_sha256']
        step = self.cut_journal.state['steps']['writer-cut-auth-successor']
        step['details']['receipt_sha256'] = binding['receipt_sha256']
        step['receipt']['receipt_sha256'] = binding['receipt_sha256']
        step['details_sha256'] = cohort.digest(step['details'])
        self.ledger.row['writer_cut_effects']['writer-cut-auth-successor'].update(
            details_sha256=step['details_sha256'], receipt_sha256=cohort.digest(step['receipt']))
        self.cut_journal.save()
        with self.assertRaises(cohort.RecoveryRequired): phase.apply()
        self.assertEqual(self.client.patches, 0)

    def test_missing_auth_successor_holds_before_any_image_patch(self):
        phase = self.new()
        phase.admission.cut.auth_successor = None
        with self.assertRaises(cohort.RecoveryRequired): phase.apply()
        self.assertEqual(self.client.patches, 0)

    def test_activation_lineage_reads_only_completed_qualified_image_receipts(self):
        phase = self.new()
        phase.apply()
        before = self.client.patches
        result = phase.verify_activation_lineage()
        self.assertEqual(len(result['images']), 23)
        self.assertEqual(self.client.patches, before)
        self.assertFalse(phase.verifying)
        step = 'full-cohort-image-voice-auth'
        phase.journal.state['steps'][step]['receipt']['spec']['replicas'] = 7
        with self.assertRaises(cohort.RecoveryRequired): phase.verify_activation_lineage()
        self.assertFalse(phase.verifying)
        self.assertEqual(self.client.patches, before)

    def test_already_target_images_are_observed_without_a_patch(self):
        names = ('voice-web', 'voice-admin', 'voice-developer-portal')
        for name in names:
            image = self.context['images'][name + '/' + cohort.CATALOG[name]]
            snap = self.journal.intent['full_cohort']['workloads'][name]
            snap['spec']['template']['spec']['containers'][0]['image'] = image
            self.client.deployments[name]['spec'] = copy.deepcopy(snap['spec'])
            original = next(o for o in self.cut_journal.intent['writer_cut']['controllers'] if o['metadata']['name'] == name)
            original['spec']['template']['spec']['containers'][0]['image'] = image
            step = 'writer-drain-deployment-' + name
            item = self.cut_journal.state['steps'][step]
            item['details']['source']['spec'] = copy.deepcopy(original['spec'])
            item['details']['target_spec'] = copy.deepcopy(snap['spec'])
            item['details_sha256'] = cohort.digest(item['details'])
            item['receipt']['spec_sha256'] = cohort.digest(snap['spec'])
            self.ledger.row['writer_cut_effects'][step].update(details_sha256=item['details_sha256'],
                                                               receipt_sha256=cohort.digest(item['receipt']))
        self.cut_journal.state['intent'] = copy.deepcopy(self.cut_journal.intent)
        self.cut_journal.save()
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        phase = self.new()
        result = phase.apply()
        self.assertEqual(len(result), 23)
        self.assertEqual(self.client.patches, 20)
        for name in names:
            step = phase.journal.state['steps']['full-cohort-image-' + name]
            self.assertEqual(step['details']['mode'], 'already-target')
            self.assertIsNone(step['details']['patch'])
            self.assertEqual(step['receipt']['generation'], phase.intent['full_cohort']['workloads'][name]['generation'])
        for name, snap in self.intent['full_cohort']['workloads'].items():
            expected = copy.deepcopy(snap['spec'])
            container = expected['template']['spec']['containers'][0]
            container['image'] = self.context['images'][name + '/' + container['name']]
            self.assertEqual(self.client.deployments[name]['spec'], expected)
            self.assertEqual(result[name]['spec'], expected)
            self.assertEqual(result[name]['resourceVersion'], '1' if name in names else '2')

    def test_source_owned_cut_capture_cannot_be_a_caller_dictionary(self):
        with self.assertRaises(cohort.RecoveryRequired):
            cohort.prepare_cut({'writers': []})

    def test_two_ledger_clients_must_observe_same_physical_ledger(self):
        phase = self.new()
        read = phase.admission.cut.fence.ledger.read
        def foreign():
            row, revision = read()
            return row, ('foreign-ledger', revision[1])
        with mock.patch.object(phase.admission.cut.fence.ledger, 'read', side_effect=foreign):
            with self.assertRaises(cohort.RecoveryRequired): phase.apply()
        self.assertEqual(self.client.patches, 0)

    def test_successor_proof_is_pinned_and_config_drift_stops_apply(self):
        name = 'voice-user'
        snap = self.journal.intent['full_cohort']['workloads'][name]
        target = copy.deepcopy(snap['spec'])
        target['template']['spec']['containers'][0]['env'].append({'name': 'ADMITTED', 'value': 'fixture'})
        snap.update(spec=target, generation=snap['generation'] + 1)
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.client.deployments[name]['spec'] = copy.deepcopy(target)
        self.client.deployments[name]['metadata']['generation'] = snap['generation']
        config_intent = {'resource_context': self.context, 'namespace_uid': 'ns', 'fixture': 'producer'}
        self.fake_config = journal.Journal(Path(self.temp.name) / 'config-journal', config_intent)
        self.addCleanup(self.fake_config.close)
        self.fake_config.state['steps']['backend-config-deployment-voice-user'] = {'status': 'done'}
        self.fake_config.save()
        self.ledger.row['backend_config_effects'] = {'backend-config-deployment-voice-user': {'status': 'done'}}
        self.fake_successors[name] = {'target_spec': target, 'receipt': {'uid': snap['uid'], 'generation': snap['generation']}}
        step = 'writer-cut-config-successor-user'
        details, receipt = {'fixture': 'producer'}, {'uid': snap['uid']}
        item = {'status': 'done', 'nonce': 'f' * 32, 'details': details,
                'details_sha256': cohort.digest(details), 'receipt': receipt}
        self.cut_journal.state['steps'][step] = item
        self.cut_journal.save()
        self.ledger.row['writer_cut_effects'][step] = {'status': 'done', 'nonce': item['nonce'],
            'details_sha256': item['details_sha256'], 'receipt_sha256': cohort.digest(receipt)}
        phase = self.new()
        original = self.client.patch
        def drift(deployment, patch):
            result = original(deployment, patch)
            self.fake_config.state['steps']['backend-config-deployment-voice-user']['status'] = 'foreign'
            return result
        self.client.patch = drift
        with self.assertRaises(cohort.RecoveryRequired): phase.apply()
        self.assertEqual(self.client.patches, 1)
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_incomplete_or_foreign_catalog_never_patches(self):
        for alteration in ('missing', 'foreign'):
            with self.subTest(alteration=alteration):
                original = copy.deepcopy(self.journal.intent)
                images = self.journal.intent['full_cohort']['images']
                if alteration == 'missing': images.pop(next(iter(images)))
                else: images['voice-renderer/renderer'] = 'ghcr.io/x@sha256:' + 'b' * 64
                self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
                with self.assertRaises(cohort.RecoveryRequired): self.new()
                self.journal.intent = original
                self.journal.state['intent'] = copy.deepcopy(original)
        self.assertEqual(self.client.patches, 0)

    def test_cut_journal_and_ledger_proof_required_before_patch(self):
        for fault in ('missing', 'foreign_context', 'ledger', 'drain_source', 'sessions', 'unqualified_config'):
            with self.subTest(fault=fault):
                if fault != 'missing': self.setUp()
                if fault == 'missing':
                    with self.assertRaises(cohort.RecoveryRequired):
                        cohort.FullCohortImageApply(self.journal, self.fence, self.client, self.lock, 'ns')
                elif fault == 'foreign_context':
                    self.cut_journal.intent['namespace_uid'] = 'foreign'
                    self.cut_journal.state['intent'] = copy.deepcopy(self.cut_journal.intent)
                    with self.assertRaises(cohort.RecoveryRequired): self.new()
                elif fault == 'unqualified_config':
                    name = sorted(self.client.deployments)[0]
                    snap = self.journal.intent['full_cohort']['workloads'][name]
                    snap['spec']['template']['spec']['containers'][0]['env'].append({'name': 'UNQUALIFIED', 'value': 'x'})
                    snap['generation'] += 1
                    self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
                    self.client.deployments[name]['spec'] = copy.deepcopy(snap['spec'])
                    self.client.deployments[name]['metadata']['generation'] = snap['generation']
                    with self.assertRaises(cohort.RecoveryRequired): self.new().apply()
                else:
                    step = 'writer-cut-drained' if fault == 'sessions' else 'writer-drain-deployment-' + sorted(self.client.deployments)[0]
                    if fault == 'ledger': self.ledger.row['writer_cut_effects'][step]['receipt_sha256'] = 'foreign'
                    else:
                        item = self.cut_journal.state['steps'][step]
                        if fault == 'drain_source': item['details']['source']['metadata']['generation'] += 1
                        if fault == 'sessions': item['receipt']['active_sessions'] = 1
                        item['details_sha256'] = cohort.digest(item['details'])
                        self.ledger.row['writer_cut_effects'][step]['details_sha256'] = item['details_sha256']
                        self.ledger.row['writer_cut_effects'][step]['receipt_sha256'] = cohort.digest(item['receipt'])
                    with self.assertRaises(cohort.RecoveryRequired): self.new().apply()
                self.assertEqual(self.client.patches, 0)

    def test_resume_generation_receipt_must_bind_image_target(self):
        phase = self.new()
        phase.apply()
        self.client.resume(self.cut_journal, self.ledger)
        step = 'writer-resume-deployment-' + sorted(self.client.deployments)[0]
        item = self.cut_journal.state['steps'][step]
        item['details']['source']['metadata']['generation'] += 1
        item['details_sha256'] = cohort.digest(item['details'])
        self.ledger.row['writer_cut_effects'][step]['details_sha256'] = item['details_sha256']
        with self.assertRaises(cohort.RecoveryRequired): phase.verify_after_resume()

    def test_wrong_patch_generation_and_final_generation_drift_hold(self):
        for fault in ('response', 'later_drift'):
            with self.subTest(fault=fault):
                if fault == 'later_drift': self.setUp()
                phase = self.new()
                original = self.client.patch
                first = []
                def patch(name, body):
                    result = original(name, body)
                    if not first: first.append(name)
                    if fault == 'response' and self.client.patches == 1:
                        result['metadata']['generation'] += 1
                    if fault == 'later_drift' and self.client.patches == 2:
                        self.client.deployments[first[0]]['metadata']['generation'] += 2
                    return result
                self.client.patch = patch
                with self.assertRaises(cohort.RecoveryRequired): phase.apply()
                self.assertEqual(self.ledger.row['status'], 'uncertain')
                self.assertEqual(self.client.patches, 1 if fault == 'response' else 2)

    def test_resume_wrong_target_and_generation_even_when_resealed(self):
        for fault in ('target', 'generation'):
            with self.subTest(fault=fault):
                if fault == 'generation': self.setUp()
                phase = self.new()
                phase.apply()
                self.client.resume(self.cut_journal, self.ledger)
                step = 'writer-resume-deployment-' + sorted(self.client.deployments)[0]
                item = self.cut_journal.state['steps'][step]
                if fault == 'target': item['details']['target_spec']['replicas'] = 2
                else: item['receipt']['generation'] += 1
                item['details_sha256'] = cohort.digest(item['details'])
                remote = self.ledger.row['writer_cut_effects'][step]
                remote['details_sha256'] = item['details_sha256']
                remote['receipt_sha256'] = cohort.digest(item['receipt'])
                with self.assertRaises(cohort.RecoveryRequired): phase.verify_after_resume()

    def test_drift_or_lost_patch_holds_without_replay(self):
        for fault in ('drift', 'lost'):
            with self.subTest(fault=fault):
                if fault == 'lost': self.setUp()
                phase = self.new()
                if fault == 'drift': self.client.deployments[sorted(self.client.deployments)[0]]['metadata']['generation'] = 3
                else: self.client.lost = True
                with self.assertRaises(cohort.RecoveryRequired): phase.apply()
                count = self.client.patches
                with self.assertRaises(cohort.RecoveryRequired): phase.apply()
                self.assertEqual(self.client.patches, count)
                self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_status_only_rv_churn_is_allowed_but_spec_aba_generation_is_not(self):
        name = sorted(self.client.deployments)[0]
        self.client.deployments[name]['metadata']['resourceVersion'] = '9'
        phase = self.new()
        original = self.client.patch
        def patch(key, body):
            result = original(key, body)
            self.client.deployments[key]['metadata']['resourceVersion'] = str(int(result['metadata']['resourceVersion']) + 1)
            return result
        self.client.patch = patch
        result = phase.apply()
        self.assertEqual(result[name]['resourceVersion'], '11')
        self.assertEqual(result[name]['generation'], 3)
        self.setUp()
        self.client.deployments[name]['metadata'].update(resourceVersion='9', generation=3)
        with self.assertRaises(cohort.RecoveryRequired): self.new().apply()
        self.assertEqual(self.client.patches, 0)

    def test_already_current_image_keeps_generation(self):
        name = 'voice-web'
        key = name + '/web'
        image = self.context['images'][key]
        self.journal.intent['full_cohort']['workloads'][name]['spec']['template']['spec']['containers'][0]['image'] = image
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.client.deployments[name]['spec']['template']['spec']['containers'][0]['image'] = image
        original = next(c for c in self.cut_journal.intent['writer_cut']['controllers'] if c['metadata']['name'] == name)
        original['spec']['template']['spec']['containers'][0]['image'] = image
        self.cut_journal.state['intent'] = copy.deepcopy(self.cut_journal.intent)
        step = 'writer-drain-deployment-' + name
        item = self.cut_journal.state['steps'][step]
        item['details']['source']['spec']['template']['spec']['containers'][0]['image'] = image
        item['details']['target_spec']['template']['spec']['containers'][0]['image'] = image
        item['receipt']['spec_sha256'] = cohort.digest(item['details']['target_spec'])
        item['details_sha256'] = cohort.digest(item['details'])
        self.ledger.row['writer_cut_effects'][step]['details_sha256'] = item['details_sha256']
        self.ledger.row['writer_cut_effects'][step]['receipt_sha256'] = cohort.digest(item['receipt'])
        receipt = self.new().apply()
        self.assertEqual(receipt[name]['generation'], 2)

    def test_partial_apply_and_service_account_drift_hold_without_replay(self):
        phase = self.new()
        original = self.client.patch
        def patch(name, body):
            result = original(name, body)
            self.client.accounts['default']['metadata']['resourceVersion'] = '2'
            return result
        self.client.patch = patch
        with self.assertRaises(cohort.RecoveryRequired): phase.apply()
        self.assertEqual(self.client.patches, 1)
        self.assertEqual(self.ledger.row['status'], 'uncertain')
        with self.assertRaises(cohort.RecoveryRequired): phase.apply()
        self.assertEqual(self.client.patches, 1)

    def test_completed_ledger_map_drift_stops_before_second_patch(self):
        for fault in ('delete', 'change', 'foreign'):
            with self.subTest(fault=fault):
                if fault != 'delete': self.setUp()
                phase = self.new()
                write = self.ledger.write
                corrupted = False
                def drift(revision, row):
                    nonlocal corrupted
                    write(revision, row)
                    records = row.get('full_cohort_effects', {})
                    if not corrupted and len(records) == 1 and next(iter(records.values()))['status'] == 'done':
                        with self.ledger.lock:
                            actual = self.ledger.row['full_cohort_effects']
                            key = next(iter(actual))
                            if fault == 'delete': del actual[key]
                            if fault == 'change': actual[key]['receipt_sha256'] = 'foreign'
                            if fault == 'foreign': actual['full-cohort-image-voice-foreign'] = {'status': 'done'}
                            self.ledger.version += 1
                        corrupted = True
                with mock.patch.object(self.ledger, 'write', side_effect=drift):
                    with self.assertRaises(cohort.RecoveryRequired): phase.apply()
                self.assertTrue(corrupted)
                self.assertEqual(self.client.patches, 1)
                self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_local_invocation_drift_stops_before_second_patch(self):
        for fault in ('delete', 'nonce'):
            with self.subTest(fault=fault):
                if fault == 'nonce': self.setUp()
                phase = self.new()
                write = self.ledger.write
                changed = False
                def drift(revision, row):
                    nonlocal changed
                    write(revision, row)
                    records = row.get('full_cohort_effects', {})
                    if not changed and len(records) == 1 and next(iter(records.values()))['status'] == 'done':
                        if fault == 'delete': self.journal.state['steps'].pop('full-cohort-invocation')
                        else: self.journal.state['steps']['full-cohort-invocation']['nonce'] = 'foreign'
                        changed = True
                with mock.patch.object(self.ledger, 'write', side_effect=drift):
                    with self.assertRaises(cohort.RecoveryRequired): phase.apply()
                self.assertTrue(changed)
                self.assertEqual(self.client.patches, 1)
                self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_unrecorded_cut_pending_step_stops_before_next_patch(self):
        phase = self.new()
        original = self.client.patch
        def patch(name, body):
            response = original(name, body)
            self.cut_journal.state['steps']['writer-cut-config-successor-foreign'] = {'status': 'pending'}
            return response
        self.client.patch = patch
        with self.assertRaises(cohort.RecoveryRequired): phase.apply()
        self.assertEqual(self.client.patches, 1)
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_client_custody_drift_stops_before_second_patch(self):
        phase = self.new()
        write = self.ledger.write
        changed = False
        def drift(revision, row):
            nonlocal changed
            write(revision, row)
            records = row.get('full_cohort_effects', {})
            if not changed and len(records) == 1 and next(iter(records.values()))['status'] == 'done':
                self.client.journal = self.cut_journal
                changed = True
        with mock.patch.object(self.ledger, 'write', side_effect=drift):
            with self.assertRaises(cohort.RecoveryRequired): phase.apply()
        self.assertTrue(changed)
        self.assertEqual(self.client.patches, 1)
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_cut_mutation_or_poison_stops_before_second_patch(self):
        for fault in ('admission', 'poison', 'retired', 'probe'):
            with self.subTest(fault=fault):
                if fault != 'admission': self.setUp()
                phase = self.new()
                cut = phase.admission.cut
                original = self.client.patch
                def patch(name, body):
                    result = original(name, body)
                    if fault == 'admission': cut.admission['controllers'].clear()
                    if fault == 'poison': cut.poisoned = True
                    if fault == 'retired': cut.retired_rs.clear() if cut.retired_rs else cut.retired_rs.add('foreign')
                    if fault == 'probe': cut.probe = lambda scope: {'scope': scope, 'active_sessions': 0, 'active_transactions': 0}
                    return result
                self.client.patch = patch
                with self.assertRaises(cohort.RecoveryRequired): phase.apply()
                self.assertEqual(self.client.patches, 1)
                self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_verify_after_resume_checks_pods_and_does_not_claim_smoke(self):
        phase = self.new()
        phase.apply()
        self.client.resume(self.cut_journal, self.ledger)
        result = phase.verify_after_resume()
        self.assertEqual(len(result), 23)
        self.assertNotIn('smoke', result)
        self.client.pods[0]['status']['containerStatuses'][0]['imageID'] = 'sha256:' + 'd' * 64
        with self.assertRaises(cohort.RecoveryRequired): phase.verify_after_resume()

    def test_resume_lineage_and_resume_can_be_rechecked_sequentially(self):
        phase=self.new(); phase.apply(); self.client.resume(self.cut_journal,self.ledger)
        before=self.client.patches
        first=phase.verify_after_resume()
        self.assertFalse(phase.verifying)
        lineage=phase.verify_activation_lineage()
        self.assertFalse(phase.verifying)
        self.assertEqual(len(lineage['images']),23)
        self.assertEqual(phase.verify_after_resume(),first)
        self.assertFalse(phase.verifying)
        self.assertEqual(self.client.patches,before)
        self.assertEqual(self.ledger.row['status'],'active')

    def test_resume_error_clears_own_flag_but_preserves_uncertainty(self):
        phase=self.new(); phase.apply(); self.client.resume(self.cut_journal,self.ledger)
        self.client.pods[0]['status']['containerStatuses'][0]['imageID']='sha256:'+'d'*64
        with self.assertRaises(cohort.RecoveryRequired): phase.verify_after_resume()
        self.assertFalse(phase.verifying)
        self.assertTrue(phase.poisoned)
        self.assertEqual(self.ledger.row['status'],'uncertain')
        with self.assertRaises(cohort.RecoveryRequired): phase.verify_activation_lineage()
        self.assertEqual(self.ledger.row['status'],'uncertain')

    def test_nested_resume_verification_refuses_without_clearing_outer_flag(self):
        phase=self.new(); phase.apply(); self.client.resume(self.cut_journal,self.ledger)
        original=self.client.call; seen=[]; entered=[False]
        def nested(*args,**kwargs):
            if not entered[0]:
                entered[0]=True
                try: phase.verify_after_resume()
                except cohort.RecoveryRequired: seen.append(phase.verifying)
                else: self.fail('nested verification was accepted')
            return original(*args,**kwargs)
        with mock.patch.object(self.client,'call',side_effect=nested):
            with self.assertRaises(cohort.RecoveryRequired): phase.verify_after_resume()
        self.assertEqual(seen,[True])
        self.assertFalse(phase.verifying)
        self.assertTrue(phase.poisoned)
        self.assertEqual(self.ledger.row['status'],'uncertain')

    def test_pod_defaults_and_injected_service_account_are_narrowly_checked(self):
        phase = self.new()
        name = 'voice-web'
        target = phase._target(name)
        pod = {'metadata': {'name': 'web-pod', 'namespace': 'voice-dev', 'uid': 'pod', 'resourceVersion': '2',
                           'labels': {'app': name}},
               'spec': copy.deepcopy(target['template']['spec']), 'status': {'phase': 'Running', 'conditions': [{'type': 'Ready', 'status': 'True'}],
                   'containerStatuses': [{'name': 'web', 'ready': True, 'imageID': 'containerd://sha256:' + 'c' * 64}]}}
        pod['spec'].update(serviceAccountName='default', serviceAccount='default', dnsPolicy='ClusterFirst',
                           restartPolicy='Always', schedulerName='default-scheduler', terminationGracePeriodSeconds=30,
                           enableServiceLinks=True, preemptionPolicy='PreemptLowerPriority', priority=0,
                           securityContext={}, nodeName='node-1', tolerations=[
                               {'key': 'node.kubernetes.io/not-ready', 'operator': 'Exists', 'effect': 'NoExecute', 'tolerationSeconds': 300},
                               {'key': 'node.kubernetes.io/unreachable', 'operator': 'Exists', 'effect': 'NoExecute', 'tolerationSeconds': 300}])
        pod['spec']['containers'][0]['volumeMounts'] = [{'name': 'kube-api-access-abc',
            'mountPath': '/var/run/secrets/kubernetes.io/serviceaccount', 'readOnly': True}]
        pod['spec']['volumes'] = [{'name': 'kube-api-access-abc', 'projected': {'defaultMode': 420, 'sources': [
            {'serviceAccountToken': {'expirationSeconds': 3607, 'path': 'token'}},
            {'configMap': {'name': 'kube-root-ca.crt', 'items': [{'key': 'ca.crt', 'path': 'ca.crt'}]}},
            {'downwardAPI': {'items': [{'path': 'namespace', 'fieldRef': {'apiVersion': 'v1', 'fieldPath': 'metadata.namespace'}}]}}]}}]
        self.assertTrue(phase._valid_pod(name, pod, target))
        bad = copy.deepcopy(pod)
        bad['spec']['volumes'][0]['projected']['sources'][1]['configMap']['name'] = 'foreign'
        self.assertFalse(phase._valid_pod(name, bad, target))
        bad = copy.deepcopy(pod)
        bad['spec']['volumes'][0]['projected']['sources'][0]['serviceAccountToken']['expirationSeconds'] = -1
        self.assertFalse(phase._valid_pod(name, bad, target))
        annotated_target = copy.deepcopy(target)
        annotated_target['template']['metadata']['annotations'] = {'source': 'admitted'}
        annotated_target['template']['spec']['containers'][0]['env'].append({'name': 'SOURCE', 'valueFrom': {
            'fieldRef': {'fieldPath': "metadata.annotations['source']"}}})
        annotated_pod = copy.deepcopy(pod)
        annotated_pod['spec']['containers'][0]['env'] = copy.deepcopy(annotated_target['template']['spec']['containers'][0]['env'])
        annotated_pod['metadata']['annotations'] = {'source': 'admitted'}
        self.assertTrue(phase._valid_pod(name, annotated_pod, annotated_target))
        annotated_pod['metadata']['annotations']['source'] = 'foreign'
        self.assertFalse(phase._valid_pod(name, annotated_pod, annotated_target))
        annotated_target['template']['metadata']['annotations'] = {}
        self.assertFalse(phase._valid_pod(name, annotated_pod, annotated_target))
        target['template']['spec']['automountServiceAccountToken'] = False
        pod['spec']['automountServiceAccountToken'] = False
        self.assertFalse(phase._valid_pod(name, pod, target))

    def test_resume_wrong_replicas_and_foreign_pod_refused(self):
        for fault in ('replicas', 'pod', 'account', 'pod_spec', 'pod_labels', 'pod_failed', 'rs_terminating'):
            with self.subTest(fault=fault):
                if fault != 'replicas': self.setUp()
                phase = self.new()
                phase.apply()
                self.client.resume(self.cut_journal, self.ledger)
                if fault == 'replicas': self.client.deployments[sorted(self.client.deployments)[0]]['spec']['replicas'] = 2
                if fault == 'pod': self.client.pods[0]['metadata']['ownerReferences'][0]['uid'] = 'foreign'
                if fault == 'account': self.client.accounts['default']['metadata']['resourceVersion'] = '2'
                if fault == 'pod_spec': self.client.pods[0]['spec']['containers'][0]['command'] = ['foreign']
                if fault == 'pod_labels': self.client.pods[0]['metadata']['labels']['app'] = 'foreign'
                if fault == 'pod_failed': self.client.pods[0]['status']['phase'] = 'Failed'
                if fault == 'rs_terminating': self.client.replicasets[0]['metadata']['deletionTimestamp'] = 'now'
                with self.assertRaises(cohort.RecoveryRequired): phase.verify_after_resume()

    def test_inherited_pull_secrets_refused_before_patch(self):
        self.journal.intent['full_cohort']['service_accounts']['default']['imagePullSecrets'] = [{'name': 'private'}]
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        with self.assertRaises(cohort.RecoveryRequired): self.new()
        self.assertEqual(self.client.patches, 0)

    def test_readiness_refuses_ledger_drift_during_final_pod_read(self):
        phase = self.new()
        phase.apply()
        self.client.resume(self.cut_journal, self.ledger)
        call = self.client.call
        pod_reads = 0
        def drift(*args, **kwargs):
            nonlocal pod_reads
            result = call(*args, **kwargs)
            if len(args) > 1 and args[1] == 'pods':
                pod_reads += 1
                if pod_reads == 2:
                    with self.ledger.lock:
                        self.ledger.row['full_cohort_effects'].pop(next(iter(self.ledger.row['full_cohort_effects'])))
                        self.ledger.version += 1
            return result
        with mock.patch.object(self.client, 'call', side_effect=drift):
            with self.assertRaises(cohort.RecoveryRequired): phase.verify_after_resume()
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_unrelated_namespace_workload_churn_does_not_block_readiness(self):
        phase = self.new()
        phase.apply()
        self.client.resume(self.cut_journal, self.ledger)
        call = self.client.call
        reads = {'replicasets': 0, 'pods': 0}
        def churn(*args, **kwargs):
            result = call(*args, **kwargs)
            kind = args[1]
            if kind in reads:
                reads[kind] += 1
                if reads[kind] == 2:
                    result['items'].append({'metadata': {'uid': 'unrelated-' + kind}})
            return result
        with mock.patch.object(self.client, 'call', side_effect=churn):
            self.assertEqual(len(phase.verify_after_resume()), 23)


if __name__ == '__main__': unittest.main()
