"""Actual WriterCut/ChatHealth receipt composition with synthetic observations."""
import copy
from pathlib import Path
import unittest
from unittest.mock import patch

import full_cohort_chat_health as health
import media_operation_journal
import writer_cut
import writer_cut_activation_test


class CutIntegrationTest(unittest.TestCase):
    def test_actual_health_receipt_blocks_realtime_if_recheck_fails(self):
        base = writer_cut_activation_test.ActivationTest('test_fixed_barriers_and_ordered_cas')
        base.setUp(); self.addCleanup(base.doCleanups)
        base.cut.prepare_activation(base.phase, 'token')
        base.cut.activate_next(); base.accept('space_health'); base.accept('publisher_health')
        self.assertEqual(base.cut.activate_next()['stage'], 'chat_health')
        journal = media_operation_journal.Journal(Path(base.temp.name)/'real-chat-health',
            {'resource_context':base.context, 'namespace_uid':'ns'})
        self.addCleanup(journal.close)
        phase = object.__new__(health.ChatHealthPhase)
        phase.journal, phase.fence, phase.client = journal, base.fence, base.client
        phase.lock, phase.namespace_uid = copy.deepcopy(base.lock), 'ns'
        proof = {'operation_id':base.context['operation_id'], 'scope':'synthetic-chat-observation'}
        marker = {'status':'done', 'nonce':'d'*32, 'operation_id':base.context['operation_id'],
                  'source_sha':base.context['commit'], 'cut_invocation':base.cut.invocation,
                  'receipt_sha256':health._digest(proof)}
        journal.state['steps'][health.STEP] = marker | {'receipt':copy.deepcopy(proof)}; journal.save()
        base.ledger.row[health.LEDGER] = copy.deepcopy(marker)
        calls = []
        def observe(*args):
            calls.append(True)
            if len(calls) == 2: raise health.HealthError('synthetic peer became unavailable')
            return copy.deepcopy(proof)
        with patch.object(health.ChatHealthPhase, '_guard'), patch.object(health.ChatHealthPhase, '_observe', side_effect=observe):
            self.assertEqual(base.cut.accept_activation_health('chat_health', phase, 'token')['stage'], 'realtime')
            before = len(base.client.patches)
            with self.assertRaises(writer_cut.RecoveryRequired): base.cut.activate_next()
            self.assertEqual(len(base.client.patches), before)
            self.assertEqual(base.ledger.row['status'], 'uncertain')


if __name__ == '__main__': unittest.main()
