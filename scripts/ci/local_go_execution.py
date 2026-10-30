"""Source transfer for an owned Go diagnostic; this does not issue a PASS receipt."""
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import tarfile
import tempfile

import go_integration_receipts as receipts
import local_go_verify
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


def _module_path(module_dir):
    if (not isinstance(module_dir, str) or not re.fullmatch(r'[A-Za-z0-9_./-]+', module_dir)
            or str(PurePosixPath(module_dir)) != module_dir
            or any(part in ('', '.', '..', '.git') for part in module_dir.split('/'))):
        raise ValueError('invalid module directory')
    return '/workspace/source/' + module_dir


def _verify_modules(owner, name, module_dir):
    """Reject changed cached module archives or extracted contents.

    Go's verifier trusts cached ziphash files; acquiring and binding those
    hashes to the captured source remains a separate preflight requirement.
    """
    _module_path(module_dir)
    command = ['sh', '-c', 'cd "/workspace/source/$1" && '
               'exec env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOMAXPROCS=2 '
               'GOMODCACHE=/workspace/go-mod go mod verify', 'voice-go-verify', module_dir]
    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        result = owner.execute(name, command, stdout, stderr, 1800)
        stdout.seek(0)
        out = stdout.read()
        stderr.seek(0)
        err = stderr.read()
        local_module = b''
        if local_go_verify.is_local_failure(result, out, err):
            stdout.seek(0)
            stdout.truncate()
            stderr.seek(0)
            stderr.truncate()
            command = ['sh', '-c', 'cd "/workspace/source/$1" && '
                       'exec env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOMAXPROCS=2 '
                       'GOMODCACHE=/workspace/go-mod go list -m -json voice/backend/pkg', 'voice-go-local-module', module_dir]
            if owner.execute(name, command, stdout, stderr, 300) != 0:
                raise ValueError('Go local replacement inspection failed')
            stdout.seek(0)
            local_module = stdout.read()
        local_go_verify.verify_result(result, out, err, local_module, '/workspace/source/src/backend/pkg')


