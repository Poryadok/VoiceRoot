import base64
import copy
import hashlib
import json
import unittest

import census_stream as module
import rollout_census


IDENTITY = {
    'operation': '012345abcdef',
    'nonce': 'a' * 32,
    'pod_uid': '10000000-0000-0000-0000-000000000001',
    'source_pvc_uid': '20000000-0000-0000-0000-000000000002',
    'verification_pvc_uid': '30000000-0000-0000-0000-000000000003',
}


def snapshot():
    return {'schema': rollout_census.SCHEMA, 'account': 'A' + 'B' * 55,
        'streams': [{'name': 'chat_events', 'created': '2026-10-05T10:00:00Z',
            'config_sha256': 'a' * 64,
            'state': {'messages': 7, 'bytes': 70, 'first_seq': 1, 'last_seq': 7,
                      'consumer_count': 1}, 'observations': {}}],
        'consumers': [{'stream': 'chat_events', 'name': 'rt_realtime1',
            'durable': 'rt_realtime1', 'created': '2026-10-05T10:00:00Z',
            'config_sha256': 'b' * 64, 'inactive_threshold': 0,
            'delivered': {'consumer_seq': 2, 'stream_seq': 7},
            'ack_floor': {'consumer_seq': 1, 'stream_seq': 2},
            'num_ack_pending': 1, 'num_redelivered': 0, 'num_pending': 3,
            'observations': {'num_waiting': 0}}]}


def canonical(row):
    return json.dumps(row, sort_keys=True, separators=(',', ':'),
                      allow_nan=False).encode('utf-8')


def frames(*, row=None, receipt_changes=None, frame_changes=None):
    value = snapshot() if row is None else row
    payload = canonical(value)
    digest = hashlib.sha256(payload).hexdigest()
    receipt = {'schema': module.RECEIPT_SCHEMA, 'phase': 'census',
        'operation': IDENTITY['operation'], 'nonce': IDENTITY['nonce'],
        'pod_uid': IDENTITY['pod_uid'], 'source_pvc_uid': IDENTITY['source_pvc_uid'],
        'verification_pvc_uid': IDENTITY['verification_pvc_uid'],
        'archive_sha256': 'c' * 64, 'census_sha256': digest, 'status': 'COMPLETE'}
    receipt.update(receipt_changes or {})
    lines = []
    for sequence, start in enumerate(range(0, len(payload), module.CHUNK_BYTES)):
        chunk = payload[start:start + module.CHUNK_BYTES]
        line = {'schema': module.SCHEMA, **IDENTITY, 'kind': 'data',
                'sequence': sequence, 'data': base64.b64encode(chunk).decode('ascii')}
        if frame_changes and sequence == 0:
            line.update(frame_changes)
        lines.append(json.dumps(line, separators=(',', ':')).encode() + b'\n')
    end = {'schema': module.SCHEMA, **IDENTITY, 'kind': 'complete',
        'frames': len(lines), 'byte_count': len(payload), 'census_sha256': digest,
        'receipt': receipt}
    lines.append(json.dumps(end, separators=(',', ':')).encode() + b'\n')
    return lines


def split_arbitrarily(lines):
    raw = b''.join(lines)
    cuts = (1, 7, 65, 4097, len(raw) // 2, len(raw) - 3)
    result = []
    prior = 0
    for boundary in cuts:
        boundary = max(prior, min(boundary, len(raw)))
        if boundary > prior:
            result.append(raw[prior:boundary])
            prior = boundary
    if prior < len(raw):
        result.append(raw[prior:])
    return result


class CensusStreamTests(unittest.TestCase):
    def consume(self, chunks, **kwargs):
        return module.consume(chunks, **IDENTITY, **kwargs)

    def test_consumes_arbitrarily_split_framed_result_and_returns_validated_snapshot(self):
        result = self.consume(split_arbitrarily(frames()))
        self.assertEqual(result['census'], rollout_census.validate_snapshot(snapshot()))
        self.assertEqual(result['census_sha256'], hashlib.sha256(canonical(snapshot())).hexdigest())
        self.assertEqual(result['receipt']['status'], 'COMPLETE')

    def test_rejects_wrong_identity_sequence_duplicate_or_noncanonical_data(self):
        cases = [
            frames(frame_changes={'sequence': 2}),
            frames(frame_changes={'source_pvc_uid': '40000000-0000-0000-0000-000000000004'}),
            [frames()[0], frames()[0], frames()[-1]],
            frames(row=json.loads(canonical(snapshot()))),
        ]
        for case in cases[:3]:
            with self.subTest(case=case), self.assertRaises(module.CensusStreamError):
                self.consume(case)
        # A valid but non-canonical JSON object is hashed as sent and must be refused.
        payload = canonical(snapshot())
        changed = payload.replace(b'{', b'{ ', 1)
        wire = [json.dumps({'schema': module.SCHEMA, **IDENTITY, 'kind': 'data',
            'sequence': 0, 'data': base64.b64encode(changed).decode('ascii')},
            separators=(',', ':')).encode() + b'\n']
        digest = hashlib.sha256(changed).hexdigest()
        receipt = {'schema': module.RECEIPT_SCHEMA, 'phase': 'census', **IDENTITY,
            'archive_sha256': 'c' * 64, 'census_sha256': digest, 'status': 'COMPLETE'}
        wire.append(json.dumps({'schema': module.SCHEMA, **IDENTITY, 'kind': 'complete',
            'frames': 1, 'byte_count': len(changed), 'census_sha256': digest,
            'receipt': receipt}, separators=(',', ':')).encode() + b'\n')
        with self.assertRaises(module.CensusStreamError):
            self.consume(wire)

    def test_rejects_changed_receipt_hash_truncation_trailing_rows_and_oversize(self):
        valid = frames()
        bad_receipt = frames(receipt_changes={'archive_sha256': 'not-a-digest'})
        for case in (bad_receipt, valid[:-1], valid + [valid[-1]]):
            with self.subTest(case=case), self.assertRaises(module.CensusStreamError):
                self.consume(case)
        with self.assertRaises(module.CensusStreamError):
            self.consume(valid, max_bytes=1)

    def test_rejects_malformed_json_duplicate_fields_and_nonfinite_census(self):
        for raw in (b'{"x":1,"x":2}\n', b'{"x":NaN}\n', b'not-json\n'):
            with self.subTest(raw=raw), self.assertRaises(module.CensusStreamError):
                self.consume([raw])


if __name__ == '__main__':
    unittest.main()
