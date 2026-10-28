"""GitHub encrypted media backup custody and verified extraction."""
import importlib.util
from datetime import datetime, timezone
import hashlib
import hmac
import io
import json
import os
from pathlib import Path
import re
import shutil
import stat
import struct
import tarfile
import tempfile
import time
import zipfile

import story_media_backup as backup

API = 'https://api.github.com/repos/Poryadok/VoiceRoot/actions/artifacts/'
spec = importlib.util.spec_from_file_location('voice_staging_github_custody',
    Path(__file__).resolve().parents[1] / 'staging/nats-rollout-preservation/github_custody.py')
transport = importlib.util.module_from_spec(spec)
spec.loader.exec_module(transport)


class CustodyError(Exception):
    pass


def verify_bundle(path, key, expected_binding):
    """Authenticate the exact private bundle before an uploader copies it."""
    try:
        path = Path(path)
        deadline = time.monotonic() + transport.TIME_LIMIT
        with backup.private_directory(path.parent) as dirfd:
            fd = os.open(path.name, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=dirfd)
            with os.fdopen(fd, 'rb') as stream:
                observed = os.fstat(stream.fileno())
                if (not stat.S_ISREG(observed.st_mode) or observed.st_uid != os.geteuid()
                        or observed.st_mode & 0o077 or observed.st_nlink != 1):
                    _fail()
                before = _bundle_identity(stream, deadline)
                _inspect_bundle(stream, key, expected_binding, deadline)
                after = _bundle_identity(stream, deadline)
                current = os.stat(path.name, dir_fd=dirfd, follow_symlinks=False)
                if (before != after or (observed.st_dev, observed.st_ino) != (current.st_dev, current.st_ino)):
                    _fail()
        return {'bytes': after[0], 'sha256': after[1]}
    except Exception:
        raise CustodyError('media backup custody rejected') from None


CHUNK = 1024 * 1024
MAX_BYTES = 64 * 1024 ** 3
TAR_MEMBER_MAX = 8 ** 11 - 1
MANIFEST_LIMIT = 1024 * 1024
MEMBER_NAMES = tuple(name + '.enc' for name in sorted(backup.MEMBERS))


def _fail():
    raise CustodyError('media backup custody rejected')


def _canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True,
                      allow_nan=False).encode('ascii')


def _unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            _fail()
        result[key] = value
    return result


def _manifest(raw, key, expected_binding):
    if not 0 < len(raw) <= MANIFEST_LIMIT:
        _fail()
    value = json.loads(raw, object_pairs_hook=_unique_object)
    if (not isinstance(value, dict) or set(value) != {'format', 'binding', 'files', 'mac'}
            or value['format'] != 1 or value['binding'] != expected_binding
            or not isinstance(value['files'], dict) or set(value['files']) != backup.MEMBERS
            or not isinstance(value['mac'], str) or len(key) != 32):
        _fail()
    unsigned = {name: value[name] for name in ('format', 'binding', 'files')}
    if not hmac.compare_digest(backup.manifest_mac(key, unsigned), value['mac']):
        _fail()
    if raw != _canonical(value):
        _fail()
    return value


def _remaining(deadline):
    if deadline is not None and time.monotonic() > deadline:
        _fail()


def _inspect_bundle(stream, key, expected_binding, deadline=None):
    stream.seek(0)
    offsets = {}
    manifest = None
    for name in ('manifest.json', *MEMBER_NAMES):
        _remaining(deadline)
        header = stream.read(512)
        if len(header) != 512:
            _fail()
        name_field = header[:100]
        actual_name, _, suffix = name_field.partition(b'\0')
        size_field = header[124:136].strip(b'\0 ')
        checksum_field = header[148:156].strip(b'\0 ')
        if (actual_name != name.encode('ascii') or suffix.strip(b'\0')
                or header[156:157] not in (b'0', b'\0') or header[157:257].strip(b'\0')
                or header[257:265] != b'ustar\x0000' or header[345:500].strip(b'\0')
                or not re.fullmatch(rb'[0-7]+', size_field)
                or not re.fullmatch(rb'[0-7]+', checksum_field)
                or int(checksum_field, 8) != sum(header[:148]) + 8 * 32 + sum(header[156:])):
            _fail()
        size = int(size_field, 8)
        if not 0 < size <= (MANIFEST_LIMIT if name == 'manifest.json' else TAR_MEMBER_MAX):
            _fail()
        expected_header = tarfile.TarInfo(name)
        expected_header.mode = 0o600
        expected_header.size = size
        if not hmac.compare_digest(header, expected_header.tobuf(format=tarfile.USTAR_FORMAT)):
            _fail()
        offsets[name] = (stream.tell(), size)
        sha = hashlib.sha256(); mac = hmac.new(key, digestmod=hashlib.sha256)
        collected = bytearray() if name == 'manifest.json' else None
        remaining = size
        while remaining:
            _remaining(deadline)
            chunk = stream.read(min(CHUNK, remaining))
            if not chunk:
                _fail()
            remaining -= len(chunk); sha.update(chunk); mac.update(chunk)
            if collected is not None:
                collected.extend(chunk)
        padding = stream.read((-size) % 512)
        if len(padding) != (-size) % 512 or padding.strip(b'\0'):
            _fail()
        if name == 'manifest.json':
            manifest = _manifest(bytes(collected), key, expected_binding)
        elif {'sha256': sha.hexdigest(), 'hmac': mac.hexdigest()} != manifest['files'][name[:-4]]:
            _fail()
    tail_bytes = 0
    for chunk in iter(lambda: stream.read(CHUNK), b''):
        _remaining(deadline)
        tail_bytes += len(chunk)
        if chunk.strip(b'\0'):
            _fail()
    if tail_bytes < 1024 or tail_bytes % 512:
        _fail()
    return manifest, offsets