def transfer_dependencies(owner, name, root, source, source_copy, source_archive_digest, acquisition, archive_path):
    """Transfer a checked cache into the owned workspace; this is not PASS."""
    root = Path(root).resolve()
    if (not isinstance(acquisition, dict) or acquisition.get('status') != 'incomplete' or
            acquisition.get('source') != source or
            acquisition.get('target') != {'goos': 'linux', 'goarch': 'amd64', 'cgo_enabled': False}):
        raise ValueError('dependency acquisition does not match captured source')
    module_dir = acquisition.get('module_dir')
    _module_path(module_dir)
    digest = acquisition.get('archive_sha256')
    host_version = acquisition.get('toolchain_version')
    if (not isinstance(digest, str) or not re.fullmatch(r'[0-9a-f]{64}', digest) or
            not isinstance(host_version, str) or
            not re.fullmatch(r'go version go[0-9]+\.[0-9]+(?:\.[0-9]+)? [a-z0-9]+/[a-z0-9]+', host_version)):
        raise ValueError('invalid dependency acquisition identity')
    snapshot.verify_materialized(root, source, source_copy)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed before dependency transfer')
    _verify_workspace(owner, name, root, source, source_archive_digest)
    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        def run(command, stdin=None):
            stdout.seek(0)
            stdout.truncate()
            stderr.seek(0)
            stderr.truncate()
            result = owner.execute(name, command, stdout, stderr, 300, stdin=stdin) if stdin is not None else owner.execute(name, command, stdout, stderr, 300)
            if result != 0:
                raise ValueError('owned dependency transfer command failed')
            stdout.seek(0)
            return stdout.read()
        remote_version = run(['go', 'version']).decode('ascii').strip()
        if (not re.fullmatch(r'go version go[0-9]+\.[0-9]+(?:\.[0-9]+)? linux/amd64', remote_version)
                or remote_version.split()[2] != host_version.split()[2]):
            raise ValueError('host and owned container Go toolchains differ')
        if run(['sh', '-c', 'test ! -e /workspace/dependencies.tar && test ! -e /workspace/go-mod']):
            raise ValueError('unexpected dependency workspace output')
        with Path(archive_path).open('rb') as archive:
            before = os.fstat(archive.fileno())
            path_info = Path(archive_path).lstat()
            if (not stat.S_ISREG(path_info.st_mode) or
                    (path_info.st_dev, path_info.st_ino) != (before.st_dev, before.st_ino) or
                    _digest(archive) != digest):
                raise ValueError('local dependency archive changed')
            archive.seek(0)
            with tarfile.open(fileobj=archive, mode='r:') as tar:
                names = set()
                for member in tar:
                    parts = member.name.split('/')
                    if (not member.isfile() or member.name in names or len(parts) < 4 or
                            parts[:2] != ['cache', 'download'] or
                            any(part in ('', '.', '..') or '\\' in part or ':' in part for part in parts)):
                        raise ValueError('invalid dependency archive entry')
                    names.add(member.name)
            archive.seek(0)
            run(['sh', '-c', 'umask 077; set -C; cat > /workspace/dependencies.tar'], stdin=archive)
            if _digest(archive) != digest:
                raise ValueError('local dependency archive changed during transfer')
            after, path_info = os.fstat(archive.fileno()), Path(archive_path).lstat()
            if (not stat.S_ISREG(path_info.st_mode) or
                    (path_info.st_dev, path_info.st_ino) != (after.st_dev, after.st_ino) or
                    (before.st_size, before.st_mtime_ns) != (after.st_size, after.st_mtime_ns)):
                raise ValueError('local dependency archive path changed during transfer')
        expected = (digest + '  /workspace/dependencies.tar\n').encode('ascii')
        if run(['sha256sum', '/workspace/dependencies.tar']) != expected:
            raise ValueError('container dependency archive differs')
        run(['mkdir', '--', '/workspace/go-mod'])
        run(['tar', '-xf', '/workspace/dependencies.tar', '-C', '/workspace/go-mod'])
        if run(['tar', '-df', '/workspace/dependencies.tar', '-C', '/workspace/go-mod']):
            raise ValueError('extracted dependency cache differs')
        if run(['sha256sum', '/workspace/dependencies.tar']) != expected:
            raise ValueError('container dependency archive changed during extraction')
    _verify_modules(owner, name, module_dir)
    _verify_workspace(owner, name, root, source, source_archive_digest)
    snapshot.verify_materialized(root, source, source_copy)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed during dependency transfer')
    return {'status': 'incomplete', 'archive_sha256': digest, 'container_toolchain_version': remote_version}


