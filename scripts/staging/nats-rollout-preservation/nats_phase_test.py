"""Source tests for the fixed non-root Kubernetes archive helper."""
import contextlib
import base64
import hashlib
import importlib.machinery
import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
import types
import unittest
from unittest.mock import Mock, patch

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
        self.verification = self.root / 'verify'; self.verification.mkdir(mode=0o700)
        self.inputs = self.root / 'inputs'; self.inputs.mkdir(mode=0o700)
        self.args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID)
        self.patches = [patch.object(phase, 'DATA_ROOT', self.source),
            patch.object(phase, 'OUT_ROOT', self.output),
            patch.object(phase, 'VERIFY_ROOT', self.verification),
            patch.object(phase, 'INPUT_ROOT', self.inputs),
            patch.dict(phase.os.environ, {'POD_UID': POD_UID})]
        for item in self.patches: item.start()

    def tearDown(self):
        for item in reversed(self.patches): item.stop()
        self.tmp.cleanup()

    def run_helper(self, function, args=None):
        output = Output()
        with patch.object(phase.sys, 'stdout', output), \
             patch.object(phase, '_require_archive_helper_identity'), \
             patch.object(phase, 'source_readable_by_archive_helper', return_value=True):
            function(args or self.args)
        return output.buffer.getvalue()

    def test_fixed_census_phase_runs_the_pinned_kernel_without_host_runtime(self):
        run = getattr(phase, 'run_verification', None)
        self.assertTrue(callable(run), 'fixed census phase is not implemented')
        receipt = {'schema': 'voice-nats-verification-v1', 'phase': 'census',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'result_sha256': 'd' * 64}
        with patch.object(phase, 'run_verification', return_value=receipt) as execute:
            result = phase.main(['--phase', 'census', '--operation', OPERATION,
                '--nonce', NONCE, '--pod-uid', POD_UID,
                '--source-pvc-uid', '11111111-1111-4111-8111-111111111111',
                '--verification-pvc-uid', '22222222-2222-4222-8222-222222222222'])
        execute.assert_called_once()
        self.assertEqual(result, receipt)

    def test_census_readback_returns_only_matching_durable_receipt(self):
        state = self.verification / '.voice-nats-verification'
        state.mkdir(mode=0o700)
        marker = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': '11111111-1111-4111-8111-111111111111',
            'verification_pvc_uid': '22222222-2222-4222-8222-222222222222',
            'state': 'STARTED'}
        census = {'schema': phase.CENSUS_SCHEMA, 'account': 'A' + 'A' * 55,
            'streams': [], 'consumers': []}
        census_bytes = json.dumps(census, sort_keys=True, separators=(',', ':'),
            allow_nan=False).encode()
        receipt = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': marker['source_pvc_uid'],
            'verification_pvc_uid': marker['verification_pvc_uid'],
            'archive_sha256': 'a' * 64,
            'census_sha256': hashlib.sha256(census_bytes).hexdigest(), 'status': 'COMPLETE'}
        (state / (OPERATION + '-' + NONCE + '.json')).write_text(json.dumps(marker))
        (state / (OPERATION + '-' + NONCE + '.receipt.json')).write_text(json.dumps(receipt))
        census_path = state / (OPERATION + '-' + NONCE + '.census.json')
        census_path.write_bytes(census_bytes)
        for path in state.iterdir():
            path.chmod(0o600)
        (self.source / 'jetstream').chmod(0o555)
        (self.source / 'jetstream' / 'meta.inf').chmod(0o444)
        self.source.chmod(0o555)
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=marker['source_pvc_uid'], verification_pvc_uid=marker['verification_pvc_uid'])
        with patch.object(phase, '_require_verification_identity'):
            self.assertEqual(phase.read_verification_receipt('census', args),
                {'receipt': receipt, 'census': census})

    def test_census_readback_rejects_digest_drift_without_replaying(self):
        state = self.verification / '.voice-nats-verification'
        state.mkdir(mode=0o700)
        source_uid = '11111111-1111-4111-8111-111111111111'
        verification_uid = '22222222-2222-4222-8222-222222222222'
        marker = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': source_uid, 'verification_pvc_uid': verification_uid,
            'state': 'STARTED'}
        census = {'schema': phase.CENSUS_SCHEMA, 'account': 'A' + 'A' * 55,
            'streams': [], 'consumers': []}
        census_bytes = json.dumps(census, sort_keys=True, separators=(',', ':'),
            allow_nan=False).encode()
        receipt = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': source_uid, 'verification_pvc_uid': verification_uid,
            'archive_sha256': 'a' * 64, 'census_sha256': 'b' * 64, 'status': 'COMPLETE'}
        (state / (OPERATION + '-' + NONCE + '.json')).write_text(json.dumps(marker))
        (state / (OPERATION + '-' + NONCE + '.receipt.json')).write_text(json.dumps(receipt))
        (state / (OPERATION + '-' + NONCE + '.census.json')).write_bytes(census_bytes)
        for path in state.iterdir(): path.chmod(0o600)
        (self.source / 'jetstream').chmod(0o555)
        (self.source / 'jetstream' / 'meta.inf').chmod(0o444)
        self.source.chmod(0o555)
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=source_uid, verification_pvc_uid=verification_uid)
        with patch.object(phase, '_require_verification_identity'), \
             patch.object(phase, 'run_verification', side_effect=AssertionError('must not replay')):
            with self.assertRaisesRegex(ValueError, 'verification_receipt_binding'):
                phase.read_verification_receipt('census', args)

    def test_census_readback_uses_bounded_ordered_frames_and_carries_exact_receipt(self):
        census = {'schema': phase.CENSUS_SCHEMA, 'account': 'A' + 'A' * 55,
            'streams': [], 'consumers': [], 'padding': 'x' * (phase.READBACK_CHUNK + 17)}
        raw = json.dumps(census, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()
        receipt = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': '11111111-1111-4111-8111-111111111111',
            'verification_pvc_uid': '22222222-2222-4222-8222-222222222222',
            'archive_sha256': 'a' * 64, 'census_sha256': hashlib.sha256(raw).hexdigest(),
            'status': 'COMPLETE'}
        output = Output()
        with patch.object(phase.sys, 'stdout', output):
            phase.stream_census_readback({'receipt': receipt, 'census': census})
        lines = output.buffer.getvalue().splitlines()
        rows = [json.loads(line) for line in lines]
        self.assertEqual(len(rows), 3)
        for line in lines:
            self.assertLessEqual(len(line), phase.MAX_READBACK_LINE - 1)
        first, second, complete = rows
        self.assertEqual([first['sequence'], second['sequence']], [0, 1])
        self.assertEqual(first['kind'], 'data')
        self.assertEqual(second['kind'], 'data')
        self.assertEqual(complete['kind'], 'complete')
        chunks = [base64.b64decode(row['data'], validate=True) for row in rows[:2]]
        self.assertEqual(b''.join(chunks), raw)
        self.assertEqual(complete['frames'], 2)
        self.assertEqual(complete['byte_count'], len(raw))
        self.assertEqual(complete['census_sha256'], hashlib.sha256(raw).hexdigest())
        self.assertEqual(complete['receipt'], receipt)

    def test_census_readback_rejects_frame_overflow_without_output(self):
        census = {'schema': phase.CENSUS_SCHEMA, 'account': 'A' + 'A' * 55,
            'streams': [], 'consumers': []}
        raw = json.dumps(census, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()
        receipt = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': '11111111-1111-4111-8111-111111111111',
            'verification_pvc_uid': '22222222-2222-4222-8222-222222222222',
            'archive_sha256': 'a' * 64,
            'census_sha256': hashlib.sha256(raw).hexdigest(), 'status': 'COMPLETE'}
        output = Output()
        with patch.object(phase.sys, 'stdout', output), \
             patch.object(phase, 'READBACK_CHUNK', 1), \
             patch.object(phase, 'MAX_READBACK_LINE', 100):
            with self.assertRaisesRegex(ValueError, 'readback_frame_size'):
                phase.stream_census_readback({'receipt': receipt, 'census': census})
        self.assertEqual(output.buffer.getvalue(), b'')

    def test_census_interruption_without_receipt_is_not_replayed(self):
        state = self.verification / '.voice-nats-verification'
        state.mkdir(mode=0o700)
        marker = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': '11111111-1111-4111-8111-111111111111',
            'verification_pvc_uid': '22222222-2222-4222-8222-222222222222',
            'state': 'STARTED'}
        marker_path = state / (OPERATION + '-' + NONCE + '.json')
        marker_path.write_text(json.dumps(marker)); marker_path.chmod(0o600)
        (self.source / 'jetstream').chmod(0o555)
        (self.source / 'jetstream' / 'meta.inf').chmod(0o444)
        self.source.chmod(0o555)
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=marker['source_pvc_uid'], verification_pvc_uid=marker['verification_pvc_uid'])
        with patch.object(phase, '_require_verification_identity'), \
             patch.object(phase, 'run_verification', side_effect=AssertionError('must not replay')):
            with self.assertRaisesRegex(ValueError, 'verification_receipt_incomplete'):
                phase.read_verification_receipt('census', args)

    def test_verification_refuses_missing_distinct_writable_copy_before_restore(self):
        verify = getattr(phase, 'validate_verification_mounts', None)
        self.assertTrue(callable(verify), 'verification PVC boundary is not implemented')
        with patch.object(phase.native_store, 'restore_closed_store',
                side_effect=AssertionError('restore must not run without a separate PVC')):
            with self.assertRaisesRegex(ValueError, 'verification_mounts'):
                verify(self.source, self.source, self.output, self.inputs,
                    source_pvc_uid='11111111-1111-4111-8111-111111111111',
                    verification_pvc_uid='11111111-1111-4111-8111-111111111111')

    def _install_verification_inputs(self):
        config = ('host: 127.0.0.1\nport: 4222\nhttp: 127.0.0.1:8222\n'
            'resolver: MEMORY\njetstream: { store_dir: /data, max_file_store: 64MB, max_memory_store: 16MB }\n')
        for name in phase.INPUT_FILES:
            path = self.inputs / name
            path.write_text(config if name == 'server.conf' else 'fixed-private-input\n')
            path.chmod(0o440)

    def test_verification_restores_to_distinct_copy_and_runs_fixed_process_chain(self):
        # A read-only source profile models the selected store's captured
        # owner/group/mode. These values must remain byte-for-byte unchanged.
        (self.source / 'jetstream').chmod(0o555)
        (self.source / 'jetstream' / 'meta.inf').chmod(0o444)
        self.source.chmod(0o555)
        before = {str(path.relative_to(self.source)): (path.lstat().st_uid,
            path.lstat().st_gid, path.lstat().st_mode, path.lstat().st_ino)
            for path in [self.source, *self.source.rglob('*')]}
        self._install_verification_inputs()
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid='11111111-1111-4111-8111-111111111111',
            verification_pvc_uid='22222222-2222-4222-8222-222222222222')
        process = Mock()
        process.poll.return_value = None
        process.wait.return_value = 0
        events = []
        def kernel_run(command, **kwargs):
            events.append(('kernel', command))
            (self.output / 'census.json').write_text(json.dumps({'schema': phase.CENSUS_SCHEMA,
                'account': 'A' + 'A' * 55, 'streams': [], 'consumers': []}))
            return types.SimpleNamespace(returncode=0)
        with patch.object(phase, '_require_verification_identity'), \
             patch.object(phase.subprocess, 'Popen', side_effect=lambda *a, **k: (events.append(('start', a[0])) or process)), \
             patch.object(phase, '_wait_nats', side_effect=lambda *_a, **_k: events.append(('healthy',))), \
             patch.object(phase.subprocess, 'run', side_effect=kernel_run), \
             patch.object(phase, 'KERNEL', Path('/voice-nats-runner/kernel')), \
             patch.object(phase, 'NATS_SERVER', Path('/usr/local/bin/nats-server')):
            receipt = phase.run_verification('census', args)
        self.assertEqual([event[0] for event in events], ['start', 'healthy', 'kernel'])
        process.terminate.assert_called_once_with()
        process.wait.assert_called_once_with(timeout=30)
        self.assertEqual(receipt['status'], 'COMPLETE')
        self.assertTrue((self.verification / 'store' / 'jetstream' / 'meta.inf').is_file())
        after = {str(path.relative_to(self.source)): (path.lstat().st_uid,
            path.lstat().st_gid, path.lstat().st_mode, path.lstat().st_ino)
            for path in [self.source, *self.source.rglob('*')]}
        self.assertEqual(after, before)
        self.assertEqual((self.verification / '.voice-nats-verification' /
            f'{OPERATION}-{NONCE}.receipt.json').is_file(), True)

    def test_verification_rejects_unreadable_by_runtime_source_profile_without_changing_it(self):
        paths = [self.source, self.source / 'jetstream', self.source / 'jetstream' / 'meta.inf']
        (self.source / 'jetstream' / 'meta.inf').chmod(0o000)
        before = [(p.lstat().st_uid, p.lstat().st_gid, p.lstat().st_mode) for p in paths]
        self.assertFalse(phase.source_readable_by_archive_helper(self.source))
        with self.assertRaisesRegex(ValueError, 'verification_mounts'):
            phase.validate_verification_mounts(self.source, self.verification,
                self.output, self.inputs,
                source_pvc_uid='11111111-1111-4111-8111-111111111111',
                verification_pvc_uid='22222222-2222-4222-8222-222222222222')
        after = [(p.lstat().st_uid, p.lstat().st_gid, p.lstat().st_mode) for p in paths]
        self.assertEqual(after, before)

    @unittest.skipUnless(os.geteuid() == 10000, 'run this ownership fixture as the canonical hub UID')
    def test_source_reader_rejects_group_readable_store_owned_by_hub_uid(self):
        paths = [self.source, self.source / 'jetstream', self.source / 'jetstream' / 'meta.inf']
        for path in paths:
            path.chmod(0o777)
        # The current NATS hub uses UID:GID 10000:10000. A readable group/other
        # mode is insufficient because the archive's O_NOATIME open needs owner.
        self.assertFalse(phase.source_readable_by_archive_helper(self.source, uid=65532, gid=10000))
        self.assertTrue(phase.source_readable_by_archive_helper(self.source, uid=10000, gid=10000))

    @unittest.skipUnless(os.geteuid() == 10000, 'run this metadata fixture as the canonical hub UID')
    def test_source_preflight_preserves_full_metadata_with_noatime_directory_walk(self):
        paths = [self.source, self.source / 'jetstream', self.source / 'jetstream' / 'meta.inf']
        self.source.chmod(0o555)
        paths[1].chmod(0o555)
        paths[2].chmod(0o444)
        def snapshot():
            return [(path.lstat().st_uid, path.lstat().st_gid, path.lstat().st_mode,
                path.lstat().st_nlink, path.lstat().st_size, path.lstat().st_atime_ns,
                path.lstat().st_mtime_ns, path.lstat().st_ctime_ns, path.lstat().st_ino)
                for path in paths]
        before = snapshot()
        self.assertTrue(phase.source_readable_by_archive_helper(self.source))
        self.assertEqual(snapshot(), before)

    @unittest.skipUnless(os.geteuid() == 10000, 'run this identity fixture as the canonical hub UID')
    def test_source_preflight_fails_closed_when_noatime_directory_open_is_denied(self):
        with patch.object(phase.os, 'open', side_effect=PermissionError('noatime denied')):
            self.assertFalse(phase.source_readable_by_archive_helper(self.source))

    def test_load_inputs_rejects_extra_candidate_file_before_writing_anything(self):
        bundle = {'schema': 'voice-nats-verification-inputs-v1', 'operation': OPERATION,
            'nonce': NONCE, 'pod_uid': POD_UID, 'files': {name: 'eA==' for name in phase.INPUT_FILES}}
        bundle['files']['candidate.py'] = 'cGFzcw=='
        with patch.object(phase, '_require_verification_identity'):
            with self.assertRaisesRegex(ValueError, 'verification_inputs'):
                phase.load_inputs(json.dumps(bundle).encode(), operation=OPERATION,
                    nonce=NONCE, pod_uid=POD_UID)
        self.assertEqual(list(self.inputs.iterdir()), [])

    def test_archive_helper_identity_matches_owner_for_noatime_source_reads(self):
        require = getattr(phase, '_require_archive_helper_identity', None)
        self.assertTrue(callable(require), 'archive identity guard is not implemented')
        with patch.object(phase.os, 'geteuid', return_value=10000), \
             patch.object(phase.os, 'getegid', return_value=10000):
            require()
        with patch.object(phase.os, 'geteuid', return_value=65532), \
             patch.object(phase.os, 'getegid', return_value=10000):
            with self.assertRaisesRegex(ValueError, 'archive_helper_identity'):
                require()

    def test_source_archive_preflights_owner_before_opening_native_store(self):
        before = [(path.lstat().st_uid, path.lstat().st_gid, path.lstat().st_mode,
            path.lstat().st_ino) for path in [self.source, *self.source.rglob('*')]]
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID)
        with patch.object(phase, '_require_archive_helper_identity'), \
             patch.object(phase, 'source_readable_by_archive_helper', return_value=False), \
             patch.object(phase.native_store, 'archive_closed_store',
                side_effect=AssertionError('must reject before opening source')):
            with self.assertRaisesRegex(ValueError, 'archive_source_permissions'):
                phase.create_and_stream(args)
        after = [(path.lstat().st_uid, path.lstat().st_gid, path.lstat().st_mode,
            path.lstat().st_ino) for path in [self.source, *self.source.rglob('*')]]
        self.assertEqual(after, before)

    def test_verification_helper_requires_the_hub_owner_without_weakening_runner_identity(self):
        with patch.object(phase.os, 'geteuid', return_value=10000), \
             patch.object(phase.os, 'getegid', return_value=10000):
            phase._require_verification_identity()
        with patch.object(phase.os, 'geteuid', return_value=65532), \
             patch.object(phase.os, 'getegid', return_value=10000):
            with self.assertRaisesRegex(ValueError, 'verification_identity'):
                phase._require_verification_identity()

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
        with patch.object(phase, '_require_archive_helper_identity'):
            with self.assertRaisesRegex(ValueError, 'archive-readback-binding'):
                phase.stream_existing(wrong)
        self.assertEqual((self.source / 'jetstream' / 'meta.inf').read_bytes(), b'cold-native-data')

    def test_existing_readback_rejects_replaced_pod_uid_without_rearchiving(self):
        self.run_helper(phase.create_and_stream)
        archive = self.output / 'archive.tar'
        original = archive.read_bytes()
        replacement = types.SimpleNamespace(operation=OPERATION, nonce=NONCE,
            pod_uid='77777777-7777-4777-8777-777777777777')
        with patch.object(phase, '_require_archive_helper_identity'), \
             patch.object(phase.native_store, 'archive_closed_store',
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
