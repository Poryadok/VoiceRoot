import copy
import unittest

import mitigate_clickhouse_memory as policy
import repair_clickhouse_memory_test as repair_test
import update_images


PINS = ('lock-uid', 'original-operation', 'store-uid', 'old-pod', 'pvc-uid')


class Fake(repair_test.Fake):
    def __init__(self):
        super().__init__()
        self.sts['metadata']['generation'] = 3
        self.sts['spec']['template']['spec']['containers'][0]['image'] = policy.IMAGE
        self.sts['spec']['template']['spec']['containers'][0]['resources']['limits']['memory'] = '4Gi'
        self.sts['status'] = {'observedGeneration': 3, 'currentRevision': 'old-revision',
                              'updateRevision': 'old-revision', 'replicas': 1,
                              'updatedReplicas': 1, 'readyReplicas': 0}
        self.pod = {'metadata': {'namespace': update_images.NS, 'name': policy.POD,
                                 'uid': 'old-pod', 'labels': {'controller-revision-hash': 'old-revision'},
                                 'ownerReferences': [{'kind': 'StatefulSet', 'name': policy.NAME,
                                                      'uid': 'store-uid', 'controller': True}]},
                    'spec': {'containers': [copy.deepcopy(self.sts['spec']['template']['spec']['containers'][0])],
                             'volumes': [{'name': 'chdata', 'persistentVolumeClaim':
                                          {'claimName': 'chdata-voice-clickhouse-0'}}]},
                    'status': {'containerStatuses': [{'name': 'clickhouse', 'ready': False,
                                                       'lastState': {'terminated': {'reason': 'OOMKilled'}}}]}}
        self.lock = {'metadata': {'namespace': update_images.NS, 'name': update_images.LOCK,
                                  'uid': 'lock-uid', 'resourceVersion': '1',
                                  'labels': dict(update_images.LABELS)},
                     'data': {'operation': 'original-operation'}}
        self.pvc['metadata'].update(namespace=update_images.NS, name='chdata-voice-clickhouse-0')
        self.pvc['spec'] = {'volumeName': 'voice-dev-clickhouse'}
        self.config = None
        self.fail_rollout = False

    def call(self, *args, body=None):
        self.calls.append((args, body))
        if args[:2] == ('get', 'namespace'):
            return {'metadata': {'name': update_images.NS, 'labels': dict(update_images.LABELS)}}
        if args[:2] == ('get', 'configmap'):
            return copy.deepcopy(self.lock if args[2] == update_images.LOCK else self.config)
        if args[:2] == ('get', 'statefulset'):
            return copy.deepcopy(self.sts)
        if args[:2] == ('get', 'pod'):
            return copy.deepcopy(self.pod)
        if args[:2] == ('get', 'pvc'):
            return copy.deepcopy(self.pvc)
        if args[:2] == ('get', 'resourcequota'):
            return {'status': {'used': {'limits.memory': '19584Mi'},
                               'hard': {'limits.memory': '20Gi'}}}
        if args[0] == 'create':
            result = copy.deepcopy(body)
            result['metadata']['uid'] = 'config-uid'
            if '--dry-run=server' not in args:
                if self.config is not None:
                    raise RuntimeError('ConfigMap already exists')
                self.config = copy.deepcopy(result)
            return result
        if args[0] == 'patch':
            expected = copy.deepcopy(self.sts)
            policy.add_clickhouse_memory_mount(expected['spec']['template']['spec'])
            if '--dry-run=server' not in args:
                self.sts = copy.deepcopy(expected)
                self.sts['metadata']['resourceVersion'] = '15'
                self.sts['metadata']['generation'] = 4
                self.sts['status']['observedGeneration'] = 4
                self.sts['status']['updateRevision'] = 'policy-revision'
            return expected
        if args[0] == 'rollout':
            if self.fail_rollout:
                raise RuntimeError('rollout timeout')
            self.sts['status'].update(currentRevision='policy-revision',
                                      updateRevision='policy-revision', readyReplicas=1)
            self.pod['metadata']['uid'] = 'replacement-pod'
            self.pod['metadata']['labels']['controller-revision-hash'] = 'policy-revision'
            self.pod['spec']['containers'][0] = copy.deepcopy(
                self.sts['spec']['template']['spec']['containers'][0])
            self.pod['status']['containerStatuses'][0]['ready'] = True
            return {}
        if args[0] == 'delete':
            self.lock = None
            return {}
        raise AssertionError(args)


