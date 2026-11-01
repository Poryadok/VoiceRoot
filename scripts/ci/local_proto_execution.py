"""Run source-bound Buf and R23 diagnostics without issuing a preflight pass.

Windows child processes run in an owned Job with bounded captured output;
POSIX timeout does not establish cleanup of descendants.
"""
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import tarfile

import go_integration_receipts
import local_private_git
import local_process_supervision as supervision
import local_source_snapshot


def _windows():
    return os.name == 'nt'


def _execute(command, *, cwd=None, env=None, input=None, timeout):
    wrapper = Path(__file__).with_name('local_golangci_capture.py')
    bounded = [sys.executable, '-B', str(wrapper), '--limit=16777216', *command]
    if _windows():
        return supervision.run_diagnostic(bounded, cwd=cwd, env=env, input=input,
                                          timeout=timeout, capture_output=True, check=False)
    return subprocess.run(bounded, cwd=cwd, env=env, input=input, timeout=timeout,
                          capture_output=True, check=False)


def _git(root, *args):
    result = _execute(['git', '-C', str(root), *args], timeout=120)
    result.check_returncode()
    return result.stdout.decode('utf-8').strip()


def _sha256(path):
    digest = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(block)
    return digest.hexdigest()


def archive_baseline(root, base, destination):
    if not isinstance(base, str) or not re.fullmatch(r'[0-9a-f]{40}', base):
        raise ValueError('a full immutable base SHA is required')
    root, destination = Path(root), Path(destination)
    if _git(root, 'rev-parse', '--verify', base + '^{commit}') != base:
        raise ValueError('base commit identity differs')
    destination.mkdir(parents=False, exist_ok=False)
    archive = destination.parent / 'base-protos.tar'
    result = _execute(['git', '-C', str(root), 'archive', '--format=tar', base, 'protos'], timeout=120)
    result.check_returncode()
    with archive.open('xb') as stream:
        stream.write(result.stdout)
    names = set()
    with tarfile.open(archive, 'r:') as tar:
        for member in tar.getmembers():
            path = PurePosixPath(member.name)
            if (not path.parts or path.parts[0] != 'protos' or path.is_absolute() or
                    any(part in ('', '.', '..') for part in path.parts) or
                    not (member.isfile() or member.isdir()) or member.name in names):
                raise ValueError('unsupported base archive entry')
            names.add(member.name)
        tar.extractall(destination, filter='data')
    if not (destination / 'protos/buf.yaml').is_file():
        raise ValueError('base Buf module is missing')
    return _sha256(archive)


def r23_summary(raw, exit_code):
    if exit_code != 0:
        raise ValueError('R23 contract tests failed')
    try:
        events = go_integration_receipts.read_events(raw.decode('utf-8'))
        summary = go_integration_receipts.validate_test_run(events, ['voice/r23-contract-test'])
    except (UnicodeError, ValueError, TypeError, KeyError) as error:
        raise ValueError('invalid R23 test event stream') from error
    if type(summary.get('passed')) is not int or summary['passed'] < 1:
        raise ValueError('R23 test inventory is empty')
    return summary


def r23_java_summary(raw, exit_code):
    summary = r23_summary(raw, exit_code)
    events = go_integration_receipts.read_events(raw.decode('utf-8'))
    expected = ('voice/r23-contract-test', 'TestR23GeneratedRuntimeConsumers/Java')
    passed = [(event.get('Package'), event.get('Test')) for event in events
              if event.get('Action') == 'pass' and event.get('Test')]
    if summary['skipped'] or passed.count(expected) != 1:
        raise ValueError('R23 Java consumer must pass without skipped tests')
    return summary


def _tool(value, name):
    located = shutil.which(str(value))
    if not located:
        raise ValueError(name + ' executable is unavailable')
    path = Path(located).resolve(strict=True)
    if not path.is_file():
        raise ValueError(name + ' executable is invalid')
    return path


def maven_options(repository):
    path = Path(repository).as_posix()
    if any(ord(character) < 32 or character in ('"', '\\') for character in path):
        raise ValueError('unsupported private Maven repository path')
    return '-Dmaven.repo.local="' + path + '"'


