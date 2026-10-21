import copy
import unittest

import environment
import repair_clickhouse_memory as repair
import update_images as update


def workload():
    return {'kind': 'StatefulSet', 'metadata': {
        'namespace': update.NS, 'name': 'voice-clickhouse', 'labels': dict(update.LABELS),
        'uid': 'store-uid', 'resourceVersion': '14'}, 'spec': {
        'replicas': 1, 'updateStrategy': {'type': 'RollingUpdate'},
        'volumeClaimTemplates': [{'metadata': {'name': 'chdata'},
                                  'spec': {'resources': {'requests': {'storage': '2Gi'}}}}],
        'template': {'spec': {'containers': [{
            'name': 'clickhouse', 'image': 'docker.io/clickhouse/clickhouse-server@sha256:' + 'a' * 64,
            'resources': {'requests': {'cpu': '20m', 'memory': '256Mi'},
                          'limits': {'cpu': '500m', 'memory': '2Gi'}},
            'volumeMounts': [{'name': 'chdata', 'mountPath': '/var/lib/clickhouse'}]}]}}}}


class Fake:
    def __init__(self):
        self.sts = workload()
        self.pvc = {'metadata': {'uid': 'pvc-uid'}, 'status': {'phase': 'Bound'}}
        self.calls = []
        self.fail_rollout = False
        self.used = '17536Mi'

    def call(self, *args, body=None):
        self.calls.append((args, body))
        if args[:2] == ('get', 'namespace'):
            return {'metadata': {'name': update.NS, 'labels': dict(update.LABELS)}}
        if args[0] == 'create':
            self.lock = copy.deepcopy(body)
            self.lock['metadata'].update(uid='lock', resourceVersion='1')
            return self.lock
        if args[:2] == ('get', 'configmap'):
            return self.lock
        if args[:2] == ('get', 'resourcequota'):
            return {'status': {'used': {'limits.memory': self.used},
                               'hard': {'limits.memory': '20Gi'}}}
        if args[:2] == ('get', 'pvc'):
            return copy.deepcopy(self.pvc)
        if args[:2] == ('get', 'statefulset'):
            return copy.deepcopy(self.sts)
        if args[0] == 'patch':
            result = copy.deepcopy(self.sts)
            result['spec']['template']['spec']['containers'][0]['resources']['limits']['memory'] = '4Gi'
            if '--dry-run=server' not in args:
                self.sts = result
            return result
        if args[0] == 'rollout' and self.fail_rollout:
            raise RuntimeError('timeout')
        return {}


class RepairTests(unittest.TestCase):
    def test_generator_reserves_4gi_for_clickhouse_without_changing_other_resources(self):
        snapshot = workload()
        snapshot['metadata']['namespace'] = 'voice-staging'
        container = environment.render([snapshot], 'pmdebook')[0]['spec']['template']['spec']['containers'][0]
        self.assertEqual(container['resources']['limits']['memory'], '4096Mi')
        self.assertEqual(container['resources']['requests']['memory'], '256Mi')

    def test_plan_is_cas_guarded_and_changes_only_memory_ceiling(self):
        before = workload()
        patch, after = repair.plan(before)
        self.assertEqual(patch[0], {'op': 'test', 'path': '/metadata/uid', 'value': 'store-uid'})
        self.assertTrue(any(p['path'] == '/metadata/resourceVersion' for p in patch))
        self.assertTrue(any(p['path'] == '/spec' and p['op'] == 'test' for p in patch))
        self.assertEqual(patch[-1]['path'], '/spec/template/spec/containers/0/resources/limits/memory')
        after['spec']['template']['spec']['containers'][0]['resources']['limits']['memory'] = '2Gi'
        self.assertEqual(after, before)

    def test_foreign_targets_or_unexpected_store_shapes_fail_closed(self):
        for mutate in (
            lambda s: s['metadata'].update(namespace='voice-staging'),
            lambda s: s['metadata'].update(name='voice-postgres'),
            lambda s: s['spec']['template']['spec']['containers'][0].update(image='clickhouse:latest'),
            lambda s: s['spec'].update(replicas=2),
            lambda s: s['spec']['updateStrategy'].update(type='OnDelete'),
            lambda s: s['spec']['template']['spec']['containers'][0]['resources']['limits'].update(memory='1Gi'),
        ):
            s = workload()
            mutate(s)
            with self.assertRaises(ValueError):
                repair.plan(s)

    def test_preview_is_readonly_and_apply_preserves_pvc_and_spec(self):
        client = Fake()
        repair.repair(client)
        self.assertFalse(any(a[0] in ('create', 'delete') for a, b in client.calls))
        self.assertTrue(all('--dry-run=server' in a for a, b in client.calls if a[0] == 'patch'))
        client = Fake()
        repair.repair(client, apply=True)
        self.assertEqual(client.sts['spec']['template']['spec']['containers'][0]['resources']['limits']['memory'], '4Gi')
        self.assertTrue(any(a[0] == 'delete' for a, b in client.calls))

    def test_quota_failure_has_no_mutation_and_rollout_failure_retains_lock(self):
        client = Fake()
        client.used = '19Gi'
        with self.assertRaises(ValueError):
            repair.repair(client, apply=True)
        self.assertFalse(any(a[0] == 'patch' and '--dry-run=server' not in a for a, b in client.calls))
        client = Fake()
        client.fail_rollout = True
        with self.assertRaises(RuntimeError):
            repair.repair(client, apply=True)
        self.assertFalse(any(a[0] == 'delete' for a, b in client.calls))

    def test_pvc_or_spec_drift_during_rollout_retains_lock(self):
        for drift in ('pvc', 'image'):
            class Drift(Fake):
                def call(self, *args, body=None):
                    result = super().call(*args, body=body)
                    if args[0] == 'rollout':
                        if drift == 'pvc':
                            self.pvc['metadata']['uid'] = 'replacement'
                        else:
                            self.sts['spec']['template']['spec']['containers'][0]['image'] = 'foreign-image'
                    return result
            client = Drift()
            with self.assertRaises(RuntimeError):
                repair.repair(client, apply=True)
            self.assertFalse(any(a[0] == 'delete' for a, b in client.calls))

    def test_already_repaired_target_is_idempotent_and_waits_for_readiness(self):
        client = Fake()
        client.sts['spec']['template']['spec']['containers'][0]['resources']['limits']['memory'] = '4Gi'
        repair.repair(client, apply=True)
        self.assertFalse(any(a[0] == 'patch' for a, b in client.calls))
        self.assertTrue(any(a[0] == 'rollout' for a, b in client.calls))


if __name__ == '__main__':
    unittest.main()
