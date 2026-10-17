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
SOURCE_PVC_UID = '11111111-1111-4111-8111-111111111111'
SOURCE_PV_UID = '55555555-5555-4555-8555-555555555555'
VERIFICATION_PVC_UID = '22222222-2222-4222-8222-222222222222'
VERIFICATION_PV_UID = '33333333-3333-4333-8333-333333333333'


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
        self.input_private = self.inputs / 'private'
        self.args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID,
            verification_pvc_uid=VERIFICATION_PVC_UID, verification_pv_uid=VERIFICATION_PV_UID)
        self.patches = [patch.object(phase, 'DATA_ROOT', self.source),
            patch.object(phase, 'OUT_ROOT', self.output),
            patch.object(phase, 'VERIFY_ROOT', self.verification),
            patch.object(phase, 'INPUT_ROOT', self.input_private),
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
        receipt = {'schema': 'voice-nats-verification-v1', 'phase': 'census', 'purpose': 'verification',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'archive_sha256': 'a' * 64, 'archive_bytes': 123, 'census_sha256': 'd' * 64,
            'status': 'COMPLETE'}
        with patch.object(phase, 'run_verification', return_value=receipt) as execute:
            result = phase.main(['--receive-archive', '--operation', OPERATION,
                '--nonce', NONCE, '--pod-uid', POD_UID,
                '--source-pvc-uid', '11111111-1111-4111-8111-111111111111',
            '--source-pv-uid', SOURCE_PV_UID,
            '--verification-pvc-uid', '22222222-2222-4222-8222-222222222222',
            '--verification-pv-uid', VERIFICATION_PV_UID,
            '--archive-sha256', 'a' * 64, '--archive-bytes', '123'])
        execute.assert_called_once()
        self.assertEqual(execute.call_args.args[0], 'census')
        self.assertEqual(execute.call_args.args[1].archive_bytes, 123)
        self.assertEqual(result, receipt)

    def test_census_readback_returns_only_matching_durable_receipt(self):
        state = self.verification / '.voice-nats-verification'
        state.mkdir(mode=0o700)
        marker = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census', 'purpose': 'verification',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': '11111111-1111-4111-8111-111111111111',
            'source_pv_uid': SOURCE_PV_UID,
            'verification_pvc_uid': '22222222-2222-4222-8222-222222222222',
            'verification_pv_uid': VERIFICATION_PV_UID, 'archive_sha256': 'a' * 64,
            'archive_bytes': 123,
            'state': 'STARTED'}
        census = {'schema': phase.CENSUS_SCHEMA, 'account': 'A' + 'A' * 55,
            'streams': [], 'consumers': []}
        census_bytes = json.dumps(census, sort_keys=True, separators=(',', ':'),
            allow_nan=False).encode()
        receipt = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census', 'purpose': 'verification',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': marker['source_pvc_uid'],
            'source_pv_uid': marker['source_pv_uid'],
            'verification_pvc_uid': marker['verification_pvc_uid'],
            'verification_pv_uid': marker['verification_pv_uid'],
            'archive_sha256': 'a' * 64, 'archive_bytes': 123,
            'census_sha256': hashlib.sha256(census_bytes).hexdigest(), 'status': 'COMPLETE'}
        (state / (OPERATION + '-' + NONCE + '.census.started.json')).write_text(json.dumps(marker))
        (state / (OPERATION + '-' + NONCE + '.receipt.json')).write_text(json.dumps(receipt))
        census_path = state / (OPERATION + '-' + NONCE + '.census.json')
        census_path.write_bytes(census_bytes)
        for path in state.iterdir():
            path.chmod(0o600)
        (self.source / 'jetstream').chmod(0o555)
        (self.source / 'jetstream' / 'meta.inf').chmod(0o444)
        self.source.chmod(0o555)
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=marker['source_pvc_uid'], source_pv_uid=marker['source_pv_uid'],
            verification_pvc_uid=marker['verification_pvc_uid'],
            verification_pv_uid=marker['verification_pv_uid'], archive_sha256='a' * 64,
            archive_bytes=123)
        with patch.object(phase, '_require_verification_identity'), \
             patch.object(phase, 'read_input_receipt', return_value={'status': 'COMPLETE'}):
            self.assertEqual(phase.read_verification_receipt('census', args),
                {'receipt': receipt, 'census': census})

    def test_census_readback_rejects_digest_drift_without_replaying(self):
        state = self.verification / '.voice-nats-verification'
        state.mkdir(mode=0o700)
        source_uid = '11111111-1111-4111-8111-111111111111'
        verification_uid = '22222222-2222-4222-8222-222222222222'
        marker = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census', 'purpose': 'verification',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': source_uid, 'source_pv_uid': SOURCE_PV_UID,
            'verification_pvc_uid': verification_uid, 'verification_pv_uid': VERIFICATION_PV_UID,
            'archive_sha256': 'a' * 64, 'archive_bytes': 123,
            'state': 'STARTED'}
        census = {'schema': phase.CENSUS_SCHEMA, 'account': 'A' + 'A' * 55,
            'streams': [], 'consumers': []}
        census_bytes = json.dumps(census, sort_keys=True, separators=(',', ':'),
            allow_nan=False).encode()
        receipt = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census', 'purpose': 'verification',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': source_uid, 'source_pv_uid': SOURCE_PV_UID,
            'verification_pvc_uid': verification_uid, 'verification_pv_uid': VERIFICATION_PV_UID,
            'archive_sha256': 'a' * 64, 'archive_bytes': 123,
            'census_sha256': 'b' * 64, 'status': 'COMPLETE'}
        (state / (OPERATION + '-' + NONCE + '.census.started.json')).write_text(json.dumps(marker))
        (state / (OPERATION + '-' + NONCE + '.receipt.json')).write_text(json.dumps(receipt))
        (state / (OPERATION + '-' + NONCE + '.census.json')).write_bytes(census_bytes)
        for path in state.iterdir(): path.chmod(0o600)
        (self.source / 'jetstream').chmod(0o555)
        (self.source / 'jetstream' / 'meta.inf').chmod(0o444)
        self.source.chmod(0o555)
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=source_uid, source_pv_uid=SOURCE_PV_UID,
            verification_pvc_uid=verification_uid, verification_pv_uid=VERIFICATION_PV_UID,
            archive_sha256='a' * 64, archive_bytes=123)
        with patch.object(phase, '_require_verification_identity'), \
             patch.object(phase, 'read_input_receipt', return_value={'status': 'COMPLETE'}), \
             patch.object(phase, 'run_verification', side_effect=AssertionError('must not replay')):
            with self.assertRaisesRegex(ValueError, 'verification_receipt_binding'):
                phase.read_verification_receipt('census', args)

    def test_census_readback_uses_bounded_ordered_frames_and_carries_exact_receipt(self):
        census = {'schema': phase.CENSUS_SCHEMA, 'account': 'A' + 'A' * 55,
            'streams': [], 'consumers': [], 'padding': 'x' * (phase.READBACK_CHUNK + 17)}
        raw = json.dumps(census, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()
        receipt = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census', 'purpose': 'verification',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': '11111111-1111-4111-8111-111111111111',
            'source_pv_uid': SOURCE_PV_UID,
            'verification_pvc_uid': '22222222-2222-4222-8222-222222222222',
            'verification_pv_uid': VERIFICATION_PV_UID,
            'archive_sha256': 'a' * 64, 'archive_bytes': 123,
            'census_sha256': hashlib.sha256(raw).hexdigest(),
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
        receipt = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census', 'purpose': 'verification',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': '11111111-1111-4111-8111-111111111111',
            'source_pv_uid': SOURCE_PV_UID,
            'verification_pvc_uid': '22222222-2222-4222-8222-222222222222',
            'verification_pv_uid': VERIFICATION_PV_UID,
            'archive_sha256': 'a' * 64, 'archive_bytes': 123,
            'census_sha256': hashlib.sha256(raw).hexdigest(), 'status': 'COMPLETE'}
        output = Output()
        with patch.object(phase.sys, 'stdout', output), \
             patch.object(phase, 'READBACK_CHUNK', 1), \
             patch.object(phase, 'MAX_READBACK_LINE', 100):
            with self.assertRaisesRegex(ValueError, 'readback_frame_size'):
                phase.stream_census_readback({'receipt': receipt, 'census': census})
        self.assertEqual(output.buffer.getvalue(), b'')

    def test_bind_only_is_fixed_nonroot_readonly_mount_probe(self):
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            role='verification')
        before = [(p.name, p.lstat().st_uid, p.lstat().st_gid, p.lstat().st_mode,
            p.lstat().st_ino) for p in self.verification.iterdir()]
        output = Output()
        with patch.object(phase.os, 'geteuid', return_value=65532), \
             patch.object(phase.os, 'getegid', return_value=65532), \
             patch.dict(phase.os.environ, {'POD_UID': POD_UID}), \
             patch.object(phase.sys, 'stdout', output):
            result = phase.main(['--bind-only', '--operation', OPERATION,
                '--nonce', NONCE, '--role', 'verification'])
        self.assertEqual(result, {'schema': 'voice-nats-bind-mount-v1',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'role': 'verification', 'mounted': True})
        after = [(p.name, p.lstat().st_uid, p.lstat().st_gid, p.lstat().st_mode,
            p.lstat().st_ino) for p in self.verification.iterdir()]
        self.assertEqual(after, before)
        self.assertEqual(output.buffer.getvalue(), b'')

    def test_bind_only_rejects_wrong_uid_or_role_before_target_io(self):
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            role='restore')
        with patch.object(phase.os, 'geteuid', return_value=10000), \
             patch.object(phase.os, 'getegid', return_value=10000), \
             patch.dict(phase.os.environ, {'POD_UID': POD_UID}), \
             patch.object(type(phase.VERIFY_ROOT), 'lstat', side_effect=AssertionError('must reject before mount I/O')):
            with self.assertRaisesRegex(ValueError, 'bind_profile'):
                phase.bind_only(args)
        args.role = 'arbitrary'
        with patch.object(phase.os, 'geteuid', return_value=65532), \
             patch.object(phase.os, 'getegid', return_value=65532), \
             patch.dict(phase.os.environ, {'POD_UID': POD_UID}), \
             patch.object(type(phase.VERIFY_ROOT), 'lstat', side_effect=AssertionError('must reject before mount I/O')):
            with self.assertRaisesRegex(ValueError, 'bind_profile'):
                phase.bind_only(args)

    def test_census_interruption_without_receipt_is_not_replayed(self):
        state = self.verification / '.voice-nats-verification'
        state.mkdir(mode=0o700)
        marker = {'schema': phase.RECEIPT_SCHEMA, 'phase': 'census', 'purpose': 'verification',
            'operation': OPERATION, 'nonce': NONCE, 'pod_uid': POD_UID,
            'source_pvc_uid': '11111111-1111-4111-8111-111111111111',
            'source_pv_uid': SOURCE_PV_UID,
            'verification_pvc_uid': '22222222-2222-4222-8222-222222222222',
            'verification_pv_uid': VERIFICATION_PV_UID,
            'archive_sha256': 'a' * 64, 'archive_bytes': 123,
            'state': 'STARTED'}
        marker_path = state / (OPERATION + '-' + NONCE + '.census.started.json')
        marker_path.write_text(json.dumps(marker)); marker_path.chmod(0o600)
        (self.source / 'jetstream').chmod(0o555)
        (self.source / 'jetstream' / 'meta.inf').chmod(0o444)
        self.source.chmod(0o555)
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=marker['source_pvc_uid'], source_pv_uid=marker['source_pv_uid'],
            verification_pvc_uid=marker['verification_pvc_uid'],
            verification_pv_uid=marker['verification_pv_uid'], archive_sha256='a' * 64,
            archive_bytes=123)
        with patch.object(phase, '_require_verification_identity'), \
             patch.object(phase, 'read_input_receipt', return_value={'status': 'COMPLETE'}), \
             patch.object(phase, 'run_verification', side_effect=AssertionError('must not replay')):
            with self.assertRaisesRegex(ValueError, 'verification_receipt_incomplete'):
                phase.read_verification_receipt('census', args)

    def test_verification_refuses_missing_distinct_writable_copy_before_restore(self):
        verify = getattr(phase, 'validate_verification_mounts', None)
        self.assertTrue(callable(verify), 'verification PVC boundary is not implemented')
        with patch.object(phase.native_store, 'restore_closed_store',
                side_effect=AssertionError('restore must not run when scratch mounts alias')):
            with self.assertRaisesRegex(ValueError, 'verification_mounts'):
                verify(self.verification, self.verification, self.inputs,
                    verification_pvc_uid='22222222-2222-4222-8222-222222222222')

    def _install_verification_inputs(self, args):
        config = ('host: 127.0.0.1\nport: 4222\nhttp: 127.0.0.1:8222\n'
            'resolver: MEMORY\njetstream: { store_dir: /data, max_file_store: 64MB, max_memory_store: 16MB }\n')
        bundle = {'schema': 'voice-nats-verification-inputs-v1', 'operation': args.operation,
            'nonce': args.nonce, 'pod_uid': args.pod_uid, 'files': {
                name: base64.b64encode((config if name == 'server.conf' else 'fixed-private-input\n').encode()).decode()
                for name in phase.INPUT_FILES}}
        with patch.object(phase, '_require_verification_identity'):
            return phase.load_inputs(json.dumps(bundle).encode(), operation=args.operation,
                nonce=args.nonce, pod_uid=args.pod_uid, binding_args=args)

    def test_verification_restores_streamed_archive_to_target_only_scratch_and_runs_fixed_chain(self):
        archive = self.root / 'source.tar'
        manifest = native_store.archive_closed_store(self.source, archive)
        tar_bytes = archive.read_bytes()
        manifest_bytes = json.dumps(manifest, sort_keys=True, separators=(',', ':'),
            ensure_ascii=False, allow_nan=False).encode('utf-8')
        envelope = (phase.CENSUS_INPUT_MAGIC + len(manifest_bytes).to_bytes(4, 'big')
            + manifest_bytes + len(tar_bytes).to_bytes(8, 'big') + tar_bytes)
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid='11111111-1111-4111-8111-111111111111',
            source_pv_uid=SOURCE_PV_UID,
            verification_pvc_uid='22222222-2222-4222-8222-222222222222',
            verification_pv_uid=VERIFICATION_PV_UID,
            archive_sha256=manifest['archive_sha256'], archive_bytes=len(tar_bytes))
        self._install_verification_inputs(args)
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
             patch.object(phase, 'DATA_ROOT', self.root / 'source-must-not-be-mounted'), \
             patch.object(phase.subprocess, 'Popen', side_effect=lambda *a, **k: (events.append(('start', a[0])) or process)), \
             patch.object(phase, '_wait_nats', side_effect=lambda *_a, **_k: events.append(('healthy',))), \
             patch.object(phase.subprocess, 'run', side_effect=kernel_run), \
             patch.object(phase, 'KERNEL', Path('/voice-nats-runner/kernel')), \
             patch.object(phase, 'NATS_SERVER', Path('/usr/local/bin/nats-server')):
            receipt = phase.run_verification('census', args, io.BytesIO(envelope))
        self.assertEqual([event[0] for event in events], ['start', 'healthy', 'kernel'])
        process.terminate.assert_called_once_with()
        process.wait.assert_called_once_with(timeout=30)
        self.assertEqual(receipt['status'], 'COMPLETE')
        self.assertEqual(receipt['purpose'], 'verification')
        self.assertEqual(receipt['archive_sha256'], manifest['archive_sha256'])
        self.assertEqual(receipt['archive_bytes'], len(tar_bytes))
        self.assertTrue((self.verification / 'store' / 'jetstream' / 'meta.inf').is_file())
        self.assertEqual((self.verification / '.voice-nats-verification' /
            f'{OPERATION}-{NONCE}.receipt.json').is_file(), True)

    def test_incomplete_or_tampered_upload_is_durable_and_never_replayed(self):
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID,
            verification_pvc_uid=VERIFICATION_PVC_UID, verification_pv_uid=VERIFICATION_PV_UID,
            archive_sha256='a' * 64, archive_bytes=10)
        self._install_verification_inputs(args)
        with patch.object(phase, '_require_verification_identity'):
            with self.assertRaisesRegex(ValueError, 'verification_input_header'):
                phase.run_verification('census', args, io.BytesIO(b'X' * len(phase.CENSUS_INPUT_MAGIC)))
        marker = self.verification / '.voice-nats-verification' / f'{OPERATION}-{NONCE}.census.started.json'
        self.assertTrue(marker.is_file())
        self.assertEqual(json.loads(marker.read_bytes())['state'], 'STARTED')
        with patch.object(phase, '_require_verification_identity'), \
             patch.object(phase.native_store, 'restore_closed_store', side_effect=AssertionError('must not restore')):
            with self.assertRaisesRegex(ValueError, 'verification_already_started'):
                phase.run_verification('census', args, io.BytesIO(b'anything'))

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
                    nonce=NONCE, pod_uid=POD_UID, binding_args=types.SimpleNamespace(
                        operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
                        source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID,
                        verification_pvc_uid=VERIFICATION_PVC_UID,
                        verification_pv_uid=VERIFICATION_PV_UID,
                        archive_sha256='a' * 64, archive_bytes=123))
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
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID)
        with patch.object(phase, '_require_archive_helper_identity'), \
             patch.object(phase, 'source_readable_by_archive_helper', return_value=False), \
             patch.object(phase.native_store, 'archive_closed_store',
                side_effect=AssertionError('must reject before opening source')):
            with self.assertRaisesRegex(ValueError, 'archive_source_permissions'):
                phase.create_and_stream(args)
        after = [(path.lstat().st_uid, path.lstat().st_gid, path.lstat().st_mode,
            path.lstat().st_ino) for path in [self.source, *self.source.rglob('*')]]
        self.assertEqual(after, before)

    def test_verification_helper_requires_fixed_scratch_uid_not_hub_or_root(self):
        with patch.object(phase.os, 'geteuid', return_value=65532), \
             patch.object(phase.os, 'getegid', return_value=65532):
            phase._require_verification_identity()
        for uid, gid in ((10000, 10000), (0, 0), (65532, 10000)):
            with patch.object(phase.os, 'geteuid', return_value=uid), \
                 patch.object(phase.os, 'getegid', return_value=gid):
                with self.subTest(uid=uid, gid=gid), self.assertRaisesRegex(ValueError, 'verification_identity'):
                    phase._require_verification_identity()

    def test_create_once_then_existing_readback_is_independent_of_changed_source(self):
        first = self.run_helper(phase.create_and_stream)
        rows = [json.loads(line) for line in first.splitlines()]
        self.assertTrue(all(row['source_pvc_uid'] == SOURCE_PVC_UID
            and row['source_pv_uid'] == SOURCE_PV_UID for row in rows))
        archive = self.output / 'archive.tar'
        original = archive.read_bytes()
        self.assertTrue((self.output / 'archive.manifest.json').is_file())
        manifest_record = json.loads((self.output / 'archive.manifest.json').read_bytes())
        self.assertEqual(manifest_record['source_pvc_uid'], SOURCE_PVC_UID)
        self.assertEqual(manifest_record['source_pv_uid'], SOURCE_PV_UID)
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
        wrong = types.SimpleNamespace(operation=OPERATION, nonce='c' * 32, pod_uid=POD_UID,
            source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID)
        with patch.object(phase, '_require_archive_helper_identity'):
            with self.assertRaisesRegex(ValueError, 'archive-readback-binding'):
                phase.stream_existing(wrong)
        self.assertEqual((self.source / 'jetstream' / 'meta.inf').read_bytes(), b'cold-native-data')

    def test_existing_readback_rejects_replaced_pod_uid_without_rearchiving(self):
        self.run_helper(phase.create_and_stream)
        archive = self.output / 'archive.tar'
        original = archive.read_bytes()
        replacement = types.SimpleNamespace(operation=OPERATION, nonce=NONCE,
            pod_uid='77777777-7777-4777-8777-777777777777',
            source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID)
        with patch.object(phase, '_require_archive_helper_identity'), \
             patch.object(phase.native_store, 'archive_closed_store',
                side_effect=AssertionError('replacement Pod must never recreate the archive')):
            with self.assertRaisesRegex(ValueError, 'archive-readback-binding'):
                phase.stream_existing(replacement)
        self.assertEqual(archive.read_bytes(), original)

    def test_existing_readback_rejects_source_pv_identity_drift_without_rearchiving(self):
        self.run_helper(phase.create_and_stream)
        wrong = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=SOURCE_PVC_UID, source_pv_uid='77777777-7777-4777-8777-777777777777')
        with patch.object(phase, '_require_archive_helper_identity'), \
             patch.object(phase.native_store, 'archive_closed_store',
                side_effect=AssertionError('changed source identity must never recreate archive')):
            with self.assertRaisesRegex(ValueError, 'archive-readback-binding'):
                phase.stream_existing(wrong)

    def test_archive_frame_schema_has_source_pvc_and_pv_uids_on_every_record(self):
        raw = self.run_helper(phase.create_and_stream)
        rows = [json.loads(line) for line in raw.splitlines()]
        self.assertEqual({row['kind'] for row in rows}, {'data', 'manifest', 'complete'})
        for row in rows:
            self.assertEqual(row['source_pvc_uid'], SOURCE_PVC_UID)
            self.assertEqual(row['source_pv_uid'], SOURCE_PV_UID)

    def test_helper_output_emits_bounded_source_bound_archive_frames(self):
        raw = self.run_helper(phase.create_and_stream)
        rows = [json.loads(line) for line in raw.splitlines()]
        self.assertTrue(rows)
        for row in rows:
            self.assertLessEqual(len(json.dumps(row, separators=(',', ':')).encode()), phase.MAX_READBACK_LINE)
            self.assertEqual(row['source_pvc_uid'], SOURCE_PVC_UID)
            self.assertEqual(row['source_pv_uid'], SOURCE_PV_UID)
        self.assertEqual(rows[-1]['kind'], 'complete')
        self.assertEqual(rows[-1]['frames'], sum(row['kind'] == 'data' for row in rows))


if __name__ == '__main__': unittest.main()