def run(root, source, base, output, go_binary='go', buf_binary='buf'):
    root, output = Path(root).resolve(), Path(output).resolve()
    if output == root or root in output.parents or output.exists():
        raise ValueError('protobuf output must be new and outside the checkout')
    output.mkdir(parents=True, exist_ok=False)
    report = {'schema': 'voice-local-protobuf-diagnostic-v1', 'status': 'failed',
              'source': source, 'base': base, 'commands': []}
    try:
        if not isinstance(source, dict) or not isinstance(source.get('head'), str):
            raise ValueError('captured source identity is required')
        head = source['head']
        if local_source_snapshot.capture(root, head) != source:
            raise ValueError('source changed before protobuf diagnostic')
        if _git(root, 'merge-base', '--is-ancestor', base, head) != '':
            raise ValueError('base ancestry is invalid')
        report['base_tree'] = _git(root, 'rev-parse', base + '^{tree}')
        go, buf = _tool(go_binary, 'Go'), _tool(buf_binary, 'Buf')
        maven, java, javac = _tool('mvn', 'Maven'), _tool('java', 'Java'), _tool('javac', 'javac')
        plugin, grpc = _tool('protoc-gen-go', 'Go protobuf generator'), _tool('protoc-gen-go-grpc', 'Go gRPC generator')
        git = _tool('git', 'Git')
        git_bash = git.parent.parent / 'bin/bash.exe' if os.name == 'nt' else Path('bash')
        bash = _tool(git_bash, 'Bash')
        if java.parent != javac.parent:
            raise ValueError('Java and javac must use the same JDK')
        hashes = {name: _sha256(path) for name, path in
                  (('go', go), ('buf', buf), ('maven', maven), ('java', java), ('javac', javac),
                   ('plugin', plugin), ('grpc', grpc), ('bash', bash), ('git', git))}
        env = {key: value for key, value in os.environ.items() if not key.upper().startswith('GIT_')}
        for key in ('GOFLAGS', 'GOPRIVATE', 'GONOPROXY', 'GONOSUMDB', 'GOINSECURE', 'GOCACHEPROG',
                    'BUF_TOKEN', 'BUF_GO_PB_CHECK_SKIP_SELF_TEST', 'MAVEN_ARGS', 'MAVEN_OPTS',
                    'JAVA_TOOL_OPTIONS', '_JAVA_OPTIONS', 'JDK_JAVA_OPTIONS'):
            env.pop(key, None)
        env.update(GOTOOLCHAIN='local', GOENV='off', GOWORK='off',
                   GOPATH=str(output / 'gopath'), GOMODCACHE=str(output / 'gomod'),
                   GOCACHE=str(output / 'gocache'), BUF_CACHE_DIR=str(output / 'buf-cache'),
                   GOPROXY='https://proxy.golang.org,direct', GOSUMDB='sum.golang.org',
                   PYTHONDONTWRITEBYTECODE='1', JAVA_HOME=str(java.parent.parent),
                   JAVA_TOOL_OPTIONS=maven_options(output / 'maven-repository'),
                   R23_CHECK_UNCOMMITTED_JAVA='1',
                   PATH=os.pathsep.join((str(go.parent), str(buf.parent), str(plugin.parent), str(grpc.parent),
                                         str(bash.parent), str(git.parent), str(maven.parent),
                                         str(java.parent), env.get('PATH', ''))))
        def probe(command):
            result = _execute(command, env=env, timeout=30)
            result.check_returncode()
            return result.stdout.decode('utf-8').strip()
        go_version = probe([str(go), 'version'])
        buf_version = probe([str(buf), '--version'])
        if not go_version.startswith('go version go1.26.10 ') or buf_version != '1.50.0':
            raise ValueError('protobuf diagnostic requires Go 1.26.10 and Buf 1.50.0')
        java_version = probe([str(javac), '-version'])
        maven_version = probe([str(maven), '--version'])
        if not re.match(r'^javac 25(?:\.|$)', java_version) or not maven_version.startswith('Apache Maven 3.9.'):
            raise ValueError('protobuf diagnostic requires JDK 25 and Maven 3.9')
        plugin_version = probe([str(plugin), '--version'])
        grpc_version = probe([str(grpc), '--version'])
        if not re.fullmatch(r'protoc-gen-go(?:\.exe)? v1\.36\.5', plugin_version) or grpc_version != 'protoc-gen-go-grpc 1.5.1':
            raise ValueError('protobuf diagnostic requires pinned Go protobuf generators')
        goroot = probe([str(go), 'env', 'GOROOT'])
        actual_go = (Path(goroot) / 'bin' / ('go.exe' if os.name == 'nt' else 'go')).resolve(strict=True)
        if actual_go != go:
            raise ValueError('Go executable is not the selected toolchain binary')
        report['tools'] = {'go': {'version': go_version, 'sha256': hashes['go']},
                           'buf': {'version': buf_version, 'sha256': hashes['buf']},
                           'maven': {'version': maven_version, 'sha256': hashes['maven']},
                           'java': {'sha256': hashes['java']},
                           'javac': {'version': java_version, 'sha256': hashes['javac']},
                           'plugin': {'version': plugin_version, 'sha256': hashes['plugin']},
                           'grpc': {'version': grpc_version, 'sha256': hashes['grpc']},
                           'bash': {'sha256': hashes['bash']}, 'git': {'sha256': hashes['git']}}
        snapshot = local_source_snapshot.materialize(root, source, output / 'source')
        private_git = local_private_git.prepare(root, source, snapshot, output / 'fixture.git', runner=_execute)
        git_env = dict(env)
        git_env.update({key: value for key, value in private_git.items() if key.upper().startswith('GIT_')})
        git_env['BUF_GO_PB_CHECK_SKIP_SELF_TEST'] = '1'
        report['base_archive_sha256'] = archive_baseline(root, base, output / 'baseline')
        commands = [
            ('buf-lint', [str(buf), 'lint'], snapshot, env),
            ('buf-format', [str(buf), 'format', '-d', '--exit-code'], snapshot, env),
            ('buf-breaking', [str(buf), 'breaking', 'protos', '--against', str(output / 'baseline/protos')], snapshot, env),
            ('go-codegen-selftest', [str(bash), 'scripts/ci/buf-go-pb-check_test.sh'], snapshot, env),
            ('go-codegen', [str(bash), 'scripts/ci/buf-go-pb-check.sh'], snapshot, git_env),
            ('auth-proto-sync', [str(bash), 'scripts/ci/check-auth-proto-sync.sh'], snapshot, env),
            ('auth-compile', [str(maven), '-B', '-DskipTests', 'compile'], snapshot / 'src/backend/auth', env),
            ('r23-contract', [str(go), 'test', '-json', '-count=1', '-v', './...'], snapshot / 'protos/voice/r23_contract_test', env),
        ]
        for name, command, cwd, command_env in commands:
            result = _execute(command, cwd=cwd, env=command_env, timeout=600)
            (output / (name + '.stdout')).write_bytes(result.stdout)
            (output / (name + '.stderr')).write_bytes(result.stderr)
            report['commands'].append({'name': name, 'argv': command, 'exit_code': result.returncode,
                                       'stdout_sha256': hashlib.sha256(result.stdout).hexdigest(),
                                       'stderr_sha256': hashlib.sha256(result.stderr).hexdigest()})
            if result.returncode:
                raise ValueError(name + ' failed; inspect its diagnostic logs')
            if name == 'r23-contract':
                report['r23'] = r23_java_summary(result.stdout, result.returncode)
            local_source_snapshot.verify_materialized(root, source, snapshot,
                                                       allowed_extra=['src/backend/auth/target/', 'gen/'])
        if local_source_snapshot.capture(root, head) != source:
            raise ValueError('source changed during protobuf diagnostic')
        if any(_sha256(path) != hashes[name] for name, path in
               (('go', go), ('buf', buf), ('maven', maven), ('java', java), ('javac', javac),
                ('plugin', plugin), ('grpc', grpc), ('bash', bash), ('git', git))):
            raise ValueError('protobuf tool binary changed during diagnostic')
        report['status'] = 'incomplete'
    except (OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError, tarfile.TarError,
            supervision.SupervisionError) as error:
        report['error'] = str(error)
    (output / 'diagnostic.json').write_text(json.dumps(report, indent=2, sort_keys=True), encoding='utf-8')
    return report
