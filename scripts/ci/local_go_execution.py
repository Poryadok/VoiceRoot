"""Source transfer for an owned Go diagnostic; this does not issue a PASS receipt."""
import hashlib
import os
from pathlib import Path
import stat
import tempfile

import local_source_snapshot as snapshot


def _digest(stream):
    stream.seek(0)
    value = hashlib.sha256()
    for block in iter(lambda: stream.read(1024 * 1024), b''):
        value.update(block)
    return value.hexdigest()


def transfer_source(owner, name, root, source, source_copy, archive_path):
    """Transfer only an archive built from the captured source into an owned volume."""
    containers = [item for item in owner.state['resources'] if item.get('kind') == 'container'
                  and item.get('name') == name and item.get('state') == 'ready']
    if len(containers) != 1 or '/workspace' not in containers[0].get('mounts', {}):
        raise ValueError('owned workspace container is required')
    digest = snapshot.archive(root, source, source_copy, archive_path)
    expected_remote = (digest + '  /workspace/candidate.tar\n').encode('ascii')
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        def run(command, stdin=None):
            out.seek(0)
            out.truncate()
            err.seek(0)
            err.truncate()
            result = owner.execute(name, command, out, err, 300, stdin=stdin) if stdin is not None else owner.execute(name, command, out, err, 300)
            if result != 0:
                raise ValueError('owned source transfer command failed')
            out.seek(0)
            return out.read()

        if run(['ls', '-A', '--', '/workspace']):
            raise ValueError('owned workspace must be empty')
        with Path(archive_path).open('rb') as archive:
            before = os.fstat(archive.fileno())
            path_info = Path(archive_path).lstat()
            if (not stat.S_ISREG(path_info.st_mode) or
                    (path_info.st_dev, path_info.st_ino) != (before.st_dev, before.st_ino)):
                raise ValueError('local archive path changed before transfer')
            if _digest(archive) != digest:
                raise ValueError('local archive changed before transfer')
            archive.seek(0)
            run(['sh', '-c', 'umask 077; set -C; cat > /workspace/candidate.tar'], stdin=archive)
            if _digest(archive) != digest:
                raise ValueError('local archive changed during transfer')
            after = os.fstat(archive.fileno())
            path_info = Path(archive_path).lstat()
            if (not stat.S_ISREG(path_info.st_mode) or
                    (path_info.st_dev, path_info.st_ino) != (after.st_dev, after.st_ino)):
                raise ValueError('local archive path changed during transfer')
            if (before.st_size, before.st_mtime_ns) != (after.st_size, after.st_mtime_ns):
                raise ValueError('local archive metadata changed during transfer')
        if run(['sha256sum', '/workspace/candidate.tar']) != expected_remote:
            raise ValueError('container archive differs from captured source')
        run(['mkdir', '--', '/workspace/source'])
        run(['tar', '-xf', '/workspace/candidate.tar', '-C', '/workspace/source'])
        if run(['sha256sum', '/workspace/candidate.tar']) != expected_remote:
            raise ValueError('container archive changed during extraction')
    snapshot.verify_materialized(root, source, source_copy)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed during transfer')
    return digest
