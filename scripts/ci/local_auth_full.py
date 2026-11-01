"""Full Auth CI diagnostic for a pre-provisioned owned private Docker pair.

No result is a preflight PASS: tool/image/cache provenance, exclusive custody,
complete inventory classification and crash acceptance remain unbound.
"""
import hashlib
import io
import re
import tarfile
import tempfile
import time
import xml.etree.ElementTree as ET
from pathlib import Path

import local_auth_private as selected
import local_chat_private
import local_go_execution
import local_source_snapshot as snapshot

LOG_LIMIT = 8 * 1024 * 1024
REPORT_LIMIT = 256 * 1024 * 1024
COPY_SCRIPT = 'test -f "$1" && test ! -L "$1" && head -c "$2" -- "$1"'


def bounded_copy(owner, client, source, destination, limit):
    if not isinstance(limit, int) or limit <= 0 or not source.startswith('/workspace/'):
        raise ValueError('invalid bounded output request')
    with Path(destination).open('xb') as out, tempfile.TemporaryFile() as err:
        code = owner.execute(client, ['sh', '-c', COPY_SCRIPT, 'voice-bounded-copy', source, str(limit + 1)],
                             out, err, 300)
    if code:
        raise ValueError('bounded output transfer failed')
    if Path(destination).stat().st_size > limit:
        raise ValueError('bounded output exceeds limit')


def wait_daemon(owner, daemon, client, timeout=60):
    address = local_chat_private.inspect_pair(owner, daemon, client, shared_workspace=True)
    deadline = time.monotonic() + timeout
    while True:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise ValueError('private Docker socket did not become ready')
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            code = owner.execute(client, ['curl', '--silent', '--show-error', '--fail', '--max-time', '2',
                                          '--unix-socket', '/socket/docker.sock', 'http://localhost/_ping'],
                                 out, err, min(5, remaining))
            out.seek(0); ready = code == 0 and out.read(16) == b'OK'
        if local_chat_private.inspect_pair(owner, daemon, client, shared_workspace=True) != address:
            raise ValueError('private Docker endpoint changed')
        if ready:
            return
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise ValueError('private Docker socket did not become ready')
        time.sleep(min(.5, remaining))


def full_command(address):
    return ['bash', '-c', 'set -eu; ulimit -f 8192; cd /workspace/build; exec env '
            'DOCKER_HOST=unix:///socket/docker.sock TESTCONTAINERS_RYUK_DISABLED=true '
            'TESTCONTAINERS_HOST_OVERRIDE="$1" VOICE_RELEASE_PROFILE=development '
            'MAVEN_ARGS="-o -ntp" MAVEN_OPTS="-Xmx512m -XX:ActiveProcessorCount=2 -Dmaven.repo.local=/workspace/m2" '
            'make --no-print-directory auth-test-ci > /workspace/auth-make.stdout 2> /workspace/auth-make.stderr',
            'voice-private-auth-full', address]


def required_suites(source_copy):
    text = (Path(source_copy) / 'scripts/ci/check-auth-testcontainers-reports.sh').read_text(encoding='utf-8')
    match = re.search(r'(?m)^required_suites=\(\s*\n(.*?)^\)\s*$', text, re.DOTALL)
    if not match:
        raise ValueError('required Auth suite inventory unavailable')
    lines = [line.strip() for line in match.group(1).splitlines() if line.strip()]
    if (not lines or any(not re.fullmatch(r'"voice\.backend\.auth\.[A-Za-z0-9_.$]+"', line) for line in lines)
            or len(set(lines)) != len(lines)):
        raise ValueError('invalid required Auth suite inventory')
    return {line[1:-1] for line in lines}


def validate_reports(raw, required):
    if not isinstance(raw, bytes) or len(raw) > 256 * 1024 * 1024 or not isinstance(required, set) or not required:
        raise ValueError('invalid full Auth report input')
    suites, skipped, tests = set(), [], 0
    try:
        with tarfile.open(fileobj=io.BytesIO(raw), mode='r:') as archive:
            members = archive.getmembers()
            if not 1 <= len(members) <= 10000:
                raise ValueError('invalid Surefire inventory')
            for item in members:
                if (not item.isfile() or not re.fullmatch(r'\./TEST-[A-Za-z0-9_.$-]+\.xml', item.name) or
                        item.name in suites or item.size > 16 * 1024 * 1024):
                    raise ValueError('unsafe or duplicate Surefire report')
                suites.add(item.name)
                data = archive.extractfile(item).read()
                if len(data) != item.size or b'<!DOCTYPE' in data.upper() or b'<!ENTITY' in data.upper():
                    raise ValueError('invalid Surefire report bytes')
                suite = ET.fromstring(data)
                name = item.name[len('./TEST-'):-len('.xml')]
                if suite.tag != 'testsuite' or suite.get('name') != name:
                    raise ValueError('Surefire suite identity differs')
                counters = [suite.get(key) for key in ('tests', 'skipped', 'failures', 'errors')]
                if any(value is None or not re.fullmatch(r'[0-9]+', value) for value in counters):
                    raise ValueError('invalid Surefire counters')
                count, skip_count, failures, errors = map(int, counters)
                cases = suite.findall('testcase')
                if len(cases) != count or failures or errors:
                    raise ValueError('incomplete or failing Surefire suite')
                suite_skips = []
                for case in cases:
                    if (not case.get('name') or not case.get('classname') or
                            case.find('failure') is not None or case.find('error') is not None):
                        raise ValueError('invalid Surefire case')
                    if case.find('skipped') is not None:
                        suite_skips.append({'suite': name, 'case': case.get('name')})
                if len(suite_skips) != skip_count or (name in required and (count == 0 or skip_count)):
                    raise ValueError('required Surefire suite missing or skipped')
                skipped.extend(suite_skips)
                tests += count
    except (tarfile.TarError, ET.ParseError, OSError) as error:
        raise ValueError('invalid Surefire report archive') from error
    actual = {name[len('./TEST-'):-len('.xml')] for name in suites}
    if not required <= actual or tests < 1:
        raise ValueError('required Surefire suite missing')
    return {'suite_count': len(actual), 'required_suite_count': len(required), 'tests': tests,
            'skipped': sorted(skipped, key=lambda x: (x['suite'], x['case'])),
            'reports_sha256': hashlib.sha256(raw).hexdigest()}


