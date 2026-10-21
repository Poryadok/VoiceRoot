import unittest
import xml.etree.ElementTree as ET

import diagnose_clickhouse_merges as merges
import expose_clickhouse_errors_test as console_test
import update_images


class MergePauseTests(unittest.TestCase):
    def test_pause_fragment_preserves_ratio_and_error_console(self):
        root = ET.fromstring(merges.paused_data()[merges.FILE])
        self.assertEqual(root.findtext('max_server_memory_usage_to_ram_ratio'), '0.8')
        self.assertEqual(root.findtext('logger/console'), '1')
        self.assertEqual(root.findtext('logger/console_log_level'), 'error')
        self.assertEqual(root.findtext('merge_tree/max_bytes_to_merge_at_max_space_in_pool'), '1')
        self.assertEqual(root.findtext('merge_tree/max_bytes_to_merge_at_min_space_in_pool'), '1')
        self.assertEqual(len(root), 3)

    def test_restore_fragment_removes_both_global_merge_caps(self):
        root = ET.fromstring(merges.console_data()[merges.FILE])
        self.assertEqual(root.findtext('max_server_memory_usage_to_ram_ratio'), '0.8')
        self.assertEqual(root.findtext('logger/console'), '1')
        self.assertIsNone(root.find('merge_tree'))
        self.assertIsNone(root.find('merge_tree/max_bytes_to_merge_at_max_space_in_pool'))
        self.assertIsNone(root.find('merge_tree/max_bytes_to_merge_at_min_space_in_pool'))

    def test_pause_and_restore_are_exact_opposite_configmap_cas(self):
        client = console_test.Fake()
        client.config['data'] = merges.console_data()
        client.lock = {'metadata': {'uid': 'held-lock', 'resourceVersion': '20',
                                    'name': update_images.LOCK, 'namespace': update_images.NS,
                                    'labels': dict(update_images.LABELS)},
                       'data': {'operation': 'held-operation'}}
        pins = console_test.PINS
        lock = ('held-lock', '20', 'held-operation')
        merges.transition(client, pins, lock, pause=True, apply=False)
        self.assertEqual(client.config['data'], merges.console_data())
        merges.transition(client, pins, lock, pause=True, apply=True)
        self.assertEqual(client.config['data'], merges.paused_data())
        client.sts['status']['readyReplicas'] = 1
        merges.transition(client, pins, lock, pause=False, apply=True)
        self.assertEqual(client.config['data'], merges.console_data())

    def test_stale_configmap_rv_retains_lock_and_does_not_patch(self):
        client = console_test.Fake()
        client.config['data'] = merges.console_data()
        client.lock = {'metadata': {'uid': 'held-lock', 'resourceVersion': '20',
                                    'name': update_images.LOCK, 'namespace': update_images.NS,
                                    'labels': dict(update_images.LABELS)},
                       'data': {'operation': 'held-operation'}}
        client.stale_config_on_apply = True
        with self.assertRaises(RuntimeError):
            merges.transition(client, console_test.PINS,
                              ('held-lock', '20', 'held-operation'), pause=True, apply=True)
        self.assertEqual(client.config['data'], merges.console_data())
        self.assertIsNotNone(client.lock)

    def test_replace_paused_pod_requires_exact_oom_uid_and_retained_lock(self):
        client = console_test.Fake()
        client.config['data'] = merges.paused_data()
        client.lock = held_lock()
        pins = console_test.PINS
        lock = ('held-lock', '20', 'held-operation')
        merges.replace_after_config(client, pins, lock, expected_data=merges.paused_data())
        self.assertEqual(client.pod['metadata']['uid'], 'oom-pod')
        merges.replace_after_config(client, pins, lock, apply=True,
                                    expected_data=merges.paused_data())
        self.assertEqual(client.pod['metadata']['uid'], 'recreated-pod')
        self.assertEqual(client.config['data'], merges.paused_data())
        self.assertEqual(client.lock['metadata']['uid'], 'held-lock')
        self.assertTrue(any(args[0] == 'delete' and '/pods/' in args[1] and
                            body['preconditions'] == {'uid': 'oom-pod', 'resourceVersion': '18'}
                            for args, body in client.calls))

    def test_replace_restored_pod_allows_ready_but_requires_exact_uid(self):
        client = console_test.Fake()
        client.config['data'] = merges.console_data()
        client.lock = held_lock()
        client.pod['status']['containerStatuses'][0]['ready'] = True
        client.pod['status']['containerStatuses'][0]['lastState'] = {}
        client.sts['status']['readyReplicas'] = 1
        lock = ('held-lock', '20', 'held-operation')
        merges.replace_after_config(client, console_test.PINS, lock, apply=True,
                                    expected_data=merges.console_data(), require_oom=False)
        self.assertEqual(client.pod['metadata']['uid'], 'recreated-pod')
        self.assertIsNotNone(client.lock)

    def test_replace_paused_refuses_wrong_config_or_changed_pod_uid(self):
        client = console_test.Fake()
        client.config['data'] = merges.console_data()
        client.lock = held_lock()
        lock = ('held-lock', '20', 'held-operation')
        with self.assertRaises(RuntimeError):
            merges.replace_after_config(client, console_test.PINS, lock,
                                        expected_data=merges.paused_data())
        client.config['data'] = merges.paused_data()
        client.pod['metadata']['uid'] = 'unexpected'
        with self.assertRaises(RuntimeError):
            merges.replace_after_config(client, console_test.PINS, lock,
                                        expected_data=merges.paused_data())

    def test_restore_refuses_wrong_lock_and_preserves_paused_config(self):
        client = console_test.Fake()
        client.config['data'] = merges.paused_data()
        client.lock = held_lock()
        client.lock['metadata']['resourceVersion'] = '21'
        with self.assertRaises(RuntimeError):
            merges.transition(client, console_test.PINS,
                              ('held-lock', '20', 'held-operation'), pause=False, apply=True)
        self.assertEqual(client.config['data'], merges.paused_data())

    def test_pause_and_restore_do_not_patch_statefulset_or_release_lock(self):
        client = console_test.Fake()
        client.config['data'] = merges.console_data()
        client.lock = held_lock()
        lock = ('held-lock', '20', 'held-operation')
        merges.transition(client, console_test.PINS, lock, pause=True, apply=True)
        merges.transition(client, console_test.PINS, lock, pause=False, apply=True)
        mutations = [(args, body) for args, body in client.calls if args[0] in ('patch', 'delete')]
        self.assertEqual(len(mutations), 4)
        self.assertTrue(all(args[:2] == ('patch', 'configmap') and
                            args[2] == merges.CONFIG for args, _ in mutations))
        self.assertIsNotNone(client.lock)


def held_lock():
    return {'metadata': {'uid': 'held-lock', 'resourceVersion': '20',
                         'name': update_images.LOCK, 'namespace': update_images.NS,
                         'labels': dict(update_images.LABELS)},
            'data': {'operation': 'held-operation'}}


if __name__ == '__main__':
    unittest.main()
