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
            '--input-pod-uid', POD_UID,
            '--archive-sha256', 'a' * 64, '--archive-bytes', '123',
            '--session-attempt', '1234abcd', '--input-bundle-sha256', 'b' * 64,
            '--input-bundle-bytes', '512'])
        execute.assert_called_once()
        self.assertEqual(execute.call_args.args[0], 'census')
        self.assertEqual(execute.call_args.args[1].archive_bytes, 123)
        self.assertEqual(result, receipt)

    def _preflight_args(self):
        return types.SimpleNamespace(operation=OPERATION, nonce=NONCE,
            pod_uid=POD_UID, job_uid='44444444-4444-4444-8444-444444444444',
            verification_attempt='1234abcd', source_pvc_uid=SOURCE_PVC_UID,
            source_pv_uid=SOURCE_PV_UID, verification_pvc_uid=VERIFICATION_PVC_UID,
            verification_pv_uid=VERIFICATION_PV_UID, source_node_name='node-a',
            source_hostname='node-a.example', verification_node_name='node-a',
            verification_hostname='node-a.example', archive_sha256='a' * 64,
            archive_bytes=123, server_config_sha256='b' * 64,
            server_config_bytes=24, phase_image='registry.example/phase@sha256:' + 'c' * 64)

    def test_closed_copy_preflight_fixed_action_graph_binds_same_pod_and_full_jsz(self):
        config_root = self.root / 'broker-config'; config_root.mkdir(mode=0o700)
        original = (phase.PREFLIGHT_ROOT, phase.PREFLIGHT_CONFIG_ROOT)
        phase.PREFLIGHT_ROOT = self.verification / '.voice-nats-closed-copy'
        phase.PREFLIGHT_CONFIG_ROOT = config_root
        self.addCleanup(setattr, phase, 'PREFLIGHT_ROOT', original[0])
        self.addCleanup(setattr, phase, 'PREFLIGHT_CONFIG_ROOT', original[1])
        args = self._preflight_args()
        config = b'host: 127.0.0.1\nport: 4222\nhttp: 127.0.0.1:8222\nresolver: MEMORY\njetstream { store_dir: /data }\n'
        args.server_config_bytes = len(config)
        args.server_config_sha256 = hashlib.sha256(config).hexdigest()
        tree = b'{"account":"A","streams":[],"server":{"id":"nats"},"padding":"' + b'x' * (20 << 10) + b'"}'
        process = {'schema': 'voice-nats-supervisor-v1', 'ok': True, 'running': True,
            'pid': 42, 'started_at_ns': 100, 'nats_binary_sha256': 'd' * 64}
        stopped = {'schema': 'voice-nats-supervisor-v1', 'ok': True, 'running': False,
            'pid': 42, 'started_at_ns': 100, 'finished_at_ns': 200, 'exit_code': 0,
            'nats_binary_sha256': 'd' * 64}
        calls = iter((process, process, process, process, process, stopped))
        class Response:
            status = 200
            def __enter__(self): return self
            def __exit__(self, *_args): return False
            def read(self, _limit): return tree
        def opener(url, timeout):
            self.assertIn('127.0.0.1:8222', url)
            self.assertGreater(timeout, 0)
            return Response()
        streamed = Output()
        with patch.object(phase, '_require_verification_identity'), \
             patch.object(phase, '_receive_archive', return_value={'archive_sha256': 'a' * 64}) as receive, \
             patch.object(phase.native_store, 'restore_closed_store') as restore, \
             patch.object(phase, '_fsync_dir'), \
             patch.object(phase.sys, 'stdout', streamed):
            receipts = {}
            receipts['load-config'] = phase._preflight_action_receipt('load-config', args, input_stream=io.BytesIO(config))
            self.assertEqual((config_root / 'server.conf').read_bytes(), config)
            receipts['receive-archive'] = phase._preflight_action_receipt('receive-archive', args, input_stream=io.BytesIO(b''))
            supervisor = lambda action: next(calls)
            receipts['broker-start'] = phase._preflight_action_receipt('broker-start', args, supervisor_call=supervisor)
            ready = phase._preflight_action_receipt('broker-ready', args, opener=opener,
                supervisor_call=supervisor)
            receipts['broker-ready'] = ready
            self.assertEqual(ready['pid'], 42)
            receipts['read-jsz'] = phase._preflight_action_receipt('read-jsz', args, opener=opener,
                supervisor_call=supervisor)
            stopped_receipt = phase._preflight_action_receipt('broker-stop', args,
                supervisor_call=supervisor)
            receipts['broker-stop'] = stopped_receipt
            self.assertEqual(stopped_receipt['exit_code'], 0)
            self.assertEqual(phase._preflight_readback('read-jsz', args)['output_bytes'], len(tree))
        action_fields = {
            'load-config': {'server_config_sha256', 'server_config_bytes'},
            'receive-archive': {'archive_sha256', 'archive_bytes', 'manifest_sha256'},
            'broker-start': {'pid', 'started_at_ns', 'nats_binary_sha256'},
            'broker-ready': {'pid', 'started_at_ns', 'nats_binary_sha256', 'ready_at_ns', 'readiness_status'},
            'read-jsz': {'pid', 'started_at_ns', 'nats_binary_sha256', 'output_name', 'output_sha256', 'output_bytes'},
            'broker-stop': {'pid', 'started_at_ns', 'nats_binary_sha256', 'finished_at_ns', 'exit_code', 'stopped_at_ns'},
        }
        identity_fields = set(phase._preflight_binding(args))
        for action, receipt in receipts.items():
            self.assertEqual(set(receipt), identity_fields | {'schema', 'state', 'action'} | action_fields[action])
            self.assertEqual(receipt['schema'], phase.PREFLIGHT_SCHEMA)
            self.assertEqual(receipt['state'], 'COMPLETE')
            self.assertEqual(receipt['action'], action)
        frames = [json.loads(line) for line in streamed.buffer.getvalue().splitlines()]
        self.assertEqual([row['sequence'] for row in frames[:-1]], list(range(len(frames) - 1)))
        common = {key: phase._preflight_binding(args)[key] for key in phase._preflight_binding(args)}
        for row in frames:
            self.assertTrue(all(row.get(key) == value for key, value in common.items()))
        self.assertTrue(all(row['schema'] == phase.PREFLIGHT_READBACK_SCHEMA for row in frames))
        self.assertEqual(frames[-1]['kind'], 'complete')
        self.assertEqual(frames[-1]['byte_count'], len(tree))
        decoded = b''.join(base64.b64decode(row['data'], validate=True) for row in frames[:-1])
        self.assertEqual(decoded, json.dumps(json.loads(tree), sort_keys=True,
            separators=(',', ':'), ensure_ascii=False, allow_nan=False).encode())
        receive.assert_called_once()
        restore.assert_called_once()

    def test_closed_copy_preflight_rejects_replacement_pod_before_state_access(self):
        args = self._preflight_args()
        with patch.object(phase, '_require_verification_identity'), \
             patch.dict(phase.os.environ, {'POD_UID': '77777777-7777-4777-8777-777777777777'}):
            with self.assertRaisesRegex(ValueError, 'preflight_identity'):
                phase._preflight_action_receipt('load-config', args, input_stream=io.BytesIO(b''))
        self.assertFalse((self.verification / '.voice-nats-closed-copy').exists())

    def test_closed_copy_preflight_started_without_complete_is_never_replayed(self):
        args = self._preflight_args()
        root = self.verification / '.voice-nats-closed-copy'; root.mkdir(mode=0o700)
        marker = {'schema': phase.PREFLIGHT_SCHEMA, 'state': 'STARTED',
            'action': 'load-config', **phase._preflight_binding(args)}
        started = root / 'load_config.started.json'
        started.write_bytes(json.dumps(marker, sort_keys=True, separators=(',', ':')).encode())
        started.chmod(0o600)
        with patch.object(phase, '_require_verification_identity'):
            with self.assertRaisesRegex(ValueError, 'preflight_action_unknown'):
                phase._preflight_action_receipt('load-config', args, input_stream=io.BytesIO(b''))

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
        args.input_pod_uid = POD_UID
        with patch.object(phase, '_require_verification_identity'), \
             patch.object(phase, 'read_census_consumption_receipt', return_value={'status': 'COMPLETE'}):
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
        args.input_pod_uid = POD_UID
        with patch.object(phase, '_require_verification_identity'), \
             patch.object(phase, 'read_census_consumption_receipt', return_value={'status': 'COMPLETE'}), \
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
            archive_bytes=123, session_attempt='1234abcd', input_pod_uid=POD_UID)
        self._install_census_inputs(args)
        with patch.dict(phase.os.environ, {'POD_UID': POD_UID}), \
             patch.object(phase, '_require_verification_identity'):
            phase.consume_census_inputs(args)
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
        args.session_attempt = getattr(args, 'session_attempt', '1234abcd')
        config = ('host: 127.0.0.1\nport: 4222\nhttp: 127.0.0.1:8222\n'
            'resolver: MEMORY\njetstream: { store_dir: /data, max_file_store: 64MB, max_memory_store: 16MB }\n')
        bundle = {'schema': 'voice-nats-verification-inputs-v1', 'operation': args.operation,
            'nonce': args.nonce, 'pod_uid': args.pod_uid, 'files': {
                name: base64.b64encode((config if name == 'server.conf' else 'fixed-private-input\n').encode()).decode()
                for name in phase.INPUT_FILES}}
        raw = json.dumps(bundle, sort_keys=True, separators=(',', ':')).encode()
        args.input_bundle_sha256 = hashlib.sha256(raw).hexdigest()
        args.input_bundle_bytes = len(raw)
        with patch.object(phase, '_require_verification_identity'):
            return phase.load_inputs(raw, operation=args.operation,
                nonce=args.nonce, pod_uid=args.pod_uid, binding_args=args)

    def _install_census_inputs(self, args, *, extra=None):
        args.session_attempt = getattr(args, 'session_attempt', '1234abcd')
        config = ('host: 127.0.0.1\nport: 4222\nhttp: 127.0.0.1:8222\n'
            'resolver: MEMORY\njetstream: { store_dir: /data, max_file_store: 64MB, max_memory_store: 16MB }\n')
        files = {'bootstrap.creds': base64.b64encode(b'fixed-bootstrap-creds\n').decode(),
            'server.conf': base64.b64encode(config.encode()).decode()}
        if extra:
            files.update({name: base64.b64encode(value).decode() for name, value in extra.items()})
        bundle = {'schema': 'voice-nats-census-inputs-v1', 'operation': args.operation,
            'nonce': args.nonce, 'pod_uid': args.pod_uid, 'files': files}
        raw = json.dumps(bundle, sort_keys=True, separators=(',', ':')).encode()
        args.input_bundle_sha256 = hashlib.sha256(raw).hexdigest()
        args.input_bundle_bytes = len(raw)
        with patch.object(phase, '_require_verification_identity'):
            return phase.load_census_inputs(raw, operation=args.operation,
                nonce=args.nonce, pod_uid=args.pod_uid, binding_args=args)

    def _consume_census_inputs(self, args):
        args.input_pod_uid = args.pod_uid
        with patch.object(phase, '_require_verification_identity'):
            return phase.consume_census_inputs(args)

    def test_census_loader_accepts_only_canonical_capture_outputs(self):
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID,
            verification_pvc_uid=VERIFICATION_PVC_UID, verification_pv_uid=VERIFICATION_PV_UID,
            archive_sha256='a' * 64, archive_bytes=123, session_attempt='1234abcd')
        receipt = self._install_census_inputs(args)
        self.assertEqual(receipt['phase'], 'census-inputs')
        self.assertEqual(receipt['file_count'], 2)
        self.assertEqual(set(path.name for path in (self.verification / 'kernelinputbundle').iterdir()),
            {'bootstrap.creds'})
        self.assertEqual(set(path.name for path in (self.verification / 'inputs' / 'public').iterdir()),
            {'server.conf'})
        args.input_pod_uid = POD_UID
        with patch.object(phase, '_require_verification_identity'):
            self.assertEqual(phase.read_census_input_receipt(args), receipt)

    def test_census_consumption_binds_new_pod_without_rewriting_loader_receipt(self):
        loader = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID,
            verification_pvc_uid=VERIFICATION_PVC_UID, verification_pv_uid=VERIFICATION_PV_UID,
            archive_sha256='a' * 64, archive_bytes=123, session_attempt='1234abcd')
        self._install_census_inputs(loader)
        load_path = (self.verification / 'state' / OPERATION / '1234abcd' /
            'census-inputs.receipt.json')
        load_bytes = load_path.read_bytes()
        session_uid = '77777777-7777-4777-8777-777777777777'
        session = types.SimpleNamespace(**{**vars(loader), 'pod_uid': session_uid,
            'input_pod_uid': POD_UID, 'input_bundle_sha256': loader.input_bundle_sha256,
            'input_bundle_bytes': loader.input_bundle_bytes})
        with patch.dict(phase.os.environ, {'POD_UID': session_uid}), \
             patch.object(phase, '_require_verification_identity'):
            receipt = phase.consume_census_inputs(session)
            self.assertEqual(phase.consume_census_inputs(session), receipt)
        self.assertEqual(receipt['pod_uid'], session_uid)
        self.assertEqual(receipt['input_pod_uid'], POD_UID)
        self.assertEqual(load_path.read_bytes(), load_bytes)
        replacement_uid = '88888888-8888-4888-8888-888888888888'
        session.pod_uid = replacement_uid
        with patch.dict(phase.os.environ, {'POD_UID': replacement_uid}), \
             patch.object(phase, '_require_verification_identity'):
            with self.assertRaisesRegex(ValueError, 'verification_input_consumption_unknown'):
                phase.consume_census_inputs(session)

    def test_census_loader_rejects_fixture_and_unneeded_jwt_material_before_projection(self):
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID,
            verification_pvc_uid=VERIFICATION_PVC_UID, verification_pv_uid=VERIFICATION_PV_UID,
            archive_sha256='a' * 64, archive_bytes=123, session_attempt='1234abcd')
        with patch.object(phase, '_require_verification_identity'):
            with self.assertRaisesRegex(ValueError, 'verification_inputs'):
                self._install_census_inputs(args, extra={
                    'fixture.json': b'{}', 'operator.jwt': b'a.b.c',
                    'account.jwt': b'a.b.c', 'system-account.jwt': b'a.b.c'})
        self.assertFalse((self.verification / 'kernelinputbundle').exists())

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
            archive_sha256=manifest['archive_sha256'], archive_bytes=len(tar_bytes),
            session_attempt='1234abcd')
        input_receipt = self._install_census_inputs(args)
        self.assertEqual(input_receipt['status'], 'COMPLETE')
        self.assertEqual(set(path.name for path in (self.verification / 'kernelinputbundle').iterdir()),
            {'bootstrap.creds'})
        self.assertEqual((self.verification / 'inputs' / 'public' / 'server.conf').read_bytes(),
            (self.input_private / 'server.conf').read_bytes())
        self._consume_census_inputs(args)
        with patch.object(phase, '_require_verification_identity'):
            args.input_pod_uid = POD_UID
            self.assertEqual(phase.read_census_input_receipt(args), input_receipt)
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

    def test_census_phase_requires_consumption_receipt_before_archive_or_broker_start(self):
        archive = self.root / 'source.tar'
        manifest = native_store.archive_closed_store(self.source, archive)
        tar_bytes = archive.read_bytes()
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID,
            verification_pvc_uid=VERIFICATION_PVC_UID, verification_pv_uid=VERIFICATION_PV_UID,
            archive_sha256=manifest['archive_sha256'], archive_bytes=len(tar_bytes),
            session_attempt='1234abcd')
        self._install_census_inputs(args)
        args.input_pod_uid = POD_UID
        with patch.object(phase, '_require_verification_identity'), \
             patch.object(phase.subprocess, 'Popen', side_effect=AssertionError('broker must not start')), \
             patch.object(phase.native_store, 'restore_closed_store', side_effect=AssertionError('archive must not restore')):
            with self.assertRaisesRegex(ValueError, 'verification_input_consumption_unknown'):
                phase.run_verification('census', args, io.BytesIO(b''))

    def test_new_session_pod_validates_loader_receipt_and_exact_persistent_bundle_before_start(self):
        loader = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID,
            verification_pvc_uid=VERIFICATION_PVC_UID, verification_pv_uid=VERIFICATION_PV_UID,
            archive_sha256='a' * 64, archive_bytes=123, session_attempt='1234abcd')
        loaded = self._install_verification_inputs(loader)
        session_uid = '77777777-7777-4777-8777-777777777777'
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=session_uid,
            input_pod_uid=POD_UID, session_attempt='1234abcd',
            input_bundle_sha256=loader.input_bundle_sha256,
            input_bundle_bytes=loader.input_bundle_bytes,
            kernel_bundle_sha256=loaded['kernel_bundle_sha256'],
            server_config_sha256=loaded['server_config_sha256'],
            archive_sha256='a' * 64, archive_bytes=123,
            source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID,
            verification_pvc_uid=VERIFICATION_PVC_UID, verification_pv_uid=VERIFICATION_PV_UID)
        with patch.object(phase, 'SESSION_UID', os.geteuid()), \
             patch.object(phase, 'SESSION_GID', os.getegid()), \
             patch.object(phase, 'SESSION_INPUT_ROOT', self.verification / 'kernelinputbundle'), \
             patch.dict(phase.os.environ, {'POD_UID': session_uid}):
            binding = phase._session_binding(args)
            root = self.verification / 'state' / OPERATION / '1234abcd'
            receipt = phase._session_consume_inputs(binding, root)
            self.assertEqual(receipt['state'], 'COMPLETE')
            self.assertEqual(receipt['input_pod_uid'], POD_UID)
            self.assertEqual(receipt['pod_uid'], session_uid)
            with self.assertRaisesRegex(ValueError, 'session_input_consumption_unknown'):
                phase._session_consume_inputs(binding, root)

    def test_incomplete_or_tampered_upload_is_durable_and_never_replayed(self):
        args = types.SimpleNamespace(operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
            source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID,
            verification_pvc_uid=VERIFICATION_PVC_UID, verification_pv_uid=VERIFICATION_PV_UID,
            archive_sha256='a' * 64, archive_bytes=10)
        args.session_attempt = '1234abcd'
        self._install_census_inputs(args)
        self._consume_census_inputs(args)
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
        raw = json.dumps(bundle, sort_keys=True, separators=(',', ':')).encode()
        with patch.object(phase, '_require_verification_identity'):
            with self.assertRaisesRegex(ValueError, 'verification_inputs'):
                phase.load_inputs(raw, operation=OPERATION,
                    nonce=NONCE, pod_uid=POD_UID, binding_args=types.SimpleNamespace(
                        operation=OPERATION, nonce=NONCE, pod_uid=POD_UID,
                        source_pvc_uid=SOURCE_PVC_UID, source_pv_uid=SOURCE_PV_UID,
                        verification_pvc_uid=VERIFICATION_PVC_UID,
                        verification_pv_uid=VERIFICATION_PV_UID,
                        archive_sha256='a' * 64, archive_bytes=123,
                        session_attempt='1234abcd',
                        input_bundle_sha256=hashlib.sha256(raw).hexdigest(),
                        input_bundle_bytes=len(raw)))
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

    def test_selected_session_receipt_is_durable_and_started_action_is_never_replayed(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            root.chmod(0o700)
            binding = {'schema': 'voice-nats-session-action-v1', 'operation': OPERATION,
                'nonce': NONCE, 'pod_uid': POD_UID, 'session_attempt': '1234abcd',
                'source_pvc_uid': SOURCE_PVC_UID, 'source_pv_uid': SOURCE_PV_UID,
                'verification_pvc_uid': VERIFICATION_PVC_UID,
                'verification_pv_uid': VERIFICATION_PV_UID}
            action = 'kernel-seed'
            payload = b'{"fixture":"fixed"}\\n'
            output_name = action.replace('-', '_') + '.output'
            def perform(_action, state):
                phase._write_exclusive(state / output_name, payload)
                return {'output_name': output_name, 'output_bytes': len(payload),
                    'output_sha256': hashlib.sha256(payload).hexdigest()}
            current_uid, current_gid = os.geteuid(), os.getegid()
            with patch.object(phase, 'SESSION_UID', current_uid), \
                 patch.object(phase, 'SESSION_GID', current_gid):
                receipt = phase._session_receipt(action, binding, root, perform=perform)
                recovered, output = phase._session_readback(action, binding, root)
                self.assertEqual(receipt, recovered)
                self.assertEqual(output.read(), payload)
                output.close()
                with self.assertRaisesRegex(ValueError, 'session_receipt_identity'):
                    phase._session_readback(action, binding | {'nonce': 'a' * 32}, root)
                self.assertEqual(phase._private_json(root / 'kernel_seed.started.json')['state'], 'STARTED')
                (root / 'kernel_seed.receipt.json').unlink()
                with self.assertRaisesRegex(ValueError, 'session_action_unknown'):
                    phase._session_receipt(action, binding, root,
                        perform=lambda *_: self.fail('STARTED action must never replay'))

    def test_selected_session_readback_requires_complete_receipt_and_rejects_tampering(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            root.chmod(0o700)
            binding = {'schema': 'voice-nats-session-action-v1', 'operation': OPERATION,
                'nonce': NONCE, 'pod_uid': POD_UID, 'session_attempt': '1234abcd',
                'source_pvc_uid': SOURCE_PVC_UID, 'source_pv_uid': SOURCE_PV_UID,
                'verification_pvc_uid': VERIFICATION_PVC_UID,
                'verification_pv_uid': VERIFICATION_PV_UID}
            current_uid, current_gid = os.geteuid(), os.getegid()
            with patch.object(phase, 'SESSION_UID', current_uid), \
                 patch.object(phase, 'SESSION_GID', current_gid):
                with self.assertRaisesRegex(ValueError, 'session_receipt_missing'):
                    phase._session_readback('kernel-census', binding, root)
                receipt = phase._session_receipt('kernel-census', binding, root, perform=lambda *_: None)
                self.assertEqual(receipt['state'], 'COMPLETE')
                path = root / 'kernel_census.receipt.json'
                path.write_text('{"tampered":true}')
                with self.assertRaisesRegex(ValueError, 'session_receipt_identity'):
                    phase._session_readback('kernel-census', binding, root)

    def test_selected_kernel_followup_uses_durable_seed_snapshot(self):
        for action, expected_phase in (('kernel-verify-closed', 'verify-closed'),
            ('kernel-drain', 'drain'), ('kernel-verify-drained', 'verify-drained')):
            with self.subTest(action=action), \
                 patch.object(phase, '_session_inputs'), \
                 patch.object(phase, 'KERNEL', Path('/trusted/kernel')), \
                 patch.object(phase.subprocess, 'run', return_value=types.SimpleNamespace(returncode=0)) as run:
                self.assertIsNone(phase._perform_session_action(action, Path('/state')))
                self.assertEqual(run.call_args.args[0], ['/trusted/kernel', '--phase', expected_phase,
                    '--session-state'])

    def test_session_readback_rejects_unrecognized_receipt_fields(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            root.chmod(0o700)
            binding = {'schema': 'voice-nats-session-action-v1', 'operation': OPERATION,
                'nonce': NONCE, 'pod_uid': POD_UID, 'session_attempt': '1234abcd',
                'source_pvc_uid': SOURCE_PVC_UID, 'source_pv_uid': SOURCE_PV_UID,
                'verification_pvc_uid': VERIFICATION_PVC_UID,
                'verification_pv_uid': VERIFICATION_PV_UID}
            current_uid, current_gid = os.geteuid(), os.getegid()
            with patch.object(phase, 'SESSION_UID', current_uid), \
                 patch.object(phase, 'SESSION_GID', current_gid):
                phase._session_receipt('broker-ready', binding, root, perform=lambda *_: None)
                path = root / 'broker_ready.receipt.json'
                receipt = json.loads(path.read_text())
                receipt['ignored'] = True
                path.unlink()
                phase._write_exclusive(path, json.dumps(receipt, sort_keys=True,
                    separators=(',', ':')).encode())
                with self.assertRaisesRegex(ValueError, 'session_receipt_identity'):
                    phase._session_readback('broker-ready', binding, root)


if __name__ == '__main__': unittest.main()
