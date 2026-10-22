#!/usr/bin/env python3
"""Guard a DEV-only, table-scoped metric_log setting and its exact rollback.

ConfigMap transitions and exact-Pod replacement are separate. The retained
updater lock is never released here. A replacement is needed because config.d
is mounted through subPath. Startup can roll the current system.metric_log to
metric_log_N; inspect table UUIDs and parts before and after each replacement.
"""
import argparse
from pathlib import Path
import sys

from expose_clickhouse_errors import (CONFIG, FILE, config_patch, console_data,
                                      replace_after_config, snapshot)
from mitigate_clickhouse_memory import verify_retained_lock
from update_images import Client


def persistent_data():
    return {FILE: Path(__file__).with_name('clickhouse-memory-metric-log-1024.xml')
            .read_text(encoding='utf-8')}


def transition(client, pins, lock_pins, enable, apply=False):
    lock = {'metadata': {'uid': lock_pins[0], 'resourceVersion': lock_pins[1]},
            'data': {'operation': lock_pins[2]}}
    before = console_data() if enable else persistent_data()
    after = persistent_data() if enable else console_data()
    cm, sts, _ = snapshot(client, pins, config_only=True, expected_data=before,
                          require_not_ready=False)
    verify_retained_lock(client, lock)
    patch = config_patch(cm, after)
    preview = client.call('patch', 'configmap', CONFIG, '--type=json', '--patch-file=-',
                          '--dry-run=server', '-o', 'json', body=patch)
    if (preview.get('metadata', {}).get('uid') != pins[4] or
            preview.get('data') != after):
        raise RuntimeError('DEV metric_log ConfigMap server dry-run drift')
    print('DEV_METRIC_LOG_CONFIG_PREVIEW mode=' + ('enable' if enable else 'rollback') +
          ' configmap_uid=' + pins[4] + ' configmap_rv=' + cm['metadata']['resourceVersion'] +
          ' lock_uid=' + lock_pins[0])
    if not apply:
        return
    try:
        check_cm, check_sts, _ = snapshot(client, pins, config_only=True,
                                          expected_data=before, require_not_ready=False)
        if (check_cm != cm or check_sts['spec'] != sts['spec'] or
                check_sts['metadata']['resourceVersion'] != sts['metadata']['resourceVersion']):
            raise RuntimeError('DEV metric_log ConfigMap target drifted')
        verify_retained_lock(client, lock)
        updated = client.call('patch', 'configmap', CONFIG, '--type=json',
                              '--patch-file=-', '-o', 'json', body=patch)
        if (updated.get('metadata', {}).get('uid') != pins[4] or
                updated.get('data') != after):
            raise RuntimeError('DEV metric_log ConfigMap patch uncertain')
        observed_cm, observed_sts, _ = snapshot(client, pins, config_only=True,
                                                expected_data=after, require_not_ready=False)
        if (observed_cm['metadata']['uid'] != pins[4] or
                observed_sts['spec'] != sts['spec'] or
                observed_sts['metadata']['resourceVersion'] != sts['metadata']['resourceVersion']):
            raise RuntimeError('DEV metric_log ConfigMap readback drift')
        verify_retained_lock(client, lock)
        print('DEV_METRIC_LOG_CONFIG_UPDATED mode=' + ('enable' if enable else 'rollback') +
              ' configmap_uid=' + pins[4] + ' configmap_rv=' +
              observed_cm['metadata']['resourceVersion'] + ' lock_retained=true')
    except (ValueError, KeyError, TypeError, RuntimeError):
        print('DEV_METRIC_LOG_LOCK_RECHECK_REQUIRED', file=sys.stderr)
        raise


def replace(client, pins, lock_pins, enabled, apply=False):
    expected_data = persistent_data() if enabled else console_data()
    if enabled:
        _, sts, pod = snapshot(client, pins, expected_data=expected_data,
                               require_oom=False, require_not_ready=False)
        if (sts['status'].get('readyReplicas') != 1 or
                pod['status']['containerStatuses'][0].get('ready') is not True):
            raise RuntimeError('DEV metric_log enable requires a Ready current Pod')
    replace_after_config(client, pins, lock_pins, apply=apply,
                         expected_data=expected_data, require_oom=False,
                         require_ready=enabled)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--mode', choices=('enable', 'replace-enabled', 'rollback',
                                           'replace-rollback'), required=True)
    parser.add_argument('--apply', action='store_true')
    for name in ('statefulset-uid', 'pvc-uid', 'pv-uid', 'configmap-uid', 'revision',
                 'statefulset-generation', 'lock-uid', 'lock-resource-version',
                 'lock-operation'):
        parser.add_argument('--expected-' + name, required=True)
    parser.add_argument('--expected-pod-uid')
    args = parser.parse_args()
    if args.mode.startswith('replace-') and not args.expected_pod_uid:
        parser.error('--expected-pod-uid required for exact replacement')
    pins = (args.expected_statefulset_uid, args.expected_pod_uid, args.expected_pvc_uid,
            args.expected_pv_uid, args.expected_configmap_uid, args.expected_revision,
            int(args.expected_statefulset_generation))
    lock_pins = (args.expected_lock_uid, args.expected_lock_resource_version,
                 args.expected_lock_operation)
    try:
        if args.mode.startswith('replace-'):
            replace(Client(), pins, lock_pins, args.mode == 'replace-enabled', args.apply)
        else:
            transition(Client(), pins, lock_pins, args.mode == 'enable', args.apply)
    except (ValueError, KeyError, TypeError, RuntimeError) as exc:
        print('DEV metric_log persistence aborted; inspect retained lock: ' + str(exc),
              file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