def _bundle_identity(stream, deadline=None):
    sha = hashlib.sha256(); count = 0
    stream.seek(0)
    for chunk in iter(lambda: stream.read(CHUNK), b''):
        _remaining(deadline)
        count += len(chunk)
        if count > MAX_BYTES:
            _fail()
        sha.update(chunk)
    if not count:
        _fail()
    return count, sha.hexdigest()


def _free_fd(fd):
    info = os.fstatvfs(fd)
    return info.f_bavail * info.f_frsize


def build_bundle(root, key, expected_binding, manifest):
    try:
        backup.verify(root, key, expected_binding, manifest)
        encoded = _canonical(manifest)
        _manifest(encoded, key, expected_binding)
        with backup.private_directory(root) as dirfd:
            estimate = 512 + len(encoded) + (-len(encoded) % 512) + 10240
            for name in MEMBER_NAMES:
                info = os.stat(name, dir_fd=dirfd, follow_symlinks=False)
                if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid()
                        or info.st_mode & 0o077 or info.st_nlink != 1 or not 0 < info.st_size <= TAR_MEMBER_MAX):
                    _fail()
                estimate += 512 + info.st_size + (-info.st_size % 512)
            if estimate > MAX_BYTES or _free_fd(dirfd) < estimate + CHUNK:
                _fail()
            fd = os.open('media-backup.tar', os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                         0o600, dir_fd=dirfd)
            with os.fdopen(fd, 'w+b') as output:
                with tarfile.open(fileobj=output, mode='w', format=tarfile.USTAR_FORMAT) as archive:
                    info = tarfile.TarInfo('manifest.json'); info.mode = 0o600; info.size = len(encoded)
                    archive.addfile(info, io.BytesIO(encoded))
                    for name in MEMBER_NAMES:
                        member_fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=dirfd)
                        with os.fdopen(member_fd, 'rb') as member:
                            info = os.fstat(member.fileno())
                            if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid()
                                    or info.st_mode & 0o077 or info.st_nlink != 1 or not 0 < info.st_size <= TAR_MEMBER_MAX):
                                _fail()
                            entry = tarfile.TarInfo(name); entry.mode = 0o600; entry.size = info.st_size
                            archive.addfile(entry, member)
                output.flush(); os.fsync(output.fileno())
                _inspect_bundle(output, key, expected_binding)
                _bundle_identity(output)
                observed = os.fstat(output.fileno())
                current = os.stat('media-backup.tar', dir_fd=dirfd, follow_symlinks=False)
                if (observed.st_dev, observed.st_ino) != (current.st_dev, current.st_ino):
                    _fail()
        return Path(root) / 'media-backup.tar'
    except Exception:
        raise CustodyError('media backup custody rejected') from None


