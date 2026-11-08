"""Selected Auth diagnostic on an already owned private daemon; never a PASS receipt.

Caller holds the scheduled slot and enforces exclusive daemon/socket access.
The supplied Maven cache, client image, helper and PostgreSQL image are not
authenticated. Failures leave cleanup and uncertain-operation recovery to owner.
"""
import copy
import hashlib
import os
from pathlib import Path, PurePosixPath
import re
import stat
import tarfile
import tempfile
import xml.etree.ElementTree as ET

import local_chat_private
import local_go_execution
import local_source_snapshot as snapshot

CLASS = 'voice.backend.auth.AccountDeletionEpochFloorJdbcIntegrationTest'
CASE = 'floorFailureRollsBackJdbcDeletionAndItsPendingFloorOperation'
BUILD_MTIME = 946684800  # 2000-01-01 UTC; Maven ignores zero-mtime resources.


def normalize_archive(source, output, expected_digest):
    """Copy the verified source archive with only regular-file mtimes changed."""
    source, output = Path(source), Path(output)
    if not re.fullmatch(r'[0-9a-f]{64}', expected_digest):
        raise ValueError('invalid source archive digest')
    def identity(path, stream, expected=None):
        opened, current = os.fstat(stream.fileno()), path.lstat()
        value = (opened.st_dev, opened.st_ino, opened.st_size, opened.st_mtime_ns)
        if (not stat.S_ISREG(opened.st_mode) or not stat.S_ISREG(current.st_mode) or
                value != (current.st_dev, current.st_ino, current.st_size, current.st_mtime_ns) or
                expected is not None and value != expected):
            raise ValueError('archive identity changed')
        return value
    with source.open('rb') as raw, output.open('x+b') as result:
        initial = identity(source, raw)
        if hashlib.file_digest(raw, 'sha256').hexdigest() != expected_digest:
            raise ValueError('source archive changed before normalization')
        identity(source, raw, initial)
        raw.seek(0)
        with tarfile.open(fileobj=raw, mode='r:') as original, tarfile.open(fileobj=result, mode='w') as target:
            seen = set()
            for item in original:
                name, path = item.name, PurePosixPath(item.name)
                if (not item.isfile() or not name or path.is_absolute() or str(path) != name or
                        any(part in ('', '.', '..') or '\\' in part or ':' in part for part in path.parts) or name in seen):
                    raise ValueError('unsafe source archive entry')
                seen.add(name)
                member = copy.copy(item)
                member.mtime = BUILD_MTIME
                target.addfile(member, original.extractfile(item))
        result.flush()
        os.fsync(result.fileno())
        identity(source, raw, initial)
        written = identity(output, result)
        raw.seek(0); result.seek(0)
        with tarfile.open(fileobj=raw, mode='r:') as original, tarfile.open(fileobj=result, mode='r:') as target:
            left, right = original.getmembers(), target.getmembers()
            if len(left) != len(right):
                raise ValueError('normalized archive entries changed')
            for a, b in zip(left, right):
                if ((a.name, a.size, a.mode, a.type) != (b.name, b.size, b.mode, b.type) or b.mtime != BUILD_MTIME or
                        hashlib.file_digest(original.extractfile(a), 'sha256').digest() !=
                        hashlib.file_digest(target.extractfile(b), 'sha256').digest()):
                    raise ValueError('normalized archive contents changed')
        identity(source, raw, initial); identity(output, result, written)
        raw.seek(0); result.seek(0)
        if hashlib.file_digest(raw, 'sha256').hexdigest() != expected_digest:
            raise ValueError('source archive changed during normalization')
        digest = hashlib.file_digest(result, 'sha256').hexdigest()
        identity(source, raw, initial); identity(output, result, written)
        return digest


def validate_report(raw):
    try:
        suite = ET.fromstring(raw)
    except ET.ParseError as error:
        raise ValueError('invalid Surefire report') from error
    if (suite.tag != 'testsuite' or suite.get('name') != CLASS or
            any(suite.get(k) != v for k, v in
                [('tests', '1'), ('failures', '0'), ('errors', '0'), ('skipped', '0')])):
        raise ValueError('unexpected Surefire suite')
    cases = suite.findall('testcase')
    if (len(cases) != 1 or cases[0].get('classname') != CLASS or cases[0].get('name') != CASE or
            any(cases[0].find(tag) is not None for tag in ('failure', 'error', 'skipped'))):
        raise ValueError('required Auth case did not pass')


