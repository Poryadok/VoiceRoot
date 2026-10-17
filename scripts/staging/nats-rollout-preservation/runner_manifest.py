"""Verify an independently pinned, immutable trusted-runner installation."""
import hashlib
import json
import os
from pathlib import Path
import re
import stat


SCHEMA = 'voice-nats-trusted-runner-manifest-v1'
PROTOCOL_SCHEMA = 'voice-staging-bridge-runner-request-v1'
REQUIRED_FILES = frozenset(('runner_service.py', 'runner_rpc.py', 'bridge_state.py', 'tokenless_job.py',
    'kube_json_api.py', 'runner_manifest.py', 'protocol-schema.json', 'runner.service',
    'runner.socket', 'nats-phase'))


class ManifestError(ValueError):
    pass


def _fail(code):
    raise ManifestError('runner_manifest_' + code)


def closure_digest(manifest):
    payload = {key: manifest[key] for key in ('runner_source_sha', 'protocol_schema', 'files', 'images')}
    canonical = json.dumps(payload, sort_keys=True, separators=(',', ':')).encode()
    return hashlib.sha256(canonical).hexdigest()


def _read_file(path, owner_uid, executable=False):
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    except OSError:
        _fail('file_unavailable')
    try:
        row = os.fstat(fd)
        mode = stat.S_IMODE(row.st_mode)
        if (not stat.S_ISREG(row.st_mode) or row.st_uid != owner_uid or row.st_nlink != 1
            or mode not in ((0o555,) if executable else (0o444,)) or not 0 < row.st_size <= 32 << 20):
            _fail('file_custody_invalid')
        content = bytearray()
        while len(content) <= row.st_size:
            part = os.read(fd, min(1 << 20, row.st_size + 1 - len(content)))
            if not part:
                break
            content.extend(part)
        after = os.fstat(fd)
        if (len(content) != row.st_size or (row.st_dev, row.st_ino, row.st_mode, row.st_uid,
            row.st_nlink, row.st_size, row.st_mtime_ns, row.st_ctime_ns) !=
            (after.st_dev, after.st_ino, after.st_mode, after.st_uid, after.st_nlink,
             after.st_size, after.st_mtime_ns, after.st_ctime_ns)):
            _fail('file_changed')
        return bytes(content)
    finally:
        os.close(fd)


def verify(root, manifest, *, owner_uid=0):
    root = Path(root)
    try:
        directory = root.lstat()
    except OSError:
        _fail('directory_unavailable')
    if (not stat.S_ISDIR(directory.st_mode) or directory.st_uid != owner_uid
        or stat.S_IMODE(directory.st_mode) != 0o555 or not re.fullmatch('[a-f0-9]{40}', root.name)):
        _fail('directory_custody_invalid')
    if (not isinstance(manifest, dict) or set(manifest) != {'schema', 'runner_source_sha', 'protocol_schema', 'files', 'images', 'closure_sha256'}
        or manifest.get('schema') != SCHEMA or manifest.get('protocol_schema') != PROTOCOL_SCHEMA
        or manifest.get('runner_source_sha') != root.name or not isinstance(manifest.get('files'), dict)
        or not manifest['files'] or not isinstance(manifest.get('images'), dict)
        or set(manifest['images']) != {'nats_phase'}):
        _fail('shape_invalid')
    if not REQUIRED_FILES <= set(manifest['files']):
        _fail('closure_incomplete')
    if (not isinstance(manifest.get('closure_sha256'), str)
        or not re.fullmatch('[a-f0-9]{64}', manifest['closure_sha256'])
        or manifest['closure_sha256'] != closure_digest(manifest)):
        _fail('closure_digest_invalid')
    image = manifest['images']['nats_phase']
    if not isinstance(image, str) or not re.fullmatch('[a-z0-9./_-]+@sha256:[a-f0-9]{64}', image):
        _fail('image_unpinned')
    if len(manifest['files']) > 256:
        _fail('file_count_invalid')
    for relative, wanted in manifest['files'].items():
        path = Path(relative)
        if (not isinstance(relative, str) or path.is_absolute() or '..' in path.parts
            or not path.parts or not isinstance(wanted, str) or not re.fullmatch('[a-f0-9]{64}', wanted)):
            _fail('file_entry_invalid')
        candidate = root / path
        for parent in (candidate.parent, *candidate.parent.parents):
            if parent == root.parent:
                break
            row = parent.lstat()
            if not stat.S_ISDIR(row.st_mode) or stat.S_ISLNK(row.st_mode) or row.st_uid != owner_uid or stat.S_IMODE(row.st_mode) != 0o555:
                _fail('parent_custody_invalid')
        content = _read_file(candidate, owner_uid, executable=relative in ('runner_service.py', 'nats-phase'))
        if hashlib.sha256(content).hexdigest() != wanted:
            _fail('file_hash_mismatch')
    return {'runner_source_sha': root.name, 'protocol_schema': PROTOCOL_SCHEMA,
            'closure_sha256': manifest['closure_sha256'], 'file_count': len(manifest['files']),
            'images': {'nats_phase': image}}


def load(root, *, owner_uid=0):
    root = Path(root)
    raw = _read_file(root / 'runner-manifest.json', owner_uid)
    try:
        manifest = json.loads(raw, object_pairs_hook=_unique_pairs)
    except (ValueError, TypeError):
        _fail('json_invalid')
    return verify(root, manifest, owner_uid=owner_uid)


def _unique_pairs(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            _fail('duplicate_field')
        result[key] = value
    return result
