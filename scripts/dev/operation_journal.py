"""Fixed, read-only DEV operation observation; never a recovery admission check."""
import argparse
import datetime as dt
import json
import os
import re
import subprocess
import sys
import stat
from pathlib import Path

NS = 'voice-dev'
LOCK = 'voice-dev-image-update-lock'
LABELS = {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'}
UUID = r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}'
IMAGE = r'[a-z0-9]+(?:[._-][a-z0-9]+)*(?::[0-9]+)?(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*@sha256:[a-f0-9]{64}'
RECEIPTS = {
    'anton12-user-space-cycle': ('cycle-operation', ('started.json', 'temporary-submitted.json', 'restore-started.json', 'complete.json')),
    'anton12-clickhouse-memory': ('memory-operation', ('started.json', 'quota-submitted.json', 'sts-submitted.json', 'ready.json')),
}


def receipt_markers(directory, names, owner):
    """Observe fixed receipt entry metadata only; never open original env/specs."""
    descriptor = None
    try:
        descriptor = os.open('/', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        for component in directory.parts[1:]:
            child = os.open(component, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=descriptor)
            os.close(descriptor)
            descriptor = child
            value = os.fstat(descriptor)
            if value.st_mode & 0o022:
                raise ValueError('writable_receipt_parent')
        parent = os.fstat(descriptor)
        if not stat.S_ISDIR(parent.st_mode) or parent.st_uid != owner or parent.st_mode & 0o077:
            raise ValueError('private_receipt_directory_required')
        markers = []
        for name in names:
            try:
                value = os.stat(name, dir_fd=descriptor, follow_symlinks=False)
            except FileNotFoundError:
                markers.append({'name': name, 'present': False})
                continue
            if (not stat.S_ISREG(value.st_mode) or value.st_uid != owner or
                    value.st_mode & 0o077 or value.st_nlink != 1):
                raise ValueError('invalid_receipt_entry')
            markers.append({'name': name, 'present': True,
                'filesystem_mtime': dt.datetime.fromtimestamp(value.st_mtime, dt.timezone.utc).isoformat(),
                'size_bytes': value.st_size})
        return {'state': 'OBSERVED_MARKERS_ONLY', 'markers': markers,
                'payload_verified': False, 'operation_outcome': 'UNKNOWN', 'executor_state': 'UNKNOWN'}
    except (OSError, ValueError, OverflowError):
        return {'state': 'UNKNOWN', 'reason': 'receipt_metadata_unavailable'}
    finally:
        if descriptor is not None:
            os.close(descriptor)


def known_receipts():
    root = Path('/home/pmd/voice-restore-anton12')
    return {key: receipt_markers(root / directory, names, os.getuid())
            for key, (directory, names) in RECEIPTS.items()}


def checked(value, pattern):
    if not isinstance(value, str) or not re.fullmatch(pattern, value):
        raise ValueError('invalid_public_field')
    return value


def identity(value, *, name=None, namespaced=False):
    meta = value['metadata']
    actual = checked(meta['name'], r'[a-z0-9][a-z0-9.-]{0,252}')
    if (name is not None and actual != name or
            namespaced and meta.get('namespace') != NS):
        raise ValueError('foreign_resource')
    return {'name': actual, 'uid': checked(meta['uid'], UUID),
            'resource_version': checked(meta['resourceVersion'], r'[0-9]{1,30}')}


def timestamp(value):
    checked(value, r'[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,9})?Z')
    dt.datetime.fromisoformat(value.replace('Z', '+00:00'))
    return value


def integer(value):
    if type(value) is not int or not 0 <= value <= 2**63 - 1:
        raise ValueError('invalid_integer')
    return value


def project(raw):
    namespace = raw['namespace']
    meta = namespace['metadata']
    if (meta.get('deletionTimestamp') or
            any(meta.get('labels', {}).get(k) != v for k, v in LABELS.items())):
        raise ValueError('dev_authority_unavailable')
    result = {'namespace': identity(namespace, name=NS)}
    lock = raw['lock']
    result['lock'] = identity(lock, name=LOCK, namespaced=True)
    result['lock']['operation_id'] = checked(lock.get('data', {}).get('operation'), r'[a-f0-9]{32}')
    result['lock']['created_at'] = timestamp(lock['metadata']['creationTimestamp'])
    # No unvalidated annotations, owner strings, errors, specs, env or Secret data.
    rows = raw['workloads']['items']
    if not isinstance(rows, list) or len(rows) > 128:
        raise ValueError('workload_inventory_invalid')
    # kubectl's combined Deployment/StatefulSet List has no single API revision.
    # Retain each constituent list's resourceVersion rather than inventing one.
    result['workload_list_versions'] = {
        kind: checked(version, r'[0-9]{1,30}')
        for kind, version in raw['workloads']['versions'].items()}
    if set(result['workload_list_versions']) != {'Deployment', 'StatefulSet'}:
        raise ValueError('missing_list_version')
    workloads = []
    seen = set()
    for value in rows:
        if value.get('kind') not in ('Deployment', 'StatefulSet'):
            raise ValueError('unexpected_resource_kind')
        row = identity(value, namespaced=True)
        if not row['name'].startswith('voice-') or (value['kind'], row['name']) in seen:
            raise ValueError('unexpected_workload')
        seen.add((value['kind'], row['name']))
        row['kind'] = value['kind']
        row['generation'] = integer(value['metadata']['generation'])
        row['observed_generation'] = integer(value.get('status', {}).get('observedGeneration', 0))
        row['ready_replicas'] = integer(value.get('status', {}).get('readyReplicas', 0))
        containers = value['spec']['template']['spec']['containers']
        if not isinstance(containers, list) or not 1 <= len(containers) <= 32:
            raise ValueError('container_inventory_invalid')
        row['images'] = []
        for container in containers:
            image = container.get('image')
            row['images'].append({
                'container': checked(container['name'], r'[a-z0-9][a-z0-9-]{0,62}'),
                'immutable_image': image if isinstance(image, str) and re.fullmatch(IMAGE, image) else None})
        workloads.append(row)
    result['workloads'] = sorted(workloads, key=lambda r: (r['kind'], r['name']))
    return result


def snapshot(collect, receipt_collect=lambda: {}):
    started = dt.datetime.now(dt.timezone.utc)
    base = {'schema': 'voice-dev-operation-observation-v1', 'environment': 'dev',
            'mutation_authority': False, 'recovery_admission': False,
            'started_at': started.isoformat()}
    try:
        before = project(collect())
        receipts_before = receipt_collect()
        after = project(collect())
        receipts_after = receipt_collect()
        if before != after or receipts_before != receipts_after:
            return dict(base, state='UNAVAILABLE', reason='changed_during_observation')
        return dict(base, **after, state='PARTIAL', consistency='STABLE_OBSERVATION',
            finished_at=dt.datetime.now(dt.timezone.utc).isoformat(),
            freshness='CURRENT_DURING_OBSERVATION_ONLY',
            scope='DEV_METADATA_AND_FIXED_RECEIPT_MARKERS', request_id=None, candidate=None,
            known_recovery_receipts=receipts_after,
            source_commit=None, phase=None, progress=None, executor_state='UNKNOWN',
            outcome='UNKNOWN', next_checkpoint='reconcile_exact_operation_with_authorized_owner',
            protected_inventory={'state': 'UNKNOWN', 'queue': 'UNKNOWN', 'inbox': 'UNKNOWN',
                'processing': 'UNKNOWN', 'global_lock': 'UNKNOWN', 'journal': 'UNKNOWN',
                'journal_version': None, 'queue_journal_consistency': 'UNKNOWN',
                'reason': 'no_sanctioned_dev_protected_inventory_source'})
    except (OSError, ValueError, KeyError, TypeError, RuntimeError, subprocess.SubprocessError):
        # Never serialize exceptions: subprocess/API errors can include secrets.
        return dict(base, state='UNAVAILABLE', reason='dev_observation_unavailable')


def kubectl(*args):
    command = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
               '--request-timeout=15s', *args, '-o', 'json']
    result = subprocess.run(command, capture_output=True, timeout=20,
        env={'PATH': '/usr/local/bin:/usr/bin:/bin', 'HOME': '/home/pmd', 'LANG': 'C.UTF-8'})
    if result.returncode or len(result.stdout) > 8 << 20:
        raise RuntimeError('read_unavailable')
    return json.loads(result.stdout)


