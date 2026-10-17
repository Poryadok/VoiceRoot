"""Bounded, identity-bound census readback over Kubernetes exec stdout."""
from __future__ import annotations

import base64
import hashlib
import json
import re

import rollout_census


SCHEMA = 'voice-nats-census-readback-v1'
RECEIPT_SCHEMA = 'voice-nats-verification-v1'
MAX_CENSUS_BYTES = rollout_census.MAX_BYTES
CHUNK_BYTES = 48 << 10
MAX_FRAME_BYTES = 96 << 10
MAX_FRAMES = (MAX_CENSUS_BYTES + CHUNK_BYTES - 1) // CHUNK_BYTES
MAX_TRANSPORT_BYTES = MAX_CENSUS_BYTES * 2 + (MAX_FRAMES * 1024) + (1 << 20)
IDENTITY_FIELDS = ('operation', 'nonce', 'pod_uid', 'source_pvc_uid', 'verification_pvc_uid')
DATA_FIELDS = frozenset(('schema', *IDENTITY_FIELDS, 'kind', 'sequence', 'data'))
COMPLETE_FIELDS = frozenset(('schema', *IDENTITY_FIELDS, 'kind', 'frames', 'byte_count',
                             'census_sha256', 'receipt'))
RECEIPT_FIELDS = frozenset(('schema', 'phase', 'operation', 'nonce', 'pod_uid',
                            'source_pvc_uid', 'verification_pvc_uid', 'archive_sha256',
                            'census_sha256', 'status'))
UUID_RE = re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')


class CensusStreamError(ValueError):
    pass


def _fail(code):
    raise CensusStreamError('census_stream_' + code) from None


