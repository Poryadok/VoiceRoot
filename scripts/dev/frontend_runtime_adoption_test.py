import copy
import unittest

import frontend_runtime_adoption as adoption


class AdoptionTest(unittest.TestCase):
    def deployment(self, app):
        name, container = adoption.APPS[app][:2]
        return {'kind': 'Deployment', 'metadata': {'namespace': 'voice-dev', 'name': name,
                'uid': 'uid-1', 'resourceVersion': '42'}, 'spec': {'replicas': 1,
                'template': {'spec': {'containers': [{'name': container, 'image': 'old'}]}}}}

    def test_complete_plan_and_compare_and_swap(self):
        old = self.deployment('web')
        image = 'ghcr.io/example/web@sha256:' + 'a' * 64
        plan = adoption.plan('web', old, image)
        self.assertEqual(plan['patch'][:3], [
            {'op': 'test', 'path': '/metadata/uid', 'value': 'uid-1'},
            {'op': 'test', 'path': '/metadata/resourceVersion', 'value': '42'},
            {'op': 'test', 'path': '/spec', 'value': old['spec']}])
        self.assertEqual(plan['old_spec'], old['spec'])
        self.assertEqual(plan['patch'][-1]['value'], plan['target_spec'])
        self.assertEqual(plan['configmap']['data']['voice-runtime-config.json'],
                         '{"version":1,"app_origin":"https://dev.comrade.click",'
                         '"api_base_url":"https://api-dev.comrade.click",'
                         '"livekit_url":"wss://livekit-dev.comrade.click"}\n')
        self.assertEqual(old, self.deployment('web'))

    def test_invalid_identity_and_collisions_fail_closed(self):
        image = 'ghcr.io/example/admin@sha256:' + 'b' * 64
        for mutation in ('uid', 'namespace', 'container', 'volume', 'mount', 'image'):
            old = self.deployment('admin')
            spec = old['spec']['template']['spec']
            if mutation == 'uid': old['metadata']['uid'] = ''
            if mutation == 'namespace': old['metadata']['namespace'] = 'voice-staging'
            if mutation == 'container': spec['containers'][0]['name'] = 'wrong'
            if mutation == 'volume': spec['volumes'] = [{'name': 'runtime-config', 'emptyDir': {}}]
            if mutation == 'mount': spec['containers'][0]['volumeMounts'] = [{'name': 'other', 'mountPath': adoption.MOUNT}]
            with self.subTest(mutation=mutation), self.assertRaises(adoption.Refused):
                adoption.plan('admin', old, 'tag:latest' if mutation == 'image' else image)

    def test_reconcile_requires_exact_uid_and_complete_spec(self):
        old = self.deployment('portal')
        plan = adoption.plan('portal', old, 'ghcr.io/example/portal@sha256:' + 'c' * 64)
        target = copy.deepcopy(old)
        target['spec'] = copy.deepcopy(plan['target_spec'])
        self.assertEqual(adoption.classify(plan, target), 'target')
        self.assertEqual(adoption.classify(plan, old), 'old')
        stale = copy.deepcopy(old)
        stale['metadata']['resourceVersion'] = '43'
        self.assertEqual(adoption.classify(plan, stale), 'conflict')
        target['metadata']['uid'] = 'different'
        self.assertEqual(adoption.classify(plan, target), 'conflict')
        target['metadata']['uid'] = 'uid-1'
        target['spec']['replicas'] = 2
        self.assertEqual(adoption.classify(plan, target), 'conflict')

    def test_public_config_rejects_wrong_tuple_and_schema(self):
        config = adoption.configmap('admin')
        config['metadata']['uid'] = 'config-uid'
        config['metadata']['resourceVersion'] = '11'
        adoption.validate_configmap('admin', config)
        for raw in ('{"version":1,"app_origin":"https://wrong","api_base_url":"https://api-dev.comrade.click"}',
                    '{"version":1,"version":1,"app_origin":"https://admin-dev.comrade.click","api_base_url":"https://api-dev.comrade.click"}',
                    '{"version":true,"app_origin":"https://admin-dev.comrade.click","api_base_url":"https://api-dev.comrade.click"}'):
            bad = copy.deepcopy(config)
            bad['data']['voice-runtime-config.json'] = raw
            with self.assertRaises(adoption.Refused): adoption.validate_configmap('admin', bad)

    def test_each_fixed_config_and_read_only_projection(self):
        for app in adoption.APPS:
            config = adoption.configmap(app)
            config['metadata']['uid'] = 'config-uid'
            config['metadata']['resourceVersion'] = '11'
            self.assertEqual(adoption.validate_configmap(app, config)['uid'], 'config-uid')
            plan = adoption.plan(app, self.deployment(app), 'ghcr.io/example/image@sha256:' + 'd' * 64)
            pod = plan['target_spec']['template']['spec']
            self.assertEqual(pod['volumes'][0]['configMap']['name'], config['metadata']['name'])
            self.assertEqual(pod['containers'][0]['volumeMounts'][0], {
                'name': 'runtime-config', 'mountPath': adoption.MOUNT,
                'subPath': adoption.KEY, 'readOnly': True})

    def test_config_identity_receipt_detects_aba_and_mutable_object(self):
        config = adoption.configmap('web')
        config['metadata'].update(uid='uid', resourceVersion='11')
        receipt = adoption.validate_configmap('web', config)
        config['metadata']['resourceVersion'] = '13'
        self.assertNotEqual(receipt, adoption.validate_configmap('web', config))
        config['immutable'] = False
        with self.assertRaises(adoption.Refused): adoption.validate_configmap('web', config)

    def test_preserves_unrelated_workload_and_old_snapshot(self):
        old = self.deployment('admin')
        pod = old['spec']['template']['spec']
        pod['containers'][0]['env'] = [{'name': 'MODE', 'value': 'existing'}]
        pod['containers'][0]['volumeMounts'] = [{'name': 'existing', 'mountPath': '/etc/existing'}]
        pod['volumes'] = [{'name': 'existing', 'emptyDir': {}}]
        before = copy.deepcopy(old)
        plan = adoption.plan('admin', old, 'ghcr.io/example/admin@sha256:' + 'e' * 64)
        result = plan['target_spec']['template']['spec']
        self.assertEqual(old, before)
        self.assertEqual(plan['old_spec'], before['spec'])
        self.assertEqual(result['containers'][0]['env'], pod['containers'][0]['env'])
        self.assertEqual(result['containers'][0]['volumeMounts'][0], pod['containers'][0]['volumeMounts'][0])
        self.assertEqual(result['volumes'][0], pod['volumes'][0])


if __name__ == '__main__': unittest.main()
