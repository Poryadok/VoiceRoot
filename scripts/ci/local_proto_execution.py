"""Run source-bound Buf and R23 diagnostics without issuing a preflight pass."""
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import tarfile

import go_integration_receipts
import local_source_snapshot


def _git(root, *args):
    return subprocess.check_output(['git', '-C', str(root), *args], stderr=subprocess.PIPE, timeout=120).decode('utf-8').strip()


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
    with archive.open('xb') as stream:
        subprocess.run(['git', '-C', str(root), 'archive', '--format=tar', base, 'protos'],
                       stdout=stream, stderr=subprocess.PIPE, timeout=120, check=True)
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


def _tool(value, name):
    located = shutil.which(str(value))
    if not located:
        raise ValueError(name + ' executable is unavailable')
    path = Path(located).resolve(strict=True)
    if not path.is_file():
        raise ValueError(name + ' executable is invalid')
    return path


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
        hashes = {'go': _sha256(go), 'buf': _sha256(buf)}
        env = dict(os.environ)
        for key in ('GOFLAGS', 'GOPRIVATE', 'GONOPROXY', 'GONOSUMDB', 'GOINSECURE', 'GOCACHEPROG', 'BUF_TOKEN'):
            env.pop(key, None)
        env.update(GOTOOLCHAIN='local', GOENV='off', GOWORK='off',
                   GOPATH=str(output / 'gopath'), GOMODCACHE=str(output / 'gomod'),
                   GOCACHE=str(output / 'gocache'), BUF_CACHE_DIR=str(output / 'buf-cache'),
                   GOPROXY='https://proxy.golang.org,direct', GOSUMDB='sum.golang.org',
                   PYTHONDONTWRITEBYTECODE='1',
                   PATH=os.pathsep.join((str(go.parent), str(buf.parent), env.get('PATH', ''))))
        go_version = subprocess.run([str(go), 'version'], env=env, capture_output=True, text=True, timeout=30, check=True).stdout.strip()
        buf_version = subprocess.run([str(buf), '--version'], env=env, capture_output=True, text=True, timeout=30, check=True).stdout.strip()
        if not go_version.startswith('go version go1.26.10 ') or buf_version != '1.50.0':
            raise ValueError('protobuf diagnostic requires Go 1.26.10 and Buf 1.50.0')
        goroot = subprocess.run([str(go), 'env', 'GOROOT'], env=env, capture_output=True, text=True, timeout=30, check=True).stdout.strip()
        actual_go = (Path(goroot) / 'bin' / ('go.exe' if os.name == 'nt' else 'go')).resolve(strict=True)
        if actual_go != go:
            raise ValueError('Go executable is not the selected toolchain binary')
        report['tools'] = {'go': {'version': go_version, 'sha256': hashes['go']},
                           'buf': {'version': buf_version, 'sha256': hashes['buf']}}
        snapshot = local_source_snapshot.materialize(root, source, output / 'source')
        report['base_archive_sha256'] = archive_baseline(root, base, output / 'baseline')
        commands = [
            ('buf-lint', [str(buf), 'lint'], snapshot),
            ('buf-format', [str(buf), 'format', '-d', '--exit-code'], snapshot),
            ('buf-breaking', [str(buf), 'breaking', 'protos', '--against', str(output / 'baseline/protos')], snapshot),
            ('r23-contract', [str(go), 'test', '-json', '-count=1', '-v', './...'], snapshot / 'protos/voice/r23_contract_test'),
        ]
        for name, command, cwd in commands:
            result = subprocess.run(command, cwd=cwd, env=env, capture_output=True, timeout=600, check=False)
            (output / (name + '.stdout')).write_bytes(result.stdout)
            (output / (name + '.stderr')).write_bytes(result.stderr)
            report['commands'].append({'name': name, 'argv': command, 'exit_code': result.returncode,
                                       'stdout_sha256': hashlib.sha256(result.stdout).hexdigest(),
                                       'stderr_sha256': hashlib.sha256(result.stderr).hexdigest()})
            if result.returncode:
                raise ValueError(name + ' failed; inspect its diagnostic logs')
            if name == 'r23-contract':
                report['r23'] = r23_summary(result.stdout, result.returncode)
            local_source_snapshot.verify_materialized(root, source, snapshot)
        if local_source_snapshot.capture(root, head) != source:
            raise ValueError('source changed during protobuf diagnostic')
        if _sha256(go) != hashes['go'] or _sha256(buf) != hashes['buf']:
            raise ValueError('protobuf tool binary changed during diagnostic')
        report['status'] = 'incomplete'
    except (OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError, tarfile.TarError) as error:
        report['error'] = str(error)
    (output / 'diagnostic.json').write_text(json.dumps(report, indent=2, sort_keys=True), encoding='utf-8')
    return report
