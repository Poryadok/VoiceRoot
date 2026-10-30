"""Source transfer for an owned Go diagnostic; this does not issue a PASS receipt."""
import hashlib
import os
from pathlib import Path, PurePosixPath
import re
import stat
import tempfile

import go_integration_receipts as receipts
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


def _verify_workspace(owner, name, root, source, archive_digest):
    if not isinstance(archive_digest, str) or not re.fullmatch(r'[0-9a-f]{64}', archive_digest):
        raise ValueError('invalid transferred archive digest')
    expected_paths = b''.join(b'./' + name + b'\0' for name, _ in snapshot._snapshot_entries(root, source))
    checks = [(['sha256sum', '/workspace/candidate.tar'],
               (archive_digest + '  /workspace/candidate.tar\n').encode('ascii')),
              (['tar', '-df', '/workspace/candidate.tar', '-C', '/workspace/source'], b''),
              (['sh', '-c', 'cd /workspace/source && find . ! -type f ! -type d -print -quit'], b''),
              (['sh', '-c', 'cd /workspace/source && find . -type f -print0 | LC_ALL=C sort -z'], expected_paths)]
    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        for command, expected in checks:
            stdout.seek(0)
            stdout.truncate()
            stderr.seek(0)
            stderr.truncate()
            if owner.execute(name, command, stdout, stderr, 300) != 0:
                raise ValueError('owned source workspace verification failed')
            stdout.seek(0)
            if stdout.read() != expected:
                raise ValueError('owned source workspace changed')


def run_package_diagnostic(owner, name, root, source, source_copy, archive_digest, module_dir, packages, output):
    """Run explicit Go packages offline in an owned workspace; never issue PASS.

    Package discovery and dependency provenance are not yet bound, so this
    diagnostic must not authorize a preflight handoff.
    """
    if (not isinstance(module_dir, str) or not re.fullmatch(r'[A-Za-z0-9_./-]+', module_dir)
            or str(PurePosixPath(module_dir)) != module_dir
            or any(part in ('', '.', '..', '.git') for part in module_dir.split('/'))):
        raise ValueError('invalid module directory')
    if (not isinstance(packages, list) or not packages
            or any(not isinstance(item, str) or not re.fullmatch(r'[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+', item)
                   or any(part in ('.', '..', '.git') for part in item.split('/')) for item in packages)
            or len(set(packages)) != len(packages)):
        raise ValueError('invalid Go package inventory')
    output = Path(output).resolve()
    root = Path(root).resolve()
    if output == root or root in output.parents or output.exists():
        raise ValueError('diagnostic output must be new and outside the source tree')
    snapshot.verify_materialized(root, source, source_copy)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed before Go diagnostic')
    _verify_workspace(owner, name, root, source, archive_digest)
    output.mkdir(parents=True)
    command = ['sh', '-c',
               'cd "/workspace/source/$1" && shift && '
               'exec env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly '
               'GOCACHE=/workspace/go-build GOMODCACHE=/workspace/go-mod '
               'go test -json -count=1 "$@"', 'voice-go-diagnostic', module_dir, *packages]
    with (output / 'events.jsonl').open('xb') as stdout, (output / 'stderr.log').open('xb') as stderr:
        result = owner.execute(name, command, stdout, stderr, 1800)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed during Go diagnostic')
    snapshot.verify_materialized(root, source, source_copy)
    _verify_workspace(owner, name, root, source, archive_digest)
    if result != 0:
        raise ValueError('Go diagnostic command failed')
    raw = (output / 'events.jsonl').read_bytes()
    summary = receipts.validate_test_run(receipts.read_events(raw.decode('utf-8')), packages)
    return {'status': 'incomplete', 'command': command, 'exit_code': result,
            'events_sha256': hashlib.sha256(raw).hexdigest(), 'tests': summary}