def cache_digest(path):
    """Validate an uncompressed regular-file-only Maven cache archive."""
    path = Path(path)
    with path.open('rb') as stream:
        before = os.fstat(stream.fileno())
        if not stat.S_ISREG(before.st_mode) or path.is_symlink():
            raise ValueError('cache archive must be a regular file')
        names = set()
        with tarfile.open(fileobj=stream, mode='r:') as archive:
            for member in archive:
                name = member.name
                p = PurePosixPath(name)
                if (not member.isfile() or not name or p.is_absolute() or str(p) != name or
                        any(part in ('', '.', '..') or '\\' in part or ':' in part for part in p.parts) or
                        name in names):
                    raise ValueError('unsafe cache archive entry')
                names.add(name)
        if not names:
            raise ValueError('empty cache archive')
        stream.seek(0)
        digest = hashlib.file_digest(stream, 'sha256').hexdigest()
        after, current = os.fstat(stream.fileno()), path.lstat()
        if ((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) !=
                (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns) or
                (after.st_dev, after.st_ino) != (current.st_dev, current.st_ino)):
            raise ValueError('cache archive changed')
        return digest


def verify_build(owner, client, root, source, build_digest):
    if not isinstance(build_digest, str) or not re.fullmatch(r'[0-9a-f]{64}', build_digest):
        raise ValueError('invalid build source digest')
    entries = [name for name, _ in snapshot._snapshot_entries(root, source)]
    if any(name.startswith(b'src/backend/auth/target/') or name == b'src/backend/auth/target' for name in entries):
        raise ValueError('captured source overlaps Maven output')
    expected = b''.join(b'./' + name + b'\0' for name in entries)
    digest_check = (['sha256sum', '/workspace/build-source.tar'],
                    (build_digest + '  /workspace/build-source.tar\n').encode('ascii'))
    checks = [digest_check, (['sh', '-c', 'test ! -L /workspace/build/src/backend/auth/target'], b''),
              (['sh', '-c', 'cd /workspace/build && find . -path ./src/backend/auth/target -prune -o ! -type f ! -type d -print -quit'], b''),
              (['tar', '-df', '/workspace/build-source.tar', '-C', '/workspace/build'], b''),
              (['sh', '-c', 'cd /workspace/build && find . -path ./src/backend/auth/target -prune -o -type f -print0 | LC_ALL=C sort -z'], expected),
              digest_check]
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        for command, expected_output in checks:
            out.seek(0); out.truncate(); err.seek(0); err.truncate()
            if owner.execute(client, command, out, err, 300):
                raise ValueError('build source verification failed')
            out.seek(0)
            if out.read() != expected_output:
                raise ValueError('build source changed')


def fixture_script():
    # The classpath marker is informational, never a preflight acceptance check.
    return ('set -eu; cd /workspace/build/src/backend/auth; pwd; '
            'report=target/surefire-reports/TEST-' + CLASS + '.xml; '
            'if test -f "$report"; then '
            'if grep -Eq \'<property name="java.class.path" value="([^\"]*:)?/workspace/build/src/backend/auth/target/test-classes(:|\")\' "$report"; '
            'then printf "test-classes-in-classpath=yes\\n"; else printf "test-classes-in-classpath=no-or-unparsed\\n"; fi; '
            'else printf "surefire-report=absent\\n"; fi; '
            'if test -f target/test-classes/application-integration.properties; '
            'then printf "test-fixture=present\\n"; else printf "test-fixture=missing\\n"; exit 1; fi; '
            'if test -f src/test/resources/application-integration.properties; '
            'then printf "source-fixture=present\\n"; else printf "source-fixture=missing\\n"; exit 1; fi; '
            'if command -v cmp >/dev/null 2>&1; then printf "cmp=present\\n"; '
            'else printf "cmp=missing\\n"; exit 1; fi; '
            'if cmp -s src/test/resources/application-integration.properties target/test-classes/application-integration.properties; '
            'then printf "fixture-compare=match\\n"; else printf "fixture-compare=mismatch-or-error\\n"; exit 1; fi; '
            'sha256sum target/test-classes/application-integration.properties')


