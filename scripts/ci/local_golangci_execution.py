"""Run affected Go lint commands from a source-bound snapshot, never issue PASS.

Input digests identify caller-supplied tools, not their provenance. The host
dependency cache and network acquisition remain outside a complete receipt.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys

import affected_check_plan
import local_process_supervision as supervision
import local_source_snapshot


TIMEOUT_SECONDS = 1200


def _windows():
    return os.name == 'nt'


def modules_for(plan):
    components = plan.get('components') if isinstance(plan, dict) else None
    services = components.get('go') if isinstance(components, dict) else None
    if not isinstance(services, list) or any(not isinstance(v, str) or
            not re.fullmatch(r'[a-z][a-z0-9]*', v) for v in services):
        raise ValueError('invalid affected Go module inventory')
    return list(dict.fromkeys(['pkg', *services]))


def _sha256(path):
    result = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            result.update(block)
    return result.hexdigest()


def _command(command, *, cwd=None, env=None, timeout=TIMEOUT_SECONDS):
    wrapper = Path(__file__).with_name('local_golangci_capture.py')
    result = supervision.run_diagnostic([sys.executable, '-B', str(wrapper), *command], cwd=cwd, env=env, timeout=timeout,
                                        capture_output=True, check=False)
    if result.returncode:
        raise subprocess.CalledProcessError(result.returncode, command,
                                            output=result.stdout, stderr=result.stderr)
    return result.stdout, result.stderr


def _check_tool(path, expected, kind):
    if not re.fullmatch(r'[0-9a-f]{64}', expected) or _sha256(path) != expected:
        raise ValueError(kind + ' executable digest differs')
    output = _command([str(path), 'version'], timeout=30)[0].decode('utf-8', errors='strict').strip()
    pattern = (r'go version go1\.26\.10 windows/amd64' if kind == 'go' else
               r'golangci-lint has version 2\.6\.1(?:\s.*)?')
    if not re.fullmatch(pattern, output):
        raise ValueError(kind + ' version differs from pinned diagnostic toolchain')
    return output


def _run_modules(source_copy, modules, go, lint, env, record=None):
    rows = []
    for action, tool, args in (('resolve', go, ['list', '-test', './...']),
                               ('lint', lint, ['run', './...'])):
        for module in modules:
            cwd = Path(source_copy) / 'src' / 'backend' / module
            command = [str(tool), *args]
            try:
                stdout, stderr = _command(command, cwd=cwd, env=env)
                exit_code = 0
            except subprocess.CalledProcessError as error:
                stdout, stderr, exit_code = error.output or b'', error.stderr or b'', error.returncode
            row = {'module': module, 'action': action, 'command': command,
                   'stdout_sha256': hashlib.sha256(stdout).hexdigest(),
                   'stderr_sha256': hashlib.sha256(stderr).hexdigest(), 'exit_code': exit_code}
            rows.append(row)
            if record is not None:
                record(row, stdout, stderr)
            if exit_code:
                raise subprocess.CalledProcessError(exit_code, command, output=stdout, stderr=stderr)
    return rows


def run(root, plan, output, go, go_sha256, lint, lint_sha256, *, base, head_ref):
    if not _windows():
        raise ValueError('Windows supervised Go lint diagnostic required')
    root, output = Path(root).resolve(), Path(output).resolve()
    go, lint = Path(go), Path(lint)
    if not go.is_absolute() or not lint.is_absolute():
        raise ValueError('absolute tool paths required')
    source = plan.get('source') if isinstance(plan, dict) else None
    if not isinstance(source, dict) or 'golangci' not in plan.get('checks', {}):
        raise ValueError('planner-selected golangci check required')
    if (not isinstance(base, str) or not re.fullmatch(r'(?:[0-9a-f]{40}|[0-9a-f]{64})', base) or
            not isinstance(head_ref, str) or not head_ref or
            affected_check_plan.build_local_plan(root, base, source['head'], head_ref) != plan):
        raise ValueError('affected lint plan differs')
    modules = modules_for(plan)
    if output.exists() or output == root or root in output.parents:
        raise ValueError('lint output must be new and outside source')
    if local_source_snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed before lint')
    versions = {'go': _check_tool(go, go_sha256, 'go'),
                'golangci': _check_tool(lint, lint_sha256, 'lint')}
    output.mkdir(parents=True)
    source_copy = local_source_snapshot.materialize(root, source, output / 'source')
    env = {k: v for k, v in os.environ.items()
           if not k.upper().startswith(('GO', 'GOLANGCI'))}
    env.update(GOENV='off', GOTOOLCHAIN='local', GOFLAGS='-mod=readonly -p=2', GOMAXPROCS='2',
               GOCACHE=str(output / 'go-cache'), GOMODCACHE=str(output / 'go-mod-cache'),
               GOLANGCI_LINT_CACHE=str(output / 'lint-cache'),
               PATH=str(go.parent) + os.pathsep + env.get('PATH', ''))
    def record(row, stdout, stderr):
        prefix = row['action'] + '-' + row['module']
        (output / (prefix + '.stdout')).write_bytes(stdout)
        (output / (prefix + '.stderr')).write_bytes(stderr)
        local_source_snapshot.verify_materialized(root, source, source_copy)
        if local_source_snapshot.capture(root, source['head']) != source:
            raise ValueError('source changed during lint')
        if _sha256(go) != go_sha256 or _sha256(lint) != lint_sha256:
            raise ValueError('lint tool changed during execution')
    rows = _run_modules(source_copy, modules, go, lint, env, record)
    report = {'schema': 'voice-local-golangci-diagnostic-v1', 'status': 'incomplete',
              'source': source, 'modules': modules, 'commands': rows, 'versions': versions,
              'tools': {'go_sha256': go_sha256, 'golangci_sha256': lint_sha256},
              'platform': {'goos': 'windows', 'goarch': 'amd64'},
              'unbound_inputs': ['tool provenance', 'dependency provenance', 'host environment',
                                 'hosted Linux lint parity']}
    (output / 'diagnostic.json').write_text(json.dumps(report, indent=2, sort_keys=True) + '\n', encoding='utf-8')
    return report
