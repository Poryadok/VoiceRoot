"""Real cut/journal with synthetic phase validators and Kubernetes."""
import copy
import sys
import types
import unittest
from pathlib import Path
from unittest import mock

import media_operation_journal as journal
import writer_cut
import writer_cut_test as fixture


class ActivationTest(unittest.TestCase):
    def setUp(self):
        fixture.WriterCutTest.setUp(self)
        sources = []
        for index, name in enumerate(('voice-space', 'voice-chat', 'voice-realtime')):
            source = fixture.workload('Deployment', name, 'uid-' + str(index), 1, 'old')
            sources.append(source)
        self.sources = sources
        self.admission['controllers'] = copy.deepcopy(sources)
        self.cut.admission = copy.deepcopy(self.admission)
        self.journal.intent['writer_cut'] = copy.deepcopy(self.admission)
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.cut.intent = copy.deepcopy(self.journal.intent)
        self.client.workloads = {(o['kind'], o['metadata']['name']): copy.deepcopy(o) for o in sources}
        self.cut.drain()
        self.cut.resume_inventory = {'fixture': True}
        self.inventory_patch = mock.patch.object(self.cut, '_resume_inventory_matches', return_value=True)
        self.inventory_patch.start(); self.addCleanup(self.inventory_patch.stop)
        ordered = []
        for source in sources:
            obj = self.client.workloads[(source['kind'], source['metadata']['name'])]
            ordered.append({'kind': obj['kind'], 'name': obj['metadata']['name'], 'uid': obj['metadata']['uid'],
                            'generation': obj['metadata']['generation'], 'spec': copy.deepcopy(obj['spec'])})
        self.plan = {'operation_id': self.context['operation_id'], 'namespace_uid': 'ns',
                     'source_sha': self.context['commit'], 'cut_invocation': self.cut.invocation,
                     'package_sha256': 'a' * 64, 'ordered': ordered,
                     'barriers': ['space_health', 'publisher_health', 'chat_health']}
        class ActivationPhase:
            def __init__(inner):
                inner.journal = journal.Journal(Path(self.temp.name) / 'activation-source',
                    {'resource_context': self.context, 'namespace_uid': 'ns'})
                self.addCleanup(inner.journal.close)
                inner.fence, inner.client, inner.lock, inner.namespace_uid = self.fence, self.client, self.lock, 'ns'
                marker = {'status': 'done', 'nonce': 'f' * 32, 'operation_id': self.context['operation_id'],
                          'receipt_sha256': writer_cut.digest(self.plan)}
                inner.journal.state['steps']['full-cohort-activation-plan'] = marker | {'receipt': copy.deepcopy(self.plan)}
                inner.journal.save(); self.ledger.row['full_cohort_activation_plan'] = marker
        self.phase = ActivationPhase()
        module = types.SimpleNamespace(ActivationPhase=ActivationPhase,
                                       verify_completed=lambda phase, cut, token: copy.deepcopy(self.plan))
        self.module_patch = mock.patch.dict(sys.modules, {'full_cohort_activation_plan': module})
        self.module_patch.start(); self.addCleanup(self.module_patch.stop)

    def accept(self, name):
        module_name, class_name = {'space_health': ('realtime_config_transition_space_health', 'SpaceHealthPhase'),
            'publisher_health': ('realtime_jwks_publisher_health', 'PublisherHealthPhase'),
            'chat_health': ('full_cohort_chat_health', 'ChatHealthPhase')}[name]
        proof = {'name': name, 'operation_id': self.context['operation_id']}
        class Phase:
            def __init__(inner):
                inner.journal = journal.Journal(Path(self.temp.name) / name,
                    {'resource_context': self.context, 'namespace_uid': 'ns'})
                self.addCleanup(inner.journal.close)
                inner.fence, inner.client, inner.lock, inner.namespace_uid = self.fence, self.client, self.lock, 'ns'
                marker = {'status': 'done', 'receipt_sha256': writer_cut.digest(proof)}
                inner.journal.state['steps'][name] = marker | {'receipt': copy.deepcopy(proof)}
                inner.journal.save(); self.ledger.row[name] = marker
        module = types.SimpleNamespace(STEP=name, LEDGER=name, verify_completed=lambda *args: copy.deepcopy(proof))
        setattr(module, class_name, Phase)
        self.module_patch2 = mock.patch.dict(sys.modules, {module_name: module})
        self.module_patch2.start(); self.addCleanup(self.module_patch2.stop)
        return self.cut.accept_activation_health(name, Phase(), 'token')

    def test_fixed_barriers_and_ordered_cas(self):
        self.assertEqual(self.cut.prepare_activation(self.phase, 'token')['stage'], 'space')
        self.assertEqual(self.cut.activate_next()['stage'], 'space_health')
        self.assertEqual(self.accept('space_health')['stage'], 'publisher_health')
        self.assertEqual(self.accept('publisher_health')['stage'], 'chat')
        self.assertEqual(self.cut.activate_next()['stage'], 'chat_health')
        self.assertEqual(self.accept('chat_health')['stage'], 'realtime')
        self.assertEqual(self.cut.activate_next()['stage'], 'done')
        self.assertEqual(self.cut.finish_activation()['status'], 'replicas-restored')

    def test_barrier_cannot_be_skipped(self):
        self.cut.prepare_activation(self.phase, 'token')
        self.cut.activate_next()
        before = len(self.client.patches)
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.activate_next()
        self.assertEqual(len(self.client.patches), before)
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_bulk_resume_cannot_bypass_fixed_barriers(self):
        targets = [copy.deepcopy(obj) for obj in self.client.workloads.values()]
        before = len(self.client.patches)
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.resume(targets)
        self.assertEqual(len(self.client.patches), before)

    def test_source_order_cannot_place_realtime_before_space_or_chat(self):
        self.plan['ordered'] = list(reversed(self.plan['ordered']))
        entry = self.phase.journal.state['steps']['full-cohort-activation-plan']
        entry['receipt'] = copy.deepcopy(self.plan)
        entry['receipt_sha256'] = writer_cut.digest(self.plan)
        self.ledger.row['full_cohort_activation_plan']['receipt_sha256'] = entry['receipt_sha256']
        self.phase.journal.save()
        before = len(self.client.patches)
        with self.assertRaises(writer_cut.RecoveryRequired): self.cut.prepare_activation(self.phase, 'token')
        self.assertEqual(len(self.client.patches), before)

    def test_source_marker_drift_after_verifier_blocks_cas(self):
        self.cut.prepare_activation(self.phase, 'token')
        verify = self.cut._activation_verify_sources
        def drift():
            verify()
            self.ledger.row['full_cohort_activation_plan']['nonce'] = 'a' * 32
            self.ledger.version += 1
        before = len(self.client.patches)
        with mock.patch.object(self.cut, '_activation_verify_sources', side_effect=drift):
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.activate_next()
        self.assertEqual(len(self.client.patches), before)
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_publisher_health_lost_before_chat_cas_holds(self):
        self.cut.prepare_activation(self.phase, 'token')
        self.cut.activate_next()
        self.accept('space_health')
        self.accept('publisher_health')
        module = sys.modules['realtime_jwks_publisher_health']
        original = module.verify_completed
        calls = []
        def lost(*args):
            calls.append(True)
            if len(calls) == 2: raise RuntimeError('publisher became unavailable')
            return original(*args)
        before = len(self.client.patches)
        with mock.patch.object(module, 'verify_completed', side_effect=lost):
            with self.assertRaises(writer_cut.RecoveryRequired): self.cut.activate_next()
        self.assertEqual(len(calls), 2)
        self.assertEqual(len(self.client.patches), before)
        self.assertEqual(self.ledger.row['status'], 'uncertain')


if __name__ == '__main__': unittest.main()
