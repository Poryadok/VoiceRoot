"""Exact two V4 failures may upgrade code without replay or journal edits."""
import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import prebuild_v5_disposition as disposition
from controller import Blocked


class DispositionTests(unittest.TestCase):
    def fixture(self, root, nonce):
        installed = root / 'installed'
        for folder in ('journal', 'responses', 'sources'):
            (installed / folder).mkdir(parents=True, exist_ok=True)
        (installed / 'sources' / nonce[:12]).mkdir()
        request = {'action': 'prepare', 'nonce': nonce, 'run_id': disposition.CASES[nonce],
                   'source_sha': disposition.SOURCE, 'mode': 'images-only', 'changed_services': ['story']}
        row = {'schema': 'voice-nats-bridge-request-v1', 'request': request,
               'request_sha256': disposition.prebuild_disposition.digest(request), 'phase': 'STARTED',
               'prepare_stage': {'name': 'nats-preflight', 'status': 'STARTED'}, 'prepare_error': 'guard_rejected'}
        path = installed / 'journal' / (nonce + '.json')
        path.write_text(json.dumps(row))
        (installed / 'responses' / path.name).write_text(json.dumps({'status': 'BLOCKED', 'error': 'exception_Blocked'}))
        marker = {'metadata': dict(disposition.prebuild_disposition.MARKER),
                  'data': {'phase': 'active', 'generation': 'r20260930a4',
                           'dataPVC': 'voice-nats-jsdata-d202610040049430b'}}
        return path, marker

    def patches(self):
        return (patch.object(disposition, 'directory', side_effect=lambda p: Path(p)),
                patch.object(disposition, 'private_json', side_effect=lambda p: json.loads(Path(p).read_bytes())),
                patch.object(disposition, 'public_read', side_effect=lambda p: Path(p).read_bytes()),
                patch.object(disposition, 'save', side_effect=lambda p, row: Path(p).write_text(json.dumps(row))))

    def invoke(self, root, path, marker, binding=disposition.V4_BINDING):
        from contextlib import ExitStack
        with ExitStack() as stack:
            for p in self.patches():stack.enter_context(p)
            return disposition.enroll(root, path, marker, binding)

    def test_both_exact_journals_preserved_and_idempotent(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for nonce in disposition.CASES:
                path, marker = self.fixture(root, nonce)
                before = path.read_bytes()
                receipt = self.invoke(root, path, marker)
                self.assertEqual(self.invoke(root, path, marker), receipt)
                self.assertEqual(path.read_bytes(), before)
                self.assertEqual(receipt['disposition'], 'FAILED_BEFORE_TRANSACTION_NO_REPLAY')

    def test_each_known_binding_and_absence_guard_rejects(self):
        for change in ('run', 'source', 'services', 'phase', 'stage', 'error', 'response',
                       'marker', 'binding', 'build', 'operation', 'unknown', 'receipt'):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as td:
                root = Path(td)
                nonce = next(iter(disposition.CASES))
                path, marker = self.fixture(root, nonce)
                row = json.loads(path.read_bytes())
                binding = disposition.V4_BINDING
                if change in ('run', 'source', 'services'):
                    key = {'run': 'run_id', 'source': 'source_sha', 'services': 'changed_services'}[change]
                    row['request'][key] = ['bot'] if change == 'services' else 'wrong'
                    row['request_sha256'] = disposition.prebuild_disposition.digest(row['request'])
                elif change == 'phase':row['phase'] = 'COMPLETE'
                elif change == 'stage':row['prepare_stage']['name'] = 'target-build'
                elif change == 'error':row['prepare_error'] = 'unexpected'
                elif change == 'response':(root / 'installed' / 'responses' / path.name).write_text('{}')
                elif change == 'marker':marker['metadata']['resourceVersion'] = 'changed'
                elif change == 'binding':binding = '0' * 64
                elif change == 'build':(root / 'installed' / 'sources' / (nonce[:12] + '-build')).mkdir()
                elif change == 'operation':(root / ('rollout-' + nonce[:12])).mkdir()
                elif change == 'unknown':path = path.with_name('a' * 64 + '.json')
                elif change == 'receipt':(root / 'installed' / ('prebuild-disposition-' + nonce[:12] + '.json')).write_text('{}')
                path.write_text(json.dumps(row))
                with self.assertRaises(Blocked):self.invoke(root, path, marker, binding)

    def test_missing_prior_v3_receipt_cannot_create_new_enrollment(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for folder in ('journal', 'responses', 'sources'):(root / 'installed' / folder).mkdir(parents=True)
            path = root / 'installed' / 'journal' / (disposition.prebuild_disposition.OPERATION + 'a' * 52 + '.json')
            with patch.object(disposition.prebuild_disposition, 'enroll') as old:
                with self.assertRaises(FileNotFoundError):self.invoke(root, path, {})
                old.assert_not_called()


if __name__ == '__main__':unittest.main()
