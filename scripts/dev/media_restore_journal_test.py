import unittest
import copy
from unittest.mock import patch
import media_backup_upload_phase as upload
import media_backup_upload_phase_test as fixture
import media_restore_journal as restore


class RestoreJournalTest(unittest.TestCase):
    def setUp(self):
        self.base = fixture.UploadTest('test_requires_complete_dump_set_and_one_shot_reservation')
        self.base.setUp()
        self.addCleanup(self.base.doCleanups)
        self.base.complete_all()
        planned = upload.prepare(self.base.phases, b'x'*32, self.base.bundle)
        row, rev = self.base.base.ledger.read()
        row['backup_upload'] = dict(planned, status='upload-running', run_id='123', run_attempt=1)
        receipt = {'schema':'voice-media-custody-v1', 'destination':'github:Poryadok/VoiceRoot',
                   'artifact_id':12, 'run_id':123, 'head_sha':planned['source_sha'],
                   **{k: planned[k] for k in ('operation_id','snapshot_id','challenge','bundle_bytes','bundle_sha256')}}
        artifact_binding = {k:v for k,v in receipt.items() if k not in ('schema','destination','artifact_id')}
        artifact_binding['created_at'] = '2026-10-28T21:00:00Z'
        self.readback_intent = {'status':'pending','artifact_id':12,'binding':artifact_binding}
        row['backup_readback'] = {'status': 'done', 'intent': self.readback_intent, 'receipt': receipt}
        self.base.base.ledger.write(rev, row)
        self.base.journal.state['steps']['backup-readback'] = row['backup_readback'].copy()
        self.base.journal.save()
        self.restore = restore.RestoreJournal(self.base.phases)
        self.name = 'voice-media-restore-' + 'a'*32
        self.labels = {'voice.test.owner': self.name}

    def test_owned_create_and_removal_are_durably_fenced(self):
        nonce = self.restore.begin('postgres', self.name, self.labels)
        self.restore.created('postgres', nonce, 'b'*64, self.labels)
        self.restore.begin_remove('postgres', 'b'*64)
        self.restore.removed('postgres', 'b'*64, True)
        self.assertEqual(self.base.base.ledger.row['restore_resources']['postgres']['status'], 'removed')
        with self.assertRaises(restore.RestoreJournalError):
            self.restore.begin('postgres', self.name, self.labels)

    def test_lost_ledger_reply_is_uncertain_and_forbids_replay(self):
        original = self.base.base.ledger.write
        def lost(rev, row):
            original(rev, row)
            if 'restore_resources' in row: raise OSError('lost')
        with patch.object(self.base.base.ledger, 'write', side_effect=lost):
            with self.assertRaises(restore.RestoreJournalError):
                self.restore.begin('postgres', self.name, self.labels)
        self.assertTrue(self.base.phases.poisoned)
        with self.assertRaises(restore.RestoreJournalError):
            self.restore.begin('postgres', self.name, self.labels)

    def test_wrong_receipt_cannot_start_resource(self):
        for field, value in [('artifact_id', True), ('artifact_id', 13), ('run_id', 124), ('head_sha', 'c'*40),
                             ('operation_id', 'different'), ('snapshot_id', 'different'),
                             ('challenge', 'different'), ('bundle_sha256', '0'*64), ('bundle_bytes', True)]:
            with self.subTest(field=field):
                row, rev = self.base.base.ledger.read()
                receipt = dict(row['backup_readback']['receipt'], **{field: value})
                changed = {'status':'done', 'intent':self.readback_intent, 'receipt':receipt}
                row['backup_readback'] = changed
                self.base.base.ledger.write(rev, row)
                self.base.journal.state['steps']['backup-readback'] = changed
                self.base.journal.save()
                with self.assertRaises(restore.RestoreJournalError):
                    self.restore.begin('postgres', self.name, self.labels)
                self.assertNotIn('restore_resources', self.base.base.ledger.row)
                planned = self.base.journal.state['steps']['backup-upload']
                receipt.update(schema='voice-media-custody-v1', destination='github:Poryadok/VoiceRoot',
                               artifact_id=12, run_id=123, head_sha=planned['source_sha'])
                receipt.update({k:planned[k] for k in ('operation_id','snapshot_id','challenge','bundle_bytes','bundle_sha256')})
                row, rev = self.base.base.ledger.read()
                row['backup_readback'] = {'status':'done','intent':self.readback_intent,'receipt':receipt}
                self.base.base.ledger.write(rev, row)
                self.base.journal.state['steps']['backup-readback'] = copy.deepcopy(row['backup_readback'])
                self.base.journal.save()

    def test_bad_intent_types_or_timestamp_rejected(self):
        for field, value in [('artifact_id', True), ('run_id', True), ('bundle_bytes', True), ('created_at', 'invalid')]:
            with self.subTest(field=field):
                row, rev = self.base.base.ledger.read()
                changed = copy.deepcopy(row['backup_readback'])
                if field == 'artifact_id': changed['intent'][field] = value
                else: changed['intent']['binding'][field] = value
                row['backup_readback'] = changed
                self.base.base.ledger.write(rev, row)
                self.base.journal.state['steps']['backup-readback'] = copy.deepcopy(changed)
                self.base.journal.save()
                with self.assertRaises(restore.RestoreJournalError):
                    self.restore.begin('postgres', self.name, self.labels)
                row, rev = self.base.base.ledger.read()
                row['backup_readback']['intent'] = copy.deepcopy(self.readback_intent)
                self.base.base.ledger.write(rev, row)
                self.base.journal.state['steps']['backup-readback'] = copy.deepcopy(row['backup_readback'])
                self.base.journal.save()


if __name__ == '__main__': unittest.main()
