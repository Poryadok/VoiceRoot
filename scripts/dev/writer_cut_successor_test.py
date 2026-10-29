"""Composition fixture: real Journals/Fence, fake source-owned producer/Kube."""
import copy
import sys
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

import media_operation_journal as journal
import writer_cut
import writer_cut_test as fixture


class BindRaceTest(unittest.TestCase):
    def setUp(self):
        fixture.WriterCutTest.setUp(self)
        source = self.sources[0]
        self.client.workloads.pop(('Deployment', 'voice-a'))
        source['metadata']['name'] = 'voice-user'
        self.client.workloads[('Deployment', 'voice-user')] = copy.deepcopy(source)
        self.admission['controllers'] = copy.deepcopy(self.sources)
        self.cut.admission = copy.deepcopy(self.admission)
        self.journal.intent['writer_cut'] = copy.deepcopy(self.admission)
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.cut.intent = copy.deepcopy(self.journal.intent)
        self.cut.drain()
        intent = {'resource_context': copy.deepcopy(self.context), 'namespace_uid': 'ns',
                  'backend_config_deployments': {'source_sha': self.context['commit'],
                       'cut_invocation': self.cut.invocation, 'services': ['user']}}
        self.config = journal.Journal(Path(self.temp.name) / 'config-journal', intent)
        self.addCleanup(self.config.close)

    def test_config_effect_inserted_during_plan_binding_holds(self):
        reserve = self.cut._reserve
        def concurrent(name, details):
            record = reserve(name, details)
            if name == 'writer-cut-config-plan':
                self.ledger.row['backend_config_effects'] = {
                    'backend-config-deployment-voice-user': {'status': 'pending'}}
                self.ledger.version += 1
            return record
        with mock.patch.object(self.cut, '_reserve', side_effect=concurrent):
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.bind_config_plan(self.config)
        self.assertEqual(self.ledger.row['status'], 'uncertain')


