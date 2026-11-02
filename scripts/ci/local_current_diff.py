"""Run the source-owned current-diff validator on a private local snapshot.

The synthetic target is not the GitHub merge candidate and cannot prove its
parents or merge tree. The resulting diagnostic is not a PR receipt. Windows
commands use an owned Job; POSIX descendant cleanup is not established.
"""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

import local_private_git
import local_process_supervision as supervision
import local_source_snapshot


def _windows():
    return os.name == 'nt'


def _execute(command, *, cwd=None, env=None, input=None, timeout):
    """Bound output before capture; Windows broker owns the command tree."""
    wrapper = Path(__file__).with_name('local_golangci_capture.py')
    bounded = [sys.executable, '-B', str(wrapper), *command]
    if _windows():
        return supervision.run_diagnostic(bounded, cwd=cwd, env=env, input=input,
                                          timeout=timeout, capture_output=True, check=False)
    return subprocess.run(bounded, cwd=cwd, env=env, input=input, timeout=timeout,
                          capture_output=True, check=False)


def run(root, source, base, output, *, merge_candidate=False):
    root, output = Path(root).resolve(), Path(output).resolve()
    if merge_candidate and (source.get('changed_paths') or source.get('raw_mismatch_paths')):
        raise ValueError('merge candidate requires clean committed source')
    if output.exists() or output == root or root in output.parents:
        raise ValueError('output must be a new directory outside the source tree')
    output.mkdir(parents=True)
    snapshot = local_source_snapshot.materialize(root, source, output / 'source')
    env = local_private_git.prepare(root, source, snapshot, output / 'git', runner=_execute)
    env['PYTHONDONTWRITEBYTECODE'] = '1'
    parents = ['-p', source['head']]
    if merge_candidate:
        # Restrict the first accepted route to an up-to-date base. This makes
        # the committed head tree also the local merge tree used by the planner.
        ancestor = _execute(['git', 'merge-base', '--is-ancestor', base, source['head']], env=env, timeout=30)
        if ancestor.returncode:
            raise ValueError('base must be an ancestor of the committed head')
        tree = _execute(['git', 'merge-tree', '--write-tree', base, source['head']], env=env, timeout=120)
        if tree.returncode or tree.stdout.decode('ascii', errors='strict').strip() != source['tree']:
            raise ValueError('local merge tree differs or conflicts')
        parents = ['-p', base, '-p', source['head']]
    commit = _execute(['git', 'commit-tree', source['tree'], *parents],
                      input=b'local diagnostic snapshot\n', env=env, timeout=30)
    commit.check_returncode()
    target = commit.stdout.decode().strip()
    _execute(['git', 'update-ref', 'refs/heads/master', target], env=env, timeout=30).check_returncode()
    _execute(['git', 'read-tree', target], env=env, timeout=30).check_returncode()
    validator = snapshot / 'scripts/ci/current-diff-contract.py'
    command = [sys.executable, str(validator), '--repo', str(snapshot), '--base', base, '--target', target]
    if merge_candidate:
        command += ['--pr-head', source['head']]
    result = _execute(command, cwd=snapshot, env=env, timeout=120)
    (output / 'stdout.log').write_bytes(result.stdout)
    (output / 'stderr.log').write_bytes(result.stderr)
    if result.returncode:
        raise ValueError('current-diff-contract failed; see stderr.log')
    try:
        contract = json.loads(result.stdout)
    except (ValueError, UnicodeError) as error:
        raise ValueError('current-diff-contract emitted invalid JSON') from error
    if (not isinstance(contract, dict) or contract.get('schema') != 'voice-current-diff-v1' or
            contract.get('base') != base or contract.get('event_base') != base or
            contract.get('target') != target or contract.get('pr_head') != (source['head'] if merge_candidate else None) or
            contract.get('cached') is not False or not isinstance(contract.get('changed_paths'), list) or
            contract.get('validator_sha256') != hashlib.sha256(validator.read_bytes()).hexdigest()):
        raise ValueError('current-diff-contract identity mismatch')
    local_source_snapshot.verify_materialized(root, source, snapshot)
    if local_source_snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed during current-diff-contract')
    report = {'schema': 'voice-local-current-diff-diagnostic-v1', 'status': 'diagnostic',
              'source': source, 'base': base, 'contract': contract,
              'stdout': 'stdout.log', 'stderr': 'stderr.log'}
    (output / 'diagnostic.json').write_text(json.dumps(report, indent=2, sort_keys=True), encoding='utf-8')
    return report


def run_merge_candidate(root, source, base, output):
    """Validate the locally constructed merge candidate for a clean head."""
    return run(root, source, base, output, merge_candidate=True)
