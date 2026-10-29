import unittest
import copy
from pathlib import Path
from unittest.mock import patch
import cohort_backup_phases_test as fixture
import cohort_upload_phase as upload


class UploadPhaseTest(unittest.TestCase):
    def setUp(self):
        self.base = fixture.CohortPhasesTest('test_exact_member_receipt_and_no_replay')
        self.base.setUp(); self.addCleanup(self.base.doCleanups)
        for member in sorted(self.base.phase.members):
            nonce = self.base.phase.begin(member)
            self.base.phase.complete(member, self.base.receipt(member, nonce))
        self.path = Path('/fixed')/self.base.binding['operation_id']/'cohort-backup'/'cohort-backup.tar'

    def install_marker(self):
        digests = {m:v['receipt_sha256'] for m,v in self.base.base.ledger.row['cohort_members'].items()}
        marker = {'status':'done','operation_id':self.base.binding['operation_id'],
            'snapshot_id':self.base.binding['snapshot_id'],'invocation_nonce':self.base.media.invocation,
            'bundle_bytes':123,'bundle_sha256':'d'*64,'manifest_sha256':'e'*64,'receipt_sha256':digests}
        self.base.base.ledger.row['cohort_bundle'] = copy.deepcopy(marker)
        self.base.journal.state['steps']['cohort-bundle'] = {'status':'done','marker':marker}
        self.base.journal.save()

    def test_reservation_is_one_shot_and_receipt_bound(self):
        self.install_marker()
        with patch.object(upload, 'ROOT', Path('/fixed')), \
             patch.object(upload.custody, 'verify_bundle', return_value={'bytes':123,'sha256':'d'*64}):
            planned = upload.prepare(self.base.phase, b'x'*32, self.path)
            self.assertEqual(planned['status'], 'dispatch-pending')
            with self.assertRaises(upload.CohortUploadError):
                upload.prepare(self.base.phase, b'x'*32, self.path)
        self.assertEqual(self.base.base.ledger.row['cohort_upload'], planned)

    def test_independent_bundle_without_completed_marker_is_refused(self):
        with patch.object(upload, 'ROOT', Path('/fixed')), \
             patch.object(upload.custody, 'verify_bundle', return_value={'bytes':123,'sha256':'d'*64}):
            with self.assertRaises(upload.CohortUploadError):
                upload.prepare(self.base.phase, b'x'*32, self.path)
        self.assertNotIn('cohort_upload', self.base.base.ledger.row)

    def test_member_drift_after_reservation_is_uncertain(self):
        self.install_marker()
        original = self.base.phase._guard
        calls = 0
        def guard():
            nonlocal calls
            original(); calls += 1
            if calls == 3:
                row, rev = self.base.base.ledger.read()
                row['cohort_members']['voice_db']['receipt_sha256'] = '0'*64
                self.base.base.ledger.write(rev, row)
        with patch.object(upload, 'ROOT', Path('/fixed')), \
             patch.object(upload.custody, 'verify_bundle', return_value={'bytes':123,'sha256':'d'*64}), \
             patch.object(self.base.phase, '_guard', side_effect=guard):
            with self.assertRaises(upload.CohortUploadError):
                upload.prepare(self.base.phase, b'x'*32, self.path)
        self.assertEqual(self.base.base.ledger.row['status'], 'uncertain')

    def test_tampered_member_refuses_upload(self):
        self.base.base.ledger.row['cohort_members']['voice_db']['receipt_sha256'] = '0'*64
        with patch.object(upload, 'ROOT', Path('/fixed')), patch.object(upload.custody, 'verify_bundle') as verify:
            with self.assertRaises(upload.CohortUploadError):
                upload.prepare(self.base.phase, b'x'*32, self.path)
        verify.assert_not_called()
        self.assertNotIn('cohort_upload', self.base.base.ledger.row)


if __name__ == '__main__': unittest.main()
