import unittest
import xml.etree.ElementTree as ET

import expose_clickhouse_errors_test as console_test
import persist_clickhouse_metric_log as persist
import update_images


def held_lock():
    return {'metadata': {'uid': 'held-lock', 'resourceVersion': '20',
                         'name': update_images.LOCK, 'namespace': update_images.NS,
                         'labels': dict(update_images.LABELS)},
            'data': {'operation': 'held-operation'}}


LOCK = ('held-lock', '20', 'held-operation')


class PersistentMetricLogTests(unittest.TestCase):
    def test_xml_changes_only_metric_log_settings(self):
        old = ET.fromstring(persist.console_data()[persist.FILE])
        new = ET.fromstring(persist.persistent_data()[persist.FILE])
        self.assertEqual(new.findtext('max_server_memory_usage_to_ram_ratio'), '0.8')
        self.assertEqual(new.findtext('logger/console'), '1')
        self.assertEqual(new.findtext('logger/console_log_level'), 'error')
        self.assertEqual(new.findtext('metric_log/settings'), 'merge_max_block_size=1024')
        self.assertIsNone(new.find('merge_tree'))
        self.assertEqual([child.tag for child in new], [child.tag for child in old] + ['metric_log'])

    def test_enable_and_rollback_use_exact_configmap_cas_and_keep_lock(self):
        client = console_test.Fake()
        client.config['data'] = persist.console_data()
        client.lock = held_lock()
        persist.transition(client, console_test.PINS, LOCK, enable=True)
        self.assertEqual(client.config['data'], persist.console_data())
        persist.transition(client, console_test.PINS, LOCK, enable=True, apply=True)
        self.assertEqual(client.config['data'], persist.persistent_data())
        persist.transition(client, console_test.PINS, LOCK, enable=False, apply=True)
        self.assertEqual(client.config['data'], persist.console_data())
        self.assertIsNotNone(client.lock)
        mutations = [args for args, _ in client.calls if args[0] in ('patch', 'delete')]
        self.assertTrue(all(args[:3] == ('patch', 'configmap', persist.CONFIG)
                            for args in mutations))

    def test_enable_refuses_wrong_config_or_stale_lock_without_mutation(self):
        client = console_test.Fake()
        client.config['data'] = persist.console_data()
        client.lock = held_lock()
        client.lock['metadata']['resourceVersion'] = '21'
        with self.assertRaises(RuntimeError):
            persist.transition(client, console_test.PINS, LOCK, enable=True, apply=True)
        self.assertEqual(client.config['data'], persist.console_data())
        client.lock = held_lock()
        client.config['data'] = {'changed.xml': 'drift'}
        with self.assertRaises(RuntimeError):
            persist.transition(client, console_test.PINS, LOCK, enable=True, apply=True)
        self.assertEqual(client.config['data'], {'changed.xml': 'drift'})

    def test_stale_configmap_rv_fails_closed_and_retains_lock(self):
        client = console_test.Fake()
        client.config['data'] = persist.console_data()
        client.lock = held_lock()
        client.stale_config_on_apply = True
        with self.assertRaises(RuntimeError):
            persist.transition(client, console_test.PINS, LOCK, enable=True, apply=True)
        self.assertEqual(client.config['data'], persist.console_data())
        self.assertIsNotNone(client.lock)

    def test_replace_enabled_requires_ready_and_exact_uid(self):
        client = console_test.Fake()
        client.config['data'] = persist.persistent_data()
        client.lock = held_lock()
        with self.assertRaises(RuntimeError):
            persist.replace(client, console_test.PINS, LOCK, enabled=True, apply=True)
        client.sts['status']['readyReplicas'] = 1
        client.pod['status']['containerStatuses'][0]['ready'] = True
        client.pod['status']['containerStatuses'][0]['lastState'] = {}
        persist.replace(client, console_test.PINS, LOCK, enabled=True, apply=True)
        self.assertEqual(client.pod['metadata']['uid'], 'recreated-pod')
        self.assertIsNotNone(client.lock)
        self.assertTrue(any(args[0] == 'delete' and '/pods/' in args[1] and
                            body['preconditions'] == {'uid': 'oom-pod', 'resourceVersion': '18'}
                            for args, body in client.calls))

    def test_replace_rollback_can_replace_failed_pod_without_config_change(self):
        client = console_test.Fake()
        client.config['data'] = persist.console_data()
        client.lock = held_lock()
        persist.replace(client, console_test.PINS, LOCK, enabled=False, apply=True)
        self.assertEqual(client.config['data'], persist.console_data())
        self.assertEqual(client.pod['metadata']['uid'], 'recreated-pod')
        self.assertIsNotNone(client.lock)


if __name__ == '__main__':
    unittest.main()
