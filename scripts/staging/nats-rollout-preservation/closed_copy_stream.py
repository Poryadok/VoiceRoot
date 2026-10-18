"""Closed-copy preflight receipt and bounded JSZ frame validation."""
from __future__ import annotations

import base64
import hashlib
import json
import re


MAX_LINE_BYTES = 96 << 10
MAX_TREE_BYTES = 64 << 20
FRAME_BYTES = 48 << 10
IDENTITY_FIELDS = frozenset((
    'operation', 'nonce', 'verification_attempt', 'pod_uid', 'job_uid',
    'source_pvc_uid', 'source_pv_uid', 'source_node_name', 'source_hostname',
    'verification_pvc_uid', 'verification_pv_uid', 'verification_node_name',
    'verification_hostname', 'archive_sha256', 'archive_bytes',
    'server_config_sha256', 'server_config_bytes', 'phase_image',
))
RECEIPT_FIELDS = {
    'load-config': frozenset(('server_config_sha256', 'server_config_bytes')),
    'receive-archive': frozenset(('archive_sha256', 'archive_bytes', 'manifest_sha256')),
    'broker-start': frozenset(('pid', 'started_at_ns', 'nats_binary_sha256')),
    'broker-ready': frozenset(('pid', 'started_at_ns', 'nats_binary_sha256',
        'ready_at_ns', 'readiness_status')),
    'read-jsz': frozenset(('pid', 'started_at_ns', 'nats_binary_sha256',
        'output_name', 'output_sha256', 'output_bytes')),
    'broker-stop': frozenset(('pid', 'started_at_ns', 'nats_binary_sha256',
        'finished_at_ns', 'exit_code', 'stopped_at_ns')),
}


class ClosedCopyStreamError(ValueError):
    pass


def _fail():
    raise ClosedCopyStreamError('closed_copy_receipt_invalid')