def _unique_pairs(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            _fail('duplicate_json_key')
        value[key] = item
    return value


def _json_line(raw):
    if len(raw) > MAX_FRAME_BYTES or not raw.endswith(b'\n') or raw.count(b'\n') != 1:
        _fail('frame_invalid')
    try:
        return json.loads(raw[:-1], object_pairs_hook=_unique_pairs,
                          parse_constant=lambda _: _fail('nonfinite_json'))
    except CensusStreamError:
        raise
    except (ValueError, TypeError, UnicodeError, RecursionError):
        _fail('frame_invalid')


def _identity(row, expected):
    return (row.get('schema') == SCHEMA and
            all(row.get(field) == expected[field] for field in IDENTITY_FIELDS))


def _validate_receipt(receipt, expected, digest):
    if (type(receipt) is not dict or set(receipt) != RECEIPT_FIELDS
        or receipt.get('schema') != RECEIPT_SCHEMA or receipt.get('phase') != 'census'
        or receipt.get('status') != 'COMPLETE'
        or any(receipt.get(field) != expected[field] for field in IDENTITY_FIELDS)
        or not isinstance(receipt.get('archive_sha256'), str)
        or not re.fullmatch(r'[a-f0-9]{64}', receipt['archive_sha256'])
        or receipt.get('census_sha256') != digest):
        _fail('receipt_invalid')


def consume(chunks, *, operation, nonce, pod_uid, source_pvc_uid, verification_pvc_uid,
            max_bytes=MAX_CENSUS_BYTES):
    """Consume raw stdout chunks, validating framed census bytes and COMPLETE receipt.

    The input chunks may split or combine JSONL records arbitrarily, as the
    Kubernetes WebSocket message boundaries do not align with helper frames.
    """
    expected = {'operation': operation, 'nonce': nonce, 'pod_uid': pod_uid,
                'source_pvc_uid': source_pvc_uid, 'verification_pvc_uid': verification_pvc_uid}
    if (not isinstance(operation, str) or not re.fullmatch(r'[a-f0-9]{12}', operation)
        or not isinstance(nonce, str) or not re.fullmatch(r'[a-f0-9]{32}', nonce)
        or not isinstance(pod_uid, str) or not UUID_RE.fullmatch(pod_uid)
        or not isinstance(source_pvc_uid, str) or not UUID_RE.fullmatch(source_pvc_uid)
        or not isinstance(verification_pvc_uid, str) or not UUID_RE.fullmatch(verification_pvc_uid)
        or source_pvc_uid == verification_pvc_uid or type(max_bytes) is not int
        or not 0 < max_bytes <= MAX_CENSUS_BYTES):
        _fail('identity_invalid')

    pending = bytearray()
    census_bytes = bytearray()
    transport_bytes = 0
    sequence = 0
    terminal = None

    def accept_line(raw):
        nonlocal sequence, terminal
        if terminal is not None:
            _fail('trailing_output')
        row = _json_line(raw)
        if type(row) is not dict or not _identity(row, expected):
            _fail('frame_identity_invalid')
        kind = row.get('kind')
        if kind == 'data':
            if (set(row) != DATA_FIELDS or type(row.get('sequence')) is not int
                or row['sequence'] != sequence or not isinstance(row.get('data'), str)
                or not row['data']):
                _fail('data_frame_invalid')
            try:
                encoded = row['data'].encode('ascii')
                data = base64.b64decode(encoded, validate=True)
            except (ValueError, UnicodeError):
                _fail('data_frame_invalid')
            if (not data or len(data) > CHUNK_BYTES
                or base64.b64encode(data) != encoded or sequence >= MAX_FRAMES):
                _fail('data_frame_invalid')
            if len(census_bytes) + len(data) > max_bytes:
                _fail('census_too_large')
            census_bytes.extend(data)
            sequence += 1
            return
        if (kind != 'complete' or set(row) != COMPLETE_FIELDS
            or type(row.get('frames')) is not int or row['frames'] != sequence
            or type(row.get('byte_count')) is not int or row['byte_count'] != len(census_bytes)
            or not isinstance(row.get('census_sha256'), str)
            or not re.fullmatch(r'[a-f0-9]{64}', row['census_sha256']) or sequence == 0):
            _fail('completion_invalid')
        digest = hashlib.sha256(census_bytes).hexdigest()
        if digest != row['census_sha256']:
            _fail('census_digest_mismatch')
        _validate_receipt(row['receipt'], expected, digest)
        terminal = row

    for chunk in chunks:
        if type(chunk) is not bytes or not chunk:
            _fail('transport_invalid')
        transport_bytes += len(chunk)
        if transport_bytes > MAX_TRANSPORT_BYTES:
            _fail('transport_too_large')
        pending.extend(chunk)
        while True:
            boundary = pending.find(b'\n')
            if boundary < 0:
                if len(pending) > MAX_FRAME_BYTES:
                    _fail('frame_too_large')
                break
            if boundary + 1 > MAX_FRAME_BYTES:
                _fail('frame_too_large')
            line = bytes(pending[:boundary + 1])
            del pending[:boundary + 1]
            accept_line(line)
    if pending or terminal is None:
        _fail('stream_truncated')
    if hashlib.sha256(census_bytes).hexdigest() != terminal['census_sha256']:
        _fail('census_digest_mismatch')
    try:
        census = json.loads(bytes(census_bytes), object_pairs_hook=_unique_pairs,
                            parse_constant=lambda _: _fail('nonfinite_census'))
        canonical = json.dumps(census, sort_keys=True, separators=(',', ':'),
                               allow_nan=False).encode('utf-8')
    except CensusStreamError:
        raise
    except (ValueError, TypeError, UnicodeError, RecursionError):
        _fail('census_invalid')
    if canonical != census_bytes:
        _fail('census_not_canonical')
    try:
        census = rollout_census.validate_snapshot(census)
    except rollout_census.CensusError:
        _fail('census_invalid')
    return {'receipt': terminal['receipt'], 'census': census,
            'census_sha256': terminal['census_sha256'], 'byte_count': len(census_bytes)}
