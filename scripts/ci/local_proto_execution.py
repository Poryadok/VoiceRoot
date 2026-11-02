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
import stat
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


def pr_regression_count(raw, exit_code):
    """Accept only a successful, non-empty unittest run with no skips."""
    text = raw.decode('utf-8', errors='strict')
    matches = re.findall(r'(?m)^Ran ([0-9]+) tests? in [0-9.]+s\s*$', text)
    if exit_code != 0 or len(matches) != 1 or int(matches[0]) < 1 or not re.search(r'(?m)^OK\s*$', text):
        raise ValueError('Buf PR regression tests did not pass without skips')
    return int(matches[0])


def validate_pr_receipt(raw, base, head, candidate):
    try:
        receipt = json.loads(raw)
    except (ValueError, UnicodeError) as error:
        raise ValueError('invalid Buf PR receipt') from error
    if (not isinstance(receipt, dict) or receipt.get('event_base') != base or receipt.get('base') != base or
            receipt.get('head') != head or receipt.get('merge') != candidate or
            receipt.get('checkout') != candidate or receipt.get('parents') != [base, head]):
        raise ValueError('Buf PR candidate identity differs')
    return receipt


def run_pr_candidate(command, snapshot, env, *, execute=_execute):
    """Expose only the owned bare fixture at .git for Buf's file URL."""
    snapshot = Path(snapshot).resolve()
    private = Path(env['GIT_DIR'])
    visible = snapshot / '.git'
    if (private != snapshot.parent / 'fixture.git' or visible.exists() or visible.is_symlink() or
            private.is_symlink() or getattr(private, 'is_junction', lambda: False)() or not private.is_dir() or
            getattr(private.lstat(), 'st_file_attributes', 0) & getattr(stat, 'FILE_ATTRIBUTE_REPARSE_POINT', 0x400) or
            private.resolve() != private or Path(env['GIT_WORK_TREE']).resolve() != snapshot):
        raise ValueError('unsafe private Buf Git fixture')
    os.replace(private, visible)
    settled = True
    try:
        candidate_env = {key: value for key, value in env.items() if key not in ('GIT_DIR', 'GIT_WORK_TREE')}
        return execute(command, cwd=snapshot, env=candidate_env, timeout=600)
    except (supervision.SupervisionError, BaseExceptionGroup):
        settled = False
        raise
    finally:
        if settled:
            os.replace(visible, private)


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


def run(root, source, base, output, go_binary='go', buf_binary='buf', *, merge_candidate=False):
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
        candidate = None
        if merge_candidate:
            if source.get('changed_paths') or source.get('raw_mismatch_paths'):
                raise ValueError('Buf PR candidate requires clean committed source')
            merged = _execute([str(git), 'merge-tree', '--write-tree', base, head], env=git_env, timeout=120)
            if merged.returncode or merged.stdout.decode('ascii').strip() != source['tree']:
                raise ValueError('Buf PR merge tree differs')
            commit = _execute([str(git), 'commit-tree', source['tree'], '-p', base, '-p', head],
                              input=b'local protobuf candidate\n', env=git_env, timeout=120)
            commit.check_returncode()
            candidate = commit.stdout.decode('ascii').strip()
            for args in (['update-ref', 'refs/heads/master', candidate], ['read-tree', candidate]):
                _execute([str(git), *args], env=git_env, timeout=120).check_returncode()
            report['candidate'] = candidate
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
        if merge_candidate:
            commands.extend([
                ('buf-pr-regressions', [sys.executable, '-B', 'scripts/ci/buf-breaking-pr_test.py'], snapshot, env),
                ('buf-pr-candidate', [sys.executable, '-B', 'scripts/ci/buf-breaking-pr.py', '--repo', str(snapshot),
                                      '--base', base, '--head', head, '--merge', candidate], snapshot, git_env),
            ])
        for name, command, cwd, command_env in commands:
            if name == 'buf-pr-candidate':
                result = run_pr_candidate(command, snapshot, command_env)
            else:
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
            elif name == 'buf-pr-regressions':
                report['pr_regressions'] = pr_regression_count(result.stderr, result.returncode)
            elif name == 'buf-pr-candidate':
                report['pr_candidate'] = validate_pr_receipt(result.stdout, base, head, candidate)
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


COMPONENT_COMMANDS = ('buf-lint', 'buf-format', 'buf-breaking', 'go-codegen-selftest', 'go-codegen',
                      'auth-proto-sync', 'auth-compile', 'r23-contract', 'buf-pr-regressions', 'buf-pr-candidate')


