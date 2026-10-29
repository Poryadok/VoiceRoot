"""Snapshot Git-visible local source without changing the real index.

This captures tracked files and non-ignored untracked files, not ignored build
inputs. It is a planning primitive, not a proof that a check actually ran.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import tempfile

from affected_check_plan import changed_paths


def _git(root, *args, env=None):
    return subprocess.check_output(['git', '-C', str(root), *args],
                                   env=env, stderr=subprocess.PIPE)


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
            'changed_paths': changed_paths(root, head, first[0])}


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
    entries = _git(root, 'ls-tree', '-r', '-z', '--full-tree', source['tree'])
    destination.mkdir(parents=True, exist_ok=False)
    digest = hashlib.sha256()
    for entry in sorted((e for e in entries.split(b'\0') if e), key=lambda e: e.split(b'\t', 1)[1]):
        if not entry:
            continue
        metadata, raw_name = entry.split(b'\t', 1)
        mode, kind, _ = metadata.decode('ascii').split()
        name = raw_name.decode('utf-8')
        parts = name.split('/')
        if (mode not in ('100644', '100755') or kind != 'blob' or
                any(part in ('', '.', '..', '.git') or '\\' in part or ':' in part
                    for part in parts)):
            raise ValueError('unsupported snapshot path or file type: ' + name)
        original = root.joinpath(*parts)
        if not original.is_file() or original.is_symlink() or root not in original.resolve().parents:
            raise ValueError('unsupported or changed source path: ' + name)
        target = destination.joinpath(*parts)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(original, target)
        shutil.copymode(original, target)
        info = target.stat()
        digest.update(len(raw_name).to_bytes(8, 'big'))
        digest.update(raw_name)
        digest.update(stat.S_IMODE(info.st_mode).to_bytes(4, 'big'))
        digest.update(b'F' + info.st_size.to_bytes(8, 'big'))
        with target.open('rb') as stream:
            for block in iter(lambda: stream.read(1024 * 1024), b''):
                digest.update(block)
    if digest.hexdigest() != source['worktree_sha256'] or capture(root, source['head']) != source:
        raise ValueError('source changed during materialization')
    return destination


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
