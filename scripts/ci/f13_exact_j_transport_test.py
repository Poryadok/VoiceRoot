"""Real fixture materialization regression; never a F13 guard/producer RED."""
import hashlib
from pathlib import Path
import tempfile
import unittest
import zipfile

import f13_exact_j_historical_red_test as harness


class HistoricalJTransport(unittest.TestCase):
    def test_actual_archive_restores_original_J_record_path(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            harness.materialize(root)
            captured = root / 'out/actual/observed/fixed3071-J-ledger.json'
            original = root / 'tmp/F13-author-547/fixed3071-J-ledger.json'
            self.assertTrue(original.is_file(), 'original literal J-ledger consumer path was not materialized')
            raw = original.read_bytes()
            self.assertEqual(len(raw), 109549632)
            self.assertEqual(hashlib.sha256(raw).hexdigest(), harness.LEDGER_SHA)
            self.assertEqual(raw, captured.read_bytes())

    def test_record_alias_rejects_wrong_declared_binding(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            for binding in (None, ('0' * 64, 109549632), (harness.LEDGER_SHA, 0)):
                with self.subTest(binding=binding):
                    logical = {} if binding is None else {harness.J_CAPTURE: binding}
                    with self.assertRaisesRegex(harness.SetupFailure, 'J_record_declared_binding'):
                        harness.materialize_J_record_alias(root, logical)
                    self.assertEqual(list(root.iterdir()), [])

    def test_record_alias_rejects_forged_captured_bytes(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            source = root / harness.J_CAPTURE.lstrip('/')
            source.parent.mkdir(parents=True)
            with zipfile.ZipFile(harness.FIXTURE / 'historical-inputs.zip') as archive:
                forged = bytearray(archive.read('objects/' + harness.LEDGER_SHA))
            self.assertEqual(len(forged), harness.J_RECORD_BYTES)
            forged[-1] ^= 1  # Same-length actual record mutation must hit SHA, not size.
            self.assertNotEqual(hashlib.sha256(forged).hexdigest(), harness.LEDGER_SHA)
            source.write_bytes(forged)
            with self.assertRaisesRegex(harness.SetupFailure, 'J_record_raw_binding'):
                harness.materialize_J_record_alias(root, {harness.J_CAPTURE: (harness.LEDGER_SHA, 109549632)})
            self.assertEqual(source.read_bytes(), forged)
            self.assertFalse((root / harness.J_ORIGINAL_RECORD.lstrip('/')).exists())

    def test_record_alias_never_overwrites_existing_target(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            source, target = root / harness.J_CAPTURE.lstrip('/'), root / harness.J_ORIGINAL_RECORD.lstrip('/')
            source.parent.mkdir(parents=True)
            source.write_bytes(b'rejection-only source control')
            target.parent.mkdir(parents=True)
            target.write_bytes(b'prior target must stay byte-exact')
            with self.assertRaisesRegex(harness.SetupFailure, 'J_record_no_overwrite'):
                harness.materialize_J_record_alias(root, {harness.J_CAPTURE: (harness.LEDGER_SHA, 109549632)})
            self.assertEqual(target.read_bytes(), b'prior target must stay byte-exact')


if __name__ == '__main__':
    unittest.main()