class PolicyTests(unittest.TestCase):
    def test_preview_dry_runs_create_and_exact_spec_patch_without_mutation(self):
        client = Fake()
        policy.mitigate(client, PINS)
        self.assertIsNone(client.config)
        self.assertEqual(client.sts['metadata']['resourceVersion'], '14')
        self.assertTrue(all('--dry-run=server' in args for args, _ in client.calls
                            if args[0] in ('create', 'patch')))
        patch = next(body for args, body in client.calls if args[0] == 'patch')
        self.assertEqual([p['op'] for p in patch[:3]], ['test', 'test', 'test'])
        self.assertEqual(patch[0]['value'], 'store-uid')
        self.assertEqual(patch[1]['value'], '14')
        self.assertEqual(patch[3]['value'][0]['configMap']['defaultMode'], 420)

    def test_apply_exact_policy_replacement_readback_then_releases_lock(self):
        client = Fake()
        ratios = []
        policy.mitigate(client, PINS, apply=True, readback=lambda: ratios.append('0.8/3.20GiB'))
        self.assertEqual(ratios, ['0.8/3.20GiB'])
        self.assertEqual(client.config['data'], policy.clickhouse_memory_config()['data'])
        self.assertEqual(client.pvc['metadata']['uid'], 'pvc-uid')
        self.assertEqual(client.pod['metadata']['uid'], 'replacement-pod')
        self.assertEqual(client.sts['spec']['template']['spec']['containers'][0]['resources']['limits']['memory'],
                         '4Gi')
        self.assertIsNone(client.lock)

    def test_foreign_pins_or_image_fail_before_create_or_patch(self):
        for drift in ('lock', 'operation', 'sts', 'pod', 'pvc', 'image', 'ready'):
            client = Fake()
            if drift == 'lock':
                client.lock['metadata']['uid'] = 'foreign'
            elif drift == 'operation':
                client.lock['data']['operation'] = 'foreign'
            elif drift == 'sts':
                client.sts['metadata']['uid'] = 'foreign'
            elif drift == 'pod':
                client.pod['metadata']['uid'] = 'foreign'
            elif drift == 'pvc':
                client.pvc['metadata']['uid'] = 'foreign'
            elif drift == 'image':
                client.sts['spec']['template']['spec']['containers'][0]['image'] = 'foreign'
            else:
                client.pod['status']['containerStatuses'][0]['ready'] = True
            with self.subTest(drift=drift), self.assertRaises((RuntimeError, ValueError)):
                policy.mitigate(client, PINS, apply=True, readback=lambda: None)
            self.assertFalse(any(args[0] in ('create', 'patch') for args, _ in client.calls))

    def test_timeout_or_effective_config_failure_retains_lock_and_configmap(self):
        for failure in ('timeout', 'readback'):
            client = Fake()
            client.fail_rollout = failure == 'timeout'
            def failed_readback():
                raise RuntimeError('effective cap missing')
            with self.subTest(failure=failure), self.assertRaises(RuntimeError):
                policy.mitigate(client, PINS, apply=True,
                                readback=failed_readback if failure == 'readback' else lambda: None)
            self.assertEqual(client.lock['metadata']['uid'], 'lock-uid')
            self.assertEqual(client.config['metadata']['uid'], 'config-uid')
            self.assertFalse(any(args[0] == 'delete' for args, _ in client.calls))

    def test_final_lock_cas_drift_retains_lock(self):
        for drift in ('uid', 'resourceVersion', 'operation'):
            client = Fake()
            def change_lock():
                field = 'data' if drift == 'operation' else 'metadata'
                client.lock[field][drift] = 'foreign'
            with self.subTest(drift=drift), self.assertRaises(RuntimeError):
                policy.mitigate(client, PINS, apply=True, readback=change_lock)
            self.assertFalse(any(args[0] == 'delete' for args, _ in client.calls))

    def test_post_rollout_identity_or_binding_drift_retains_lock(self):
        for drift in ('pod_namespace', 'pod_owner', 'pod_volume', 'pvc_volume',
                      'observed_generation', 'revision', 'ready'):
            class Drift(Fake):
                def call(self, *args, body=None):
                    result = super().call(*args, body=body)
                    if args[0] == 'rollout':
                        if drift == 'pod_namespace':
                            self.pod['metadata']['namespace'] = 'foreign'
                        elif drift == 'pod_owner':
                            self.pod['metadata']['ownerReferences'][0]['uid'] = 'foreign'
                        elif drift == 'pod_volume':
                            self.pod['spec']['volumes'][0]['persistentVolumeClaim']['claimName'] = 'foreign'
                        elif drift == 'pvc_volume':
                            self.pvc['spec']['volumeName'] = 'foreign'
                        elif drift == 'observed_generation':
                            self.sts['status']['observedGeneration'] = 3
                        elif drift == 'revision':
                            self.sts['status']['currentRevision'] = 'old-revision'
                        else:
                            self.sts['status']['readyReplicas'] = 0
                    return result
            client = Drift()
            with self.subTest(drift=drift), self.assertRaises(RuntimeError):
                policy.mitigate(client, PINS, apply=True, readback=lambda: None)
            self.assertIsNotNone(client.lock)
            self.assertFalse(any(args[0] == 'delete' for args, _ in client.calls))


if __name__ == '__main__':
    unittest.main()
