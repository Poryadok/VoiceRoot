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
import local_private_git
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


def run(root, plan, output, go, go_sha256, lint, lint_sha256, *, base, head_ref, verify_module_inputs=False):
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
           if not k.upper().startswith(('GO', 'GOLANGCI', 'GIT_', 'PYTHON'))}
    temp = output/'temp'; temp.mkdir()
    env.update(GOENV='off', GOTOOLCHAIN='local', GOFLAGS='-mod=readonly -p=2', GOMAXPROCS='2',
               GOCACHE=str(output / 'go-cache'), GOMODCACHE=str(output / 'go-mod-cache'),
               GOLANGCI_LINT_CACHE=str(output / 'lint-cache'),
               TEMP=str(temp), TMP=str(temp), TMPDIR=str(temp), GOTMPDIR=str(temp), PYTHONDONTWRITEBYTECODE='1',
               PATH=str(go.parent) + os.pathsep + env.get('PATH', ''))
    private_git = (local_private_git.prepare(root, source, source_copy, output/'fixture.git',
                                            runner=lambda command, **kwargs: _private_git_execute(command, **kwargs))
                   if verify_module_inputs else None)
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
    if verify_module_inputs:
        git_env = dict(env)
        git_env.update({key:value for key,value in private_git.items() if key.upper().startswith('GIT_')})
        stdout, stderr = _command([sys.executable, '-B', str(source_copy/'scripts/ci/check_go_modules_clean.py')],
                                  cwd=source_copy, env=git_env)
        (output/'module-clean.stdout').write_bytes(stdout); (output/'module-clean.stderr').write_bytes(stderr)
        report['module_check'] = {'exit_code':0, 'stdout_sha256':hashlib.sha256(stdout).hexdigest(),
                                  'stderr_sha256':hashlib.sha256(stderr).hexdigest()}
        local_source_snapshot.verify_materialized(root,source,source_copy)
        if (local_source_snapshot.capture(root,source['head']) != source or
                _sha256(go) != go_sha256 or _sha256(lint) != lint_sha256):
            raise ValueError('source or lint tools changed during module check')
    (output / 'diagnostic.json').write_text(json.dumps(report, indent=2, sort_keys=True) + '\n', encoding='utf-8')
    return report


def _private_git_execute(command, *, env=None, input=None, timeout=30):
    wrapper = Path(__file__).with_name('local_golangci_capture.py')
    return supervision.run_diagnostic([sys.executable, '-B', str(wrapper), *command], env=env, input=input,
                                      timeout=timeout, capture_output=True, check=False)


def _evidence_file(path):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > 1024*1024:
        raise ValueError('unsafe or oversized lint evidence')
    raw = path.read_bytes()
    if len(raw) > 1024*1024: raise ValueError('oversized lint evidence')
    return raw


def verify_component_evidence(output, plan, report, tools):
    modules = modules_for(plan)
    if (not isinstance(report, dict) or report.get('schema') != 'voice-local-golangci-diagnostic-v1' or
            report.get('status') != 'incomplete' or report.get('source') != plan.get('source') or
            report.get('modules') != modules or report.get('tools') != tools or
            report.get('platform') != {'goos':'windows','goarch':'amd64'}):
        raise ValueError('lint component identity differs')
    rows = report.get('commands')
    if (not isinstance(rows,list) or any(not isinstance(row,dict) for row in rows) or
            [(row.get('action'),row.get('module')) for row in rows] !=
            [(action,module) for action in ('resolve','lint') for module in modules] or
            any(type(row.get('exit_code')) is not int or row['exit_code'] != 0 for row in rows)):
        raise ValueError('lint module inventory differs')
    output = Path(output)
    for row in rows:
        for suffix in ('stdout','stderr'):
            raw = _evidence_file(output/(row['action']+'-'+row['module']+'.'+suffix))
            if hashlib.sha256(raw).hexdigest() != row.get(suffix+'_sha256'):
                raise ValueError('lint log digest differs')
    clean = report.get('module_check')
    if not isinstance(clean,dict) or type(clean.get('exit_code')) is not int or clean['exit_code'] != 0:
        raise ValueError('Go module check missing')
    for suffix in ('stdout','stderr'):
        raw = _evidence_file(output/('module-clean.'+suffix))
        if hashlib.sha256(raw).hexdigest() != clean.get(suffix+'_sha256'):
            raise ValueError('Go module check log differs')
        if suffix == 'stdout' and raw.splitlines() != [b'Go module inputs match the CI checkout.']:
            raise ValueError('Go module check did not complete')


def verify_component(output, plan, base, report, tools):
    if (not isinstance(report,dict) or report.get('schema') != 'voice-local-golangci-component-v1' or
            report.get('status') != 'complete' or report.get('scope') != 'golangci' or
            report.get('source') != plan.get('source') or report.get('base') != base or
            report.get('diagnostic') != 'worker/diagnostic.json' or report.get('tools') != tools):
        raise ValueError('lint component identity differs')
    output = Path(output)
    diagnostic = json.loads(_evidence_file(output/'worker/diagnostic.json'))
    verify_component_evidence(output/'worker', plan, diagnostic, tools)


def run_component(root, base, head, head_ref, output, *, go, go_sha256, lint, lint_sha256):
    """Execute selected local Windows lint, never a whole-preflight pass."""
    import local_preflight
    root, output = Path(root).resolve(), Path(output).resolve()
    if not _windows() or output.exists() or output == root or root in output.parents:
        raise ValueError('lint component requires Windows and new external output')
    local_preflight._handoff_refs(root,base,head,head_ref)
    plan = affected_check_plan.build_local_plan(root,base,head,head_ref)
    source = plan['source']
    if ('golangci' not in plan['checks'] or source.get('head') != head or source.get('changed_paths') or
            source.get('raw_mismatch_paths')):
        raise ValueError('lint component requires selected clean committed source')
    output.mkdir(parents=True)
    diagnostic = run(root, plan, output/'worker', go, go_sha256, lint, lint_sha256,
                     base=base, head_ref=head_ref, verify_module_inputs=True)
    tools = {'go_sha256':go_sha256,'golangci_sha256':lint_sha256}
    verify_component_evidence(output/'worker',plan,diagnostic,tools)
    if (local_source_snapshot.capture(root,head) != source or
            affected_check_plan.build_local_plan(root,base,head,head_ref) != plan):
        raise ValueError('lint source or selection changed')
    local_preflight._handoff_refs(root,base,head,head_ref)
    report={'schema':'voice-local-golangci-component-v1','status':'complete','scope':'golangci',
            'source':source,'base':base,'head_ref':head_ref,'diagnostic':'worker/diagnostic.json','tools':tools,
            'limitations':['Windows lint differs from hosted Linux',
                           'tool and dependency publisher provenance is not authenticated',
                           'fetched refs are not a live remote observation']}
    verify_component(output,plan,base,report,tools)
    (output/'golangci-component.json').write_text(json.dumps(report,indent=2,sort_keys=True)+'\n',encoding='utf-8')
    return report
