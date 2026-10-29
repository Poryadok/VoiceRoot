"""Strict GitHub ciphertext custody for a verified PostgreSQL cohort."""
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
import story_media_backup_cohort as cohort
import story_media_backup_custody as common

API = common.API
transport = common.transport


class CohortCustodyError(Exception):
    pass


def _fail():
    raise CohortCustodyError('cohort custody rejected')


def _names(binding, databases):
    members = cohort._members(binding, databases)
    receipts = binding.get('dump_receipts')
    if (not isinstance(receipts, dict) or set(receipts) != members
            or any(not isinstance(v, str) or not re.fullmatch(r'[0-9a-f]{64}', v) for v in receipts.values())):
        _fail()
    return tuple(name + '.enc' for name in sorted(members))


def _manifest(raw, key, binding, databases):
    if not 0 < len(raw) <= common.MANIFEST_LIMIT or not isinstance(key, bytes) or len(key) != 32:
        _fail()
    value = json.loads(raw, object_pairs_hook=common._unique_object)
    names = _names(binding, databases)
    if (not isinstance(value, dict) or set(value) != {'format', 'binding', 'files', 'mac'}
            or type(value['format']) is not int or value['format'] != 1
            or common._canonical(value['binding']) != common._canonical(binding) or not isinstance(value['files'], dict)
            or set(value['files']) != {name[:-4] for name in names} or not isinstance(value['mac'], str)):
        _fail()
    unsigned = {name: value[name] for name in ('format', 'binding', 'files')}
    if not hmac.compare_digest(cohort._mac(key, unsigned), value['mac']) or raw != common._canonical(value):
        _fail()
    return value


def _inspect(stream, key, binding, databases, deadline=None):
    stream.seek(0); offsets = {}; manifest = None
    for name in ('manifest.json', *_names(binding, databases)):
        common._remaining(deadline)
        header = stream.read(512)
        if len(header) != 512: _fail()
        size_field = header[124:136].strip(b'\0 ')
        if not re.fullmatch(rb'[0-7]+', size_field): _fail()
        size = int(size_field, 8)
        if not 0 < size <= (common.MANIFEST_LIMIT if name == 'manifest.json' else common.TAR_MEMBER_MAX): _fail()
        canonical = tarfile.TarInfo(name); canonical.mode = 0o600; canonical.size = size
        if not hmac.compare_digest(header, canonical.tobuf(format=tarfile.USTAR_FORMAT)): _fail()
        offsets[name] = (stream.tell(), size)
        sha = hashlib.sha256(); mac = hmac.new(key, digestmod=hashlib.sha256)
        collected = bytearray() if name == 'manifest.json' else None
        remaining = size
        while remaining:
            common._remaining(deadline)
            chunk = stream.read(min(common.CHUNK, remaining))
            if not chunk: _fail()
            remaining -= len(chunk); sha.update(chunk); mac.update(chunk)
            if collected is not None: collected.extend(chunk)
        padding = stream.read((-size) % 512)
        if len(padding) != (-size) % 512 or padding.strip(b'\0'): _fail()
        if name == 'manifest.json': manifest = _manifest(bytes(collected), key, binding, databases)
        elif {'sha256': sha.hexdigest(), 'hmac': mac.hexdigest()} != manifest['files'][name[:-4]]: _fail()
    tail = 0
    for chunk in iter(lambda: stream.read(common.CHUNK), b''):
        common._remaining(deadline); tail += len(chunk)
        if chunk.strip(b'\0'): _fail()
    if tail < 1024 or tail % 512: _fail()
    return manifest, offsets


