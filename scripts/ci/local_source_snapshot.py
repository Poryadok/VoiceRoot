"""Snapshot Git-visible local source without changing the real index.

This captures tracked files and non-ignored untracked files, not ignored build
inputs. It is a planning primitive, not a proof that a check actually ran.
Each Git wait is bounded. On Windows the owned Job broker handles descendants;
its 64 MiB per-stream capture limit fails closed. The POSIX timeout does not
establish descendant cleanup. Process startup, filesystem scanning, the total
multi-command capture and other planner Git calls remain separately unbounded.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import stat
import subprocess
import tarfile
import tempfile

from affected_check_plan import changed_paths
import local_process_supervision as supervision


GIT_TIMEOUT_SECONDS = 600


def _windows():
    return os.name == 'nt'


def _git(root, *args, env=None):
    command = ['git', '-C', str(root), *args]
    if _windows():
        result = supervision.run_diagnostic(command, env=env, timeout=GIT_TIMEOUT_SECONDS,
                                            capture_output=True, check=False)
        if result.returncode:
            raise subprocess.CalledProcessError(result.returncode, command,
                                                output=result.stdout, stderr=result.stderr)
        return result.stdout
    # Files prevent a surviving descendant holding a pipe from blocking the
    # timeout return. POSIX descendant cleanup is not established here.
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        result = subprocess.run(command, env=env, stdout=out, stderr=err,
                                timeout=GIT_TIMEOUT_SECONDS, check=False)
        out.seek(0); err.seek(0)
        stdout, stderr = out.read(), err.read()
    if result.returncode:
        raise subprocess.CalledProcessError(result.returncode, command, output=stdout, stderr=stderr)
    return stdout


def _reject_unresolved_inputs(index):
    for entry in index.split(b'\0'):
        if entry.startswith(b'160000 '):
            raise ValueError('gitlink inputs are not supported by local snapshots')
        if entry.startswith(b'120000 '):
            raise ValueError('symbolic link inputs are not supported by local snapshots')


def _capture_tree(root, head):
    env = os.environ.copy()
    for key in ('GIT_DIR', 'GIT_WORK_TREE', 'GIT_INDEX_FILE', 'GIT_COMMON_DIR'):
        env.pop(key, None)
    with tempfile.TemporaryDirectory(prefix='voice-ci-index-') as directory:
        env['GIT_INDEX_FILE'] = str(Path(directory) / 'index')
        _git(root, 'read-tree', head, env=env)
        _git(root, 'add', '-A', '--', '.', env=env)
        entries = _git(root, 'ls-files', '--stage', '-z', env=env)
        _reject_unresolved_inputs(entries)
        tree = _git(root, 'write-tree', env=env).decode('ascii').strip()
        digest = hashlib.sha256()
        object_format = _git(root, 'rev-parse', '--show-object-format', env=env).decode('ascii').strip()
        if object_format not in ('sha1', 'sha256'):
            raise ValueError('unsupported Git object format')
        raw_mismatches = []
        for entry in entries.split(b'\0'):
            if not entry:
                continue
            metadata, raw_name = entry.split(b'\t', 1)
            mode, oid, stage = metadata.decode('ascii').split()
            if stage != '0':
                raise ValueError('unmerged source index')
            name = raw_name.decode('utf-8')
            path = root / name
            info = path.lstat()
            digest.update(len(raw_name).to_bytes(8, 'big'))
            digest.update(raw_name)
            digest.update(stat.S_IMODE(info.st_mode).to_bytes(4, 'big'))
            if stat.S_ISLNK(info.st_mode):
                raise ValueError('symbolic link inputs are not supported by local snapshots')
            elif stat.S_ISREG(info.st_mode):
                digest.update(b'F' + info.st_size.to_bytes(8, 'big'))
                blob = hashlib.new(object_format, b'blob ' + str(info.st_size).encode() + b'\0')
                with path.open('rb') as stream:
                    for block in iter(lambda: stream.read(1024 * 1024), b''):
                        digest.update(block)
                        blob.update(block)
            else:
                raise ValueError('unsupported source file type: ' + name)
            if blob.hexdigest() != oid or (os.name != 'nt' and mode in ('100644', '100755') and
                                           bool(info.st_mode & 0o111) != (mode == '100755')):
                raw_mismatches.append(name)
        return tree, digest.hexdigest(), raw_mismatches


def capture(root, head):
    """Return a stable content tree and changed paths against an exact HEAD.

    The capture is repeated to reject ordinary concurrent source changes. A
    preflight must also recheck this identity after each command and account
    separately for ignored build inputs before treating any receipt as valid.
    """
    root = Path(root).resolve()
    if not re.fullmatch(r'(?:[0-9a-f]{40}|[0-9a-f]{64})', head):
        raise ValueError('full HEAD identity required')
    actual = _git(root, 'rev-parse', 'HEAD').decode('ascii').strip()
    if actual != head:
        raise ValueError('HEAD changed or does not match the requested identity')
    _reject_unresolved_inputs(_git(root, 'ls-files', '--stage', '-z'))
    first = _capture_tree(root, head)
    second = _capture_tree(root, head)
    if first != second or _git(root, 'rev-parse', 'HEAD').decode('ascii').strip() != head:
        raise ValueError('source changed during capture')
    return {'schema': 'voice-local-source-snapshot-v1', 'head': head,
            'tree': first[0], 'worktree_sha256': first[1],
            'raw_mismatch_paths': first[2],
            'changed_paths': changed_paths(root, head, first[0], git=_git)}


def _snapshot_entries(root, source):
    entries = _git(root, 'ls-tree', '-r', '-z', '--full-tree', source['tree'])
    for entry in sorted((e for e in entries.split(b'\0') if e), key=lambda e: e.split(b'\t', 1)[1]):
        metadata, raw_name = entry.split(b'\t', 1)
        mode, kind, _ = metadata.decode('ascii').split()
        name = raw_name.decode('utf-8')
        parts = name.split('/')
        if (mode not in ('100644', '100755') or kind != 'blob' or
                any(part in ('', '.', '..', '.git') or '\\' in part or ':' in part
                    for part in parts)):
            raise ValueError('unsupported snapshot path or file type: ' + name)
        yield raw_name, parts


def verify_materialized(root, source, destination, allowed_extra=()):
    """Check captured files and reject extra files outside declared outputs.

    Allowed output paths are not bound by this check and cannot by themselves
    support a complete preflight receipt.
    """
    root = Path(root).resolve()
    destination = Path(destination).resolve()
    if not isinstance(allowed_extra, (list, tuple)):
        raise ValueError('invalid declared output paths')
    files, directories = set(), set()
    for name in allowed_extra:
        if not isinstance(name, str) or not name or '\\' in name or ':' in name:
            raise ValueError('invalid declared output path')
        value = name[:-1] if name.endswith('/') else name
        path = PurePosixPath(value)
        if (not value or value in ('.', '..') or path.is_absolute() or str(path) != value or
                any(part in ('', '.', '..', '.git') for part in path.parts)):
            raise ValueError('invalid declared output path')
        (directories if name.endswith('/') else files).add(value)
    entries = list(_snapshot_entries(root, source))
    tracked = {raw_name.decode('utf-8') for raw_name, _ in entries}
    digest = hashlib.sha256()
    for raw_name, parts in entries:
        path = destination.joinpath(*parts)
        try:
            info = path.lstat()
            if not stat.S_ISREG(info.st_mode) or destination not in path.resolve().parents:
                raise ValueError('snapshot changed: invalid file')
            digest.update(len(raw_name).to_bytes(8, 'big'))
            digest.update(raw_name)
            digest.update(stat.S_IMODE(info.st_mode).to_bytes(4, 'big'))
            digest.update(b'F' + info.st_size.to_bytes(8, 'big'))
            with path.open('rb') as stream:
                for block in iter(lambda: stream.read(1024 * 1024), b''):
                    digest.update(block)
        except OSError as error:
            raise ValueError('snapshot changed: missing file') from error
    if digest.hexdigest() != source['worktree_sha256']:
        raise ValueError('snapshot changed: captured bytes or mode differ')
    def fail_walk(error):
        raise ValueError('snapshot changed: directory cannot be inspected') from error
    for parent, child_dirs, child_files in os.walk(destination, followlinks=False, onerror=fail_walk):
        parent_path = Path(parent)
        for name in list(child_dirs):
            path = parent_path / name
            relative = path.relative_to(destination).as_posix()
            if path.is_symlink() or getattr(path, 'is_junction', lambda: False)():
                raise ValueError('snapshot changed: unexpected link ' + relative)
            if relative in directories:
                child_dirs.remove(name)
        for name in child_files:
            path = parent_path / name
            relative = path.relative_to(destination).as_posix()
            if not stat.S_ISREG(path.lstat().st_mode) or (relative not in tracked and relative not in files):
                raise ValueError('snapshot changed: unexpected file ' + relative)


def materialize(root, source, destination):
    """Copy the exact Git-visible working-tree bytes to a new private directory.

    This does not capture ignored inputs or prove that a command ran. A failed
    copy is retained for inspection and must never be used as evidence.
    """
    root = Path(root).resolve()
    destination = Path(destination).resolve()
    if destination == root or root in destination.parents:
        raise ValueError('snapshot destination must be outside the source tree')
    if destination.exists():
        raise ValueError('snapshot destination already exists')
    if capture(root, source['head']) != source:
        raise ValueError('source changed before materialization')
    destination.mkdir(parents=True, exist_ok=False)
    for raw_name, parts in _snapshot_entries(root, source):
        name = raw_name.decode('utf-8')
        original = root.joinpath(*parts)
        if not original.is_file() or original.is_symlink() or root not in original.resolve().parents:
            raise ValueError('unsupported or changed source path: ' + name)
        target = destination.joinpath(*parts)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(original, target)
        shutil.copymode(original, target)
    try:
        verify_materialized(root, source, destination)
    except ValueError as error:
        raise ValueError('source changed during materialization') from error
    if capture(root, source['head']) != source:
        raise ValueError('source changed during materialization')
    return destination


def archive(root, source, destination, output):
    """Archive exactly the verified snapshot and return its SHA-256 digest."""
    root, destination, output = Path(root).resolve(), Path(destination).resolve(), Path(output).resolve()
    if (output == root or root in output.parents or output == destination or
            destination in output.parents or output.exists()):
        raise ValueError('archive must be a new file outside both source trees')
    verify_materialized(root, source, destination)
    content_hash = hashlib.sha256()

    class HashingReader:
        def __init__(self, stream):
            self.stream = stream

        def read(self, size=-1):
            data = self.stream.read(size)
            content_hash.update(data)
            return data

    with output.open('x+b') as raw:
        with tarfile.open(fileobj=raw, mode='w') as tar:
            for name, parts in _snapshot_entries(root, source):
                path = destination.joinpath(*parts)
                before = path.lstat()
                if not stat.S_ISREG(before.st_mode) or destination not in path.resolve().parents:
                    raise ValueError('snapshot changed: invalid archive input')
                with path.open('rb') as stream:
                    current = os.fstat(stream.fileno())
                    if (current.st_dev, current.st_ino, current.st_size, current.st_mtime_ns) != (
                            before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns):
                        raise ValueError('snapshot changed before archive read')
                    content_hash.update(len(name).to_bytes(8, 'big'))
                    content_hash.update(name)
                    content_hash.update(stat.S_IMODE(current.st_mode).to_bytes(4, 'big'))
                    content_hash.update(b'F' + current.st_size.to_bytes(8, 'big'))
                    info = tarfile.TarInfo(name.decode('utf-8'))
                    info.size = current.st_size
                    info.mode = stat.S_IMODE(current.st_mode)
                    tar.addfile(info, HashingReader(stream))
                    after = os.fstat(stream.fileno())
                    if (after.st_size, after.st_mtime_ns) != (current.st_size, current.st_mtime_ns):
                        raise ValueError('snapshot changed during archive read')
        if content_hash.hexdigest() != source['worktree_sha256']:
            raise ValueError('archive bytes differ from captured source')
        raw.flush()
        raw.seek(0)
        archived_hash = hashlib.sha256()
        entries = list(_snapshot_entries(root, source))
        with tarfile.open(fileobj=raw, mode='r:') as check:
            members = check.getmembers()
            if len(members) != len(entries):
                raise ValueError('archive entries differ from captured source')
            for member, (name, _) in zip(members, entries):
                if not member.isfile() or member.name != name.decode('utf-8'):
                    raise ValueError('archive entries differ from captured source')
                archived_hash.update(len(name).to_bytes(8, 'big'))
                archived_hash.update(name)
                archived_hash.update(member.mode.to_bytes(4, 'big'))
                archived_hash.update(b'F' + member.size.to_bytes(8, 'big'))
                with check.extractfile(member) as stream:
                    for block in iter(lambda: stream.read(1024 * 1024), b''):
                        archived_hash.update(block)
        if archived_hash.hexdigest() != source['worktree_sha256']:
            raise ValueError('archive bytes differ from captured source')
        digest = hashlib.sha256()
        raw.seek(0)
        for block in iter(lambda: raw.read(1024 * 1024), b''):
            digest.update(block)
        opened, path_info = os.fstat(raw.fileno()), output.lstat()
        if (not stat.S_ISREG(path_info.st_mode) or
                (opened.st_dev, opened.st_ino) != (path_info.st_dev, path_info.st_ino)):
            raise ValueError('archive path changed during archival')
    verify_materialized(root, source, destination)
    if capture(root, source['head']) != source:
        raise ValueError('source changed during archival')
    return digest.hexdigest()


def main():
    parser = argparse.ArgumentParser(description='Plan Git-visible local source changes')
    parser.add_argument('--base', required=True, help='full base commit identity')
    parser.add_argument('--head', required=True, help='full checked-out HEAD identity')
    parser.add_argument('--head-ref', default='')
    args = parser.parse_args()
    from affected_check_plan import build_local_plan
    root = Path(__file__).resolve().parents[2]
    print(json.dumps(build_local_plan(root, args.base, args.head, args.head_ref),
                     sort_keys=True, indent=2))


if __name__ == '__main__':
    main()
