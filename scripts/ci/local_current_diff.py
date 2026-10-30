"""Run the source-owned current-diff validator on a private local snapshot.

The synthetic target is not the GitHub merge candidate and cannot prove its
parents or merge tree. The resulting diagnostic is not a PR receipt.
"""
import hashlib
import json
from pathlib import Path
import subprocess
import sys

import local_private_git
import local_source_snapshot


def run(root, source, base, output):
    root, output = Path(root).resolve(), Path(output).resolve()
    if output.exists() or output == root or root in output.parents:
        raise ValueError('output must be a new directory outside the source tree')
    output.mkdir(parents=True)
    snapshot = local_source_snapshot.materialize(root, source, output / 'source')
    env = local_private_git.prepare(root, source, snapshot, output / 'git')
    env['PYTHONDONTWRITEBYTECODE'] = '1'
    target = subprocess.check_output(['git', 'commit-tree', source['tree'], '-p', source['head']],
                                     input=b'local diagnostic snapshot\n', env=env, timeout=30).decode().strip()
    subprocess.run(['git', 'update-ref', 'refs/heads/master', target], env=env, check=True, timeout=30)
    subprocess.run(['git', 'read-tree', target], env=env, check=True, timeout=30)
    validator = snapshot / 'scripts/ci/current-diff-contract.py'
    result = subprocess.run([sys.executable, str(validator), '--repo', str(snapshot), '--base', base,
                             '--target', target], cwd=snapshot, env=env, capture_output=True, timeout=120)
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
            contract.get('target') != target or contract.get('pr_head') is not None or
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
