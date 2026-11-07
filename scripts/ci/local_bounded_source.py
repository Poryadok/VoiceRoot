"""Create a capped archive of one clean committed checkout without Git writes.

This is one diagnostic input stage, not a bounded Go execution or receipt.
The caller must supervise the process and retain exclusive source/output custody.
"""
import hashlib
import os
from pathlib import Path
import re
import stat
import tarfile
import time

import local_current_diff

MAX_FILES = 100000
MAX_SOURCE_BYTES = 512 * 1024 * 1024
MAX_ARCHIVE_BYTES = 1024 * 1024 * 1024
MAX_SECONDS = 1800
CHUNK = 1024 * 1024


class _CappedWriter:
    def __init__(self, stream, maximum, deadline):
        self.stream, self.maximum, self.deadline = stream, maximum, deadline
        self.count = 0
        self.digest = hashlib.sha256()

    def write(self, data):
        _deadline(self.deadline)
        if self.count + len(data) > self.maximum:
            raise ValueError('source archive exceeds write limit')
        written = self.stream.write(data)
        if written != len(data):
            raise OSError('short source archive write')
        self.count += written
        self.digest.update(data)
        return written

    def tell(self):
        return self.count

    def flush(self):
        self.stream.flush()


def _deadline(deadline):
    if time.monotonic() >= deadline:
        raise TimeoutError('bounded source deadline expired')


def _limits(max_files, max_source_bytes, max_archive_bytes, timeout):
    values = ((max_files, MAX_FILES), (max_source_bytes, MAX_SOURCE_BYTES),
              (max_archive_bytes, MAX_ARCHIVE_BYTES), (timeout, MAX_SECONDS))
    if any(type(value) is not int or not 0 < value <= maximum for value, maximum in values):
        raise ValueError('bounded source limits exceed sealed ceilings')


def _git(root, args, deadline):
    _deadline(deadline)
    env = {key: value for key, value in os.environ.items() if not key.upper().startswith('GIT_')}
    env.update(GIT_OPTIONAL_LOCKS='0', GIT_NO_LAZY_FETCH='1', GIT_NO_REPLACE_OBJECTS='1',
               GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull, GIT_TERMINAL_PROMPT='0')
    command = ['git', '--no-optional-locks', '-c', 'core.fsmonitor=false',
               '-c', 'core.untrackedCache=false', '-C', str(root), *args]
    result = local_current_diff._execute(command, env=env, timeout=max(0.001, deadline - time.monotonic()),
                                         output_limit=16 * 1024 * 1024)
    if result.returncode:
        raise ValueError('bounded source Git read failed')
    _deadline(deadline)
    return result.stdout


def _identity(root, head, deadline):
    if _git(root, ['rev-parse', '--verify', 'HEAD^{commit}'], deadline).strip() != head.encode('ascii'):
        raise ValueError('bounded source HEAD differs')
    if (_git(root, ['diff', '--cached', '--name-only', '-z', '--no-ext-diff', '--no-textconv',
                    head, '--'], deadline) or
            _git(root, ['ls-files', '--others', '--exclude-standard', '-z'], deadline)):
        raise ValueError('bounded source requires a clean checkout')
    tree = _git(root, ['rev-parse', '--verify', head + '^{tree}'], deadline).decode('ascii').strip()
    if not re.fullmatch(r'[0-9a-f]{40}', tree):
        raise ValueError('bounded source tree identity is invalid')
    return tree


def _entries(root, head, max_files, max_source_bytes, deadline):
    raw = _git(root, ['ls-tree', '-r', '-z', '-l', '--full-tree', head], deadline)
    if not raw.endswith(b'\0'):
        raise ValueError('bounded source tree inventory is incomplete')
    entries, total = [], 0
    for row in raw[:-1].split(b'\0'):
        try:
            metadata, name = row.split(b'\t', 1)
            mode, kind, oid, size = metadata.decode('ascii').split()
            text = name.decode('utf-8', errors='strict')
            parts = text.split('/')
            if (mode not in ('100644', '100755') or kind != 'blob' or
                    not re.fullmatch(r'[0-9a-f]{40}', oid) or not size.isdecimal() or
                    any(not part or part in ('.', '..', '.git') or '\\' in part or ':' in part
                        for part in parts)):
                raise ValueError()
            length = int(size)
        except (UnicodeError, ValueError):
            raise ValueError('bounded source tree contains an unsafe entry') from None
        total += length
        entries.append((name, parts, mode, oid, length))
        if len(entries) > max_files or total > max_source_bytes:
            raise ValueError('bounded source input exceeds limit')
    if not entries or len({entry[0] for entry in entries}) != len(entries):
        raise ValueError('bounded source tree is empty or duplicated')
    return sorted(entries, key=lambda item: item[0])


