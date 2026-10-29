"""Real frontend executor with synthetic Kubernetes and durable common fence."""
import copy
import json
import unittest

import frontend_runtime_adoption as adoption
import frontend_runtime_executor as executor
import frontend_runtime_executor_test as fixture


class ReuseTest(unittest.TestCase):
    def setUp(self):
        fixture.ExecutorTest.setUp(self)
        self.config = adoption.configmap('web')
        self.config['metadata'].update(uid='qualified-cm', resourceVersion='11')
        self.cluster.configs[self.config['metadata']['name']] = copy.deepcopy(self.config)
        old = self.cluster.deployments['voice-web']
        old_image = 'ghcr.io/x/web@sha256:' + 'd' * 64
        bootstrap = adoption.plan('web', old, old_image)
        bootstrap['target_spec']['template']['spec']['volumes'][-1]['configMap']['defaultMode'] = 420
        bootstrap['patch'][-1]['value'] = copy.deepcopy(bootstrap['target_spec'])
        self.cluster.patch('voice-web', bootstrap['patch'])
        self.cluster.patched = 0
        original_patch = self.cluster.patch
        def next_pod(name, patch):
            result = original_patch(name, patch)
            self.cluster.pods[0]['metadata']['uid'] = 'pod-successor'
            return result
        self.cluster.patch = next_pod
        self.journal.intent['frontend_existing_config'] = {'web': adoption.validate_configmap('web', self.config)}
        self.pin_deployment()

    def pin_deployment(self):
        obj = self.cluster.deployments['voice-web']
        self.journal.intent['frontend_deployment_baseline']['web'] = {'uid': obj['metadata']['uid'],
            'generation': obj['metadata']['generation'], 'spec': copy.deepcopy(obj['spec'])}
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.phase = executor.FrontendExecutor(self.journal, self.fence, self.cluster, self.lock, 'ns',
                                                self.images, self.platform_digests, self.cluster)

    def test_existing_immutable_config_is_reused_with_new_admitted_image(self):
        result = self.phase.run('web')
        self.assertEqual(result['config']['uid'], 'qualified-cm')
        self.assertEqual(self.cluster.created, 0)
        self.assertEqual(self.cluster.patched, 1)
        self.assertEqual(self.cluster.deployments['voice-web']['spec']['template']['spec']['containers'][0]['image'],
                         self.images['voice-web/web'])
        self.assertEqual(self.phase.verify_current('web')['mode'], 'verify-current')

    def test_final_recheck_detects_changed_served_bytes(self):
        self.phase.run('web')
        self.cluster.wrong_bytes = True
        with self.assertRaises(executor.RecoveryRequired): self.phase.verify_current('web')
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_already_target_checks_actual_pod_without_patch(self):
        image = self.images['voice-web/web']
        self.cluster.deployments['voice-web']['spec']['template']['spec']['containers'][0]['image'] = image
        self.cluster.rs[0]['spec']['template']['spec']['containers'][0]['image'] = image
        self.cluster.pods[0]['spec']['containers'][0]['image'] = image
        self.pin_deployment()
        result = self.phase.run('web')
        self.assertEqual(result['mode'], 'already-target')
        self.assertEqual(self.cluster.patched, 0)
        self.assertEqual(self.cluster.created, 0)
        self.assertEqual(result['pods'][0][1], 'pod-new')

    def test_missing_or_changed_pinned_config_never_mutates(self):
        for change in ('uid', 'rv', 'bytes', 'immutable', 'absent'):
            with self.subTest(change=change):
                self.setUp()
                cm = self.cluster.configs[self.config['metadata']['name']]
                if change == 'uid': cm['metadata']['uid'] = 'other'
                if change == 'rv': cm['metadata']['resourceVersion'] = '12'
                if change == 'bytes': cm['data'][adoption.KEY] = cm['data'][adoption.KEY].replace('"version":1', '"version": 1')
                if change == 'immutable': cm['immutable'] = False
                if change == 'absent': self.cluster.configs.clear()
                with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
                self.assertEqual(self.cluster.created, 0)
                self.assertEqual(self.cluster.patched, 0)

    def test_projection_drift_and_missing_pin_fail(self):
        self.cluster.deployments['voice-web']['spec']['template']['spec']['containers'][0]['volumeMounts'][0]['readOnly'] = False
        with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
        self.assertEqual(self.cluster.patched, 0)
        self.setUp()
        self.journal.intent['frontend_existing_config'] = {}
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.phase = executor.FrontendExecutor(self.journal, self.fence, self.cluster, self.lock, 'ns',
                                                self.images, self.platform_digests, self.cluster)
        with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
        self.assertEqual(self.cluster.patched, 0)

    def test_existing_baseline_drift_and_unqualified_noop_hold(self):
        for change in ('command', 'generation', 'uid', 'already-target'):
            with self.subTest(change=change):
                self.setUp()
                obj = self.cluster.deployments['voice-web']
                if change == 'command': obj['spec']['template']['spec']['containers'][0]['command'] = ['foreign']
                if change == 'generation': obj['metadata']['generation'] += 1
                if change == 'uid': obj['metadata']['uid'] = 'foreign-uid'
                if change == 'already-target': obj['spec']['template']['spec']['containers'][0]['image'] = self.images['voice-web/web']
                with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
                self.assertEqual(self.cluster.patched, 0)

    def test_lost_image_response_requires_hold_and_no_retry(self):
        self.cluster.lose_patch = True
        with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
        self.assertEqual(self.cluster.patched, 1)
        self.assertEqual(self.ledger.row['status'], 'uncertain')
        with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
        self.assertEqual(self.cluster.patched, 1)

    def test_new_source_cycle_reuses_actual_config_without_old_source_claim(self):
        self.context['commit'] = 'f' * 40
        self.fence.context = copy.deepcopy(self.context)
        self.card['candidate']['commit'] = self.context['commit']
        self.ledger.row = self.context | {'status': 'active', 'step': 'admitted'}
        self.journal.intent['resource_context'] = copy.deepcopy(self.context)
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.lock['data']['resource_context'] = json.dumps(self.context, sort_keys=True, separators=(',', ':'))
        self.phase = executor.FrontendExecutor(self.journal, self.fence, self.cluster, self.lock, 'ns',
                                                self.images, self.platform_digests, self.cluster)
        self.assertEqual(self.phase.run('web')['config']['uid'], 'qualified-cm')
        self.assertEqual(self.cluster.created, 0)


if __name__ == '__main__': unittest.main()