def run_selected(owner, daemon, client, root, source, source_copy, cache, output):
    root, output = Path(root).resolve(), Path(output).resolve()
    if output.exists() or output == root or root in output.parents:
        raise ValueError('output must be new and outside source tree')
    def verify_source():
        snapshot.verify_materialized(root, source, source_copy)
        if snapshot.capture(root, source['head']) != source:
            raise ValueError('source changed during Auth diagnostic')
    address = local_chat_private.inspect_pair(owner, daemon, client)
    verify_source()
    digest = cache_digest(cache)
    output.mkdir(parents=True)
    source_digest = local_go_execution.transfer_source(owner, client, root, source, source_copy, output / 'source.tar')
    build_digest = normalize_archive(output / 'source.tar', output / 'build-source.tar', source_digest)
    def run(command, stdin=None):
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            if owner.execute(client, command, out, err, 1800, stdin=stdin):
                raise ValueError('Auth client command failed')
            out.seek(0)
            return out.read()
    with Path(cache).open('rb') as stream:
        run(['sh', '-c', 'umask 077; set -C; cat > /workspace/maven-cache.tar'], stream)
    if cache_digest(cache) != digest or run(['sha256sum', '/workspace/maven-cache.tar']) != (digest + '  /workspace/maven-cache.tar\n').encode():
        raise ValueError('Maven cache transfer changed')
    run(['sh', '-c', 'test ! -e /workspace/m2 && mkdir /workspace/m2 && tar -xf /workspace/maven-cache.tar -C /workspace/m2'])
    with (output / 'build-source.tar').open('rb') as stream:
        run(['sh', '-c', 'umask 077; set -C; cat > /workspace/build-source.tar'], stream)
    if run(['sha256sum', '/workspace/build-source.tar']) != (build_digest + '  /workspace/build-source.tar\n').encode():
        raise ValueError('build source transfer changed')
    local_go_execution._verify_workspace(owner, client, root, source, source_digest)
    run(['sh', '-c', 'test ! -e /workspace/build && mkdir /workspace/build && tar -xf /workspace/build-source.tar -C /workspace/build && tar -df /workspace/build-source.tar -C /workspace/build'])
    verify_build(owner, client, root, source, build_digest)
    if local_chat_private.inspect_pair(owner, daemon, client) != address:
        raise ValueError('private daemon address changed')
    command = ['sh', '-c', 'cd /workspace/build/src/backend/auth && exec env '
               'DOCKER_HOST=unix:///socket/docker.sock TESTCONTAINERS_RYUK_DISABLED=true '
               'TESTCONTAINERS_HOST_OVERRIDE="$1" VOICE_RELEASE_PROFILE=development '
               'MAVEN_OPTS="-Xmx512m -XX:ActiveProcessorCount=2" '
               'mvn -o -B -ntp -Dmaven.repo.local=/workspace/m2 '
               '-Dtest=AccountDeletionEpochFloorJdbcIntegrationTest#' + CASE + ' '
               '-DargLine="-Xmx768m -XX:ActiveProcessorCount=2" test', 'voice-private-auth', address]
    with (output / 'maven.stdout').open('xb') as out, (output / 'maven.stderr').open('xb') as err:
        code = owner.execute(client, command, out, err, 1800)
    with (output / 'fixture-diagnostic.txt').open('xb') as out, (output / 'fixture-diagnostic.stderr').open('xb') as err:
        fixture = ['sh', '-c', fixture_script()]
        if owner.execute(client, fixture, out, err, 30):
            raise ValueError('Auth test fixture diagnostics failed')
    if code:
        raise ValueError('Auth Maven test failed')
    report = run(['cat', '/workspace/build/src/backend/auth/target/surefire-reports/TEST-' + CLASS + '.xml'])
    validate_report(report)
    (output / 'surefire.xml').write_bytes(report)
    if local_chat_private.inspect_pair(owner, daemon, client) != address:
        raise ValueError('private daemon address changed')
    local_go_execution._verify_workspace(owner, client, root, source, source_digest)
    verify_build(owner, client, root, source, build_digest)
    verify_source()
    return {'status': 'incomplete', 'source': source, 'source_archive_sha256': source_digest,
            'build_archive_sha256': build_digest, 'build_mtime': BUILD_MTIME,
            'maven_cache_sha256': digest, 'report_sha256': hashlib.sha256(report).hexdigest(), 'command': command,
            'unbound_inputs': ['Maven cache and tool provenance', 'image and helper provenance',
                               'exclusive daemon and socket access', 'Maven process and daemon crash acceptance',
                               'archive and report size and filesystem I/O bounds']}