def _pairs(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            _fail()
        result[key] = value
    return result


def _json_line(raw: bytes) -> tuple[dict, bytes]:
    if (not isinstance(raw, bytes) or not 1 <= len(raw) <= MAX_LINE_BYTES
        or not raw.endswith(b'\n') or raw.count(b'\n') != 1):
        _fail()
    try:
        encoded = raw[:-1]
        row = json.loads(encoded.decode('utf-8'), object_pairs_hook=_pairs,
            parse_constant=lambda value: (_ for _ in ()).throw(ValueError(value)))
        canonical = json.dumps(row, sort_keys=True, separators=(',', ':'),
            ensure_ascii=False, allow_nan=False).encode('utf-8')
    except (UnicodeDecodeError, ValueError, TypeError, RecursionError):
        _fail()
    if not isinstance(row, dict) or canonical != encoded:
        _fail()
    return row, encoded


def _identity(expected: object) -> dict:
    if not isinstance(expected, dict) or set(expected) != IDENTITY_FIELDS:
        _fail()
    for key in ('operation', 'nonce', 'verification_attempt'):
        pattern = {'operation': r'[a-f0-9]{12}', 'nonce': r'[a-f0-9]{32}',
            'verification_attempt': r'[a-f0-9]{8}'}[key]
        if not isinstance(expected[key], str) or not re.fullmatch(pattern, expected[key]):
            _fail()
    for key in ('pod_uid', 'job_uid', 'source_pvc_uid', 'source_pv_uid',
        'verification_pvc_uid', 'verification_pv_uid'):
        if not isinstance(expected[key], str) or not re.fullmatch(
            r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', expected[key]):
            _fail()
    for key in ('source_node_name', 'source_hostname', 'verification_node_name', 'verification_hostname'):
        if not isinstance(expected[key], str) or not expected[key]:
            _fail()
    for key in ('archive_sha256', 'server_config_sha256'):
        if not isinstance(expected[key], str) or not re.fullmatch(r'[a-f0-9]{64}', expected[key]):
            _fail()
    for key, maximum in (('archive_bytes', (48 << 30) + (262144 * 1024) + (1 << 20),
        ), ('server_config_bytes', 256 << 10)):
        if type(expected[key]) is not int or not 0 < expected[key] <= maximum:
            _fail()
    if (not isinstance(expected['phase_image'], str)
        or not re.fullmatch(r'[a-z0-9./_-]+@sha256:[a-f0-9]{64}', expected['phase_image'])):
        _fail()
    return dict(expected)


def identity_from_request(request: dict, binding: dict, pod_uid: str,
                          job_uid: str, phase_image: str) -> dict:
    try:
        import verification_job
        row = verification_job.validate_request(request)
        storage = verification_job.validate_closed_copy_binding(row, binding)
        if row.get('phase') != 'closed-copy-preflight':
            _fail()
        return _identity({
            'operation': row['operation'], 'nonce': row['nonce'],
            'verification_attempt': storage['verification_attempt'],
            'pod_uid': pod_uid, 'job_uid': job_uid,
            'source_pvc_uid': storage['source_uid'], 'source_pv_uid': storage['source_pv_uid'],
            'source_node_name': storage['source_node_name'], 'source_hostname': storage['source_hostname'],
            'verification_pvc_uid': storage['verification_uid'],
            'verification_pv_uid': storage['verification_pv_uid'],
            'verification_node_name': storage['verification_node_name'],
            'verification_hostname': storage['verification_hostname'],
            'archive_sha256': storage['archive_sha256'], 'archive_bytes': storage['archive_bytes'],
            'server_config_sha256': storage['server_config_sha256'],
            'server_config_bytes': storage['server_config_bytes'],
            'phase_image': verification_job.validate_image(phase_image),
        })
    except (ImportError, KeyError, TypeError, ValueError):
        _fail()


def receipt(raw: bytes, *, identity: dict, action: str) -> dict:
    expected = _identity(identity)
    if action not in RECEIPT_FIELDS:
        _fail()
    row, _ = _json_line(raw)
    if (set(row) != IDENTITY_FIELDS | RECEIPT_FIELDS[action] | {
        'schema', 'state', 'action'} or row.get('schema') != 'voice-nats-closed-copy-preflight-v1'
        or row.get('state') != 'COMPLETE' or row.get('action') != action
        or any(row.get(key) != value for key, value in expected.items())):
        _fail()
    for key in ('manifest_sha256', 'nats_binary_sha256', 'output_sha256'):
        if key in row and (not isinstance(row[key], str) or not re.fullmatch(r'[a-f0-9]{64}', row[key])):
            _fail()
    for key in ('archive_bytes', 'server_config_bytes', 'pid', 'started_at_ns',
        'ready_at_ns', 'finished_at_ns', 'stopped_at_ns', 'output_bytes'):
        if key in row and (type(row[key]) is not int or row[key] <= 0):
            _fail()
    if 'exit_code' in row and row['exit_code'] != 0:
        _fail()
    if 'readiness_status' in row and row['readiness_status'] != 200:
        _fail()
    if 'output_name' in row and row['output_name'] != 'read_jsz.output':
        _fail()
    if 'output_bytes' in row and row['output_bytes'] > MAX_TREE_BYTES:
        _fail()
    return row


def consume_jsz(raw: bytes, *, identity: dict, read_receipt: dict) -> dict:
    expected = _identity(identity)
    if not isinstance(raw, bytes) or len(raw) > MAX_TREE_BYTES * 2:
        _fail()
    data = bytearray()
    lines = raw.splitlines(keepends=True)
    if not lines or any(len(line) > MAX_LINE_BYTES for line in lines):
        _fail()
    complete = None
    short_chunk_seen = False
    for index, line in enumerate(lines):
        row, _ = _json_line(line)
        common = {'schema': 'voice-nats-closed-copy-readback-v1', **expected}
        if any(row.get(key) != value for key, value in common.items()):
            _fail()
        if row.get('kind') == 'data':
            if (complete is not None or short_chunk_seen
                or set(row) != set(common) | {'kind', 'sequence', 'data'}):
                _fail()
            if type(row['sequence']) is not int or row['sequence'] != index:
                _fail()
            if not isinstance(row['data'], str):
                _fail()
            try:
                chunk = base64.b64decode(row['data'], validate=True)
            except (ValueError, base64.binascii.Error):
                _fail()
            if not 0 < len(chunk) <= FRAME_BYTES:
                _fail()
            if len(chunk) < FRAME_BYTES:
                short_chunk_seen = True
            data.extend(chunk)
            if len(data) > MAX_TREE_BYTES:
                _fail()
        elif row.get('kind') == 'complete':
            if complete is not None or index != len(lines) - 1 or set(row) != set(common) | {
                'kind', 'status', 'frames', 'byte_count', 'output_sha256', 'receipt'}:
                _fail()
            if (row.get('status') != 'COMPLETE' or type(row.get('frames')) is not int
                or row['frames'] != index or type(row.get('byte_count')) is not int
                or row['byte_count'] != len(data) or not isinstance(row.get('output_sha256'), str)
                or not re.fullmatch(r'[a-f0-9]{64}', row['output_sha256'])
                or hashlib.sha256(data).hexdigest() != row['output_sha256']):
                _fail()
            try:
                nested = json.dumps(row.get('receipt'), sort_keys=True, separators=(',', ':'),
                    ensure_ascii=False, allow_nan=False).encode('utf-8') + b'\n'
            except (TypeError, ValueError):
                _fail()
            if receipt(nested, identity=expected, action='read-jsz') != read_receipt:
                _fail()
            if (read_receipt.get('output_bytes') != len(data)
                or read_receipt.get('output_sha256') != row['output_sha256']):
                _fail()
            complete = row
        else:
            _fail()
    if complete is None or not data:
        _fail()
    try:
        tree = json.loads(bytes(data).decode('utf-8'), object_pairs_hook=_pairs,
            parse_constant=lambda value: (_ for _ in ()).throw(ValueError(value)))
        canonical = json.dumps(tree, sort_keys=True, separators=(',', ':'),
            ensure_ascii=False, allow_nan=False).encode('utf-8')
    except (UnicodeDecodeError, ValueError, TypeError, RecursionError):
        _fail()
    if not isinstance(tree, dict) or canonical != bytes(data):
        _fail()
    return tree
