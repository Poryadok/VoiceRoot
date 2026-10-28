import copy
import unittest
from pathlib import Path
from unittest.mock import patch
import media_backup_phase as media
import media_backup_phase_test as fixture
import media_operation_journal as journal
import cohort_backup_phases as cohort


class CohortPhasesTest(unittest.TestCase):
    def setUp(self):
        self.base = fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.base.setUp(); self.addCleanup(self.base.doCleanups)
        engine = {'namespace_uid':'ns', **{k:self.base.capture[k] for k in
                    ('pod_uid','pvc_uid','pv_uid','container_id','restart_count')}}
        sources = {'roles':{'engine':engine,'server_start':'2026-10-28T20:00:00Z'},
                   'voice_db':{'engine':engine,'server_start':'2026-10-28T20:00:00Z','database_oid':55}}
        self.binding = {'operation_id':self.base.context['operation_id'], 'snapshot_id':self.base.capture['snapshot_id'],
                        'source_sha':self.base.context['commit'], 'databases':['voice_db'], 'sources':sources}
        intent = self.base.intent | {'cohort_binding': self.binding}
        self.journal = journal.Journal(Path(self.base.temp.name)/'cohort-journal', intent)
        self.addCleanup(self.journal.close)
        self.media = media.BackupPhases(self.journal, self.base.fence, fixture.Client(self.base.lock), self.base.lock, 'ns')
        self.phase = cohort.CohortPhases(self.media)

    def receipt(self, member, nonce):
        source = self.binding['sources'][member]
        return {'operation_id':self.binding['operation_id'], 'snapshot_id':self.binding['snapshot_id'],
                'member':member, **{k:self.base.capture[k] for k in ('pod_uid','pvc_uid','pv_uid','container_id','restart_count')},
                'server_start':source['server_start'], 'database_oid':source.get('database_oid'),
                'attempt_nonce':nonce, 'exit':0, 'scope':'dump-terminal-only'}

    def test_exact_member_receipt_and_no_replay(self):
        nonce = self.phase.begin('voice_db')
        self.phase.complete('voice_db', self.receipt('voice_db', nonce))
        self.assertEqual(self.base.ledger.row['cohort_members']['voice_db']['status'], 'done')
        with self.assertRaises(cohort.CohortPhaseError): self.phase.begin('voice_db')
        with self.assertRaises(cohort.CohortPhaseError): self.phase.begin('file_db')

    def test_wrong_oid_or_lost_commit_never_succeeds(self):
        nonce = self.phase.begin('voice_db')
        with self.assertRaises(cohort.CohortPhaseError):
            self.phase.complete('voice_db', self.receipt('voice_db', nonce) | {'database_oid':56})
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')

    def test_roles_and_database_are_sequential_with_distinct_nonces(self):
        first = self.phase.begin('roles')
        self.phase.complete('roles', self.receipt('roles', first))
        second = self.phase.begin('voice_db')
        self.assertNotEqual(first, second)
        self.phase.complete('voice_db', self.receipt('voice_db', second))
        self.assertEqual(set(self.base.ledger.row['cohort_members']), {'roles','voice_db'})

    def test_lost_cas_and_stale_claim_refuse_replay(self):
        original = self.base.ledger.write
        def lost(revision, row):
            original(revision, row)
            if 'cohort_members' in row: raise OSError('lost reply')
        with patch.object(self.base.ledger, 'write', side_effect=lost):
            with self.assertRaises(cohort.CohortPhaseError): self.phase.begin('voice_db')
        self.assertTrue(self.media.poisoned)
        with self.assertRaises(cohort.CohortPhaseError): self.phase.begin('roles')

    def test_pending_member_cannot_overlap(self):
        self.phase.begin('voice_db')
        with self.assertRaises(cohort.CohortPhaseError): self.phase.begin('roles')
        self.assertTrue(self.media.poisoned)

    def test_cohort_pending_refuses_media(self):
        self.phase.begin('voice_db')
        with self.assertRaises(media.RecoveryRequired): self.media.begin('file_db')
        self.assertTrue(self.media.poisoned)

    def test_media_pending_refuses_cohort(self):
        self.media.begin('file_db')
        with self.assertRaises(cohort.CohortPhaseError): self.phase.begin('voice_db')
        self.assertTrue(self.media.poisoned)

    def test_boolean_engine_restart_is_rejected(self):
        self.media.intent['cohort_binding']['sources']['roles']['engine']['restart_count'] = True
        with self.assertRaises(cohort.CohortPhaseError): cohort.CohortPhases(self.media)

    def test_terminal_ledger_drift_is_uncertain(self):
        nonce = self.phase.begin('voice_db')
        actual_guard = self.phase._guard
        calls = 0
        def guard():
            nonlocal calls
            actual_guard()
            calls += 1
            if calls == 2:
                row, rev = self.base.ledger.read()
                row['cohort_members']['voice_db']['receipt_sha256'] = '0'*64
                self.base.ledger.write(rev, row)
        with patch.object(self.phase, '_guard', side_effect=guard):
            with self.assertRaises(cohort.CohortPhaseError):
                self.phase.complete('voice_db', self.receipt('voice_db', nonce))
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')


if __name__ == '__main__': unittest.main()
