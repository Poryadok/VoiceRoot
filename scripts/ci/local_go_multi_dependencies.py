"""Merge two source-bound Go dependency closures for Notification diagnostics.

The caller must control the source snapshot and cache files throughout. These
checks do not authenticate Go, proxies, images or the private daemon.
"""
from contextlib import ExitStack
import hashlib
import os
from pathlib import Path
import re
import stat
import tarfile

import local_go_dependencies
import local_go_execution
import local_source_snapshot as snapshot

MODULES = ('src/backend/notification', 'src/backend/user')
TARGET = {'goos': 'linux', 'goarch': 'amd64', 'cgo_enabled': False}
MAX_ARCHIVE = 2 * 1024 * 1024 * 1024
MAX_TOTAL = 1024 * 1024 * 1024
MAX_MEMBER = 256 * 1024 * 1024
MAX_ENTRIES = 100000


def _hash(stream):
    stream.seek(0)
    digest = hashlib.sha256()
    for block in iter(lambda: stream.read(1024 * 1024), b''):
        digest.update(block)
    return digest.hexdigest()


def _member_name(member):
    parts = member.name.split('/')
    if (not member.isfile() or member.issparse() or len(parts) < 4 or parts[:2] != ['cache', 'download'] or
            any(part in ('', '.', '..') or '\\' in part or ':' in part for part in parts) or
            member.size < 0 or member.size > MAX_MEMBER):
        raise ValueError('invalid Go cache archive member')
    return member.name


def merge(rows, source, output):
    """Merge exact receipts and bounded, regular tar entries; conflicting bytes fail."""
    if (not isinstance(rows, list) or len(rows) != len(MODULES) or
            any(not isinstance(row, tuple) or len(row) != 2 or not isinstance(row[0], dict) for row in rows) or
            [row[0].get('module_dir') for row in rows] != list(MODULES)):
        raise ValueError('Go module inventory differs')
    common = rows[0][0]
    for receipt, _ in rows:
        if (receipt.get('status') != 'incomplete' or receipt.get('source') != source or
                receipt.get('target') != TARGET or receipt.get('toolchain_sha256') != common.get('toolchain_sha256') or
                not isinstance(receipt.get('toolchain_sha256'), str) or
                not re.fullmatch(r'[0-9a-f]{64}', receipt['toolchain_sha256']) or
                receipt.get('toolchain_version') != common.get('toolchain_version') or
                not isinstance(receipt.get('toolchain_version'), str) or
                not re.fullmatch(r'go version go[0-9]+\.[0-9]+(?:\.[0-9]+)? [a-z0-9]+/[a-z0-9]+', receipt['toolchain_version']) or
                not isinstance(receipt.get('archive_sha256'), str) or
                not re.fullmatch(r'[0-9a-f]{64}', receipt['archive_sha256'])):
            raise ValueError('Go acquisition identity differs')
    output = Path(output)
    entries, identifiers, total, count = {}, set(), 0, 0
    with ExitStack() as stack:
        sources = []
        for receipt, path_value in rows:
            path = Path(path_value)
            if path.is_symlink():
                raise ValueError('Go cache archive must be a regular file')
            stream = stack.enter_context(path.open('rb'))
            info, path_info = os.fstat(stream.fileno()), path.lstat()
            identity = (info.st_dev, info.st_ino)
            if (not stat.S_ISREG(info.st_mode) or not stat.S_ISREG(path_info.st_mode) or
                    identity != (path_info.st_dev, path_info.st_ino) or identity in identifiers or
                    info.st_size > MAX_ARCHIVE or _hash(stream) != receipt['archive_sha256']):
                raise ValueError('Go cache archive identity changed')
            identifiers.add(identity)
            stream.seek(0)
            tar = stack.enter_context(tarfile.open(fileobj=stream, mode='r:'))
            sources.append((path, stream, info, receipt, tar))
            seen = set()
            for member in tar:
                name = _member_name(member)
                count += 1
                total += member.size
                if count > MAX_ENTRIES or total > MAX_TOTAL or name in seen:
                    raise ValueError('Go cache archive inventory exceeds limits or duplicates')
                seen.add(name)
                digest = hashlib.sha256()
                with tar.extractfile(member) as item:
                    for block in iter(lambda: item.read(1024 * 1024), b''):
                        digest.update(block)
                identity = (member.size, digest.hexdigest())
                if name in entries and entries[name][0] != identity:
                    raise ValueError('Go cache archive conflict')
                entries.setdefault(name, (identity, tar, member))
        with output.open('xb') as raw, tarfile.open(fileobj=raw, mode='w') as merged:
            for name, (identity, tar, member) in sorted(entries.items()):
                normalized = tarfile.TarInfo(name)
                normalized.size, normalized.mode = identity[0], 0o600
                with tar.extractfile(member) as item:
                    merged.addfile(normalized, item)
        with tarfile.open(output, mode='r:') as merged:
            members = merged.getmembers()
            if [item.name for item in members] != sorted(entries):
                raise ValueError('merged Go cache archive inventory differs')
            for member in members:
                name = _member_name(member)
                digest = hashlib.sha256()
                with merged.extractfile(member) as item:
                    for block in iter(lambda: item.read(1024 * 1024), b''):
                        digest.update(block)
                if (member.size, digest.hexdigest()) != entries[name][0]:
                    raise ValueError('merged Go cache archive bytes differ')
        for path, stream, before, receipt, _ in sources:
            after, current = os.fstat(stream.fileno()), path.lstat()
            if (path.is_symlink() or not stat.S_ISREG(current.st_mode) or
                    (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns) !=
                    (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) or
                    (current.st_dev, current.st_ino) != (after.st_dev, after.st_ino) or
                    _hash(stream) != receipt['archive_sha256']):
                raise ValueError('Go cache archive changed during merge')
    return {'status': 'incomplete', 'source': source, 'module_dir': MODULES[0], 'module_dirs': list(MODULES),
            'target': TARGET, 'toolchain_sha256': common['toolchain_sha256'],
            'toolchain_version': common['toolchain_version'], 'archive_sha256': local_go_dependencies._sha256(output),
            'input_archive_sha256': [receipt['archive_sha256'] for receipt, _ in rows]}


def acquire(root, source, source_copy, output, go_binary='go'):
    """Acquire both exact module closures separately, then merge them."""
    root, source_copy, output = Path(root).resolve(), Path(source_copy).resolve(), Path(output).resolve()
    if output.exists() or output == root or root in output.parents or output == source_copy or source_copy in output.parents:
        raise ValueError('multi-module output must be new and outside source trees')
    output.mkdir(parents=True)
    rows = []
    for module in MODULES:
        directory = output / module.rsplit('/', 1)[-1]
        receipt = local_go_dependencies.acquire(root, source, source_copy, module, directory, go_binary)
        rows.append((receipt, directory / 'download-cache.tar'))
    snapshot.verify_materialized(root, source, source_copy)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed before multi-module cache merge')
    result = merge(rows, source, output / 'download-cache.tar')
    snapshot.verify_materialized(root, source, source_copy)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed during multi-module cache merge')
    return result


def transfer(owner, client, root, source, source_copy, source_digest, acquisition, archive_path):
    if not isinstance(acquisition, dict) or acquisition.get('module_dirs') != list(MODULES):
        raise ValueError('multi-module inventory differs')
    result = local_go_execution.transfer_dependencies(owner, client, root, source, source_copy,
                                                       source_digest, acquisition, archive_path)
    local_go_execution._verify_modules(owner, client, MODULES[1])
    return result
