"""Supervise the owned staging shard; never promote it to a CI pass.

The worker and assets require trusted caller custody. Process startup,
filesystem operations, disk growth and daemon-side cancellation are not bounded
by the broker. Uncertain cleanup requires manual journal/slot reconciliation.
"""
import json
import math
from pathlib import Path
import re
from collections import Counter

import local_ci_staging_request as contract
import local_nats_hosted_diagnostic as bootstrap
import local_process_supervision as supervision


class ExecutionError(RuntimeError):
    """The shard did not produce a complete, cleaned diagnostic receipt."""


def _validate_journal(output, identity):
    """Validate producer journal structure, not independent Docker absence."""
    slot, daemon, raw_path = identity['slot'], identity['daemon'], identity['journal']
    if (type(slot) is not int or slot not in (0, 1) or not isinstance(daemon, str) or
            not re.fullmatch(r'[A-Za-z0-9_.:-]{1,128}', daemon) or not isinstance(raw_path, str)):
        raise ValueError('invalid ownership identity')
    path = Path(raw_path)
    run_id = path.parent.name
    if (not path.is_absolute() or not re.fullmatch(r'voiceci-[0-9a-f]{32}', run_id) or
            path != output / 'owner' / 'tmp' / 'ci-local' / run_id / 'ownership.json' or
            path.resolve(strict=True) != path):
        raise ValueError('invalid ownership journal path')
    with path.open('rb') as stream:
        data = stream.read(1024 * 1024 + 1)
    if len(data) > 1024 * 1024:
        raise ValueError('ownership journal too large')
    journal = json.loads(data)
    if (not isinstance(journal, dict) or set(journal) != {'version', 'run_id', 'daemon', 'resources'} or
            type(journal['version']) is not int or journal['version'] != 1 or
            journal['run_id'] != run_id or journal['daemon'] != daemon or not isinstance(journal['resources'], list)):
        raise ValueError('ownership journal identity differs')
    resources = journal['resources']
    if (len(resources) != 10 or any(not isinstance(row, dict) for row in resources) or
            Counter(row.get('kind') for row in resources) != {'network': 1, 'volume': 4, 'container': 5}):
        raise ValueError('ownership journal inventory differs')
    ids, nonces = set(), set()
    for row in resources:
        kind, nonce, name, resource_id = row.get('kind'), row.get('nonce'), row.get('name'), row.get('id')
        if (not isinstance(nonce, str) or not re.fullmatch(r'[0-9a-f]{32}', nonce) or nonce in nonces or
                name != f'{run_id}-{kind}-{nonce}' or row.get('state') != 'removed' or
                not isinstance(resource_id, str) or resource_id in ids or
                (resource_id != name if kind == 'volume' else not re.fullmatch(r'[0-9a-f]{64}', resource_id))):
            raise ValueError('ownership journal resource differs')
        ids.add(resource_id); nonces.add(nonce)


def _worker_command():
    command = bootstrap._worker_command(Path(__file__).with_name('local_ci_staging_worker.py'))
    original = repr(sorted(bootstrap.PROJECT_MODULES))
    modules = bootstrap.PROJECT_MODULES | {'local_ci_staging_request', 'local_ci_matrix_inventory'}
    if command[5].count(original) != 1:
        raise ExecutionError('worker import allowlist changed')
    command[5] = command[5].replace(original, repr(sorted(modules)), 1)
    return command


def run(request, *, timeout=7200):
    contract.validate(request)
    if (isinstance(timeout, bool) or not isinstance(timeout, (int, float)) or
            not math.isfinite(timeout) or not 0 < timeout <= 7200):
        raise ValueError('invalid staging timeout')
    raw = json.dumps(request, allow_nan=False, separators=(',', ':')).encode('utf-8')
    if len(raw) > 1024 * 1024:
        raise ValueError('staging request is too large')
    try:
        result = supervision.run_diagnostic(_worker_command(), timeout=timeout,
                                            input=raw, capture_output=True, check=False)
    except BaseException as error:
        error.add_note('Worker cleanup may be uncertain; inspect the requested output journal and slot.')
        raise
    if (result.returncode != 0 or result.stdout.splitlines().count(b'STAGING_SHARD_DIAGNOSTIC_OK') != 1 or
            result.stdout.splitlines().count(b'OWNED_RESOURCES_CLEANED') != 1):
        error = ExecutionError('staging worker failed or cleanup is unproven; inspect its journal')
        error.worker_returncode, error.worker_stderr = result.returncode, result.stderr
        raise error
    output = Path(request['output'])
    try:
        identity = json.loads((output / 'identity.json').read_text(encoding='utf-8'))
        counts = json.loads((output / 'suite-counts.json').read_text(encoding='utf-8'))
        if (identity['source'] != request['commit'] or counts['shard'] != request['shard'] or
                type(counts['recipes']) is not int or counts['recipes'] != {'staging': 34, 'proofs': 15, 'runtime': 23, 'policy': 73}[request['shard']] or
                type(counts['hosted_go_tests']) is not int or
                (counts['hosted_go_tests'] < 1 if request['shard'] == 'staging' else counts['hosted_go_tests'] != 0) or
                type(counts['hosted_policy_tests']) is not int or
                (counts['hosted_policy_tests'] < 1 if request['shard'] == 'policy' else counts['hosted_policy_tests'] != 0)):
            raise ValueError('invalid staging receipt')
        _validate_journal(output.resolve(), identity)
    except (OSError, ValueError, TypeError, KeyError):
        raise ExecutionError('staging worker returned invalid receipt') from None
    report = {'schema': 'voice-local-ci-staging-v1', 'status': 'incomplete',
              'source': request['commit'], 'base': request['base'], 'shard': request['shard'], 'recipes': counts['recipes'],
              'hosted_go_tests': counts['hosted_go_tests'], 'hosted_policy_tests': counts['hosted_policy_tests'], 'journal': identity['journal'],
              'slot': identity['slot']}
    with (output / 'diagnostic.json').open('x', encoding='utf-8') as stream:
        json.dump(report, stream, sort_keys=True)
    return report
