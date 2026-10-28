"""Fault tests for ciphertext upload intent, without GitHub or DEV writes."""
import copy
import hashlib
import json
from pathlib import Path
import sys
import types
import unittest
from unittest import mock

import media_backup_phase as backup_phase
import media_backup_phase_test as fixture
import media_backup_upload_phase as upload
import media_operation_journal as journal
import story_media_backup as backup
import story_media_backup_custody as custody


class UploadTest(unittest.TestCase):
    def setUp(self):
        self.base = fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.base.setUp()
        self.addCleanup(self.base.doCleanups)
        self.intent = copy.deepcopy(self.base.intent)
        ctx = self.base.context
        self.intent['backup_binding'] = {'operation_id': ctx['operation_id'],
            'snapshot_id': self.base.capture['snapshot_id'], 'source_sha': ctx['commit'],
            **{k: ctx[k] for k in ('project', 'resource', 'actor', 'claim', 'window', 'generation', 'revision')},
            'namespace_uid': 'ns', **{k: self.base.capture[k] for k in
                ('pod_uid', 'pvc_uid', 'pv_uid', 'container_id', 'restart_count')},
            'restore_inputs': {'catalog_sha256': '1' * 64, 'roles_catalog_sha256': '2' * 64,
                'migration_catalog_sha256': '3' * 64, 'migrate_binary_sha256': '4' * 64,
                'postgres_image': 'example/postgres@sha256:' + '5' * 64,
                'migrate_image': 'example/migrate@sha256:' + '6' * 64, 'postgres_major': 16}}
        self.intent['upload_workflow_sha'] = self.base.context['commit']
        self.journal = journal.Journal(Path(self.base.temp.name) / 'upload-journal', self.intent)
        self.addCleanup(self.journal.close)
        self.phases = backup_phase.BackupPhases(self.journal, self.base.fence,
                fixture.Client(self.base.lock), self.base.lock, 'ns')
        self.root = Path(self.base.temp.name)
        self.bundle = self.root / self.base.context['operation_id'] / 'backup' / 'media-backup.tar'
        self.stub = types.SimpleNamespace(verify_bundle=lambda path, key, binding: {'bytes': 1234, 'sha256': 'e' * 64})
        self.patches = [mock.patch.object(upload, 'ROOT', self.root),
                        mock.patch.dict(sys.modules, {'story_media_backup_custody': self.stub})]
        for patch in self.patches:
            patch.start()
            self.addCleanup(patch.stop)

    def complete_all(self):
        for member in sorted(backup_phase.MEMBERS):
            nonce = self.phases.begin(member)
            receipt = self.base.capture | {'operation_id': self.base.context['operation_id'],
                'member': member, 'exit': 0, 'scope': 'dump-terminal-only', 'attempt_nonce': nonce}
            self.phases.complete(member, receipt)

    def final_binding(self):
        digests = {member: hashlib.sha256(json.dumps(self.journal.state['steps']['dump-' + member]['receipt'],
                   sort_keys=True, separators=(',', ':')).encode()).hexdigest() for member in backup_phase.MEMBERS}
        return self.intent['backup_binding'] | {'dump_receipts': digests}

    def test_requires_complete_dump_set_and_one_shot_reservation(self):
        with self.assertRaises(backup_phase.RecoveryRequired):
            upload.prepare(self.phases, b'x' * 32, self.bundle)
        self.complete_all()
        planned = upload.prepare(self.phases, b'x' * 32, self.bundle)
        self.assertEqual(planned['status'], 'dispatch-pending')
        self.assertEqual(self.base.ledger.row['backup_upload'], planned)
        with self.assertRaises(backup_phase.RecoveryRequired):
            upload.prepare(self.phases, b'x' * 32, self.bundle)

    def test_wrong_path_or_tampered_member_never_reserves(self):
        self.complete_all()
        with self.assertRaises(backup_phase.RecoveryRequired):
            upload.prepare(self.phases, b'x' * 32, self.root / 'foreign')
        self.base.ledger.row['backup_members']['file_db']['receipt_sha256'] = '0' * 64
        with self.assertRaises(backup_phase.RecoveryRequired):
            upload.prepare(self.phases, b'x' * 32, self.bundle)
        self.assertNotIn('backup_upload', self.base.ledger.row)

    def test_changed_workflow_never_reserves(self):
        changed = copy.deepcopy(self.intent)
        changed['upload_workflow_sha'] = 'f' * 40
        self.journal = journal.Journal(Path(self.base.temp.name) / 'changed-journal', changed)
        self.addCleanup(self.journal.close)
        self.phases = backup_phase.BackupPhases(self.journal, self.base.fence,
                fixture.Client(self.base.lock), self.base.lock, 'ns')
        self.complete_all()
        self.phases._guard()
        with self.assertRaises(backup_phase.RecoveryRequired):
            upload.prepare(self.phases, b'x' * 32, self.bundle)
        self.assertNotIn('backup_upload', self.base.ledger.row)

    def test_lost_ledger_reply_marks_uncertain_and_refuses_retry(self):
        self.complete_all()
        original = self.base.ledger.write
        def lost_after_commit(revision, row):
            original(revision, row)
            if 'backup_upload' in row:
                raise OSError('lost reply')
        with mock.patch.object(self.base.ledger, 'write', side_effect=lost_after_commit):
            with self.assertRaises(backup_phase.RecoveryRequired):
                upload.prepare(self.phases, b'x' * 32, self.bundle)
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')
        with self.assertRaises(backup_phase.RecoveryRequired):
            upload.prepare(self.phases, b'x' * 32, self.bundle)

    def test_real_encrypted_bundle_verification_and_tamper_rejection(self):
        self.complete_all()
        self.bundle.parent.mkdir(parents=True, mode=0o700)
        key = bytes(range(32))
        final_binding = self.final_binding()
        manifest = backup.export(self.bundle.parent, key, final_binding,
                                 lambda member, out: out.write(('synthetic-' + member).encode()))
        built = custody.build_bundle(self.bundle.parent, key, final_binding, manifest)
        with mock.patch.dict(sys.modules, {'story_media_backup_custody': custody}):
            planned = upload.prepare(self.phases, key, built)
        self.assertEqual(planned['bundle_sha256'], custody.verify_bundle(built, key, final_binding)['sha256'])

    def test_real_bundle_tamper_is_not_reserved(self):
        self.complete_all()
        self.bundle.parent.mkdir(parents=True, mode=0o700)
        key = bytes(range(32))
        final_binding = self.final_binding()
        manifest = backup.export(self.bundle.parent, key, final_binding,
                                 lambda member, out: out.write(('synthetic-' + member).encode()))
        custody.build_bundle(self.bundle.parent, key, final_binding, manifest)
        with self.bundle.open('r+b') as stream:
            stream.seek(0)
            stream.write(b'X')
        with mock.patch.dict(sys.modules, {'story_media_backup_custody': custody}):
            with self.assertRaises(backup_phase.RecoveryRequired):
                upload.prepare(self.phases, key, self.bundle)
        self.assertNotIn('backup_upload', self.base.ledger.row)

    def test_authenticated_bundle_with_wrong_dump_receipt_is_refused(self):
        self.complete_all()
        self.bundle.parent.mkdir(parents=True, mode=0o700)
        key = bytes(range(32))
        wrong = self.final_binding()
        wrong['dump_receipts']['file_db'] = '0' * 64
        manifest = backup.export(self.bundle.parent, key, wrong,
                                 lambda member, out: out.write(('synthetic-' + member).encode()))
        custody.build_bundle(self.bundle.parent, key, wrong, manifest)
        with mock.patch.dict(sys.modules, {'story_media_backup_custody': custody}):
            with self.assertRaises(backup_phase.RecoveryRequired):
                upload.prepare(self.phases, key, self.bundle)
        self.assertNotIn('backup_upload', self.base.ledger.row)


if __name__ == '__main__': unittest.main()