class SuccessorTest(unittest.TestCase):
    def setUp(self):
        fixture.WriterCutTest.setUp(self)
        source = self.sources[0]
        self.client.workloads.pop(('Deployment', 'voice-a'))
        source['metadata']['name'] = 'voice-user'
        self.client.workloads[('Deployment', 'voice-user')] = copy.deepcopy(source)
        other = self.sources[1]
        self.client.workloads.pop(('StatefulSet', 'voice-b'))
        other['kind'] = 'Deployment'
        other['metadata']['name'] = 'voice-notification'
        self.client.workloads[('Deployment', 'voice-notification')] = copy.deepcopy(other)
        self.admission['controllers'] = copy.deepcopy(self.sources)
        self.cut.admission = copy.deepcopy(self.admission)
        self.journal.intent['writer_cut'] = copy.deepcopy(self.admission)
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.cut.intent = copy.deepcopy(self.journal.intent)
        self.cut.drain()
        self.config_intent = {'resource_context': copy.deepcopy(self.context), 'namespace_uid': 'ns',
            'backend_config_deployments': {'source_sha': self.context['commit'],
                'cut_invocation': self.cut.invocation, 'services': ['user', 'notification']}}
        self.config = journal.Journal(Path(self.temp.name) / 'config-journal', self.config_intent)
        self.addCleanup(self.config.close)
        self.cut.bind_config_plan(self.config)
        current = self.client.workloads[('Deployment', 'voice-user')]
        source = copy.deepcopy(current)
        target = copy.deepcopy(source['spec'])
        target['template']['spec']['containers'][0]['env'] = [{'name': 'CONFIG', 'value': 'fixture'}]
        current['spec'] = copy.deepcopy(target)
        current['metadata']['generation'] += 1
        current['metadata']['resourceVersion'] = '8'
        current['status']['observedGeneration'] = current['metadata']['generation']
        self.step = 'backend-config-deployment-voice-user'
        receipt = {'uid': source['metadata']['uid'], 'spec_sha256': writer_cut.digest(target),
                   'responseResourceVersion': '8', 'resourceVersion': '8', 'generation': current['metadata']['generation']}
        details = {'operation_id': self.context['operation_id'], 'invocation_nonce': self.cut.invocation,
                   'source': source, 'target_spec': target, 'patch': [
                       {'op': 'test', 'path': '/metadata/uid', 'value': source['metadata']['uid']},
                       {'op': 'test', 'path': '/metadata/resourceVersion', 'value': source['metadata']['resourceVersion']},
                       {'op': 'test', 'path': '/spec', 'value': source['spec']},
                       {'op': 'replace', 'path': '/spec', 'value': target}]}
        entry = {'status': 'done', 'nonce': 'a' * 32, 'details': details,
                 'details_sha256': writer_cut.digest(details), 'receipt': receipt}
        self.config.state['steps'][self.step] = copy.deepcopy(entry)
        self.config.save()
        ledger_record = {'status': 'done', 'nonce': entry['nonce'], 'details_sha256': entry['details_sha256'],
                         'receipt_sha256': writer_cut.digest(receipt)}
        self.ledger.row['backend_config_effects'] = {self.step: copy.deepcopy(ledger_record)}
        self.ledger.version += 1
        proof = {'source': source, 'target_spec': target, 'receipt': receipt, 'ledger_record': ledger_record}
        self.source_module = SimpleNamespace(verify_deployment_successor=lambda cut, config, service: copy.deepcopy(proof))

    def test_authenticated_config_successor_preserves_cut(self):
        with mock.patch.dict(sys.modules, {'backend_config_transition': self.source_module}):
            result = self.cut.accept_config_successor(self.config, 'user')
            self.assertIsInstance(result, dict)
            self.assertEqual(result['successor']['generation'], 3)
            self.assertEqual(self.cut.verify_drained()['active_sessions'], 0)
            self.assertEqual(self.cut.admission['controllers'][0]['spec']['replicas'], 2)

    def test_config_ledger_nonce_tamper_is_not_admitted(self):
        self.ledger.row['backend_config_effects'][self.step]['nonce'] = 'b' * 32
        with mock.patch.dict(sys.modules, {'backend_config_transition': self.source_module}):
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.accept_config_successor(self.config, 'user')
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_skipped_cut_predecessor_is_not_admitted_even_with_coherent_hashes(self):
        entry = self.config.state['steps'][self.step]
        entry['details']['source']['metadata']['generation'] += 1
        entry['details']['patch'][2]['value']['template']['spec']['containers'][0]['image'] = 'foreign'
        entry['details']['source']['spec'] = copy.deepcopy(entry['details']['patch'][2]['value'])
        entry['details_sha256'] = writer_cut.digest(entry['details'])
        record = self.ledger.row['backend_config_effects'][self.step]
        record['details_sha256'] = entry['details_sha256']
        proof = {'source': entry['details']['source'], 'target_spec': entry['details']['target_spec'],
                 'receipt': entry['receipt'], 'ledger_record': record}
        self.source_module.verify_deployment_successor = lambda cut, config, service: copy.deepcopy(proof)
        with mock.patch.dict(sys.modules, {'backend_config_transition': self.source_module}):
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.accept_config_successor(self.config, 'user')

    def test_changed_live_target_blocks_acceptance(self):
        self.client.workloads[('Deployment', 'voice-user')]['spec']['replicas'] = 1
        with mock.patch.dict(sys.modules, {'backend_config_transition': self.source_module}):
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.accept_config_successor(self.config, 'user')

    def test_completed_config_proof_changed_after_acceptance_holds(self):
        with mock.patch.dict(sys.modules, {'backend_config_transition': self.source_module}):
            self.cut.accept_config_successor(self.config, 'user')
            self.ledger.row['backend_config_effects'][self.step]['nonce'] = 'b' * 32
            self.ledger.version += 1
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.verify_drained()
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_config_proof_changed_after_acceptance_blocks_resume(self):
        with mock.patch.dict(sys.modules, {'backend_config_transition': self.source_module}):
            self.cut.accept_config_successor(self.config, 'user')
            self.ledger.row['backend_config_effects'][self.step]['nonce'] = 'b' * 32
            self.ledger.version += 1
            targets = [copy.deepcopy(self.client.workloads[(o['kind'], o['metadata']['name'])]) for o in self.sources]
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.resume(targets)
        self.assertEqual(len(self.client.patches), 2)

    def test_incomplete_bound_plan_blocks_resume(self):
        with mock.patch.dict(sys.modules, {'backend_config_transition': self.source_module}):
            self.cut.accept_config_successor(self.config, 'user')
            targets = [copy.deepcopy(self.client.workloads[(o['kind'], o['metadata']['name'])]) for o in self.sources]
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.resume(targets)
        self.assertEqual(len(self.client.patches), 2)

    def test_config_ledger_drift_after_final_guard_blocks_patch(self):
        with mock.patch.dict(sys.modules, {'backend_config_transition': self.source_module}):
            self.cut.accept_config_successor(self.config, 'user')
            guard = self.cut._guard
            count = 0
            def drift_after_guard():
                nonlocal count
                guard()
                pending = self.ledger.row.get('writer_cut_effects', {}).get('writer-resume-deployment-voice-user')
                if pending and pending.get('status') == 'pending':
                    count += 1
                    if count == 2:
                        self.ledger.row['backend_config_effects'][self.step]['nonce'] = 'b' * 32
                        self.ledger.version += 1
            source = copy.deepcopy(self.client.workloads[('Deployment', 'voice-user')])
            with mock.patch.object(self.cut, '_guard', side_effect=drift_after_guard):
                with self.assertRaises(writer_cut.RecoveryRequired): self.cut._effect('resume', source, 2)
        self.assertEqual(len(self.client.patches), 2)

    def test_skipped_service_order_holds(self):
        with mock.patch.dict(sys.modules, {'backend_config_transition': self.source_module}):
            with self.assertRaises(writer_cut.RecoveryRequired):
                self.cut.accept_config_successor(self.config, 'notification')

    def test_incremental_successors_retain_previous_proof(self):
        user_proof = self.source_module.verify_deployment_successor(self.cut, self.config, 'user')
        with mock.patch.dict(sys.modules, {'backend_config_transition': self.source_module}):
            self.cut.accept_config_successor(self.config, 'user')
            self.assertEqual(self.cut.verify_drained()['active_sessions'], 0)
            current = self.client.workloads[('Deployment', 'voice-notification')]
            source = copy.deepcopy(current)
            target = copy.deepcopy(source['spec'])
            target['template']['spec']['containers'][0]['env'] = [{'name': 'CONFIG', 'value': 'next'}]
            current['spec'] = copy.deepcopy(target)
            current['metadata']['generation'] += 1
            current['metadata']['resourceVersion'] = '9'
            current['status']['observedGeneration'] = current['metadata']['generation']
            receipt = {'uid': source['metadata']['uid'], 'spec_sha256': writer_cut.digest(target),
                       'responseResourceVersion': '9', 'resourceVersion': '9', 'generation': current['metadata']['generation']}
            details = {'operation_id': self.context['operation_id'], 'invocation_nonce': self.cut.invocation,
                       'source': source, 'target_spec': target, 'patch': [
                           {'op': 'test', 'path': '/metadata/uid', 'value': source['metadata']['uid']},
                           {'op': 'test', 'path': '/metadata/resourceVersion', 'value': source['metadata']['resourceVersion']},
                           {'op': 'test', 'path': '/spec', 'value': source['spec']},
                           {'op': 'replace', 'path': '/spec', 'value': target}]}
            entry = {'status': 'done', 'nonce': 'c' * 32, 'details': details,
                     'details_sha256': writer_cut.digest(details), 'receipt': receipt}
            step = 'backend-config-deployment-voice-notification'
            self.config.state['steps'][step] = copy.deepcopy(entry)
            self.config.save()
            record = {'status': 'done', 'nonce': entry['nonce'], 'details_sha256': entry['details_sha256'],
                      'receipt_sha256': writer_cut.digest(receipt)}
            self.ledger.row['backend_config_effects'][step] = copy.deepcopy(record)
            self.ledger.version += 1
            next_proof = {'source': source, 'target_spec': target, 'receipt': receipt, 'ledger_record': record}
            self.source_module.verify_deployment_successor = lambda cut, config, service: copy.deepcopy(
                {'user': user_proof, 'notification': next_proof}[service])
            result = self.cut.accept_config_successor(self.config, 'notification')
            self.assertEqual(result['successor']['service'], 'notification')
            self.assertEqual(self.cut.verify_drained()['active_transactions'], 0)
            proof = self.cut.current_cut_proof()
            self.assertEqual(proof['invocation_nonce'], self.cut.invocation)
            self.assertEqual(proof['writers'][0]['spec'], self.client.workloads[('Deployment', 'voice-user')]['spec'])
            self.assertEqual(proof['writers'][0]['successor']['step'], 'writer-cut-config-successor-user')
            self.config.state['steps']['backend-config-deployment-voice-extra'] = {'status': 'pending'}
            self.ledger.row['backend_config_effects']['backend-config-deployment-voice-extra'] = {'status': 'pending'}
            self.ledger.version += 1
            targets = [copy.deepcopy(self.client.workloads[(o['kind'], o['metadata']['name'])]) for o in self.sources]
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.resume(targets)
            self.assertEqual(len(self.client.patches), 2)

    def test_incomplete_service_set_holds(self):
        self.ledger.row['backend_config_effects'].pop(self.step)
        with mock.patch.dict(sys.modules, {'backend_config_transition': self.source_module}):
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.accept_config_successor(self.config, 'user')

    def test_lost_done_response_never_replays_acceptance(self):
        write = self.ledger.write
        def lost(revision, row):
            write(revision, row)
            if row.get('writer_cut_effects', {}).get('writer-cut-config-successor-user', {}).get('status') == 'done':
                raise OSError('lost response after commit')
        with mock.patch.dict(sys.modules, {'backend_config_transition': self.source_module}), \
             mock.patch.object(self.ledger, 'write', side_effect=lost):
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.accept_config_successor(self.config, 'user')
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.accept_config_successor(self.config, 'user')
