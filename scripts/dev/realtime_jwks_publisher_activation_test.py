"""Real WriterCut and publisher health journals; synthetic source and Kube boundaries."""
import copy
from pathlib import Path
import unittest
from unittest.mock import patch

import full_cohort_package_phase
import media_operation_journal
import realtime_jwks_publisher as publisher
import realtime_jwks_publisher_health as health
import realtime_jwks_publisher_phase_test as publisher_fixture
import resource_fencing
import writer_cut
import writer_cut_activation_test as fixture
import writer_cut_test


class JointActivationTest(unittest.TestCase):
    def setUp(self):
        self.base = fixture.ActivationTest('test_fixed_barriers_and_ordered_cas'); self.base.setUp()
        self.addCleanup(self.base.doCleanups)
        self.base.cut.prepare_activation(self.base.phase, 'token')
        self.base.cut.activate_next(); self.base.accept('space_health')
        intent = {'resource_context':self.base.context, 'namespace_uid':'ns'}
        self.journal = media_operation_journal.Journal(Path(self.base.temp.name) / 'publisher-health', intent)
        self.addCleanup(self.journal.close)
        self.client = writer_cut_test.Client(self.base.lock, self.journal)
        self.fence = resource_fencing.Fence(self.base.context, lambda: copy.deepcopy(self.base.card),
                    publisher_fixture.View(self.base.ledger, self.client))
        self.publisher = object.__new__(publisher.PublisherPhase)
        self.publisher.fence = self.base.fence; self.publisher.lock = self.base.lock; self.publisher.namespace_uid = 'ns'
        self.publisher._guard = lambda: None
        self.package = object.__new__(full_cohort_package_phase.PackagePhase)
        self.package.fence = self.base.fence; self.package.lock = self.base.lock; self.package.namespace_uid = 'ns'
        self.package.verify_current = lambda token: None
        self.phase = health.PublisherHealthPhase(self.journal, self.fence, self.client, self.base.lock,
                                                  'ns', self.publisher, self.package)
        self.proof = {'fixture':'bounded observation'}

    def test_actual_health_and_cut_barrier_then_chat(self):
        self.assertEqual(self.base.cut.verify_activation_stage()['stage'], 'publisher_health')
        with patch.object(self.phase, '_observe', return_value=self.proof):
            self.phase.run('token', self.base.cut)
            self.assertEqual(self.base.cut.accept_activation_health('publisher_health', self.phase, 'token')['stage'], 'chat')
            self.assertEqual(self.base.cut.activate_next()['stage'], 'chat_health')
        marker = self.base.ledger.row['writer_cut_effects']['writer-cut-publisher-health']
        self.assertEqual(marker['status'], 'done')

    def test_real_observation_receipt_crosses_actual_cut_barrier(self):
        proof = {'operation_id':self.base.context['operation_id'], 'candidate_source_sha':self.base.context['commit'],
                 'namespace_uid':'ns', 'runtime':{'pod_name':'publisher'},
                 'public':{'sha256':'a'*64}, 'tls':{'certificate_sha256':'b'*64}}
        with patch.object(publisher, 'verify_publisher', return_value=proof), \
             patch.object(health, '_routing', return_value={'endpoint':'synthetic'}), \
             patch.object(health, '_materials', return_value=(b'public', b'ca', b'cert')), \
             patch.object(health, '_tunnel', return_value=b'public'), \
             patch.object(health.crypto, 'der', return_value=b'ca-der'):
            receipt = self.phase.run('token', self.base.cut)
            self.assertEqual(receipt['activation_plan_sha256'], self.base.cut.activation['plan_sha256'])
            self.assertEqual(self.base.cut.accept_activation_health('publisher_health', self.phase, 'token')['stage'], 'chat')
            self.assertEqual(self.base.cut.activate_next()['stage'], 'chat_health')

    def test_cut_health_barrier_drift_holds_before_chat(self):
        with patch.object(self.phase, '_observe', return_value=self.proof):
            self.phase.run('token', self.base.cut)
            self.base.cut.accept_activation_health('publisher_health', self.phase, 'token')
            self.base.ledger.row['writer_cut_effects']['writer-cut-publisher-health']['nonce'] = 'f'*32
            before = len(self.base.client.patches)
            with self.assertRaises(writer_cut.RecoveryRequired): self.base.cut.activate_next()
            self.assertEqual(len(self.base.client.patches), before)
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')


if __name__ == '__main__': unittest.main()
