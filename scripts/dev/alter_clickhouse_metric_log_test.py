import copy
from datetime import datetime, timedelta, timezone
import unittest

import alter_clickhouse_metric_log as ddl
import diagnose_clickhouse_merges_test as pause_test
import expose_clickhouse_errors_test as console_test


UUID = '8b7111c7-f407-4312-8a7f-c17ad7090ceb'
PARTS = [{'partition': '202610', 'name': '202610_1_1_0', 'rows': 100, 'bytes_on_disk': 200,
          'hash_of_all_files': 'part-hash'}]


class FakeClickHouse:
    def __init__(self):
        self.name = 'metric_log'
        self.uuid = UUID
        self.engine_full = 'MergeTree PARTITION BY toYYYYMM(event_date) ORDER BY (event_date, event_time) SETTINGS index_granularity = 8192'
        self.create_query = 'CREATE TABLE system.metric_log (event_date Date) ENGINE = ' + self.engine_full
        self.parts = copy.deepcopy(PARTS)
        self.active_merges = 0
        self.active_ddl = 0
        self.cap = 1
        self.min_cap = 1
        self.force_merge_age = 0
        self.alters = []

    def query(self, sql):
        if 'FROM system.tables' in sql:
            return [{'name': self.name, 'uuid_text': self.uuid, 'engine': 'MergeTree',
                     'engine_full': self.engine_full, 'create_table_query': self.create_query}]
        if 'FROM system.parts' in sql:
            return copy.deepcopy(self.parts)
        if 'FROM system.merges' in sql:
            return [{'active': self.active_merges}]
        if 'FROM system.merge_tree_settings' in sql:
            return [{'name': 'max_bytes_to_merge_at_max_space_in_pool', 'value': str(self.cap)},
                    {'name': 'max_bytes_to_merge_at_min_space_in_pool',
                     'value': str(self.min_cap)},
                    {'name': 'min_age_to_force_merge_seconds',
                     'value': str(self.force_merge_age)}]
        if 'FROM system.processes' in sql:
            return [{'active': self.active_ddl}]
        raise AssertionError(sql)

    def alter(self, table, reset=False):
        self.alters.append((table, reset))
        if reset:
            self.engine_full = self.engine_full.replace(', merge_max_block_size = 1024', '')
        else:
            self.engine_full += ', merge_max_block_size = 1024'
        self.create_query = 'CREATE TABLE system.' + self.name + ' (event_date Date) ENGINE = ' + self.engine_full


def ready_kube():
    client = console_test.Fake()
    client.config['data'] = ddl.paused_data()
    client.lock = pause_test.held_lock()
    client.pod['status']['containerStatuses'][0]['ready'] = True
    client.sts['status']['readyReplicas'] = 1
    return client


def run(client, ch, apply=False, reset=False, **overrides):
    before = ddl.inspect(ch, UUID, '202610')
    pins = console_test.PINS
    kwargs = dict(target_uuid=UUID, partition='202610', expected_name=before['name'],
                  expected_create_sha256=before['create_sha256'],
                  expected_parts_sha256=before['parts_sha256'],
                  baseline_parts=before['parts'],
                  expected_cm_rv='19',
                  pause_start=datetime.now(timezone.utc), apply=apply, reset=reset)
    kwargs.update(overrides)
    return ddl.guarded_alter(client, ch, pins, ('held-lock', '20', 'held-operation'), **kwargs)


