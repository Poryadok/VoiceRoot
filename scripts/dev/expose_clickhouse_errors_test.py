import copy
import unittest
import xml.etree.ElementTree as ET

import expose_clickhouse_errors as diagnostic
import mitigate_clickhouse_memory as policy
import mitigate_clickhouse_memory_test as policy_test
import update_images


PINS = ('store-uid', 'oom-pod', 'pvc-uid', 'pv-uid', 'config-uid', 'policy-revision', 4)


class Fake(policy_test.Fake):
    def __init__(self):
        super().__init__()
        self.sts['metadata']['generation'] = 4
        self.sts['spec']['podManagementPolicy'] = 'OrderedReady'
        policy.add_clickhouse_memory_mount(self.sts['spec']['template']['spec'])
        self.sts['status'] = {'observedGeneration': 4, 'currentRevision': 'policy-revision',
                              'updateRevision': 'policy-revision', 'replicas': 1,
                              'updatedReplicas': 1, 'readyReplicas': 0}
        self.pod['metadata'].update(uid='oom-pod', resourceVersion='18')
        self.pod['metadata']['labels']['controller-revision-hash'] = 'policy-revision'
        self.pod['spec']['containers'] = copy.deepcopy(self.sts['spec']['template']['spec']['containers'])
        self.pod['spec']['volumes'].extend(copy.deepcopy(self.sts['spec']['template']['spec']['volumes']))
        self.config = policy.clickhouse_memory_config()
        self.config['metadata'].update(uid='config-uid', resourceVersion='19')
        self.lock = None
        self.pv = {'metadata': {'uid': 'pv-uid'}, 'status': {'phase': 'Bound'},
                   'spec': {'claimRef': {'uid': 'pvc-uid', 'namespace': update_images.NS},
                            'persistentVolumeReclaimPolicy': 'Retain'}}
        self.delete_failed = False

    def call(self, *args, body=None):
        if args[:2] == ('get', 'resourcequota'):
            self.calls.append((args, body))
            return {'status': {'used': {'limits.memory': self.used},
                               'hard': {'limits.memory': '20Gi'}}}
        if args[:2] == ('get', 'pv'):
            self.calls.append((args, body))
            return copy.deepcopy(self.pv)
        if args[0] == 'create' and body['metadata']['name'] == update_images.LOCK:
            self.calls.append((args, body))
            if self.lock is not None:
                raise RuntimeError('lock exists')
            self.lock = copy.deepcopy(body)
            self.lock['metadata'].update(uid='new-lock', resourceVersion='20')
            return copy.deepcopy(self.lock)
        if args[:2] == ('patch', 'configmap'):
            self.calls.append((args, body))
            result = copy.deepcopy(self.config)
            result['data'] = diagnostic.console_data()
            if '--dry-run=server' not in args:
                self.config = copy.deepcopy(result)
                self.config['metadata']['resourceVersion'] = '21'
            return result
        if args[0] == 'delete' and '/pods/' in args[1]:
            self.calls.append((args, body))
            if self.delete_failed and 'dryRun=All' not in args[1]:
                raise RuntimeError('delete outcome uncertain')
            result = copy.deepcopy(self.pod)
            if 'dryRun=All' not in args[1]:
                self.pod['metadata']['uid'] = 'recreated-pod'
            return result
        return super().call(*args, body=body)