def _evidence_file(path):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > 16*1024*1024:
        raise ValueError('unsafe or oversized protobuf evidence')
    data = path.read_bytes()
    if len(data) > 16*1024*1024: raise ValueError('oversized protobuf evidence')
    return data


def verify_component_evidence(output, source, base, report):
    output = Path(output)
    if (not isinstance(report, dict) or report.get('schema') != 'voice-local-protobuf-diagnostic-v1' or
            report.get('status') != 'incomplete' or report.get('source') != source or report.get('base') != base or
            not isinstance(report.get('candidate'), str) or not re.fullmatch(r'[0-9a-f]{40}', report['candidate'])):
        raise ValueError('protobuf diagnostic identity differs')
    commands = report.get('commands')
    if (not isinstance(commands, list) or any(not isinstance(row, dict) for row in commands) or
            [row.get('name') for row in commands] != list(COMPONENT_COMMANDS) or
            any(type(row.get('exit_code')) is not int or row['exit_code'] != 0 for row in commands)):
        raise ValueError('protobuf command evidence differs')
    if hashlib.sha256(_evidence_file(output/'base-protos.tar')).hexdigest() != report.get('base_archive_sha256'):
        raise ValueError('protobuf baseline archive differs')
    logs = {}
    for row in commands:
        for suffix in ('stdout', 'stderr'):
            raw = _evidence_file(output/(row['name']+'.'+suffix))
            if hashlib.sha256(raw).hexdigest() != row.get(suffix+'_sha256'):
                raise ValueError('protobuf log digest differs')
            logs[row['name']+'.'+suffix] = raw
    if (r23_java_summary(logs['r23-contract.stdout'], 0) != report.get('r23') or
            pr_regression_count(logs['buf-pr-regressions.stderr'], 0) != report.get('pr_regressions') or
            validate_pr_receipt(logs['buf-pr-candidate.stdout'], base, source['head'], report['candidate']) != report.get('pr_candidate')):
        raise ValueError('protobuf parsed evidence differs')


def verify_component(output, source, base, report):
    if (not isinstance(report, dict) or report.get('schema') != 'voice-local-protobuf-component-v1' or
            report.get('status') != 'complete' or report.get('scope') != 'protobuf' or
            report.get('source') != source or report.get('base') != base or
            report.get('diagnostic') != 'worker/diagnostic.json'):
        raise ValueError('protobuf component identity differs')
    output = Path(output)
    diagnostic = json.loads(_evidence_file(output/'worker/diagnostic.json'))
    verify_component_evidence(output/'worker', source, base, diagnostic)


def run_component(root, base, head, head_ref, output, go_binary='go', buf_binary='buf'):
    """Execute the selected protobuf component, never the whole preflight."""
    import affected_check_plan
    import local_preflight
    root, output = Path(root).resolve(), Path(output).resolve()
    if not _windows() or output.exists() or output == root or root in output.parents:
        raise ValueError('protobuf component requires Windows and new external output')
    local_preflight._handoff_refs(root, base, head, head_ref)
    plan = affected_check_plan.build_local_plan(root, base, head, head_ref)
    source = plan['source']
    if ('protobuf' not in plan['checks'] or source.get('head') != head or source.get('changed_paths') or
            source.get('raw_mismatch_paths')):
        raise ValueError('protobuf component requires selected clean committed source')
    output.mkdir(parents=True)
    diagnostic = run(root, source, base, output/'worker', go_binary, buf_binary, merge_candidate=True)
    verify_component_evidence(output/'worker', source, base, diagnostic)
    if (local_source_snapshot.capture(root, head) != source or
            affected_check_plan.build_local_plan(root, base, head, head_ref) != plan):
        raise ValueError('protobuf source or selection changed')
    local_preflight._handoff_refs(root, base, head, head_ref)
    report = {'schema':'voice-local-protobuf-component-v1', 'status':'complete', 'scope':'protobuf',
              'source':source, 'base':base, 'head_ref':head_ref, 'diagnostic':'worker/diagnostic.json',
              'limitations':['tool and dependency publisher provenance is not authenticated',
                             'merge candidate is local, not the GitHub merge commit',
                             'fetched refs are not a live remote observation']}
    verify_component(output, source, base, report)
    (output/'protobuf-component.json').write_text(json.dumps(report, indent=2, sort_keys=True)+'\n', encoding='utf-8')
    return report