def _path(root, parts):
    path = root
    for part in parts:
        path = path / part
        if path.is_symlink() or getattr(path, 'is_junction', lambda: False)():
            raise ValueError('bounded source path contains a link')
    if root not in path.resolve(strict=True).parents:
        raise ValueError('bounded source path escapes checkout')
    return path


def _visit(root, entries, deadline, tar=None):
    worktree = hashlib.sha256()
    for name, parts, mode, oid, expected_size in entries:
        _deadline(deadline)
        path = _path(root, parts)
        before = path.lstat()
        if not stat.S_ISREG(before.st_mode) or before.st_size != expected_size:
            raise ValueError('bounded source file size or type differs')
        if os.name != 'nt' and bool(before.st_mode & 0o111) != (mode == '100755'):
            raise ValueError('bounded source executable mode differs')
        worktree.update(len(name).to_bytes(8, 'big')); worktree.update(name)
        worktree.update(stat.S_IMODE(before.st_mode).to_bytes(4, 'big'))
        worktree.update(b'F' + expected_size.to_bytes(8, 'big'))
        blob = hashlib.sha1(b'blob ' + str(expected_size).encode('ascii') + b'\0')
        with path.open('rb') as stream:
            opened = os.fstat(stream.fileno())
            if (opened.st_dev, opened.st_ino, opened.st_size, opened.st_mtime_ns) != (
                    before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns):
                raise ValueError('bounded source changed before reading')
            class Reader:
                remaining = expected_size
                def read(self, size=-1):
                    _deadline(deadline)
                    amount = min(CHUNK, self.remaining) if size < 0 else min(size, CHUNK, self.remaining)
                    data = stream.read(amount)
                    if amount and not data:
                        raise ValueError('bounded source file was truncated')
                    self.remaining -= len(data)
                    blob.update(data); worktree.update(data)
                    return data
            reader = Reader()
            if tar is None:
                while reader.remaining:
                    reader.read(CHUNK)
            else:
                info = tarfile.TarInfo(name.decode('utf-8'))
                info.size = expected_size; info.mode = stat.S_IMODE(before.st_mode)
                tar.addfile(info, reader)
            if reader.remaining or stream.read(1):
                raise ValueError('bounded source file length changed')
            after = os.fstat(stream.fileno())
        current = path.lstat()
        if ((after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns) !=
                (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) or
                (current.st_dev, current.st_ino, current.st_size, current.st_mtime_ns) !=
                (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) or blob.hexdigest() != oid):
            raise ValueError('bounded source bytes changed or differ from Git')
    return worktree.hexdigest()


def _verify_archive(raw, entries, expected_hash, expected_worktree, deadline):
    raw.seek(0)
    digest = hashlib.sha256()
    while True:
        _deadline(deadline)
        block = raw.read(CHUNK)
        if not block:
            break
        digest.update(block)
    if digest.hexdigest() != expected_hash:
        raise ValueError('bounded source archive bytes changed')
    raw.seek(0)
    worktree = hashlib.sha256()
    with tarfile.open(fileobj=raw, mode='r|') as tar:
        for name, _parts, _mode, oid, length in entries:
            _deadline(deadline)
            member = tar.next()
            if (member is None or not member.isfile() or member.name != name.decode('utf-8') or
                    member.size != length):
                raise ValueError('bounded source archive inventory differs')
            worktree.update(len(name).to_bytes(8, 'big')); worktree.update(name)
            worktree.update(member.mode.to_bytes(4, 'big'))
            worktree.update(b'F' + length.to_bytes(8, 'big'))
            blob = hashlib.sha1(b'blob ' + str(length).encode('ascii') + b'\0')
            remaining = length
            with tar.extractfile(member) as stream:
                while remaining:
                    _deadline(deadline)
                    data = stream.read(min(CHUNK, remaining))
                    if not data:
                        raise ValueError('bounded source archive member is truncated')
                    remaining -= len(data)
                    blob.update(data); worktree.update(data)
            if blob.hexdigest() != oid:
                raise ValueError('bounded source archive member differs')
        if tar.next() is not None or worktree.hexdigest() != expected_worktree:
            raise ValueError('bounded source archive differs from source')


