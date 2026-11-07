"""Run the source-owned current-diff validator on a private local snapshot.

The synthetic target is not the GitHub merge candidate and cannot prove its
parents or merge tree. The resulting diagnostic is not a PR receipt. Windows
commands use an owned Job; POSIX descendant cleanup is not established.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys

import local_private_git
import local_process_supervision as supervision
import local_source_snapshot


def _windows():
    return os.name == 'nt'


def _execute(command, *, cwd=None, env=None, input=None, timeout, output_limit=None):
    """Bound output before capture; Windows broker owns the command tree."""
    wrapper = Path(__file__).with_name('local_golangci_capture.py')
    bounded = [sys.executable, '-B', str(wrapper)]
    if output_limit is not None:
        if type(output_limit) is not int or not 0 < output_limit <= 16 * 1024 * 1024:
            raise ValueError('invalid command output limit')
        bounded.append('--limit=' + str(output_limit))
    bounded.extend(command)
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


def _remote_refs(root, remote, refs):
    result = _execute(['git', '-C', str(root), 'ls-remote', '--refs', remote, *refs], timeout=60)
    if result.returncode:
        raise ValueError('live remote refs could not be verified')
    found = {}
    try:
        for line in result.stdout.decode('ascii', errors='strict').splitlines():
            sha, ref = line.split('\t', 1)
            if not re.fullmatch(r'[0-9a-f]{40}', sha) or ref in found:
                raise ValueError('live remote refs are malformed or duplicated')
            found[ref] = sha
    except (UnicodeError, ValueError) as error:
        raise ValueError('live remote refs are malformed') from error
    if set(found) != set(refs):
        raise ValueError('live remote refs are missing')
    return found


def run_fetched_merge_candidate(root, base, head, head_ref, candidate_ref, output, *, remote='origin'):
    """Run the canonical contract on a freshly fetched PR merge ref.

    This receipt completes only current-diff-contract. It requires the checked
    out source to be the exact clean PR head, compares advertised remote refs
    before and after execution, and runs the validator from the fetched merge
    tree. Aggregate local preflight remains incomplete.
    """
    root, output = Path(root).resolve(), Path(output).resolve()
    if output.exists() or output == root or root in output.parents:
        raise ValueError('output must be a new directory outside the source tree')
    if (not re.fullmatch(r'[0-9a-f]{40}', base) or not re.fullmatch(r'[0-9a-f]{40}', head) or
            not isinstance(head_ref, str) or not head_ref or head_ref.startswith('-') or
            not re.fullmatch(r'refs/pull/[1-9][0-9]*/merge', str(candidate_ref)) or
            not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]*', str(remote))):
        raise ValueError('full commit identities and valid PR refs are required')

    def git(*args, timeout=30):
        result = _execute(['git', '-C', str(root), *args], timeout=timeout)
        if result.returncode:
            raise ValueError('candidate Git check failed: ' + args[0])
        return result.stdout.decode('ascii', errors='strict').strip()

    git('check-ref-format', 'refs/heads/' + head_ref)
    expected = {'refs/heads/master': base, 'refs/heads/' + head_ref: head}
    remote_names = [*expected, candidate_ref]
    before = _remote_refs(root, remote, remote_names)
    if any(before.get(ref) != sha for ref, sha in expected.items()):
        raise ValueError('live remote refs differ from the requested base or head')
    candidate = before[candidate_ref]
    if (git('rev-parse', '--verify', 'HEAD^{commit}') != head or
            git('status', '--porcelain', '--untracked-files=all') or
            git('rev-parse', '--verify', base + '^{commit}') != base or
            git('rev-parse', '--verify', head + '^{commit}') != head):
        raise ValueError('checkout must be clean and at the exact PR head with base objects available')

    private_ref = 'refs/codex/current-diff/' + candidate
    if _execute(['git', '-C', str(root), 'show-ref', '--verify', '--quiet', private_ref], timeout=30).returncode == 0:
        raise ValueError('private candidate ref already exists')
    output.mkdir(parents=True)
    fetched = False
    try:
        fetch = _execute(['git', '-C', str(root), 'fetch', '--no-tags', '--force', remote,
                          '+' + candidate_ref + ':' + private_ref], timeout=120)
        if fetch.returncode:
            raise ValueError('PR merge ref fetch failed')
        fetched = True
        if git('rev-parse', '--verify', private_ref + '^{commit}') != candidate:
            raise ValueError('fetched PR merge ref differs from the live remote')
        parents = git('rev-list', '--parents', '-n', '1', candidate).split()[1:]
        if parents != [base, head]:
            raise ValueError('fetched PR merge candidate parents differ')
        merge_tree = _execute(['git', '-C', str(root), 'merge-tree', '--write-tree', base, head], timeout=120)
        if merge_tree.returncode or merge_tree.stdout.decode('ascii', errors='strict').strip() != git(
                'rev-parse', candidate + '^{tree}'):
            raise ValueError('fetched PR merge candidate tree differs or conflicts')
        validator_result = _execute(['git', '-C', str(root), 'show', candidate + ':scripts/ci/current-diff-contract.py'],
                                    timeout=30)
        if validator_result.returncode or len(validator_result.stdout) > 1024 * 1024:
            raise ValueError('candidate current-diff validator is missing or oversized')
        validator_bytes = validator_result.stdout
        trusted_result = _execute(['git', '-C', str(root), 'show', base + ':scripts/ci/current-diff-contract.py'],
                                  timeout=30)
        if (trusted_result.returncode or len(trusted_result.stdout) > 1024 * 1024 or
                validator_bytes != trusted_result.stdout):
            raise ValueError('candidate validator differs from the trusted event-base validator')
        validator = output / 'current-diff-contract.py'
        validator.write_bytes(validator_bytes)
        command = [sys.executable, '-B', str(validator), '--repo', str(root), '--base', base,
                   '--pr-head', head, '--target', candidate]
        result = _execute(command, cwd=root, timeout=120)
        (output / 'stdout.log').write_bytes(result.stdout)
        (output / 'stderr.log').write_bytes(result.stderr)
        if result.returncode:
            raise ValueError('current-diff-contract failed; see stderr.log')
        try:
            contract = json.loads(result.stdout)
        except (ValueError, UnicodeError) as error:
            raise ValueError('current-diff-contract emitted invalid JSON') from error
        validator_sha256 = hashlib.sha256(validator_bytes).hexdigest()
        if (not isinstance(contract, dict) or contract.get('schema') != 'voice-current-diff-v1' or
                contract.get('base') != base or contract.get('event_base') != base or
                contract.get('target') != candidate or contract.get('pr_head') != head or
                contract.get('cached') is not False or not isinstance(contract.get('changed_paths'), list) or
                contract.get('validator_sha256') != validator_sha256):
            raise ValueError('current-diff-contract identity mismatch')
        after = _remote_refs(root, remote, remote_names)
        if after != before:
            raise ValueError('live remote refs changed during current-diff-contract')
    finally:
        if fetched:
            cleanup = _execute(['git', '-C', str(root), 'update-ref', '-d', private_ref, candidate], timeout=30)
            if cleanup.returncode:
                raise ValueError('private candidate ref cleanup failed')

    report = {'schema': 'voice-local-current-diff-component-v1', 'status': 'complete',
              'check': 'current-diff-contract', 'aggregate_preflight': 'incomplete',
              'base': base, 'head': head, 'head_ref': head_ref, 'candidate_ref': candidate_ref,
              'candidate': candidate, 'parents': parents, 'validator_sha256': validator_sha256,
              'contract': contract, 'stdout': 'stdout.log', 'stderr': 'stderr.log',
              'remote_refs_before': before, 'remote_refs_after': after}
    (output / 'current-diff-component.json').write_text(
        json.dumps(report, indent=2, sort_keys=True), encoding='utf-8')
    return report
