#!/usr/bin/env python3
"""Guard a paused DEV metric_log ALTER by exact table UUID and part hashes.

This command never changes the ConfigMap, StatefulSet, Pod, PVC or updater lock.
The separately reviewed pause/restore commands own the bounded diagnostic window.
An ALTER has no ClickHouse-side UUID CAS: a concurrent table rename between the
last read and ALTER is an unresolved race, so require a stable Ready Pod and
exclusive updater lock and retain that lock on any uncertain result.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

from diagnose_clickhouse_merges import paused_data
from expose_clickhouse_errors import POD, snapshot
from mitigate_clickhouse_memory import verify_retained_lock
from update_images import Client, NS

UUID = re.compile(r'[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}\Z')
TABLE = re.compile(r'metric_log(?:_[0-9]+)?\Z')
PARTITION = re.compile(r'[0-9]{6}\Z')
MERGE_OVERRIDES = ('max_bytes_to_merge_at_max_space_in_pool',
                   'max_bytes_to_merge_at_min_space_in_pool',
                   'min_age_to_force_merge_seconds',
                   'min_age_to_force_merge_on_partition_only',
                   'enable_max_bytes_limit_for_min_age_to_force_merge')


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':'))
                          .encode('utf-8')).hexdigest()


def parts_subset(baseline, current):
    """Allow new active parts, but never lose or change a pinned source part."""
    def keyed(parts):
        result = {}
        for part in parts:
            key = (part['partition'], part['name'])
            if key in result or not {'rows', 'bytes_on_disk', 'hash_of_all_files'} <= part.keys():
                raise RuntimeError('duplicate or incomplete metric_log part identity')
            result[key] = part
        return result

    old, now = keyed(baseline), keyed(current)
    return all(now.get(key) == part for key, part in old.items())


def validate_baseline(baseline, target_uuid, partition, name, create_hash,
                      parts_hash, pause_start, pod_uid, cm_uid, lock_pins):
    parts = baseline['parts']
    if (baseline['name'] != name or baseline['uuid'] != target_uuid or
            baseline['partition'] != partition or
            baseline['effective_pause_start_utc'] != pause_start.isoformat() or
            baseline['pod_uid'] != pod_uid or baseline['configmap_uid'] != cm_uid or
            baseline['lock_uid'] != lock_pins[0] or
            baseline['lock_resource_version'] != lock_pins[1] or
            baseline['lock_operation'] != lock_pins[2] or
            baseline['merge_cap'] != 1 or baseline['merge_min_cap'] != 1 or
            baseline['create_sha256'] != create_hash or
            hashlib.sha256(baseline['create_query'].encode()).hexdigest() != create_hash or
            baseline['parts_sha256'] != parts_hash or digest(parts) != parts_hash or
            not parts or any(p['partition'] != partition for p in parts) or
            not parts_subset(parts, parts) or
            baseline['rows'] != sum(int(p['rows']) for p in parts) or
            baseline['bytes'] != sum(int(p['bytes_on_disk']) for p in parts)):
        raise RuntimeError('baseline inventory is stale or internally inconsistent')
    return parts, baseline['configmap_resource_version']


class ClickHouse:
    def query(self, sql):
        command = ['kubectl', '-n', NS, 'exec', 'pod/' + POD, '--',
                   'clickhouse-client', '--query', sql, '--format', 'JSON']
        try:
            result = subprocess.run(command, capture_output=True, timeout=8)
        except (OSError, subprocess.TimeoutExpired) as exc:
            raise RuntimeError('ClickHouse query transport unavailable') from exc
        if result.returncode:
            raise RuntimeError('ClickHouse query failed; retain updater lock')
        return json.loads(result.stdout)['data']

    def alter(self, table, reset=False):
        verb = ('RESET SETTING merge_max_block_size' if reset else
                'MODIFY SETTING merge_max_block_size=1024')
        command = ['kubectl', '-n', NS, 'exec', 'pod/' + POD, '--',
                   'clickhouse-client', '--query', 'ALTER TABLE system.' + table + ' ' + verb]
        try:
            result = subprocess.run(command, capture_output=True, timeout=8)
        except (OSError, subprocess.TimeoutExpired) as exc:
            raise RuntimeError('ClickHouse ALTER outcome uncertain; retain updater lock') from exc
        if result.returncode:
            raise RuntimeError('ClickHouse ALTER failed; retain updater lock')


def inspect(ch, target_uuid, partition):
    if not UUID.fullmatch(target_uuid) or not PARTITION.fullmatch(partition):
        raise ValueError('invalid target UUID or partition')
    tables = ch.query("SELECT name,toString(uuid) AS uuid_text,engine,engine_full,"
                      "create_table_query FROM system.tables WHERE database='system' "
                      "AND toString(uuid)='" + target_uuid + "' FORMAT JSON")
    if len(tables) != 1 or tables[0]['uuid_text'] != target_uuid or not TABLE.fullmatch(tables[0]['name']):
        raise RuntimeError('failing metric_log UUID is not uniquely present')
    table = tables[0]
    if table['engine'] != 'MergeTree' or not table['create_table_query'].startswith('CREATE TABLE system.'):
        raise RuntimeError('target is not the expected system MergeTree log')
    if any(setting in table['engine_full'] for setting in MERGE_OVERRIDES):
        raise RuntimeError('target has a table-level merge/forced-merge override; pause not proven')
    name = table['name']
    parts = ch.query("SELECT partition,name,rows,bytes_on_disk,hash_of_all_files FROM system.parts "
                     "WHERE database='system' AND table='" + name + "' AND partition='" +
                     partition + "' AND active ORDER BY name FORMAT JSON")
    merges = ch.query("SELECT count() AS active FROM system.merges WHERE database='system' "
                      "AND table='" + name + "' FORMAT JSON")
    settings = ch.query("SELECT name,value FROM system.merge_tree_settings WHERE name IN "
                        "('max_bytes_to_merge_at_max_space_in_pool',"
                        "'max_bytes_to_merge_at_min_space_in_pool',"
                        "'min_age_to_force_merge_seconds') FORMAT JSON")
    settings = {row['name']: int(row['value']) for row in settings}
    ddl = ch.query("SELECT count() AS active FROM system.processes WHERE query_kind IN "
                   "('Alter','Create','Drop','Rename','Exchange','Optimize') FORMAT JSON")
    if (len(merges) != 1 or len(ddl) != 1 or int(merges[0]['active']) != 0 or
            int(ddl[0]['active']) != 0 or not parts or
            set(settings) != {'max_bytes_to_merge_at_max_space_in_pool',
                              'max_bytes_to_merge_at_min_space_in_pool',
                              'min_age_to_force_merge_seconds'} or
            settings['min_age_to_force_merge_seconds'] != 0):
        raise RuntimeError('paused target has active merge/DDL, forced merge or no source parts')
    return {'name': name, 'uuid': target_uuid, 'engine_full': table['engine_full'],
            'create_query': table['create_table_query'],
            'create_sha256': hashlib.sha256(table['create_table_query'].encode()).hexdigest(),
            'parts': parts, 'parts_sha256': digest(parts), 'rows': sum(int(p['rows']) for p in parts),
            'bytes': sum(int(p['bytes_on_disk']) for p in parts),
            'merge_cap': settings['max_bytes_to_merge_at_max_space_in_pool'],
            'merge_min_cap': settings['max_bytes_to_merge_at_min_space_in_pool']}


def guarded_alter(kube, ch, pins, lock_pins, target_uuid, partition,
                  expected_name, expected_create_sha256, expected_parts_sha256,
                  baseline_parts, expected_cm_rv,
                  pause_start, apply=False, reset=False):
    lock = {'metadata': {'uid': lock_pins[0], 'resourceVersion': lock_pins[1]},
            'data': {'operation': lock_pins[2]}}

    def kube_check():
        elapsed = (datetime.now(timezone.utc) - pause_start).total_seconds()
        if elapsed < 0 or elapsed >= 180:
            raise RuntimeError('three-minute effective pause window expired; restore now')
        cm, sts, pod = snapshot(kube, pins, expected_data=paused_data(),
                                require_oom=False, require_not_ready=False)
        if (cm['metadata']['resourceVersion'] != expected_cm_rv or
                sts['status'].get('readyReplicas') != 1 or
                pod['status']['containerStatuses'][0].get('ready') is not True):
            raise RuntimeError('paused ClickHouse Pod is not Ready')
        verify_retained_lock(kube, lock)

    kube_check()
    before = inspect(ch, target_uuid, partition)
    if (before['name'] != expected_name or before['create_sha256'] != expected_create_sha256 or
            digest(baseline_parts) != expected_parts_sha256 or
            not parts_subset(baseline_parts, before['parts']) or before['merge_cap'] != 1 or
            before['merge_min_cap'] != 1):
        raise RuntimeError('paused metric_log UUID, DDL, parts or effective merge caps drifted')
    if reset and 'merge_max_block_size = 1024' not in before['engine_full']:
        raise RuntimeError('rollback target is not explicitly set to 1024')
    if not reset and 'merge_max_block_size' in before['engine_full']:
        raise RuntimeError('target already has a merge block override')
    print('DEV_METRIC_LOG_ALTER_PREVIEW name=' + before['name'] + ' uuid=' + target_uuid +
          ' create_sha256=' + before['create_sha256'] + ' parts_sha256=' +
          before['parts_sha256'] + ' rows=' + str(before['rows']) + ' bytes=' +
          str(before['bytes']) + ' merge_cap=' + str(before['merge_cap']) +
          ' merge_min_cap=' + str(before['merge_min_cap']))
    if not apply:
        return before
    kube_check()
    again = inspect(ch, target_uuid, partition)
    if (again['name'] != before['name'] or again['uuid'] != before['uuid'] or
            again['create_sha256'] != before['create_sha256'] or
            again['engine_full'] != before['engine_full'] or
            again['merge_cap'] != 1 or again['merge_min_cap'] != 1 or
            not parts_subset(before['parts'], again['parts'])):
        raise RuntimeError('metric_log target drifted immediately before ALTER')
    elapsed = (datetime.now(timezone.utc) - pause_start).total_seconds()
    if elapsed < 0 or elapsed >= 180:
        raise RuntimeError('three-minute effective pause window expired; restore now')
    ch.alter(again['name'], reset=reset)
    after = inspect(ch, target_uuid, partition)
    kube_check()
    if (after['name'] != again['name'] or after['uuid'] != again['uuid'] or
            not parts_subset(again['parts'], after['parts']) or
            after['merge_cap'] != 1 or
            after['merge_min_cap'] != 1 or
            ('merge_max_block_size = 1024' in after['engine_full']) == reset):
        raise RuntimeError('metric_log ALTER readback uncertain; retain updater lock')
    print('DEV_METRIC_LOG_ALTER_APPLIED name=' + after['name'] + ' uuid=' + target_uuid +
          ' parts_sha256=' + after['parts_sha256'] + ' lock_retained=true')
    return after


def inventory(kube, ch, pins, lock_pins, target_uuid, partition, pause_start):
    elapsed = (datetime.now(timezone.utc) - pause_start).total_seconds()
    if elapsed < 0 or elapsed >= 180:
        raise RuntimeError('three-minute effective pause window expired; restore now')
    cm, sts, pod = snapshot(kube, pins, expected_data=paused_data(),
                            require_oom=False, require_not_ready=False)
    if (sts['status'].get('readyReplicas') != 1 or
            pod['status']['containerStatuses'][0].get('ready') is not True):
        raise RuntimeError('paused ClickHouse Pod is not Ready')
    lock = {'metadata': {'uid': lock_pins[0], 'resourceVersion': lock_pins[1]},
            'data': {'operation': lock_pins[2]}}
    verify_retained_lock(kube, lock)
    result = inspect(ch, target_uuid, partition)
    verify_retained_lock(kube, lock)
    if result['merge_cap'] != 1 or result['merge_min_cap'] != 1:
        raise RuntimeError('global automatic merge max/min caps are not both 1')
    result.update(partition=partition, effective_pause_start_utc=pause_start.isoformat(),
                  pod_uid=pins[1], configmap_uid=pins[4],
                  configmap_resource_version=cm['metadata']['resourceVersion'],
                  lock_uid=lock_pins[0], lock_resource_version=lock_pins[1],
                  lock_operation=lock_pins[2])
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true')
    parser.add_argument('--reset-setting', action='store_true')
    parser.add_argument('--inventory-file', help='read-only full DDL and parts evidence JSON')
    parser.add_argument('--baseline-file', help='exact read-only inventory JSON for ALTER')
    for name in ('statefulset-uid', 'pod-uid', 'pvc-uid', 'pv-uid', 'configmap-uid',
                 'revision', 'statefulset-generation', 'lock-uid', 'lock-resource-version',
                 'lock-operation', 'table-uuid', 'partition'):
        parser.add_argument('--expected-' + name, required=True)
    for name in ('table-name', 'create-sha256', 'parts-sha256'):
        parser.add_argument('--expected-' + name)
    parser.add_argument('--effective-pause-start-utc', required=True,
                        help='UTC timestamp when the Ready pod first confirmed pause=1')
    args = parser.parse_args()
    if args.inventory_file and (args.apply or args.reset_setting):
        parser.error('inventory is read-only and cannot combine with ALTER')
    if args.inventory_file and args.baseline_file:
        parser.error('inventory cannot use an ALTER baseline file')
    if not args.inventory_file and not all((args.expected_table_name,
                                            args.expected_create_sha256,
                                            args.expected_parts_sha256, args.baseline_file)):
        parser.error('ALTER preview/apply needs inventory file, name, DDL and parts hashes')
    pins = (args.expected_statefulset_uid, args.expected_pod_uid, args.expected_pvc_uid,
            args.expected_pv_uid, args.expected_configmap_uid, args.expected_revision,
            int(args.expected_statefulset_generation))
    lock = (args.expected_lock_uid, args.expected_lock_resource_version,
            args.expected_lock_operation)
    try:
        pause_start = datetime.fromisoformat(args.effective_pause_start_utc.replace('Z', '+00:00'))
        if pause_start.tzinfo is None:
            raise ValueError('effective pause start must include UTC timezone')
        if args.inventory_file:
            result = inventory(Client(), ClickHouse(), pins, lock, args.expected_table_uuid,
                               args.expected_partition, pause_start)
            Path(args.inventory_file).write_text(json.dumps(result, sort_keys=True, indent=2),
                                                 encoding='utf-8')
            print('DEV_METRIC_LOG_INVENTORY name=' + result['name'] + ' uuid=' + result['uuid'] +
                  ' create_sha256=' + result['create_sha256'] + ' parts_sha256=' +
                  result['parts_sha256'] + ' rows=' + str(result['rows']) +
                  ' bytes=' + str(result['bytes']) + ' file=' + args.inventory_file)
        else:
            baseline = json.loads(Path(args.baseline_file).read_text(encoding='utf-8'))
            baseline_parts, cm_rv = validate_baseline(
                baseline, args.expected_table_uuid, args.expected_partition,
                args.expected_table_name, args.expected_create_sha256,
                args.expected_parts_sha256, pause_start, args.expected_pod_uid,
                args.expected_configmap_uid, lock)
            guarded_alter(Client(), ClickHouse(), pins, lock, args.expected_table_uuid,
                          args.expected_partition, args.expected_table_name,
                          args.expected_create_sha256, args.expected_parts_sha256,
                          baseline_parts, cm_rv,
                          pause_start, apply=args.apply, reset=args.reset_setting)
    except (ValueError, KeyError, TypeError, RuntimeError) as exc:
        print('DEV metric_log ALTER aborted; inspect retained lock: ' + str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
