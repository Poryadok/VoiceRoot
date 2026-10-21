#!/usr/bin/env python3
"""Resume the retained DEV ClickHouse updater operation with one 0.8 memory policy.

This command never acquires or replaces the existing lock. A failed or uncertain
create, patch, rollout, or readback retains it for operator reconciliation.
"""
import argparse
import copy
from pathlib import Path
import re
import subprocess
import sys

import environment
from repair_clickhouse_memory import memory_bytes, verify_pvc
from update_images import Client, LABELS, LOCK, NS, release_lock, verify_ownership

NAME = 'voice-clickhouse'
POD = NAME + '-0'
CONFIG = 'voice-clickhouse-memory-policy'
FILE = '90-voice-dev-memory.xml'
IMAGE = ('docker.io/clickhouse/clickhouse-server@sha256:'
         'b002e56ed5c16e224c312527f6fcba7e77216fec5d7a88a7828f59efc614feb5')


def clickhouse_memory_config():
    return {'apiVersion': 'v1', 'kind': 'ConfigMap',
            'metadata': environment.metadata(CONFIG),
            'data': {FILE: Path(__file__).with_name('clickhouse-memory-0.8.xml').read_text(encoding='utf-8')}}


def add_clickhouse_memory_mount(pod):
    if (len(pod.get('containers', [])) != 1 or
            pod['containers'][0].get('name') != 'clickhouse' or
            pod.get('volumes') or
            any(m.get('mountPath') == '/etc/clickhouse-server/config.d/' + FILE
                for m in pod['containers'][0].get('volumeMounts', []))):
        raise ValueError('unexpected ClickHouse config mount baseline')
    pod['volumes'] = [{'name': 'clickhouse-memory-policy',
                       'configMap': {'name': CONFIG, 'defaultMode': 420,
                                     'items': [{'key': FILE, 'path': FILE}]}}]
    pod['containers'][0].setdefault('volumeMounts', []).append({
        'name': 'clickhouse-memory-policy',
        'mountPath': '/etc/clickhouse-server/config.d/' + FILE,
        'subPath': FILE, 'readOnly': True})


def verify_retained_lock(client, original):
    current = client.call('get', 'configmap', LOCK, '-o', 'json')
    meta, expected = current['metadata'], original['metadata']
    if (meta.get('namespace') != NS or meta.get('name') != LOCK or
            meta.get('uid') != expected['uid'] or
            meta.get('resourceVersion') != expected['resourceVersion'] or
            any(meta.get('labels', {}).get(k) != v for k, v in LABELS.items()) or
            current.get('data') != {'operation': original['data']['operation']}):
        raise RuntimeError('retained DEV updater lock CAS changed')
    return current


def verify_pvc_binding(client, uid, expected_pv=None):
    verify_pvc(client, uid)
    pvc = client.call('get', 'pvc', 'chdata-voice-clickhouse-0', '-o', 'json')
    meta = pvc['metadata']
    volume = pvc.get('spec', {}).get('volumeName')
    if (meta.get('namespace') != NS or meta.get('name') != 'chdata-voice-clickhouse-0' or
            meta.get('uid') != uid or pvc.get('status', {}).get('phase') != 'Bound' or
            not volume or expected_pv is not None and volume != expected_pv):
        raise RuntimeError('DEV ClickHouse PVC binding changed')
    return volume


def read_ratio_and_cap():
    command = ['kubectl', '--namespace', NS, 'exec', POD, '-c', 'clickhouse', '--',
               'clickhouse', 'extract-from-config',
               '--config-file=/etc/clickhouse-server/config.xml',
               '--key=max_server_memory_usage_to_ram_ratio']
    try:
        result = subprocess.run(command, capture_output=True, timeout=30)
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise RuntimeError('ClickHouse config readback unavailable') from exc
    if result.returncode or result.stdout.strip() != b'0.8':
        raise RuntimeError('ClickHouse effective memory ratio is not 0.8')
    # The startup log reports the *derived* server cap, unlike config.xml's 0.
    command = ['kubectl', '--namespace', NS, 'exec', POD, '-c', 'clickhouse', '--',
               'grep', 'Setting max_server_memory_usage was set to',
               '/var/log/clickhouse-server/clickhouse-server.log']
    try:
        result = subprocess.run(command, capture_output=True, timeout=30)
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise RuntimeError('ClickHouse memory cap log unavailable') from exc
    if result.returncode or not re.search(rb'3[.]20 GiB.*0[.]80 max_server_memory_usage_to_ram_ratio',
                                          result.stdout):
        raise RuntimeError('ClickHouse derived 3.20 GiB server cap not observed')


