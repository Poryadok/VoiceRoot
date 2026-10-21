#!/usr/bin/env python3
"""Expose DEV ClickHouse startup errors without changing memory or data.

Preview uses server dry-runs only. Apply creates the shared updater lock and
CAS-patches the existing policy ConfigMap. The default mode also gracefully
deletes only the pinned OOM pod with UID/resourceVersion preconditions.
--config-only never reads or deletes a Pod; a later natural replacement consumes
the updated read-only config.d subPath. The lock is retained for operator
readback. This command never force-deletes or unlocks anything.
"""
import argparse
from pathlib import Path
import sys

from mitigate_clickhouse_memory import (CONFIG, FILE, IMAGE, NAME, POD,
                                        clickhouse_memory_config,
                                        verify_retained_lock)
from repair_clickhouse_memory import memory_bytes
from update_images import Client, LABELS, LOCK, NS, acquire_lock, verify_ownership

PVC = 'chdata-voice-clickhouse-0'
PV = 'voice-dev-clickhouse'


def console_data():
    return {FILE: Path(__file__).with_name('clickhouse-memory-console.xml').read_text(encoding='utf-8')}


def snapshot(client, pins, console=False, config_only=False):
    sts_uid, pod_uid, pvc_uid, pv_uid, config_uid, revision, generation = pins
    verify_ownership(client)
    cm = client.call('get', 'configmap', CONFIG, '-o', 'json')
    cm_meta = cm['metadata']
    expected_data = console_data() if console else clickhouse_memory_config()['data']
    if (cm_meta.get('namespace') != NS or cm_meta.get('name') != CONFIG or
            cm_meta.get('uid') != config_uid or not cm_meta.get('resourceVersion') or
            any(cm_meta.get('labels', {}).get(k) != v for k, v in LABELS.items()) or
            cm.get('data') != expected_data):
        raise RuntimeError('DEV ClickHouse memory ConfigMap is not the pinned policy')
    sts = client.call('get', 'statefulset', NAME, '-o', 'json')
    meta, spec, status = sts['metadata'], sts['spec'], sts['status']
    pod_spec = spec['template']['spec']
    if (sts.get('kind') != 'StatefulSet' or meta.get('namespace') != NS or
            meta.get('name') != NAME or meta.get('uid') != sts_uid or
            meta.get('generation') != generation or not meta.get('resourceVersion') or
            any(meta.get('labels', {}).get(k) != v for k, v in LABELS.items()) or
            spec.get('replicas') != 1 or spec.get('podManagementPolicy') != 'OrderedReady' or
            spec.get('updateStrategy', {}).get('type') != 'RollingUpdate' or
            spec.get('updateStrategy', {}).get('rollingUpdate', {}).get('partition', 0) != 0 or
            status.get('observedGeneration') != generation or
            status.get('currentRevision') != revision or status.get('updateRevision') != revision or
            status.get('updatedReplicas') != 1 or status.get('readyReplicas', 0) != 0 or
            len(pod_spec.get('containers', [])) != 1 or
            pod_spec.get('volumes') != [{'name': 'clickhouse-memory-policy',
                                        'configMap': {'name': CONFIG, 'defaultMode': 420,
                                                      'items': [{'key': FILE, 'path': FILE}]}}]):
        raise RuntimeError('DEV ClickHouse StatefulSet is not the exact failed ratio 0.8 revision')
    container = pod_spec['containers'][0]
    if (container.get('name') != 'clickhouse' or container.get('image') != IMAGE or
            memory_bytes(container['resources']['limits']['memory']) != memory_bytes('4Gi') or
            container['resources']['requests'].get('memory') != '256Mi' or
            container.get('volumeMounts') != [
                {'name': 'chdata', 'mountPath': '/var/lib/clickhouse'},
                {'name': 'clickhouse-memory-policy',
                 'mountPath': '/etc/clickhouse-server/config.d/' + FILE,
                 'subPath': FILE, 'readOnly': True}]):
        raise RuntimeError('DEV ClickHouse image, memory or mounts changed')
    pod = None
    if not config_only:
        pod = client.call('get', 'pod', POD, '-o', 'json')
        pm = pod['metadata']
        owners = pm.get('ownerReferences', [])
        statuses = pod.get('status', {}).get('containerStatuses', [])
        if (pm.get('namespace') != NS or pm.get('name') != POD or pm.get('uid') != pod_uid or
                not pm.get('resourceVersion') or pm.get('deletionTimestamp') or
                len(owners) != 1 or owners[0].get('kind') != 'StatefulSet' or
                owners[0].get('name') != NAME or owners[0].get('uid') != sts_uid or
                owners[0].get('controller') is not True or
                pm.get('labels', {}).get('controller-revision-hash') != revision or
                pod['spec'].get('containers') != pod_spec['containers'] or
                not any(v.get('name') == 'chdata' and
                        v.get('persistentVolumeClaim', {}).get('claimName') == PVC
                        for v in pod['spec'].get('volumes', [])) or
                len(statuses) != 1 or statuses[0].get('ready') is not False or
                statuses[0].get('lastState', {}).get('terminated', {}).get('reason') != 'OOMKilled'):
            raise RuntimeError('DEV ClickHouse pod is not the pinned OOM target')
    pvc = client.call('get', 'pvc', PVC, '-o', 'json')
    pvc_meta = pvc['metadata']
    pv = client.call('get', 'pv', PV, '-o', 'json')
    if (pvc_meta.get('namespace') != NS or pvc_meta.get('name') != PVC or
            pvc_meta.get('uid') != pvc_uid or pvc.get('status', {}).get('phase') != 'Bound' or
            pvc.get('spec', {}).get('volumeName') != PV or
            pv['metadata'].get('uid') != pv_uid or pv.get('status', {}).get('phase') != 'Bound' or
            pv['spec'].get('claimRef', {}).get('uid') != pvc_uid or
            pv['spec']['claimRef'].get('namespace') != NS or
            pv['spec'].get('persistentVolumeReclaimPolicy') != 'Retain'):
        raise RuntimeError('DEV ClickHouse PVC/PV identity or binding changed')
    quota = client.call('get', 'resourcequota', 'dev-budget', '-o', 'json')['status']
    if (memory_bytes(quota['used']['limits.memory']) > memory_bytes(quota['hard']['limits.memory']) or
            memory_bytes(quota['hard']['limits.memory']) != memory_bytes('20Gi')):
        raise RuntimeError('DEV memory quota changed')
    return cm, sts, pod


