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
import local_rootless_profile as rootless_profile
from local_ci_staging_worker import CLI_IMAGE, GO_IMAGE, NODE_IMAGE, PYTHON_CLIENT


class ExecutionError(RuntimeError):
    """The shard did not produce a complete, cleaned diagnostic receipt."""


def _validate_journal(output, identity, shard):
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
    inventories = {'staging': {'network': 1, 'volume': 4, 'container': 5},
                   'proofs': {'network': 1, 'volume': 4, 'container': 5},
                   'policy': {'network': 1, 'volume': 4, 'container': 5},
                   'runtime': {'network': 2, 'volume': 5, 'container': 7}}
    if (shard not in inventories or any(not isinstance(row, dict) for row in resources) or
            Counter(row.get('kind') for row in resources) != inventories[shard]):
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
    if shard == 'runtime':
        containers = [row for row in resources if row['kind'] == 'container']
        isolated = [row for row in containers if row.get('profile') == 'phase0-isolated-v1']
        rootless = [row for row in containers if row.get('profile') == 'rootless-unix-v1']
        ordinary = [row for row in containers if 'profile' not in row]
        networks = {row['id'] for row in resources if row['kind'] == 'network'}
        volumes = {row['name'] for row in resources if row['kind'] == 'volume'}
        if len(isolated) != 1 or len(rootless) != 1 or len(ordinary) != 5:
            raise ValueError('runtime container profiles differ')
        phase0 = isolated[0]
        mounts = phase0.get('mounts')
        if (phase0.get('network') != 'none' or not isinstance(mounts, dict) or
                set(mounts) != {'/workspace/repo'} or mounts['/workspace/repo'] not in volumes or
                phase0.get('readonly_mounts') != ['/workspace/repo'] or
                phase0.get('command') != ['sleep', 'infinity']):
            raise ValueError('runtime phase0 isolation differs')
        source_volume = mounts['/workspace/repo']
        loaders = [row for row in ordinary if row.get('mounts') == {'/phase0-source': source_volume}]
        if (len(loaders) != 1 or loaders[0].get('command') != ['sleep', 'infinity'] or
                loaders[0].get('network') not in networks or rootless[0].get('network') not in networks or
                loaders[0]['network'] == rootless[0]['network'] or
                any(row.get('network') != rootless[0]['network'] for row in ordinary if row is not loaders[0]) or
                any(not isinstance(row.get('mounts'), dict) or source_volume in row['mounts'].values()
                    for row in containers if row is not phase0 and row is not loaders[0])):
            raise ValueError('runtime phase0 source or network binding differs')
        daemon = rootless[0]
        daemon_mounts = daemon.get('mounts')
        if (daemon.get('image') != rootless_profile.IMAGE or daemon.get('ports') != [] or
                not isinstance(daemon_mounts, dict) or set(daemon_mounts) != rootless_profile.TARGETS or
                any(value not in volumes for value in daemon_mounts.values()) or
                len(set(daemon_mounts.values())) != 3 or 'command' in daemon):
            raise ValueError('runtime rootless daemon specification differs')
        if (phase0.get('image') != PYTHON_CLIENT or phase0.get('ports') != [] or
                loaders[0].get('image') != PYTHON_CLIENT or loaders[0].get('ports') != []):
            raise ValueError('runtime phase0 client image or ports differ')
        peers = [row for row in ordinary if row is not loaders[0]]
        if (Counter(row.get('image') for row in peers) !=
                Counter((CLI_IMAGE, PYTHON_CLIENT, GO_IMAGE, NODE_IMAGE))):
            raise ValueError('runtime client image inventory differs')
        for peer in peers:
            expected_targets = {'/run/user/1000', '/workspace'} if peer['image'] == PYTHON_CLIENT else {'/run/user/1000'}
            peer_mounts = peer.get('mounts')
            if (peer.get('ports') != [] or peer.get('command') != ['sleep', 'infinity'] or
                    not isinstance(peer_mounts, dict) or set(peer_mounts) != expected_targets or
                    any(value not in volumes for value in peer_mounts.values()) or
                    peer_mounts['/run/user/1000'] != daemon_mounts['/run/user/1000'] or
                    ('/workspace' in peer_mounts and peer_mounts['/workspace'] in daemon_mounts.values())):
                raise ValueError('runtime client mount or command differs')


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
                type(counts['recipes']) is not int or counts['recipes'] != {'staging': 34, 'proofs': 15, 'runtime': 23, 'policy': 76}[request['shard']] or
                type(counts['hosted_go_tests']) is not int or
                (counts['hosted_go_tests'] < 1 if request['shard'] == 'staging' else counts['hosted_go_tests'] != 0) or
                type(counts['hosted_policy_tests']) is not int or
                (counts['hosted_policy_tests'] < 1 if request['shard'] == 'policy' else counts['hosted_policy_tests'] != 0)):
            raise ValueError('invalid staging receipt')
        _validate_journal(output.resolve(), identity, request['shard'])
    except (OSError, ValueError, TypeError, KeyError):
        raise ExecutionError('staging worker returned invalid receipt') from None
    report = {'schema': 'voice-local-ci-staging-v1', 'status': 'incomplete',
              'source': request['commit'], 'base': request['base'], 'shard': request['shard'], 'recipes': counts['recipes'],
              'hosted_go_tests': counts['hosted_go_tests'], 'hosted_policy_tests': counts['hosted_policy_tests'], 'journal': identity['journal'],
              'slot': identity['slot']}
    with (output / 'diagnostic.json').open('x', encoding='utf-8') as stream:
        json.dump(report, stream, sort_keys=True)
    return report