def snapshot(client, pins):
    lock_uid, operation, sts_uid, pod_uid, pvc_uid = pins
    verify_ownership(client)
    lock = client.call('get', 'configmap', LOCK, '-o', 'json')
    lm = lock['metadata']
    if (lm.get('namespace') != NS or lm.get('name') != LOCK or
            any(lm.get('labels', {}).get(k) != v for k, v in LABELS.items()) or
            lm.get('uid') != lock_uid or not lm.get('resourceVersion') or
            lock.get('data') != {'operation': operation}):
        raise RuntimeError('retained DEV updater lock changed')
    sts = client.call('get', 'statefulset', NAME, '-o', 'json')
    meta, spec = sts['metadata'], sts['spec']
    pod_spec = spec['template']['spec']
    if (sts.get('kind') != 'StatefulSet' or meta.get('namespace') != NS or
            meta.get('name') != NAME or meta.get('uid') != sts_uid or
            any(meta.get('labels', {}).get(k) != v for k, v in LABELS.items()) or
            spec.get('replicas') != 1 or spec.get('updateStrategy', {}).get('type') != 'RollingUpdate' or
            spec.get('updateStrategy', {}).get('rollingUpdate', {}).get('partition', 0) != 0 or
            len(pod_spec.get('containers', [])) != 1 or
            [c['metadata']['name'] for c in spec.get('volumeClaimTemplates', [])] != ['chdata'] or
            spec['template']['spec'].get('volumes') or
            sts.get('status', {}).get('observedGeneration') != meta.get('generation') or
            sts['status'].get('currentRevision') != sts['status'].get('updateRevision') or
            sts['status'].get('updatedReplicas') != 1 or
            sts['status'].get('readyReplicas', 0) != 0):
        raise RuntimeError('DEV ClickHouse is not the exact 4Gi stalled StatefulSet')
    container = pod_spec['containers'][0]
    if (container.get('name') != 'clickhouse' or
            container.get('image') != IMAGE or
            memory_bytes(container['resources']['limits']['memory']) != memory_bytes('4Gi') or
            container['resources']['requests'].get('memory') != '256Mi' or
            container.get('volumeMounts') != [{'name': 'chdata', 'mountPath': '/var/lib/clickhouse'}]):
        raise RuntimeError('DEV ClickHouse image, 4Gi cap or storage changed')
    volume = verify_pvc_binding(client, pvc_uid)
    pod = client.call('get', 'pod', POD, '-o', 'json')
    pm = pod['metadata']
    owners = pm.get('ownerReferences', [])
    status = pod.get('status', {}).get('containerStatuses', [])
    if (pm.get('namespace') != NS or pm.get('name') != POD or pm.get('uid') != pod_uid or
            pm.get('deletionTimestamp') or len(owners) != 1 or
            owners[0].get('uid') != sts_uid or owners[0].get('kind') != 'StatefulSet' or
            owners[0].get('name') != NAME or owners[0].get('controller') is not True or
            pm.get('labels', {}).get('controller-revision-hash') != sts['status']['updateRevision'] or
            pod['spec']['containers'][0]['image'] != container['image'] or
            memory_bytes(pod['spec']['containers'][0]['resources']['limits']['memory']) != memory_bytes('4Gi') or
            not any(v.get('name') == 'chdata' and
                    v.get('persistentVolumeClaim', {}).get('claimName') == 'chdata-voice-clickhouse-0'
                    for v in pod['spec'].get('volumes', [])) or
            len(status) != 1 or status[0].get('ready') is not False or
            status[0].get('lastState', {}).get('terminated', {}).get('reason') != 'OOMKilled'):
        raise RuntimeError('DEV ClickHouse pod is not the pinned 4Gi OOM target')
    quota = client.call('get', 'resourcequota', 'dev-budget', '-o', 'json')['status']
    if memory_bytes(quota['used']['limits.memory']) > memory_bytes(quota['hard']['limits.memory']):
        raise RuntimeError('DEV memory quota is exceeded')
    return lock, sts, pod, volume


def plan(sts):
    expected = copy.deepcopy(sts)
    add_clickhouse_memory_mount(expected['spec']['template']['spec'])
    patch = [{'op': 'test', 'path': '/metadata/uid', 'value': sts['metadata']['uid']},
             {'op': 'test', 'path': '/metadata/resourceVersion',
              'value': sts['metadata']['resourceVersion']},
             {'op': 'test', 'path': '/spec', 'value': sts['spec']},
             {'op': 'add', 'path': '/spec/template/spec/volumes',
              'value': expected['spec']['template']['spec']['volumes']},
             {'op': 'add', 'path': '/spec/template/spec/containers/0/volumeMounts/-',
              'value': expected['spec']['template']['spec']['containers'][0]['volumeMounts'][-1]}]
    return patch, expected


