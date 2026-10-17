"""Bounded, replay-safe archive transfer from one owned Kubernetes Pod log.

The helper emits UTF-8 JSONL frames because the Kubernetes ``pods/log``
subresource is a text log API. Archive bytes are base64 encoded in bounded
frames; the trusted runner reconstructs and verifies the canonical tar before
making it visible to transaction state.
"""
from __future__ import annotations

import base64
import hashlib
import json
import os
from pathlib import Path
import re
import stat


SCHEMA = 'voice-nats-archive-stream-v1'
MAX_SOURCE_BYTES = 48 << 30
MAX_ENTRIES = 262144
# POSIX tar adds headers/padding for each bounded native entry. Keep the
# transport cap above the canonical payload limit without making it unbounded.
MAX_ARCHIVE_BYTES = MAX_SOURCE_BYTES + (MAX_ENTRIES * 1024) + (1 << 20)
MAX_FRAME_BYTES = 96 << 10
MAX_MANIFEST_BYTES = 512 << 20
FIELDS = frozenset(('schema', 'operation', 'nonce', 'pod_uid', 'source_pvc_uid', 'source_pv_uid',
                    'kind', 'sequence', 'data'))
MANIFEST_FIELDS = FIELDS
END_FIELDS = frozenset(('schema', 'operation', 'nonce', 'pod_uid', 'source_pvc_uid', 'source_pv_uid', 'kind', 'frames',
                        'manifest_frames', 'archive_sha256', 'manifest_sha256'))


class ArchiveStreamError(ValueError):
    pass


def _fail(code):
    raise ArchiveStreamError('archive_stream_' + code)


def _frame(raw, *, operation, nonce, pod_uid, source_pvc_uid, source_pv_uid,
           sequence, manifest_sequence):
    if len(raw) > MAX_FRAME_BYTES or not raw.endswith(b'\n') or raw.count(b'\n') != 1:
        _fail('frame_invalid')
    try:
        row = json.loads(raw[:-1], object_pairs_hook=_unique_pairs)
    except (ValueError, TypeError, UnicodeError):
        _fail('frame_invalid')
    if not isinstance(row, dict):
        _fail('frame_invalid')
    if row.get('kind') in ('data', 'manifest'):
        expected_sequence = manifest_sequence if row['kind'] == 'manifest' else sequence
        if (set(row) != FIELDS or row.get('schema') != SCHEMA or row.get('operation') != operation
            or row.get('nonce') != nonce or row.get('pod_uid') != pod_uid
            or row.get('source_pvc_uid') != source_pvc_uid or row.get('source_pv_uid') != source_pv_uid
            or type(row.get('sequence')) is not int or row['sequence'] != expected_sequence
            or not isinstance(row.get('data'), str) or not row['data']):
            _fail('frame_identity_invalid')
        if row['kind'] not in ('data', 'manifest'):
            _fail('frame_kind_invalid')
        try:
            encoded = row['data'].encode('ascii')
            chunk = base64.b64decode(encoded, validate=True)
        except (ValueError, UnicodeError):
            _fail('frame_data_invalid')
        if not chunk or len(chunk) > 64 << 10 or base64.b64encode(chunk) != encoded:
            _fail('frame_data_invalid')
        return row, chunk
    if (set(row) != END_FIELDS or row.get('schema') != SCHEMA or row.get('operation') != operation
        or row.get('nonce') != nonce or row.get('pod_uid') != pod_uid
        or row.get('source_pvc_uid') != source_pvc_uid or row.get('source_pv_uid') != source_pv_uid
        or row.get('kind') != 'complete' or type(row.get('frames')) is not int
        or row['frames'] != sequence or type(row.get('manifest_frames')) is not int
        or not isinstance(row.get('archive_sha256'), str)
        or not re.fullmatch('[a-f0-9]{64}', row['archive_sha256'])
        or not isinstance(row.get('manifest_sha256'), str)
        or not re.fullmatch('[a-f0-9]{64}', row['manifest_sha256'])):
        _fail('completion_identity_invalid')
    return row, None


