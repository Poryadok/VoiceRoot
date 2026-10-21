#!/usr/bin/env python3
"""Preview/apply the reviewed DEV ClickHouse 2Gi -> 4Gi memory repair.

Run only under a current TT DEV resource claim after checking node headroom.
Uses the shared DEV updater lock; never unlocks an interrupted operation.
No image, request, PVC, data, query profile or thread-pool changes are admitted.
"""
import argparse
import copy
import re
import sys

from update_images import Client, LABELS, NS, acquire_lock, release_lock, verify_ownership

NAME = 'voice-clickhouse'
PVC = 'chdata-voice-clickhouse-0'


def memory_bytes(value):
    match = re.fullmatch(r'([0-9]+)(Ki|Mi|Gi)?', value)
    if not match:
        raise ValueError('unsupported memory quantity')
    return int(match[1]) * {'Ki': 1024, 'Mi': 1024 ** 2, 'Gi': 1024 ** 3, None: 1}[match[2]]


def plan(sts):
    meta, spec = sts['metadata'], sts['spec']
    if (sts.get('kind') != 'StatefulSet' or meta.get('namespace') != NS or
            meta.get('name') != NAME or any(meta.get('labels', {}).get(k) != v for k, v in LABELS.items()) or
            spec.get('replicas') != 1 or spec.get('updateStrategy', {}).get('type') != 'RollingUpdate' or
            spec.get('updateStrategy', {}).get('rollingUpdate', {}).get('partition', 0) != 0):
        raise ValueError('unexpected DEV ClickHouse target')
    containers = spec['template']['spec']['containers']
    if len(containers) != 1:
        raise ValueError('exact ClickHouse container required')
    container = containers[0]
    if (container.get('name') != 'clickhouse' or not re.fullmatch(
            r'docker.io/clickhouse/clickhouse-server@sha256:[0-9a-f]{64}', container.get('image', '')) or
            {'name': 'chdata', 'mountPath': '/var/lib/clickhouse'} not in container.get('volumeMounts', []) or
            [c['metadata']['name'] for c in spec.get('volumeClaimTemplates', [])] != ['chdata'] or
            container['resources']['requests'].get('memory') != '256Mi' or
            container['resources']['limits'].get('memory') not in ('2Gi', '2048Mi', '4Gi', '4096Mi')):
        raise ValueError('unexpected ClickHouse image, storage or resource baseline')
    expected = copy.deepcopy(sts)
    expected['spec']['template']['spec']['containers'][0]['resources']['limits']['memory'] = '4Gi'
    patch = [{'op': 'test', 'path': '/metadata/uid', 'value': meta['uid']},
             {'op': 'test', 'path': '/metadata/resourceVersion', 'value': meta['resourceVersion']},
             {'op': 'test', 'path': '/spec', 'value': spec},
             {'op': 'replace', 'path': '/spec/template/spec/containers/0/resources/limits/memory', 'value': '4Gi'}]
    return patch, expected


def verify_pvc(client, uid=None):
    pvc = client.call('get', 'pvc', PVC, '-o', 'json')
    actual = pvc['metadata']['uid']
    if pvc.get('status', {}).get('phase') != 'Bound' or uid is not None and uid != actual:
        raise RuntimeError('ClickHouse PVC identity or binding changed')
    return actual


def verify_quota(client, sts):
    quota = client.call('get', 'resourcequota', 'dev-budget', '-o', 'json')['status']
    old = sts['spec']['template']['spec']['containers'][0]['resources']['limits']['memory']
    target = memory_bytes(quota['used']['limits.memory']) - memory_bytes(old) + memory_bytes('4Gi')
    if target > memory_bytes(quota['hard']['limits.memory']):
        raise ValueError('DEV memory limit quota insufficient for repair')


def repair(client, apply=False):
    verify_ownership(client)
    lock = acquire_lock(client) if apply else None
    mutation_started = success = False
    try:
        sts = client.call('get', 'statefulset', NAME, '-o', 'json')
        patch, expected = plan(sts)
        pvc_uid = verify_pvc(client)
        verify_quota(client, sts)
        old = sts['spec']['template']['spec']['containers'][0]['resources']['limits']['memory']
        changed = memory_bytes(old) != memory_bytes('4Gi')
        if changed:
            preview = client.call('patch', 'statefulset', NAME, '--type=json', '--patch-file=-',
                                  '--dry-run=server', '-o', 'json', body=patch)
            if preview.get('spec') != expected['spec']:
                raise RuntimeError('server dry-run changed unexpected specification fields')
        print('DEV_CLICKHOUSE_MEMORY_PREVIEW old=' + old + ' target=4Gi pvc_uid=' + pvc_uid)
        if not apply:
            return
        verify_ownership(client, lock)
        verify_pvc(client, pvc_uid)
        verify_quota(client, sts)
        if changed:
            mutation_started = True
            result = client.call('patch', 'statefulset', NAME, '--type=json', '--patch-file=-', '-o', 'json', body=patch)
            if result.get('spec') != expected['spec']:
                raise RuntimeError('StatefulSet specification changed beyond memory ceiling')
        client.call('rollout', 'status', 'statefulset/' + NAME, '--timeout=600s')
        verify_ownership(client, lock)
        actual = client.call('get', 'statefulset', NAME, '-o', 'json')
        # Quantity spelling may remain 4096Mi on an already-repaired target.
        if not changed:
            expected = sts
        if actual['metadata']['uid'] != sts['metadata']['uid'] or actual.get('spec') != expected['spec']:
            raise RuntimeError('ClickHouse specification drift during rollout')
        verify_pvc(client, pvc_uid)
        success = True
        print('DEV_CLICKHOUSE_MEMORY_READY limit=4Gi pvc_uid=' + pvc_uid)
    finally:
        if lock is not None:
            if success or not mutation_started:
                release_lock(client, lock)
            else:
                print('DEV_CLICKHOUSE_LOCK_RETAINED', file=sys.stderr)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true')
    args = parser.parse_args()
    try:
        repair(Client(), args.apply)
    except (ValueError, KeyError, TypeError, RuntimeError):
        print('DEV ClickHouse repair failed; inspect workload and retained DEV lock', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
