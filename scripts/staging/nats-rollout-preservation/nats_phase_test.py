"""Source tests for the fixed non-root Kubernetes archive helper."""
import contextlib
import hashlib
import importlib.machinery
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import types
import unittest
from unittest.mock import patch

import archive_stream
import native_store


HELPER_PATH = Path(__file__).with_name('nats-phase')
loader = importlib.machinery.SourceFileLoader('nats_phase', str(HELPER_PATH))
spec = importlib.util.spec_from_loader('nats_phase', loader)
phase = importlib.util.module_from_spec(spec)
loader.exec_module(phase)

OPERATION = 'a' * 12
NONCE = 'b' * 32
POD_UID = '66666666-6666-4666-8666-666666666666'


class Output:
    def __init__(self): self.buffer = io.BytesIO()


class NatsPhaseTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name); self.root.chmod(0o700)
        self.source = self.root / 'data'; self.source.mkdir(mode=0o700)
        (self.source / 'jetstream').mkdir(mode=0o700)
        (self.source / 'jetstream' / 'meta.inf').write_bytes(b'cold-native-data')
        self.output = self.root / 'out'; self.output.mkdir(mode=0o700)
        self.args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID)
        self.patches = [patch.object(phase, 'DATA_ROOT', self.source),
            patch.object(phase, 'OUT_ROOT', self.output)]
        for item in self.patches: item.start()

    def tearDown(self):
        for item in reversed(self.patches): item.stop()
        self.tmp.cleanup()

    def run_helper(self, function, args=None):
        output = Output()
        with patch.object(phase.sys, 'stdout', output):
            function(args or self.args)
        return output.buffer.getvalue()

    def test_create_once_then_existing_readback_is_independent_of_changed_source(self):
        first = self.run_helper(phase.create_and_stream)
        archive = self.output / 'archive.tar'
        original = archive.read_bytes()
        self.assertTrue((self.output / 'archive.manifest.json').is_file())
        self.assertTrue(native_store.verify_archive(archive,
            json.loads((self.output / 'archive.manifest.json').read_bytes())['manifest']))
        (self.source / 'jetstream' / 'meta.inf').write_bytes(b'source-changed-after-cut')
        second = self.run_helper(phase.stream_existing)
        self.assertEqual(first, second)
        self.assertEqual(archive.read_bytes(), original)
        with self.assertRaisesRegex(ValueError, 'archive_already_started'):
            self.run_helper(phase.create_and_stream)

    def test_existing_readback_rejects_wrong_nonce_without_rearchiving(self):
        self.run_helper(phase.create_and_stream)
        wrong = types.SimpleNamespace(operation=OPERATION, nonce='c' * 32, pod_uid=POD_UID)
        with self.assertRaisesRegex(ValueError, 'archive-readback-binding'):
            phase.stream_existing(wrong)
        self.assertEqual((self.source / 'jetstream' / 'meta.inf').read_bytes(), b'cold-native-data')

    def test_existing_readback_rejects_replaced_pod_uid_without_rearchiving(self):
        self.run_helper(phase.create_and_stream)
        archive = self.output / 'archive.tar'
        original = archive.read_bytes()
        replacement = types.SimpleNamespace(operation=OPERATION, nonce=NONCE,
            pod_uid='77777777-7777-4777-8777-777777777777')
        with patch.object(phase.native_store, 'archive_closed_store',
                side_effect=AssertionError('replacement Pod must never recreate the archive')):
            with self.assertRaisesRegex(ValueError, 'archive-readback-binding'):
                phase.stream_existing(replacement)
        self.assertEqual(archive.read_bytes(), original)

    def test_helper_output_consumes_bounded_archive_and_manifest_frames(self):
        raw = self.run_helper(phase.create_and_stream)
        artifacts = self.root / 'private'; artifacts.mkdir(mode=0o700)
        target = artifacts / 'cold.tar'
        events = []
        result = archive_stream.consume((raw[i:i+4096] for i in range(0, len(raw), 4096)), target,
            operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            persist_intent=events.append, persist_result=events.append,
            verify_archive=native_store.verify_archive)
        self.assertEqual(result['archive_sha256'], hashlib.sha256(target.read_bytes()).hexdigest())
        self.assertEqual(events[-1]['state'], 'ARCHIVE_RESULT_READBACK')


if __name__ == '__main__': unittest.main()