def capture_archive(root, head, output, *, max_files, max_source_bytes, max_archive_bytes, timeout):
    """Stream one sealed clean checkout to an exclusive capped tar file."""
    _limits(max_files, max_source_bytes, max_archive_bytes, timeout)
    if not isinstance(head, str) or not re.fullmatch(r'[0-9a-f]{40}', head):
        raise ValueError('full SHA-1 head is required')
    root, output = Path(root).resolve(strict=True), Path(output).absolute()
    if (output.exists() or output.is_symlink() or output.parent.resolve(strict=True) != output.parent or
            output == root or root in output.parents):
        raise ValueError('bounded source output must be new and outside checkout')
    deadline = time.monotonic() + timeout
    tree = _identity(root, head, deadline)
    entries = _entries(root, head, max_files, max_source_bytes, deadline)
    with output.open('x+b') as raw:
        writer = _CappedWriter(raw, max_archive_bytes, deadline)
        with tarfile.open(fileobj=writer, mode='w|') as tar:
            digest = _visit(root, entries, deadline, tar)
        writer.flush()
        os.fsync(raw.fileno())
        _verify_archive(raw, entries, writer.digest.hexdigest(), digest, deadline)
        opened, current = os.fstat(raw.fileno()), output.lstat()
        if (not stat.S_ISREG(current.st_mode) or
                (opened.st_dev, opened.st_ino) != (current.st_dev, current.st_ino) or opened.st_size != writer.count):
            raise ValueError('bounded source archive path changed')
    if (_identity(root, head, deadline) != tree or
            _entries(root, head, max_files, max_source_bytes, deadline) != entries or
            _visit(root, entries, deadline) != digest):
        raise ValueError('bounded source changed during archival')
    source = {'schema': 'voice-local-source-snapshot-v1', 'head': head, 'tree': tree,
              'worktree_sha256': digest, 'raw_mismatch_paths': [], 'changed_paths': []}
    return {'schema': 'voice-bounded-source-archive-v1', 'source': source,
            'archive_sha256': writer.digest.hexdigest(), 'archive_bytes': writer.count}


def verify_archive(root, source, archive_path, receipt):
    """Revalidate an internally produced archive before transfer; no PASS implied."""
    if (not isinstance(receipt, dict) or set(receipt) != {'schema', 'source', 'archive_sha256', 'archive_bytes'} or
            receipt['schema'] != 'voice-bounded-source-archive-v1' or receipt['source'] != source or
            not isinstance(receipt['archive_sha256'], str) or
            not re.fullmatch(r'[0-9a-f]{64}', receipt['archive_sha256']) or
            type(receipt['archive_bytes']) is not int or not 0 < receipt['archive_bytes'] <= MAX_ARCHIVE_BYTES):
        raise ValueError('bounded source archive receipt differs')
    root = Path(root).resolve(strict=True)
    path = Path(archive_path).absolute()
    if path.parent.resolve(strict=True) != path.parent or path.is_symlink() or root in path.parents:
        raise ValueError('bounded source archive path differs')
    deadline = time.monotonic() + MAX_SECONDS
    tree = _identity(root, source['head'], deadline)
    entries = _entries(root, source['head'], MAX_FILES, MAX_SOURCE_BYTES, deadline)
    if tree != source['tree']:
        raise ValueError('bounded source tree changed')
    with path.open('rb') as raw:
        opened, current = os.fstat(raw.fileno()), path.lstat()
        if (not stat.S_ISREG(current.st_mode) or opened.st_size != receipt['archive_bytes'] or
                (opened.st_dev, opened.st_ino) != (current.st_dev, current.st_ino)):
            raise ValueError('bounded source archive path or size changed')
        _verify_archive(raw, entries, receipt['archive_sha256'], source['worktree_sha256'], deadline)
        after, current = os.fstat(raw.fileno()), path.lstat()
        if ((opened.st_dev, opened.st_ino, opened.st_size, opened.st_mtime_ns) !=
                (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns) or
                (after.st_dev, after.st_ino) != (current.st_dev, current.st_ino)):
            raise ValueError('bounded source archive changed during verification')
    if _identity(root, source['head'], deadline) != tree or _visit(root, entries, deadline) != source['worktree_sha256']:
        raise ValueError('bounded source changed during archive verification')
    return receipt['archive_sha256']