def run_full(owner, daemon, client, root, source, source_copy, cache, output):
    """Execute the canonical make target, preserving an incomplete diagnostic."""
    root, output = Path(root).resolve(), Path(output).resolve()
    if output.exists() or output == root or root in output.parents:
        raise ValueError('output must be new and outside source tree')
    def verify_source():
        snapshot.verify_materialized(root, source, source_copy)
        if snapshot.capture(root, source['head']) != source:
            raise ValueError('source changed during full Auth diagnostic')
    address = local_chat_private.inspect_pair(owner, daemon, client, shared_workspace=True)
    verify_source()
    required = required_suites(source_copy)
    digest = selected.cache_digest(cache)
    output.mkdir(parents=True)
    source_digest = local_go_execution.transfer_source(owner, client, root, source, source_copy, output / 'source.tar')
    build_digest = selected.normalize_archive(output / 'source.tar', output / 'build-source.tar', source_digest)
    def run(command, stdin=None):
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            if owner.execute(client, command, out, err, 1800, stdin=stdin):
                raise ValueError('full Auth client command failed')
            out.seek(0)
            return out.read()
    with Path(cache).open('rb') as stream:
        run(['sh', '-c', 'umask 077; set -C; cat > /workspace/maven-cache.tar'], stream)
    if (selected.cache_digest(cache) != digest or run(['sha256sum', '/workspace/maven-cache.tar']) !=
            (digest + '  /workspace/maven-cache.tar\n').encode()):
        raise ValueError('Maven cache transfer changed')
    run(['sh', '-c', 'test ! -e /workspace/m2 && mkdir /workspace/m2 && tar -xf /workspace/maven-cache.tar -C /workspace/m2'])
    with (output / 'build-source.tar').open('rb') as stream:
        run(['sh', '-c', 'umask 077; set -C; cat > /workspace/build-source.tar'], stream)
    if run(['sha256sum', '/workspace/build-source.tar']) != (build_digest + '  /workspace/build-source.tar\n').encode():
        raise ValueError('build source transfer changed')
    local_go_execution._verify_workspace(owner, client, root, source, source_digest)
    run(['sh', '-c', 'test ! -e /workspace/build && mkdir /workspace/build && tar -xf /workspace/build-source.tar -C /workspace/build && tar -df /workspace/build-source.tar -C /workspace/build'])
    selected.verify_build(owner, client, root, source, build_digest)
    if local_chat_private.inspect_pair(owner, daemon, client, shared_workspace=True) != address:
        raise ValueError('private daemon address changed')
    command = full_command(address)
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        code = owner.execute(client, command, out, err, 7200)
    bounded_copy(owner, client, '/workspace/auth-make.stdout', output / 'make.stdout', LOG_LIMIT)
    bounded_copy(owner, client, '/workspace/auth-make.stderr', output / 'make.stderr', LOG_LIMIT)
    if code:
        raise ValueError('canonical Auth make target failed')
    archive_command = ['bash', '-o', 'pipefail', '-c',
        'set -eu; ulimit -f 262144; cd /workspace/build/src/backend/auth/target/surefire-reports; '
        '{ find . -maxdepth 1 -type f -name "TEST-*.xml" -print0 | LC_ALL=C sort -z | '
        'tar --null -T - -cf /workspace/auth-reports.tar; } > /workspace/auth-report.stdout 2> /workspace/auth-report.stderr']
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        report_code = owner.execute(client, archive_command, out, err, 300)
    bounded_copy(owner, client, '/workspace/auth-report.stdout', output / 'reports.stdout', LOG_LIMIT)
    bounded_copy(owner, client, '/workspace/auth-report.stderr', output / 'reports.stderr', LOG_LIMIT)
    if report_code:
        raise ValueError('cannot collect Auth Surefire reports')
    archive_path = output / 'reports.tar'
    bounded_copy(owner, client, '/workspace/auth-reports.tar', archive_path, REPORT_LIMIT)
    report_summary = validate_reports(archive_path.read_bytes(), required)
    if local_chat_private.inspect_pair(owner, daemon, client, shared_workspace=True) != address:
        raise ValueError('private daemon address changed')
    local_go_execution._verify_workspace(owner, client, root, source, source_digest)
    selected.verify_build(owner, client, root, source, build_digest)
    verify_source()
    return {'schema': 'voice-local-auth-full-diagnostic-v1', 'status': 'incomplete', 'source': source,
            'source_archive_sha256': source_digest, 'build_archive_sha256': build_digest,
            'maven_cache_sha256': digest, 'command': command, 'exit_code': code, 'reports': report_summary,
            'unbound_inputs': ['Maven cache and tool provenance', 'image and helper provenance',
                               'exclusive daemon and workspace custody', 'complete source test inventory classification',
                               'Maven process and daemon crash acceptance', 'archive and filesystem I/O bounds']}