def config_patch(cm):
    meta = cm['metadata']
    return [{'op': 'test', 'path': '/metadata/uid', 'value': meta['uid']},
            {'op': 'test', 'path': '/metadata/resourceVersion', 'value': meta['resourceVersion']},
            {'op': 'test', 'path': '/data', 'value': cm['data']},
            {'op': 'replace', 'path': '/data/' + FILE, 'value': console_data()[FILE]}]


def delete_options(pod):
    meta = pod['metadata']
    return {'apiVersion': 'v1', 'kind': 'DeleteOptions',
            'preconditions': {'uid': meta['uid'], 'resourceVersion': meta['resourceVersion']}}


def dry_run(client, cm, pod):
    preview_cm = client.call('patch', 'configmap', CONFIG, '--type=json', '--patch-file=-',
                             '--dry-run=server', '-o', 'json', body=config_patch(cm))
    if (preview_cm.get('metadata', {}).get('uid') != cm['metadata']['uid'] or
            preview_cm.get('data') != console_data()):
        raise RuntimeError('DEV ClickHouse console ConfigMap server dry-run drift')
    if pod is None:
        return
    path = '/api/v1/namespaces/' + NS + '/pods/' + POD
    preview_pod = client.call('delete', '--raw=' + path + '?dryRun=All', '-f', '-',
                              body=delete_options(pod))
    if preview_pod.get('metadata', {}).get('uid') != pod['metadata']['uid']:
        raise RuntimeError('DEV ClickHouse Pod delete server dry-run drift')