def consume(lines, artifact, *, operation, nonce, pod_uid, source_pvc_uid, source_pv_uid,
            persist_intent, persist_result,
            max_bytes=MAX_ARCHIVE_BYTES, verify_archive=None, allow_existing=False):
    """Reconstruct one already-completed archive Job's stream into private custody.

    Callers may invoke this again only for the same durable Job/Pod UID. It never
    creates or starts a Job. A partial stream is discarded and has no artifact
    at the requested path; the caller must retain the journaled unknown state.
    """
    if (not isinstance(operation, str) or not re.fullmatch('[a-f0-9]{12}', operation)
        or not isinstance(nonce, str) or not re.fullmatch('[a-f0-9]{32}', nonce)
        or not isinstance(pod_uid, str) or not re.fullmatch('[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', pod_uid)
        or not isinstance(source_pvc_uid, str) or not re.fullmatch('[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', source_pvc_uid)
        or not isinstance(source_pv_uid, str) or not re.fullmatch('[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', source_pv_uid)
        or type(max_bytes) is not int or not 0 < max_bytes <= MAX_ARCHIVE_BYTES):
        _fail('identity_invalid')
    target = Path(artifact)
    parent = target.parent.lstat()
    if (not stat.S_ISDIR(parent.st_mode) or stat.S_ISLNK(parent.st_mode)
        or parent.st_uid != os.geteuid() or stat.S_IMODE(parent.st_mode) != 0o700):
        _fail('directory_custody_invalid')
    if type(allow_existing) is not bool:
        _fail('recovery_mode_invalid')
    pending = target.with_name('.' + target.name + '.' + nonce + '.stream')
    try:
        stale_fd = os.open(pending, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    except FileNotFoundError:
        stale_fd = None
    if stale_fd is not None:
        try:
            stale = os.fstat(stale_fd)
            if (not stat.S_ISREG(stale.st_mode) or stale.st_uid != os.geteuid()
                or stat.S_IMODE(stale.st_mode) != 0o600 or stale.st_nlink != 1
                or stale.st_size > MAX_ARCHIVE_BYTES):
                _fail('stale_stream_custody_invalid')
        finally:
            os.close(stale_fd)
        pending.unlink()
        directory = os.open(target.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try: os.fsync(directory)
        finally: os.close(directory)
    fd = os.open(pending, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    digest = hashlib.sha256()
    total = 0
    sequence = 0
    manifest_sequence = 0
    manifest_raw = bytearray()
    manifest_started = False
    buffer = bytearray()
    manifest = None
    ended = False
    try:
        persist_intent({'state': 'ARCHIVE_STREAM_INTENT', 'operation': operation,
                        'nonce': nonce, 'pod_uid': pod_uid, 'source_pvc_uid': source_pvc_uid,
                        'source_pv_uid': source_pv_uid, 'artifact': target.name})
        with os.fdopen(fd, 'wb') as output:
            for part in lines:
                if not isinstance(part, (bytes, bytearray)):
                    _fail('transport_chunk_invalid')
                buffer.extend(part)
                if len(buffer) > MAX_FRAME_BYTES and b'\n' not in buffer:
                    _fail('frame_too_large')
                while b'\n' in buffer:
                    boundary = buffer.index(b'\n') + 1
                    raw = bytes(buffer[:boundary]); del buffer[:boundary]
                    if ended:
                        _fail('trailing_data')
                    row, chunk = _frame(raw, operation=operation, nonce=nonce,
                        pod_uid=pod_uid, source_pvc_uid=source_pvc_uid, source_pv_uid=source_pv_uid,
                        sequence=sequence, manifest_sequence=manifest_sequence)
                    if chunk is None:
                        ended = True
                        if digest.hexdigest() != row['archive_sha256']:
                            _fail('archive_digest_mismatch')
                        if (row['manifest_frames'] != manifest_sequence
                            or hashlib.sha256(manifest_raw).hexdigest() != row['manifest_sha256']
                            or not manifest_raw or len(manifest_raw) > MAX_MANIFEST_BYTES):
                            _fail('manifest_digest_mismatch')
                        try:
                            manifest = json.loads(manifest_raw, object_pairs_hook=_unique_pairs)
                        except (ValueError, TypeError, UnicodeError):
                            _fail('manifest_invalid')
                        if not isinstance(manifest, dict):
                            _fail('manifest_invalid')
                    elif row['kind'] == 'manifest':
                        manifest_started = True
                        manifest_sequence += 1
                        manifest_raw.extend(chunk)
                        if len(manifest_raw) > MAX_MANIFEST_BYTES:
                            _fail('manifest_too_large')
                    else:
                        if manifest_started:
                            _fail('archive_frame_after_manifest')
                        total += len(chunk)
                        if total > max_bytes:
                            _fail('archive_size_limit')
                        output.write(chunk); digest.update(chunk); sequence += 1
            output.flush()
            os.fsync(output.fileno())
        if buffer or not ended or manifest is None or total == 0:
            _fail('stream_incomplete')
        if verify_archive is None or verify_archive(pending, manifest) is not True:
            _fail('canonical_archive_verification_failed')
        after = pending.lstat()
        if (not stat.S_ISREG(after.st_mode) or after.st_uid != os.geteuid()
            or stat.S_IMODE(after.st_mode) != 0o600 or after.st_nlink != 1 or after.st_size != total):
            _fail('artifact_custody_invalid')
        # The destination is intentionally create-only. Reconciliation uses a
        # fresh nonce-scoped scratch file and compares its proven digest before
        # any caller adopts it as the canonical operation artifact.
        try:
            os.link(pending, target, follow_symlinks=False)
            pending.unlink()
        except FileExistsError:
            if not allow_existing:
                raise
            current = target.lstat()
            if (not stat.S_ISREG(current.st_mode) or current.st_uid != os.geteuid()
                or stat.S_IMODE(current.st_mode) != 0o600 or current.st_nlink not in (1, 2)
                or current.st_size != total or verify_archive(target, manifest) is not True):
                _fail('existing_artifact_recovery_mismatch')
            if current.st_nlink == 2:
                linked = pending.lstat()
                if (not stat.S_ISREG(linked.st_mode) or linked.st_dev != current.st_dev
                    or linked.st_ino != current.st_ino):
                    _fail('existing_artifact_recovery_mismatch')
            pending.unlink()
        folder = os.open(parent_path := target.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try: os.fsync(folder)
        finally: os.close(folder)
        result = {'state': 'ARCHIVE_RESULT_READBACK', 'operation': operation, 'nonce': nonce,
                  'pod_uid': pod_uid, 'source_pvc_uid': source_pvc_uid, 'source_pv_uid': source_pv_uid,
                  'artifact': target.name, 'bytes': total,
                  'archive_sha256': digest.hexdigest(),
                  'manifest_sha256': hashlib.sha256(json.dumps(manifest, sort_keys=True,
                      separators=(',', ':')).encode()).hexdigest()}
        persist_result(result)
        return {'artifact': target, 'manifest': manifest, **result}
    except BaseException:
        try: pending.unlink(missing_ok=True)
        except OSError: pass
        raise


def _unique_pairs(items):
    row = {}
    for key, value in items:
        if key in row:
            _fail('duplicate_field')
        row[key] = value
    return row