def build_bundle(root, key, binding, databases, manifest):
    try:
        frozen = json.loads(json.dumps({'binding': binding, 'databases': databases, 'manifest': manifest},
                                       sort_keys=True, separators=(',', ':'), allow_nan=False))
        binding, databases, manifest = frozen['binding'], frozen['databases'], frozen['manifest']
        cohort.verify(root, key, binding, databases, manifest)
        names = _names(binding, databases); encoded = common._canonical(manifest)
        _manifest(encoded, key, binding, databases)
        with backup.private_directory(root) as dirfd:
            estimate = 512 + len(encoded) + (-len(encoded) % 512) + 10240
            for name in names:
                info = os.stat(name, dir_fd=dirfd, follow_symlinks=False)
                if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid() or info.st_mode & 0o077
                        or info.st_nlink != 1 or not 0 < info.st_size <= common.TAR_MEMBER_MAX): _fail()
                estimate += 512 + info.st_size + (-info.st_size % 512)
            if estimate > common.MAX_BYTES or common._free_fd(dirfd) < estimate + common.CHUNK: _fail()
            fd = os.open('cohort-backup.tar', os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=dirfd)
            with os.fdopen(fd, 'w+b') as output:
                with tarfile.open(fileobj=output, mode='w', format=tarfile.USTAR_FORMAT) as archive:
                    entry = tarfile.TarInfo('manifest.json'); entry.mode = 0o600; entry.size = len(encoded)
                    archive.addfile(entry, io.BytesIO(encoded))
                    for name in names:
                        fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=dirfd)
                        with os.fdopen(fd, 'rb') as member:
                            info = os.fstat(member.fileno())
                            if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid() or info.st_mode & 0o077
                                    or info.st_nlink != 1 or not 0 < info.st_size <= common.TAR_MEMBER_MAX): _fail()
                            entry = tarfile.TarInfo(name); entry.mode = 0o600; entry.size = info.st_size
                            archive.addfile(entry, member)
                output.flush(); os.fsync(output.fileno()); _inspect(output, key, binding, databases)
                common._bundle_identity(output)
                observed = os.fstat(output.fileno()); current = os.stat('cohort-backup.tar', dir_fd=dirfd, follow_symlinks=False)
                if (observed.st_dev, observed.st_ino) != (current.st_dev, current.st_ino): _fail()
        return Path(root) / 'cohort-backup.tar'
    except Exception:
        raise CohortCustodyError('cohort custody rejected') from None


def verify_bundle(path, key, binding, databases):
    try:
        frozen = json.loads(json.dumps({'binding': binding, 'databases': databases},
                                       sort_keys=True, separators=(',', ':'), allow_nan=False))
        binding, databases = frozen['binding'], frozen['databases']
        path = Path(path); deadline = time.monotonic() + transport.TIME_LIMIT
        with backup.private_directory(path.parent) as dirfd:
            fd = os.open(path.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=dirfd)
            with os.fdopen(fd, 'rb') as stream:
                info = os.fstat(stream.fileno())
                if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid() or info.st_mode & 0o077
                        or info.st_nlink != 1): _fail()
                before = common._bundle_identity(stream, deadline)
                _inspect(stream, key, binding, databases, deadline)
                after = common._bundle_identity(stream, deadline)
                current = os.stat(path.name, dir_fd=dirfd, follow_symlinks=False)
                if before != after or (info.st_dev, info.st_ino) != (current.st_dev, current.st_ino): _fail()
        return {'bytes': after[0], 'sha256': after[1]}
    except Exception:
        raise CohortCustodyError('cohort custody rejected') from None