def expose(client, pins, apply=False, config_only=False):
    cm, sts, pod = snapshot(client, pins, config_only=config_only)
    dry_run(client, cm, pod)
    print('DEV_CLICKHOUSE_CONSOLE_PREVIEW ' +
          ('config_only=true' if config_only else 'pod_uid=' + pod['metadata']['uid']))
    if not apply:
        return
    lock = acquire_lock(client)
    try:
        check_cm, check_sts, check_pod = snapshot(client, pins, config_only=config_only)
        if (check_cm != cm or check_sts['metadata']['resourceVersion'] != sts['metadata']['resourceVersion'] or
                check_sts['spec'] != sts['spec'] or
                (not config_only and check_pod['metadata']['uid'] != pod['metadata']['uid'])):
            raise RuntimeError('DEV ClickHouse diagnostic target changed before mutation')
        verify_retained_lock(client, lock)
        patched = client.call('patch', 'configmap', CONFIG, '--type=json', '--patch-file=-',
                              '-o', 'json', body=config_patch(cm))
        if (patched.get('metadata', {}).get('uid') != pins[4] or patched.get('data') != console_data()):
            raise RuntimeError('DEV ClickHouse console ConfigMap patch outcome uncertain')
        updated_cm, current_sts, current_pod = snapshot(client, pins, console=True,
                                                       config_only=config_only)
        if (updated_cm['metadata']['uid'] != cm['metadata']['uid'] or
                current_sts['metadata']['resourceVersion'] != sts['metadata']['resourceVersion'] or
                current_sts['spec'] != sts['spec']):
            raise RuntimeError('DEV ClickHouse diagnostic baseline changed after ConfigMap patch')
        if config_only:
            verify_retained_lock(client, lock)
            print('DEV_CLICKHOUSE_CONSOLE_CONFIG_UPDATED configmap_uid=' + pins[4] +
                  ' lock_uid=' + lock['metadata']['uid'] + ' lock_retained=true')
            return
        verify_retained_lock(client, lock)
        dry_path = '/api/v1/namespaces/' + NS + '/pods/' + POD + '?dryRun=All'
        options = delete_options(current_pod)
        preview = client.call('delete', '--raw=' + dry_path, '-f', '-', body=options)
        if preview.get('metadata', {}).get('uid') != pins[1]:
            raise RuntimeError('DEV ClickHouse Pod delete precondition dry-run drift')
        result = client.call('delete', '--raw=/api/v1/namespaces/' + NS + '/pods/' + POD,
                             '-f', '-', body=options)
        if result.get('metadata', {}).get('uid') != pins[1]:
            raise RuntimeError('DEV ClickHouse Pod delete outcome uncertain')
        print('DEV_CLICKHOUSE_CONSOLE_REPLACEMENT_SUBMITTED old_pod_uid=' + pins[1] +
              ' lock_uid=' + lock['metadata']['uid'] + ' lock_retained=true')
    except (ValueError, KeyError, TypeError, RuntimeError):
        print('DEV_CLICKHOUSE_LOCK_RETAINED', file=sys.stderr)
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true')
    parser.add_argument('--config-only', action='store_true',
                        help='CAS-patch only the existing ConfigMap; never delete a pod')
    for name in ('statefulset-uid', 'pvc-uid', 'pv-uid',
                 'configmap-uid', 'revision', 'statefulset-generation'):
        parser.add_argument('--expected-' + name, required=True)
    parser.add_argument('--expected-pod-uid')
    args = parser.parse_args()
    if not args.config_only and not args.expected_pod_uid:
        parser.error('--expected-pod-uid is required without --config-only')
    pins = (args.expected_statefulset_uid, args.expected_pod_uid, args.expected_pvc_uid,
            args.expected_pv_uid, args.expected_configmap_uid, args.expected_revision,
            int(args.expected_statefulset_generation))
    try:
        expose(Client(), pins, args.apply, args.config_only)
    except (ValueError, KeyError, TypeError, RuntimeError):
        print('DEV ClickHouse console diagnostic failed; inspect retained lock and workload',
              file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