def readback_extract(token, binding, artifact_id, bundle, key, expected_binding, destination):
    try:
        keys = {'operation_id', 'snapshot_id', 'challenge', 'run_id', 'head_sha',
                'created_at', 'bundle_bytes', 'bundle_sha256'}
        if (not isinstance(token, str) or not token or '\r' in token or '\n' in token
                or not isinstance(binding, dict) or set(binding) != keys
                or not re.fullmatch(r'media-c1-[a-f0-9]{32}', binding['operation_id'])
                or not re.fullmatch(r'capture-[a-f0-9]{32}', binding['snapshot_id'])
                or not re.fullmatch(r'[a-f0-9]{32}', binding['challenge'])
                or type(binding['run_id']) is not int or binding['run_id'] <= 0
                or not re.fullmatch(r'[a-f0-9]{40}', binding['head_sha'])
                or type(binding['bundle_bytes']) is not int or not 0 < binding['bundle_bytes'] <= MAX_BYTES
                or not re.fullmatch(r'[a-f0-9]{64}', binding['bundle_sha256'])
                or type(artifact_id) is not int or artifact_id <= 0
                or expected_binding.get('operation_id') != binding['operation_id']
                or expected_binding.get('snapshot_id') != binding['snapshot_id']):
            _fail()
        start = transport._timestamp(binding['created_at'])
        now = datetime.now(timezone.utc)
        if start > now or (now - start).total_seconds() > transport.TIME_LIMIT:
            _fail()
        deadline = time.monotonic() + transport.TIME_LIMIT
        bundle = Path(bundle)
        with backup.private_directory(bundle.parent) as dirfd:
            fd = os.open(bundle.name, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=dirfd)
            with os.fdopen(fd, 'rb') as local:
                info = os.fstat(local.fileno())
                if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid()
                        or info.st_mode & 0o077 or info.st_nlink != 1
                        or _bundle_identity(local, deadline) != (binding['bundle_bytes'], binding['bundle_sha256'])):
                    _fail()
        headers = {'Authorization': 'Bearer ' + token, 'Accept': 'application/vnd.github+json',
                   'X-GitHub-Api-Version': '2022-11-28', 'User-Agent': 'Voice-media-custody'}
        url = API + str(artifact_id)
        raw = io.BytesIO()
        transport._download(url, headers, raw, 65536, deadline)
        meta = json.loads(raw.getvalue(), object_pairs_hook=_unique_object)
        limit = binding['bundle_bytes'] + max(CHUNK, binding['bundle_bytes'] // 100)
        if (meta['id'] != artifact_id or meta['url'] != url or meta['expired'] is not False
                or meta['name'] != 'voice-media-backup-' + binding['operation_id'] + '-' + binding['challenge']
                or meta['workflow_run']['id'] != binding['run_id'] or meta['workflow_run']['head_sha'] != binding['head_sha']
                or not start <= transport._timestamp(meta['created_at']) <= now
                or transport._timestamp(meta['expires_at']) <= now or type(meta['size_in_bytes']) is not int
                or not 0 < meta['size_in_bytes'] <= limit):
            _fail()
        with backup.private_directory(destination) as destination_fd:
            if os.listdir(destination_fd):
                _fail()
            temp_path = tempfile.gettempdir()
            same_device = os.stat(temp_path).st_dev == os.fstat(destination_fd).st_dev
            temporary_required = meta['size_in_bytes'] + binding['bundle_bytes'] + CHUNK
            destination_required = binding['bundle_bytes'] + CHUNK
            if same_device:
                temporary_required += destination_required
            if (shutil.disk_usage(temp_path).free < temporary_required
                    or (not same_device and _free_fd(destination_fd) < destination_required)):
                _fail()
        with tempfile.TemporaryFile() as remote, tempfile.TemporaryFile() as restored_bundle:
            count = transport._download(url + '/zip', headers, remote, limit, deadline, redirects=True)
            if count != meta['size_in_bytes']:
                _fail()
            remote.seek(-min(count, 65557), 2); tail = remote.read(65557)
            offset = tail.rfind(b'PK\x05\x06')
            if offset < 0 or len(tail) != offset + 22:
                _fail()
            _, disk, directory_disk, disk_count, total_count, directory_bytes, _, comment = struct.unpack('<4s4H2LH', tail[offset:])
            if disk or directory_disk or disk_count != 1 or total_count != 1 or directory_bytes > 8192 or comment:
                _fail()
            remote.seek(0)
            with zipfile.ZipFile(remote) as archive:
                items = archive.infolist()
                if len(items) != 1:
                    _fail()
                item = items[0]; mode = item.external_attr >> 16
                if (item.filename != 'media-backup.tar' or item.is_dir() or item.flag_bits & 1
                        or stat.S_IFMT(mode) not in (0, stat.S_IFREG) or item.file_size != binding['bundle_bytes']
                        or item.header_offset != 0 or item.compress_size < binding['bundle_bytes'] * .95
                        or item.compress_type not in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED)):
                    _fail()
                with archive.open(item) as member:
                    copied = 0
                    for chunk in iter(lambda: member.read(CHUNK), b''):
                        _remaining(deadline)
                        copied += len(chunk)
                        if copied > binding['bundle_bytes']:
                            _fail()
                        restored_bundle.write(chunk)
                    if copied != binding['bundle_bytes']:
                        _fail()
            if _bundle_identity(restored_bundle, deadline) != (binding['bundle_bytes'], binding['bundle_sha256']):
                _fail()
            manifest, offsets = _inspect_bundle(restored_bundle, key, expected_binding, deadline)
            with backup.private_directory(destination) as dirfd:
                if os.listdir(dirfd):
                    _fail()
                for name in ('manifest.json', *MEMBER_NAMES):
                    offset, size = offsets[name]
                    restored_bundle.seek(offset)
                    fd = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=dirfd)
                    with os.fdopen(fd, 'wb') as output:
                        remaining = size
                        while remaining:
                            _remaining(deadline)
                            chunk = restored_bundle.read(min(CHUNK, remaining))
                            if not chunk:
                                _fail()
                            output.write(chunk)
                            remaining -= len(chunk)
                        output.flush(); os.fsync(output.fileno())
            backup.verify(destination, key, expected_binding, manifest)
            _remaining(deadline)
        receipt = {'schema': 'voice-media-custody-v1', 'destination': 'github:Poryadok/VoiceRoot',
                   'artifact_id': artifact_id, **{name: binding[name] for name in keys if name != 'created_at'}}
        return receipt, manifest
    except Exception:
        raise CustodyError('media backup custody rejected') from None
