import unittest
from unittest.mock import patch
from datetime import datetime, timedelta, timezone
import io
import json
import sys
import zipfile
import media_backup_readback as readback
import media_backup_upload_phase as upload
import media_backup_upload_phase_test as fixture
import story_media_backup as backup
import story_media_backup_custody as custody


class WorkflowEvidenceTest(unittest.TestCase):
    def test_running_or_wrong_run_cannot_authorize_readback(self):
        expected = {'run_id': '123', 'source_sha': 'a'*40, 'challenge': 'b'*32,
                    'operation_id': 'media-c1-'+'c'*32}
        run = {'id': 123, 'head_sha': 'a'*40, 'head_branch': 'master', 'event': 'workflow_dispatch',
               'path': '.github/workflows/dev-media-backup-upload.yml', 'run_attempt': 1,
               'status': 'completed', 'conclusion': 'success', 'created_at': '2026-10-28T21:00:00Z'}
        artifact = {'total_count': 1, 'artifacts': [{'id': 456, 'name':
                    'voice-media-backup-' + expected['operation_id'] + '-' + expected['challenge'],
                    'created_at': '2026-10-28T21:03:00Z'}]}
        for field, value in [('status', 'in_progress'), ('conclusion', 'failure'), ('head_sha', 'd'*40),
                             ('run_attempt', 2), ('head_branch', 'feature')]:
            bad = dict(run, **{field: value})
            with patch.object(readback, 'get_json', side_effect=[bad, artifact]):
                with self.assertRaises(readback.ReadbackError):
                    readback.workflow_evidence('token', expected)
        with patch.object(readback, 'get_json', side_effect=[run, artifact]):
            self.assertEqual(readback.workflow_evidence('token', expected), (456, '2026-10-28T21:03:00Z'))


class ReadbackPhaseTest(unittest.TestCase):
    def setUp(self):
        self.base = fixture.UploadTest('test_requires_complete_dump_set_and_one_shot_reservation')
        self.base.setUp()
        self.addCleanup(self.base.doCleanups)
        self.base.complete_all()
        self.base.bundle.parent.parent.mkdir(mode=0o700)
        self.base.bundle.parent.mkdir(mode=0o700)
        binding = self.base.final_binding()
        manifest = backup.export(self.base.bundle.parent, b'x'*32, binding,
                                 lambda member, out: out.write(('synthetic-' + member).encode()))
        custody.build_bundle(self.base.bundle.parent, b'x'*32, binding, manifest)
        with patch.dict(sys.modules, {'story_media_backup_custody': custody}):
            planned = upload.prepare(self.base.phases, b'x'*32, self.base.bundle)
        row, rev = self.base.base.ledger.read()
        row['backup_upload'] = dict(planned, status='upload-running', run_id='123', run_attempt=1)
        self.base.base.ledger.write(rev, row)
        self.destination = self.base.root / planned['operation_id'] / ('readback-123-' + planned['challenge'])

    def test_success_records_terminal_readback(self):
        def extract(token, binding, artifact, bundle, key, expected, destination):
            return ({'schema': 'voice-media-custody-v1', 'destination': 'github:Poryadok/VoiceRoot',
                     'artifact_id': artifact, **{k: v for k, v in binding.items() if k != 'created_at'}}, {'manifest': 1})
        with patch.object(readback, 'workflow_evidence', return_value=(456, '2026-10-28T21:00:00Z')), \
             patch.object(readback.custody, 'readback_extract', side_effect=extract), \
             patch.object(readback.backup, 'verify'):
            receipt, _ = readback.run(self.base.phases, 'token', b'x'*32, self.base.bundle, self.destination, lambda: None)
        self.assertEqual(receipt['artifact_id'], 456)
        self.assertEqual(self.base.base.ledger.row['backup_readback']['status'], 'done')
        with self.assertRaises(readback.ReadbackError):
            readback.run(self.base.phases, 'token', b'x'*32, self.base.bundle, self.destination, lambda: None)

    def test_readback_failure_retains_uncertain_fence(self):
        with patch.object(readback, 'workflow_evidence', return_value=(456, '2026-10-28T21:00:00Z')), \
             patch.object(readback.custody, 'readback_extract', side_effect=OSError('private')):
            with self.assertRaises(readback.ReadbackError):
                readback.run(self.base.phases, 'token', b'x'*32, self.base.bundle, self.destination, lambda: None)
        self.assertEqual(self.base.base.ledger.row['status'], 'uncertain')
        self.assertTrue(self.base.phases.poisoned)

    def test_wrong_receipt_after_extract_is_uncertain(self):
        with patch.object(readback, 'workflow_evidence', return_value=(456, '2026-10-28T21:00:00Z')), \
             patch.object(readback.custody, 'readback_extract', return_value=({'artifact_id': 999}, {})):
            with self.assertRaises(readback.ReadbackError):
                readback.run(self.base.phases, 'token', b'x'*32, self.base.bundle, self.destination, lambda: None)
        self.assertEqual(self.base.base.ledger.row['status'], 'uncertain')
        self.assertNotEqual(self.base.base.ledger.row['backup_readback']['status'], 'done')

    def test_changed_reservation_is_rejected_before_directory_or_extract(self):
        calls = 0
        def drain():
            nonlocal calls
            calls += 1
            if calls == 3:
                row, rev = self.base.base.ledger.read()
                row['backup_readback']['artifact_id'] = 999
                self.base.base.ledger.write(rev, row)
        with patch.object(readback, 'workflow_evidence', return_value=(456, '2026-10-28T21:00:00Z')), \
             patch.object(readback.custody, 'readback_extract') as extract:
            with self.assertRaises(readback.ReadbackError):
                readback.run(self.base.phases, 'token', b'x'*32, self.base.bundle, self.destination, drain)
        extract.assert_not_called()
        self.assertFalse(self.destination.exists())
        self.assertEqual(self.base.base.ledger.row['status'], 'uncertain')

    def test_real_full_readback_and_decrypt(self):
        planned = self.base.base.ledger.row['backup_upload']
        now = datetime.now(timezone.utc)
        created = (now - timedelta(seconds=1)).isoformat()
        raw = self.base.bundle.read_bytes()
        archive = io.BytesIO()
        with zipfile.ZipFile(archive, 'w', compression=zipfile.ZIP_STORED) as stream:
            stream.writestr('media-backup.tar', raw)
        zipped = archive.getvalue()
        meta = {'id': 456, 'url': custody.API+'456', 'expired': False,
                'name': 'voice-media-backup-' + planned['operation_id'] + '-' + planned['challenge'],
                'created_at': now.isoformat(), 'expires_at': (now + timedelta(days=1)).isoformat(),
                'workflow_run': {'id': 123, 'head_sha': planned['source_sha']}, 'size_in_bytes': len(zipped)}
        def download(url, headers, target, limit, deadline, redirects=False):
            data = zipped if redirects else json.dumps(meta).encode()
            target.write(data)
            return len(data)
        with patch.object(readback, 'workflow_evidence', return_value=(456, created)), \
             patch.object(custody.transport, '_download', side_effect=download):
            receipt, manifest = readback.run(self.base.phases, 'fixture-token', b'x'*32,
                                               self.base.bundle, self.destination, lambda: None)
        self.assertEqual(receipt['artifact_id'], 456)
        for member in readback.MEMBERS:
            output = io.BytesIO()
            backup.decrypt(self.destination, b'x'*32, self.base.final_binding(), manifest, member, output)
            self.assertEqual(output.getvalue(), ('synthetic-' + member).encode())


if __name__ == '__main__': unittest.main()