class ConsoleTests(unittest.TestCase):
    def test_fragment_only_adds_console_error_output_to_existing_ratio(self):
        root = ET.fromstring(diagnostic.console_data()[diagnostic.FILE])
        self.assertEqual(root.tag, 'clickhouse')
        self.assertEqual(root.findtext('max_server_memory_usage_to_ram_ratio'), '0.8')
        self.assertEqual(root.findtext('logger/console'), '1')
        self.assertEqual(root.findtext('logger/console_log_level'), 'error')
        self.assertEqual(len(root), 2)

    def test_preview_only_server_dry_runs_exact_config_and_pod(self):
        client = Fake()
        diagnostic.expose(client, PINS)
        self.assertIsNone(client.lock)
        self.assertEqual(client.config['data'], policy.clickhouse_memory_config()['data'])
        self.assertEqual(client.pod['metadata']['uid'], 'oom-pod')
        patch = next(body for args, body in client.calls if args[:2] == ('patch', 'configmap'))
        self.assertEqual([p['op'] for p in patch], ['test', 'test', 'test', 'replace'])
        self.assertEqual(patch[0]['value'], 'config-uid')
        self.assertEqual(patch[1]['value'], '19')
        self.assertEqual(patch[2]['value'], policy.clickhouse_memory_config()['data'])
        deletes = [(args, body) for args, body in client.calls if args[0] == 'delete']
        self.assertEqual(len(deletes), 1)
        self.assertIn('dryRun=All', deletes[0][0][1])
        self.assertEqual(deletes[0][1]['preconditions'], {'uid': 'oom-pod', 'resourceVersion': '18'})

    def test_apply_patches_only_config_and_gracefully_deletes_exact_pod_with_lock_retained(self):
        client = Fake()
        diagnostic.expose(client, PINS, apply=True)
        self.assertEqual(client.config['data'], diagnostic.console_data())
        self.assertEqual(client.pod['metadata']['uid'], 'recreated-pod')
        self.assertEqual(client.sts['metadata']['resourceVersion'], '14')
        self.assertEqual(client.lock['metadata']['uid'], 'new-lock')
        deletes = [(args, body) for args, body in client.calls if args[0] == 'delete']
        self.assertEqual(len(deletes), 3)
        self.assertEqual(len([1 for args, _ in deletes if 'dryRun=All' not in args[1]]), 1)
        self.assertEqual(deletes[-1][1]['preconditions'], {'uid': 'oom-pod', 'resourceVersion': '18'})

    def test_foreign_target_fails_before_lock_or_mutation(self):
        for drift in ('sts', 'pod', 'owner', 'ready', 'config', 'pvc', 'pv', 'quota'):
            client = Fake()
            if drift == 'sts':
                client.sts['metadata']['uid'] = 'foreign'
            elif drift == 'pod':
                client.pod['metadata']['uid'] = 'foreign'
            elif drift == 'owner':
                client.pod['metadata']['ownerReferences'][0]['uid'] = 'foreign'
            elif drift == 'ready':
                client.pod['status']['containerStatuses'][0]['ready'] = True
            elif drift == 'config':
                client.config['data'] = {'foreign': 'setting'}
            elif drift == 'pvc':
                client.pvc['metadata']['uid'] = 'foreign'
            elif drift == 'pv':
                client.pv['metadata']['uid'] = 'foreign'
            else:
                client.used = '21Gi'
            with self.subTest(drift=drift), self.assertRaises((RuntimeError, ValueError)):
                diagnostic.expose(client, PINS, apply=True)
            self.assertIsNone(client.lock)
            self.assertFalse(any(args[0] in ('create', 'patch', 'delete') for args, _ in client.calls
                                 if '--dry-run=server' not in args and
                                 not (args[0] == 'delete' and 'dryRun=All' in args[1])))

    def test_drift_after_lock_or_uncertain_delete_retains_lock(self):
        class Drift(Fake):
            def call(self, *args, body=None):
                result = super().call(*args, body=body)
                if args[0] == 'create' and body['metadata']['name'] == update_images.LOCK:
                    self.config['metadata']['resourceVersion'] = 'foreign'
                return result
        client = Drift()
        with self.assertRaises(RuntimeError):
            diagnostic.expose(client, PINS, apply=True)
        self.assertIsNotNone(client.lock)
        self.assertEqual(client.config['data'], policy.clickhouse_memory_config()['data'])
        self.assertFalse(any(args[0] == 'delete' and 'dryRun=All' not in args[1]
                             for args, _ in client.calls))
        client = Fake()
        client.delete_failed = True
        with self.assertRaises(RuntimeError):
            diagnostic.expose(client, PINS, apply=True)
        self.assertIsNotNone(client.lock)
        self.assertEqual(client.config['data'], diagnostic.console_data())
        self.assertEqual(client.pod['metadata']['uid'], 'oom-pod')


if __name__ == '__main__':
    unittest.main()