def readback_extract(token, artifact_binding, artifact_id, bundle, key, binding, databases, destination):
    try:
        frozen = json.loads(json.dumps({'artifact': artifact_binding, 'binding': binding, 'databases': databases},
                                       sort_keys=True, separators=(',', ':'), allow_nan=False))
        artifact_binding, binding, databases = frozen['artifact'], frozen['binding'], frozen['databases']
        fields = {'operation_id', 'snapshot_id', 'challenge', 'run_id', 'head_sha',
                  'created_at', 'bundle_bytes', 'bundle_sha256'}
        if (not isinstance(token, str) or not token or '\r' in token or '\n' in token
                or not isinstance(artifact_binding, dict) or set(artifact_binding) != fields
                or artifact_binding['operation_id'] != binding['operation_id']
                or artifact_binding['snapshot_id'] != binding['snapshot_id']
                or not re.fullmatch(r'[0-9a-f]{32}', artifact_binding['challenge'])
                or type(artifact_binding['run_id']) is not int or artifact_binding['run_id'] <= 0
                or not re.fullmatch(r'[0-9a-f]{40}', artifact_binding['head_sha'])
                or type(artifact_binding['bundle_bytes']) is not int or not 0 < artifact_binding['bundle_bytes'] <= common.MAX_BYTES
                or not re.fullmatch(r'[0-9a-f]{64}', artifact_binding['bundle_sha256'])
                or type(artifact_id) is not int or artifact_id <= 0): _fail()
        _names(binding, databases)
        start = transport._timestamp(artifact_binding['created_at']); now = datetime.now(timezone.utc)
        if start > now or (now-start).total_seconds() > transport.TIME_LIMIT: _fail()
        deadline = time.monotonic() + transport.TIME_LIMIT
        local = verify_bundle(bundle, key, binding, databases)
        if (local['bytes'], local['sha256']) != (artifact_binding['bundle_bytes'], artifact_binding['bundle_sha256']): _fail()
        headers = {'Authorization': 'Bearer ' + token, 'Accept': 'application/vnd.github+json',
                   'X-GitHub-Api-Version': '2022-11-28', 'User-Agent': 'Voice-cohort-custody'}
        url = API + str(artifact_id); raw = io.BytesIO()
        transport._download(url, headers, raw, 65536, deadline)
        meta = json.loads(raw.getvalue(), object_pairs_hook=common._unique_object)
        limit = artifact_binding['bundle_bytes'] + max(common.CHUNK, artifact_binding['bundle_bytes'] // 100)
        if (meta['id'] != artifact_id or meta['url'] != url or meta['expired'] is not False
                or meta['name'] != 'voice-cohort-backup-' + binding['operation_id'] + '-' + artifact_binding['challenge']
                or meta['workflow_run']['id'] != artifact_binding['run_id']
                or meta['workflow_run']['head_sha'] != artifact_binding['head_sha']
                or not start <= transport._timestamp(meta['created_at']) <= now
                or transport._timestamp(meta['expires_at']) <= now or type(meta['size_in_bytes']) is not int
                or not 0 < meta['size_in_bytes'] <= limit): _fail()
        with backup.private_directory(destination) as dirfd:
            if os.listdir(dirfd): _fail()
            tempdir = tempfile.gettempdir(); same = os.stat(tempdir).st_dev == os.fstat(dirfd).st_dev
            temp_need = meta['size_in_bytes'] + artifact_binding['bundle_bytes'] + common.CHUNK
            dest_need = artifact_binding['bundle_bytes'] + common.CHUNK
            if same: temp_need += dest_need
            if shutil.disk_usage(tempdir).free < temp_need or (not same and common._free_fd(dirfd) < dest_need): _fail()
        with tempfile.TemporaryFile() as remote, tempfile.TemporaryFile() as restored:
            count = transport._download(url + '/zip', headers, remote, limit, deadline, redirects=True)
            if count != meta['size_in_bytes']: _fail()
            transport._bound_zip_directory(remote, count)
            remote.seek(0)
            with zipfile.ZipFile(remote) as archive:
                items = archive.infolist()
                if len(items) != 1: _fail()
                item = items[0]; mode = item.external_attr >> 16
                if (item.filename != 'cohort-backup.tar' or item.is_dir() or item.flag_bits & 1
                        or stat.S_IFMT(mode) not in (0, stat.S_IFREG) or item.file_size != artifact_binding['bundle_bytes']
                        or item.header_offset != 0 or item.compress_size < artifact_binding['bundle_bytes'] * .95
                        or item.compress_type not in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED)): _fail()
                with archive.open(item) as source:
                    copied = 0
                    for chunk in iter(lambda: source.read(common.CHUNK), b''):
                        common._remaining(deadline); copied += len(chunk)
                        if copied > artifact_binding['bundle_bytes']: _fail()
                        restored.write(chunk)
                    if copied != artifact_binding['bundle_bytes']: _fail()
            if common._bundle_identity(restored, deadline) != (artifact_binding['bundle_bytes'], artifact_binding['bundle_sha256']): _fail()
            manifest, offsets = _inspect(restored, key, binding, databases, deadline)
            with backup.private_directory(destination) as dirfd:
                if os.listdir(dirfd): _fail()
                for name in ('manifest.json', *_names(binding, databases)):
                    offset, size = offsets[name]; restored.seek(offset)
                    fd = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=dirfd)
                    with os.fdopen(fd, 'wb') as output:
                        remaining = size
                        while remaining:
                            common._remaining(deadline); chunk = restored.read(min(common.CHUNK, remaining))
                            if not chunk: _fail()
                            output.write(chunk); remaining -= len(chunk)
                        output.flush(); os.fsync(output.fileno())
            cohort.verify(destination, key, binding, databases, manifest); common._remaining(deadline)
        receipt = {'schema': 'voice-cohort-custody-v1', 'destination': 'github:Poryadok/VoiceRoot',
                   'artifact_id': artifact_id, **{name: artifact_binding[name] for name in fields if name != 'created_at'}}
        return receipt, manifest
    except Exception:
        raise CohortCustodyError('cohort custody rejected') from None