def discover_packages(owner, name, root, source, source_copy, archive_digest, module_dir, output):
    """Discover the complete current module inventory offline in the owned source."""
    module_path = _module_path(module_dir)
    output = Path(output).resolve()
    root = Path(root).resolve()
    if output == root or root in output.parents or output.exists():
        raise ValueError('discovery output must be new and outside the source tree')
    snapshot.verify_materialized(root, source, source_copy)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed before Go discovery')
    _verify_modules(owner, name, module_dir)
    _verify_workspace(owner, name, root, source, archive_digest)
    output.mkdir(parents=True)
    command = ['sh', '-c', 'cd "/workspace/source/$1" && '
               'exec env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOMAXPROCS=2 '
               'GOCACHE=/workspace/go-build GOMODCACHE=/workspace/go-mod go list -json ./...',
               'voice-go-discovery', module_dir]
    with (output / 'packages.json').open('xb') as stdout, (output / 'stderr.log').open('xb') as stderr:
        result = owner.execute(name, command, stdout, stderr, 1800)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed during Go discovery')
    snapshot.verify_materialized(root, source, source_copy)
    _verify_modules(owner, name, module_dir)
    _verify_workspace(owner, name, root, source, archive_digest)
    if result != 0:
        raise ValueError('Go package discovery failed')
    raw = (output / 'packages.json').read_bytes()
    contents = raw.decode('utf-8')
    decoder = json.JSONDecoder()
    items = []
    while contents.strip():
        contents = contents.lstrip()
        item, end = decoder.raw_decode(contents)
        items.append(item)
        contents = contents[end:]
    tests, no_tests = [], []
    for item in items:
        if not isinstance(item, dict) or item.get('Error') or item.get('Incomplete'):
            raise ValueError('invalid Go package discovery result')
        module = item.get('Module')
        name_value = item.get('ImportPath')
        directory = item.get('Dir')
        if (not isinstance(module, dict) or module.get('Dir') != module_path or not module.get('Main')
                or not isinstance(module.get('Path'), str) or not isinstance(name_value, str)
                or not (name_value == module['Path'] or name_value.startswith(module['Path'] + '/'))
                or not isinstance(directory, str) or not (directory == module_path or directory.startswith(module_path + '/'))
                or any(part in ('.', '..') for part in directory.split('/'))):
            raise ValueError('package lies outside the selected Go module')
        if name_value != module['Path'] + directory[len(module_path):]:
            raise ValueError('Go package path does not match its source directory')
        if any(not isinstance(item.get(key, []), list) for key in ('TestGoFiles', 'XTestGoFiles')):
            raise ValueError('invalid Go package test inventory')
        (tests if item.get('TestGoFiles') or item.get('XTestGoFiles') else no_tests).append(name_value)
    if not items or len(set(tests + no_tests)) != len(items):
        raise ValueError('empty or duplicate Go package inventory')
    return {'status': 'incomplete', 'command': command, 'exit_code': result,
            'output_sha256': hashlib.sha256(raw).hexdigest(),
            'packages_with_tests': sorted(tests), 'packages_without_tests': sorted(no_tests)}


def run_package_diagnostic(owner, name, root, source, source_copy, archive_digest, module_dir, packages, output,
                           no_test_packages=()):
    """Run explicit Go packages offline in an owned workspace; never issue PASS.

    Package discovery and dependency provenance are not yet bound, so this
    diagnostic must not authorize a preflight handoff.
    """
    _module_path(module_dir)
    if (not isinstance(packages, list) or not packages
            or any(not isinstance(item, str) or not re.fullmatch(r'[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+', item)
                   or any(part in ('.', '..', '.git') for part in item.split('/')) for item in packages)
            or len(set(packages)) != len(packages)):
        raise ValueError('invalid Go package inventory')
    if (not isinstance(no_test_packages, (list, tuple)) or
            any(not isinstance(item, str) or item not in packages for item in no_test_packages) or
            len(set(no_test_packages)) != len(no_test_packages)):
        raise ValueError('invalid Go no-test package inventory')
    output = Path(output).resolve()
    root = Path(root).resolve()
    if output == root or root in output.parents or output.exists():
        raise ValueError('diagnostic output must be new and outside the source tree')
    snapshot.verify_materialized(root, source, source_copy)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed before Go diagnostic')
    _verify_modules(owner, name, module_dir)
    _verify_workspace(owner, name, root, source, archive_digest)
    output.mkdir(parents=True)
    command = ['sh', '-c',
               'cd "/workspace/source/$1" && shift && '
               'exec env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOMAXPROCS=2 '
               'GOCACHE=/workspace/go-build GOMODCACHE=/workspace/go-mod '
               'go test -short -json -count=1 -p 2 "$@"', 'voice-go-diagnostic', module_dir, *packages]
    with (output / 'events.jsonl').open('xb') as stdout, (output / 'stderr.log').open('xb') as stderr:
        result = owner.execute(name, command, stdout, stderr, 1800)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed during Go diagnostic')
    snapshot.verify_materialized(root, source, source_copy)
    _verify_modules(owner, name, module_dir)
    _verify_workspace(owner, name, root, source, archive_digest)
    if result != 0:
        raise ValueError('Go diagnostic command failed')
    raw = (output / 'events.jsonl').read_bytes()
    summary = receipts.validate_test_run(receipts.read_events(raw.decode('utf-8')), packages, no_test_packages)
    return {'status': 'incomplete', 'command': command, 'exit_code': result,
            'events_sha256': hashlib.sha256(raw).hexdigest(), 'tests': summary}