class MetricLogAlterTests(unittest.TestCase):
    def test_inventory_captures_full_ddl_and_part_hashes_without_mutation(self):
        client, ch = ready_kube(), FakeClickHouse()
        result = ddl.inventory(client, ch, console_test.PINS,
                               ('held-lock', '20', 'held-operation'), UUID, '202610',
                               datetime.now(timezone.utc))
        self.assertEqual(result['create_query'], ch.create_query)
        self.assertEqual(result['parts'], PARTS)
        self.assertEqual(result['configmap_resource_version'], '19')
        self.assertEqual(result['pod_uid'], 'oom-pod')
        self.assertEqual(ch.alters, [])

    def test_baseline_file_must_match_exact_pause_window_and_self_hashes(self):
        client, ch = ready_kube(), FakeClickHouse()
        started = datetime.now(timezone.utc)
        baseline = ddl.inventory(client, ch, console_test.PINS,
                                 ('held-lock', '20', 'held-operation'), UUID, '202610', started)

        def validate(candidate, at=started):
            return ddl.validate_baseline(candidate, UUID, '202610', 'metric_log',
                                         baseline['create_sha256'], baseline['parts_sha256'],
                                         at, 'oom-pod', 'config-uid',
                                         ('held-lock', '20', 'held-operation'))

        self.assertEqual(validate(baseline), (PARTS, '19'))
        with self.assertRaises(RuntimeError):
            validate(baseline, started + timedelta(seconds=1))
        for key, value in (('pod_uid', 'stale-pod'), ('partition', '202609'),
                           ('rows', 101), ('bytes', 201),
                           ('configmap_uid', 'other-config'),
                           ('lock_operation', 'other-operation'),
                           ('create_query', 'changed DDL')):
            changed = copy.deepcopy(baseline)
            changed[key] = value
            with self.assertRaises(RuntimeError):
                validate(changed)
        changed = copy.deepcopy(baseline)
        changed['parts'][0]['hash_of_all_files'] = 'altered'
        with self.assertRaises(RuntimeError):
            validate(changed)

    def test_inventory_refuses_unpaused_or_expired_window(self):
        client, ch = ready_kube(), FakeClickHouse()
        ch.cap = 150 * 1024**3
        with self.assertRaises(RuntimeError):
            ddl.inventory(client, ch, console_test.PINS, ('held-lock', '20', 'held-operation'),
                          UUID, '202610', datetime.now(timezone.utc))
        ch.cap = 1
        ch.min_cap = 1048576
        with self.assertRaises(RuntimeError):
            ddl.inventory(client, ch, console_test.PINS, ('held-lock', '20', 'held-operation'),
                          UUID, '202610', datetime.now(timezone.utc))
        ch.min_cap = 1
        with self.assertRaises(RuntimeError):
            ddl.inventory(client, ch, console_test.PINS, ('held-lock', '20', 'held-operation'),
                          UUID, '202610', datetime.now(timezone.utc) - timedelta(seconds=181))

    def test_preview_is_read_only(self):
        client, ch = ready_kube(), FakeClickHouse()
        before = run(client, ch)
        self.assertEqual(before['parts_sha256'], ddl.digest(PARTS))
        self.assertEqual(ch.alters, [])
        self.assertIsNotNone(client.lock)

    def test_apply_preserves_uuid_parts_and_lock(self):
        client, ch = ready_kube(), FakeClickHouse()
        after = run(client, ch, apply=True)
        self.assertEqual(ch.alters, [('metric_log', False)])
        self.assertEqual(after['uuid'], UUID)
        self.assertEqual(after['parts_sha256'], ddl.digest(PARTS))
        self.assertIn('merge_max_block_size = 1024', after['engine_full'])
        self.assertIsNotNone(client.lock)

    def test_wrong_target_uuid_or_part_hash_refuses_alter(self):
        client, ch = ready_kube(), FakeClickHouse()
        with self.assertRaises(RuntimeError):
            run(client, ch, apply=True, target_uuid='11111111-1111-1111-1111-111111111111')
        with self.assertRaises(RuntimeError):
            run(client, ch, apply=True, expected_parts_sha256='0' * 64)
        self.assertEqual(ch.alters, [])

    def test_append_only_parts_are_allowed_before_and_after_alter(self):
        client, ch = ready_kube(), FakeClickHouse()
        baseline = ddl.inspect(ch, UUID, '202610')
        ch.parts.append({'partition': '202610', 'name': '202610_2_2_0',
                         'rows': 50, 'bytes_on_disk': 100, 'hash_of_all_files': 'new-hash'})
        original = ch.alter

        def append_after(table, reset=False):
            original(table, reset)
            ch.parts.append({'partition': '202610', 'name': '202610_3_3_0',
                             'rows': 25, 'bytes_on_disk': 75,
                             'hash_of_all_files': 'post-hash'})

        ch.alter = append_after
        result = run(client, ch, apply=True, baseline_parts=baseline['parts'],
                     expected_parts_sha256=baseline['parts_sha256'])
        self.assertEqual(result['rows'], 175)
        self.assertEqual(ch.alters, [('metric_log', False)])

    def test_missing_or_changed_baseline_part_refuses_alter(self):
        for mutate in (lambda parts: parts.clear(),
                       lambda parts: parts[0].update(rows=101),
                       lambda parts: parts[0].update(bytes_on_disk=201),
                       lambda parts: parts[0].update(hash_of_all_files='altered'),
                       lambda parts: parts[0].update(partition='202609')):
            client, ch = ready_kube(), FakeClickHouse()
            baseline = ddl.inspect(ch, UUID, '202610')
            mutate(ch.parts)
            with self.assertRaises(RuntimeError):
                run(client, ch, apply=True, baseline_parts=baseline['parts'],
                    expected_parts_sha256=baseline['parts_sha256'])
            self.assertEqual(ch.alters, [])

    def test_same_count_different_checksum_refuses_alter(self):
        client, ch = ready_kube(), FakeClickHouse()
        baseline = ddl.inspect(ch, UUID, '202610')
        ch.parts = [{'partition': '202610', 'name': '202610_2_2_0',
                     'rows': 100, 'bytes_on_disk': 200,
                     'hash_of_all_files': 'different'}]
        with self.assertRaises(RuntimeError):
            run(client, ch, apply=True, baseline_parts=baseline['parts'],
                expected_parts_sha256=baseline['parts_sha256'])
        self.assertEqual(ch.alters, [])

    def test_active_merge_or_unpaused_global_cap_refuses_alter(self):
        client, ch = ready_kube(), FakeClickHouse()
        ch.active_merges = 1
        with self.assertRaises(RuntimeError):
            run(client, ch, apply=True)
        ch.active_merges = 0
        ch.cap = 150 * 1024**3
        with self.assertRaises(RuntimeError):
            run(client, ch, apply=True)
        ch.cap = 1
        ch.min_cap = 1048576
        with self.assertRaises(RuntimeError):
            run(client, ch, apply=True)
        self.assertEqual(ch.alters, [])

    def test_table_level_cap_or_forced_merge_override_refuses_global_pause(self):
        client, ch = ready_kube(), FakeClickHouse()
        for setting in ddl.MERGE_OVERRIDES:
            ch.engine_full += ', ' + setting + ' = 150000000000'
            with self.assertRaises(RuntimeError):
                run(client, ch, apply=True)
            ch.engine_full = ch.engine_full.rsplit(', ', 1)[0]
        ch.force_merge_age = 1
        with self.assertRaises(RuntimeError):
            run(client, ch, apply=True)
        self.assertEqual(ch.alters, [])

    def test_concurrent_ddl_refuses_alter(self):
        client, ch = ready_kube(), FakeClickHouse()
        ch.active_ddl = 1
        with self.assertRaises(RuntimeError):
            run(client, ch, apply=True)
        self.assertEqual(ch.alters, [])

    def test_late_merge_ddl_or_cap_drift_refuses_alter(self):
        for marker, field, value in (('FROM system.merges', 'active_merges', 1),
                                     ('FROM system.processes', 'active_ddl', 1),
                                     ('FROM system.merge_tree_settings', 'min_cap', 1048576)):
            client, ch = ready_kube(), FakeClickHouse()
            original = ch.query
            visits = 0

            def late_drift(sql):
                nonlocal visits
                if marker in sql:
                    visits += 1
                    if visits == 3:
                        setattr(ch, field, value)
                return original(sql)

            ch.query = late_drift
            with self.assertRaises(RuntimeError):
                run(client, ch, apply=True)
            self.assertEqual(ch.alters, [])

    def test_not_ready_or_changed_lock_refuses_alter(self):
        client, ch = ready_kube(), FakeClickHouse()
        client.pod['status']['containerStatuses'][0]['ready'] = False
        with self.assertRaises(RuntimeError):
            run(client, ch, apply=True)
        client.pod['status']['containerStatuses'][0]['ready'] = True
        client.lock['metadata']['resourceVersion'] = '21'
        with self.assertRaises(RuntimeError):
            run(client, ch, apply=True)
        client.lock['metadata']['resourceVersion'] = '20'
        client.config['metadata']['resourceVersion'] = '21'
        with self.assertRaises(RuntimeError):
            run(client, ch, apply=True)
        self.assertEqual(ch.alters, [])

    def test_expired_pause_window_refuses_alter(self):
        client, ch = ready_kube(), FakeClickHouse()
        with self.assertRaises(RuntimeError):
            run(client, ch, apply=True,
                pause_start=datetime.now(timezone.utc) - timedelta(seconds=181))
        self.assertEqual(ch.alters, [])

    def test_exact_reset_restores_ddl_without_part_or_uuid_loss(self):
        client, ch = ready_kube(), FakeClickHouse()
        run(client, ch, apply=True)
        after = run(client, ch, apply=True, reset=True)
        self.assertEqual(ch.alters, [('metric_log', False), ('metric_log', True)])
        self.assertNotIn('merge_max_block_size', after['engine_full'])
        self.assertEqual(after['parts_sha256'], ddl.digest(PARTS))

    def test_post_alter_part_drift_retains_lock_and_reports_uncertain(self):
        client, ch = ready_kube(), FakeClickHouse()
        original = ch.alter

        def drift(table, reset=False):
            original(table, reset)
            ch.parts[0]['rows'] += 1

        ch.alter = drift
        with self.assertRaises(RuntimeError):
            run(client, ch, apply=True)
        self.assertEqual(ch.alters, [('metric_log', False)])
        self.assertIsNotNone(client.lock)


if __name__ == '__main__':
    unittest.main()