def collect():
    # Fixed GET-only calls; no arbitrary command, target, path, request or nonce.
    namespace = kubectl('get', 'namespace', NS)
    lock = kubectl('-n', NS, 'get', 'configmap', LOCK)
    deployments = kubectl('-n', NS, 'get', 'deployment')
    statefulsets = kubectl('-n', NS, 'get', 'statefulset')
    return {'namespace': namespace, 'lock': lock,
            'workloads': {'versions': {'Deployment': deployments['metadata']['resourceVersion'],
                'StatefulSet': statefulsets['metadata']['resourceVersion']},
                'items': deployments['items'] + statefulsets['items']}}


def main():
    class Parser(argparse.ArgumentParser):
        def error(self, message):
            # argparse normally echoes rejected values, including a supplied
            # token/DSN/path. Reject selectors without reproducing any input.
            print(json.dumps({'state': 'UNAVAILABLE', 'reason': 'unsupported_arguments'}))
            raise SystemExit(2)
    Parser(description=__doc__).parse_args()
    if sys.platform != 'linux' or os.geteuid() == 0:
        print(json.dumps({'state': 'UNAVAILABLE', 'reason': 'ordinary_linux_operator_required'}))
        return 2
    result = snapshot(collect, known_receipts)
    print(json.dumps(result, sort_keys=True))
    # PARTIAL is deliberately nonzero: scripts must not mistake it for admission.
    return 3 if result['state'] == 'PARTIAL' else 2


if __name__ == '__main__':
    raise SystemExit(main())