def mitigate(client, pins, apply=False, readback=read_ratio_and_cap):
    lock, sts, pod, original_volume = snapshot(client, pins)
    config = clickhouse_memory_config()
    patch, expected = plan(sts)
    preview_config = client.call('create', '-f', '-', '--dry-run=server', '-o', 'json', body=config)
    if preview_config.get('data') != config['data']:
        raise RuntimeError('DEV ClickHouse ConfigMap server dry-run drift')
    preview_sts = client.call('patch', 'statefulset', NAME, '--type=json',
                              '--patch-file=-', '--dry-run=server', '-o', 'json', body=patch)
    if preview_sts.get('spec') != expected['spec']:
        raise RuntimeError('DEV ClickHouse StatefulSet server dry-run drift')
    print('DEV_CLICKHOUSE_POLICY_PREVIEW ratio=0.8 limit=4Gi pod_uid=' + pins[3])
    if not apply:
        return
    check_lock, check_sts, check_pod, check_volume = snapshot(client, pins)
    if (check_lock != lock or check_sts['metadata']['resourceVersion'] != sts['metadata']['resourceVersion'] or
            check_sts['spec'] != sts['spec'] or check_pod['metadata']['uid'] != pod['metadata']['uid'] or
            check_volume != original_volume):
        raise RuntimeError('DEV ClickHouse policy target changed before mutation')
    try:
        created = client.call('create', '-f', '-', '-o', 'json', body=config)
        if (not created.get('metadata', {}).get('uid') or
                created.get('data') != config['data']):
            raise RuntimeError('DEV ClickHouse ConfigMap create outcome uncertain')
        if client.call('get', 'configmap', CONFIG, '-o', 'json')['metadata']['uid'] != created['metadata']['uid']:
            raise RuntimeError('DEV ClickHouse ConfigMap identity drift')
        verify_retained_lock(client, lock)
        verify_pvc_binding(client, pins[4], original_volume)
        result = client.call('patch', 'statefulset', NAME, '--type=json', '--patch-file=-',
                             '-o', 'json', body=patch)
        if result.get('spec') != expected['spec']:
            raise RuntimeError('DEV ClickHouse patch changed other spec fields')
        client.call('rollout', 'status', 'statefulset/' + NAME, '--timeout=600s')
        verify_retained_lock(client, lock)
        actual_sts = client.call('get', 'statefulset', NAME, '-o', 'json')
        actual_pod = client.call('get', 'pod', POD, '-o', 'json')
        actual_meta = actual_pod['metadata']
        actual_owners = actual_meta.get('ownerReferences', [])
        actual_status = actual_sts.get('status', {})
        if (actual_sts['metadata'].get('namespace') != NS or
                actual_sts['metadata']['uid'] != pins[2] or
                actual_sts.get('spec') != expected['spec'] or
                actual_status.get('observedGeneration') != actual_sts['metadata'].get('generation') or
                actual_status.get('replicas') != 1 or
                actual_sts.get('status', {}).get('readyReplicas') != 1 or
                actual_sts['status'].get('updatedReplicas') != 1 or
                actual_sts['status'].get('currentRevision') != actual_sts['status'].get('updateRevision') or
                actual_meta.get('namespace') != NS or actual_meta.get('name') != POD or
                actual_meta.get('uid') == pins[3] or actual_meta.get('deletionTimestamp') or
                len(actual_owners) != 1 or actual_owners[0].get('kind') != 'StatefulSet' or
                actual_owners[0].get('name') != NAME or actual_owners[0].get('uid') != pins[2] or
                actual_owners[0].get('controller') is not True or
                actual_meta.get('labels', {}).get('controller-revision-hash') !=
                actual_sts['status']['updateRevision'] or
                actual_pod['spec']['containers'][0]['image'] != IMAGE or
                memory_bytes(actual_pod['spec']['containers'][0]['resources']['limits']['memory']) !=
                memory_bytes('4Gi') or
                not any(v.get('name') == 'chdata' and
                        v.get('persistentVolumeClaim', {}).get('claimName') ==
                        'chdata-voice-clickhouse-0' for v in actual_pod['spec'].get('volumes', [])) or
                not any(m.get('name') == 'clickhouse-memory-policy' and
                        m.get('readOnly') is True and m.get('subPath') == FILE and
                        m.get('mountPath') == '/etc/clickhouse-server/config.d/' + FILE
                        for m in actual_pod['spec']['containers'][0].get('volumeMounts', [])) or
                actual_pod['status']['containerStatuses'][0].get('ready') is not True):
            raise RuntimeError('DEV ClickHouse policy rollout not ready on exact image')
        verify_pvc_binding(client, pins[4], original_volume)
        if client.call('get', 'configmap', CONFIG, '-o', 'json')['metadata']['uid'] != created['metadata']['uid']:
            raise RuntimeError('DEV ClickHouse ConfigMap changed during rollout')
        readback()
        verify_retained_lock(client, lock)
        release_lock(client, lock)
        print('DEV_CLICKHOUSE_POLICY_READY ratio=0.8 cap=3.20GiB pod_uid=' +
              actual_pod['metadata']['uid'] + ' pvc_uid=' + pins[4])
    except (ValueError, KeyError, TypeError, RuntimeError):
        print('DEV_CLICKHOUSE_LOCK_RETAINED', file=sys.stderr)
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true')
    for name in ('lock-uid', 'lock-operation', 'statefulset-uid', 'pod-uid', 'pvc-uid'):
        parser.add_argument('--expected-' + name, required=True)
    args = parser.parse_args()
    pins = (args.expected_lock_uid, args.expected_lock_operation,
            args.expected_statefulset_uid, args.expected_pod_uid, args.expected_pvc_uid)
    try:
        mitigate(Client(), pins, args.apply)
    except (ValueError, KeyError, TypeError, RuntimeError):
        print('DEV ClickHouse policy mitigation failed; inspect workload and retained lock', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
