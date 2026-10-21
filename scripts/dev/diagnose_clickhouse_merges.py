#!/usr/bin/env python3
"""Preview or CAS-change only DEV ClickHouse merge diagnostic ConfigMap data.

Pause and restore require an independently retained updater lock. Both modes
leave the lock in place. The XML uses a read-only subPath mount, so a separate
reviewed exact-Pod replacement is required to make either transition effective.
Do not leave the pause effective beyond the bounded diagnostic interval.
"""
import argparse
from pathlib import Path
import sys

from expose_clickhouse_errors import (CONFIG, FILE, console_data, config_patch,
                                      replace_after_config, snapshot)
from mitigate_clickhouse_memory import verify_retained_lock
from update_images import Client


def paused_data():
    return {FILE: Path(__file__).with_name('clickhouse-memory-merge-pause.xml').read_text(encoding='utf-8')}


def transition(client, pins, lock_pins, pause, apply=False):
    lock_uid, lock_rv, operation = lock_pins
    expected_lock = {'metadata': {'uid': lock_uid, 'resourceVersion': lock_rv},
                     'data': {'operation': operation}}
    before = console_data() if pause else paused_data()
    after = paused_data() if pause else console_data()
    cm, sts, _ = snapshot(client, pins, config_only=True, expected_data=before,
                          require_not_ready=pause)
    verify_retained_lock(client, expected_lock)
    patch = config_patch(cm, after)
    preview = client.call('patch', 'configmap', CONFIG, '--type=json', '--patch-file=-',
                          '--dry-run=server', '-o', 'json', body=patch)
    if (preview.get('metadata', {}).get('uid') != pins[4] or preview.get('data') != after):
        raise RuntimeError('DEV ClickHouse merge diagnostic dry-run drift')
    print('DEV_CLICKHOUSE_MERGES_PREVIEW mode=' + ('pause' if pause else 'restore') +
          ' configmap_uid=' + pins[4] + ' lock_uid=' + lock_uid)
    if not apply:
        return
    try:
        check_cm, check_sts, _ = snapshot(client, pins, config_only=True,
                                          expected_data=before,
                                          require_not_ready=pause)
        if (check_cm != cm or
                (pause and check_sts['metadata']['resourceVersion'] !=
                 sts['metadata']['resourceVersion']) or check_sts['spec'] != sts['spec']):
            raise RuntimeError('DEV ClickHouse merge diagnostic target changed')
        verify_retained_lock(client, expected_lock)
        result = client.call('patch', 'configmap', CONFIG, '--type=json',
                             '--patch-file=-', '-o', 'json', body=patch)
        if (result.get('metadata', {}).get('uid') != pins[4] or
                result.get('data') != after):
            raise RuntimeError('DEV ClickHouse merge diagnostic patch outcome uncertain')
        observed_cm, observed_sts, _ = snapshot(client, pins, config_only=True,
                                                expected_data=after,
                                                require_not_ready=pause)
        if (observed_cm['metadata']['uid'] != pins[4] or
                (pause and observed_sts['metadata']['resourceVersion'] !=
                 sts['metadata']['resourceVersion']) or observed_sts['spec'] != sts['spec']):
            raise RuntimeError('DEV ClickHouse merge diagnostic readback drift')
        verify_retained_lock(client, expected_lock)
        print('DEV_CLICKHOUSE_MERGES_CONFIG_UPDATED mode=' +
              ('pause' if pause else 'restore') + ' configmap_uid=' + pins[4] +
              ' lock_uid=' + lock_uid + ' lock_retained=true')
    except (ValueError, KeyError, TypeError, RuntimeError):
        print('DEV_CLICKHOUSE_MERGES_LOCK_RECHECK_REQUIRED', file=sys.stderr)
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--mode', choices=('pause', 'replace-paused', 'restore',
                                           'replace-restored'), required=True)
    parser.add_argument('--apply', action='store_true')
    for name in ('statefulset-uid', 'pvc-uid', 'pv-uid', 'configmap-uid',
                 'revision', 'statefulset-generation', 'lock-uid',
                 'lock-resource-version', 'lock-operation'):
        parser.add_argument('--expected-' + name, required=True)
    parser.add_argument('--expected-pod-uid')
    args = parser.parse_args()
    if args.mode.startswith('replace-') and not args.expected_pod_uid:
        parser.error('--expected-pod-uid is required for exact replacement')
    pins = (args.expected_statefulset_uid, args.expected_pod_uid, args.expected_pvc_uid,
            args.expected_pv_uid, args.expected_configmap_uid, args.expected_revision,
            int(args.expected_statefulset_generation))
    lock_pins = (args.expected_lock_uid, args.expected_lock_resource_version,
                 args.expected_lock_operation)
    try:
        if args.mode.startswith('replace-'):
            replace_after_config(Client(), pins, lock_pins, args.apply,
                                 expected_data=(paused_data() if args.mode == 'replace-paused'
                                                else console_data()),
                                 require_oom=args.mode == 'replace-paused')
        else:
            transition(Client(), pins, lock_pins, args.mode == 'pause', args.apply)
    except (ValueError, KeyError, TypeError, RuntimeError):
        print('DEV ClickHouse merge diagnostic failed; inspect retained lock and workload',
              file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
