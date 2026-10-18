import base64
import hashlib
import json
import unittest

import closed_copy_stream as subject


IDENTITY = {
    'operation': 'a' * 12, 'nonce': 'b' * 32, 'verification_attempt': 'c' * 8,
    'pod_uid': '00000000-0000-0000-0000-000000000001',
    'job_uid': '00000000-0000-0000-0000-000000000002',
    'source_pvc_uid': '00000000-0000-0000-0000-000000000003',
    'source_pv_uid': '00000000-0000-0000-0000-000000000004',
    'source_node_name': 'node-a', 'source_hostname': 'host-a',
    'verification_pvc_uid': '00000000-0000-0000-0000-000000000005',
    'verification_pv_uid': '00000000-0000-0000-0000-000000000006',
    'verification_node_name': 'node-a', 'verification_hostname': 'host-a',
    'archive_sha256': 'd' * 64, 'archive_bytes': 12,
    'server_config_sha256': 'e' * 64, 'server_config_bytes': 34,
    'phase_image': 'registry.example/phase@sha256:' + 'f' * 64,
}


def line(row):
    return json.dumps(row, sort_keys=True, separators=(',', ':'), allow_nan=False).encode() + b'\n'


def action_receipt(action, extra):
    return line({'schema': 'voice-nats-closed-copy-preflight-v1', 'state': 'COMPLETE',
        'action': action, **IDENTITY, **extra})


class ClosedCopyStreamTests(unittest.TestCase):
    def test_fixed_action_receipts_are_exact_and_identity_bound(self):
        extras = {
            'load-config': {'server_config_sha256': IDENTITY['server_config_sha256'],
                'server_config_bytes': IDENTITY['server_config_bytes']},
            'receive-archive': {'archive_sha256': IDENTITY['archive_sha256'],
                'archive_bytes': IDENTITY['archive_bytes'], 'manifest_sha256': '1' * 64},
            'broker-start': {'pid': 41, 'started_at_ns': 100, 'nats_binary_sha256': '2' * 64},
            'broker-ready': {'pid': 41, 'started_at_ns': 100, 'nats_binary_sha256': '2' * 64,
                'ready_at_ns': 110, 'readiness_status': 200},
            'read-jsz': {'pid': 41, 'started_at_ns': 100, 'nats_binary_sha256': '2' * 64,
                'output_name': 'read_jsz.output', 'output_sha256': '3' * 64, 'output_bytes': 7},
            'broker-stop': {'pid': 41, 'started_at_ns': 100, 'nats_binary_sha256': '2' * 64,
                'finished_at_ns': 120, 'exit_code': 0, 'stopped_at_ns': 121},
        }
        for action, extra in extras.items():
            with self.subTest(action=action):
                result = subject.receipt(action_receipt(action, extra), identity=IDENTITY, action=action)
                self.assertEqual(result['action'], action)

    def test_action_receipt_rejects_identity_status_and_shape_drift(self):
        raw = action_receipt('broker-start', {'pid': 41, 'started_at_ns': 100,
            'nats_binary_sha256': '2' * 64})
        parsed = json.loads(raw)
        for patch in ({'pod_uid': IDENTITY['job_uid']}, {'job_uid': IDENTITY['pod_uid']},
            {'state': 'STARTED'}, {'extra': True}):
            changed = dict(parsed); changed.update(patch)
            with self.subTest(patch=patch), self.assertRaises(subject.ClosedCopyStreamError):
                subject.receipt(line(changed), identity=IDENTITY, action='broker-start')
        with self.assertRaises(subject.ClosedCopyStreamError):
            subject.receipt(raw[:-1], identity=IDENTITY, action='broker-start')
        with self.assertRaises(subject.ClosedCopyStreamError):
            subject.receipt(raw, identity=IDENTITY, action='broker-status')

    def test_jsz_frames_return_only_canonical_identity_bound_tree(self):
        tree_bytes = b'{"accounts":[],"server_id":"test"}'
        digest = hashlib.sha256(tree_bytes).hexdigest()
        read_receipt = subject.receipt(action_receipt('read-jsz', {
            'pid': 41, 'started_at_ns': 100, 'nats_binary_sha256': '2' * 64,
            'output_name': 'read_jsz.output', 'output_sha256': digest,
            'output_bytes': len(tree_bytes)}), identity=IDENTITY, action='read-jsz')
        common = {'schema': 'voice-nats-closed-copy-readback-v1', **IDENTITY}
        data = {**common, 'kind': 'data', 'sequence': 0,
            'data': base64.b64encode(tree_bytes).decode('ascii')}
        complete = {**common, 'kind': 'complete', 'status': 'COMPLETE', 'frames': 1,
            'byte_count': len(tree_bytes), 'output_sha256': digest, 'receipt': read_receipt}
        raw = line(data) + line(complete)
        self.assertEqual(subject.consume_jsz(raw, identity=IDENTITY,
            read_receipt=read_receipt), {'accounts': [], 'server_id': 'test'})

    def test_jsz_frames_reject_gaps_mutation_duplicates_and_hash_mismatch(self):
        tree_bytes = b'{"server_id":"test"}'
        digest = hashlib.sha256(tree_bytes).hexdigest()
        read_receipt = subject.receipt(action_receipt('read-jsz', {
            'pid': 41, 'started_at_ns': 100, 'nats_binary_sha256': '2' * 64,
            'output_name': 'read_jsz.output', 'output_sha256': digest,
            'output_bytes': len(tree_bytes)}), identity=IDENTITY, action='read-jsz')
        common = {'schema': 'voice-nats-closed-copy-readback-v1', **IDENTITY}
        data = {**common, 'kind': 'data', 'sequence': 0,
            'data': base64.b64encode(tree_bytes).decode('ascii')}
        complete = {**common, 'kind': 'complete', 'status': 'COMPLETE', 'frames': 1,
            'byte_count': len(tree_bytes), 'output_sha256': digest, 'receipt': read_receipt}
        cases = []
        cases.append(line({**data, 'sequence': 1}) + line(complete))
        cases.append(line(data) + line({**complete, 'output_sha256': '4' * 64}))
        cases.append(line(data) + line(complete) + line(complete))
        cases.append(line(data) + line({**complete, 'pod_uid': IDENTITY['job_uid']}))
        cases.append(line(data).replace(b'"kind":"data"', b'"kind":"data","kind":"data"') + line(complete))
        for raw in cases:
            with self.subTest(raw=raw[:100]), self.assertRaises(subject.ClosedCopyStreamError):
                subject.consume_jsz(raw, identity=IDENTITY, read_receipt=read_receipt)


if __name__ == '__main__':
    unittest.main()
