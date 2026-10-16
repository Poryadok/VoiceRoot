import copy
import unittest

import environment as dev


def source():
    return [
        {'apiVersion': 'apps/v1', 'kind': 'Deployment',
         'metadata': {'name': 'voice-chat', 'namespace': 'voice-staging', 'uid': 'old'},
         'spec': {'replicas': 1, 'selector': {'matchLabels': {'app': 'voice-chat'}},
                  'template': {'metadata': {'labels': {'app': 'voice-chat'}}, 'spec': {
                      'containers': [{'name': 'chat', 'image': 'registry/chat@sha256:' + 'a' * 64,
                                      'envFrom': [{'secretRef': {'name': 'voice-app-secrets'}}]}]}}}},
        {'apiVersion': 'v1', 'kind': 'Service',
         'metadata': {'name': 'voice-livekit', 'namespace': 'voice-staging'},
         'spec': {'type': 'NodePort', 'clusterIP': '10.43.1.1',
                  'ports': [{'name': 'rtc-tcp', 'port': 7881, 'targetPort': 7881,
                             'nodePort': 30881},
                            {'name': 'rtc-udp', 'port': 7882, 'targetPort': 7882,
                             'protocol': 'UDP', 'nodePort': 30882}]}},
        {'apiVersion': 'v1', 'kind': 'ConfigMap',
         'metadata': {'name': 'voice-app-config', 'namespace': 'voice-staging'},
         'data': {'AUTH_OAUTH_PUBLIC_API_BASE_URL': 'https://voice.comrade.click'}},
    ]


class IsolationTests(unittest.TestCase):
    def test_volume_binding_annotations_do_not_survive_rendering(self):
        meta = {'annotations': {'pv.kubernetes.io/bind-completed': 'yes',
                               'volume.kubernetes.io/selected-node': 'old',
                               'volume.beta.kubernetes.io/storage-provisioner': 'old'}}
        dev.clean_meta(meta)
        self.assertEqual(meta['annotations'], {})

    def test_render_retargets_without_mutating_source_or_reusing_server_identity(self):
        snapshot = source()
        original = copy.deepcopy(snapshot)
        output = dev.render(snapshot, 'pmdebook')
        self.assertEqual(snapshot, original)
        workload = next(x for x in output if x['kind'] == 'Deployment')
        self.assertEqual(workload['metadata']['namespace'], 'voice-dev')
        self.assertNotIn('uid', workload['metadata'])
        self.assertEqual(workload['spec']['replicas'], 0)
        self.assertFalse(workload['spec']['template']['spec']['automountServiceAccountToken'])
        self.assertIn('limits', workload['spec']['template']['spec']['containers'][0]['resources'])
        config = next(x for x in output if x['metadata']['name'] == 'voice-app-config')
        self.assertEqual(config['data']['AUTH_OAUTH_PUBLIC_API_BASE_URL'], 'https://api-dev.comrade.click')

    def test_livekit_ports_do_not_collide_with_staging(self):
        svc = next(x for x in dev.render(source(), 'pmdebook') if x['kind'] == 'Service')
        self.assertNotIn('clusterIP', svc['spec'])
        self.assertEqual([p['nodePort'] for p in svc['spec']['ports']], [31081, 31082])
        self.assertEqual([p['targetPort'] for p in svc['spec']['ports']], [31081, 31082])

    def test_cross_environment_references_and_pod_host_access_fail_closed(self):
        for field in ({'hostNetwork': True}, {'volumes': [{'name': 'bad', 'hostPath': {'path': '/'}}]}):
            snapshot = source()
            snapshot[0]['spec']['template']['spec'].update(field)
            with self.assertRaises(ValueError):
                dev.render(snapshot, 'pmdebook')
        snapshot = source()
        snapshot[2]['data']['OTHER'] = 'postgres://voice-postgres.voice-prod:5432/db'
        with self.assertRaises(ValueError):
            dev.render(snapshot, 'pmdebook')

    def test_secrets_cluster_objects_and_unpinned_images_cannot_be_cloned(self):
        for item in ({'kind': 'Secret'}, {'kind': 'ClusterRole'}):
            with self.assertRaises(ValueError):
                dev.render([item], 'pmdebook')
        snapshot = source()
        snapshot[0]['spec']['template']['spec']['containers'][0]['image'] = 'registry/chat:latest'
        with self.assertRaises(ValueError):
            dev.render(snapshot, 'pmdebook')

    def test_network_isolation_and_home_storage_are_explicit_and_bounded(self):
        output = dev.foundation('pmdebook')
        policies = [x for x in output if x['kind'] == 'NetworkPolicy']
        self.assertTrue(any(x['spec'].get('policyTypes') == ['Ingress', 'Egress'] for x in policies))
        pvs = [x for x in output if x['kind'] == 'PersistentVolume']
        self.assertEqual(len(pvs), 4)
        for pv in pvs:
            self.assertTrue(pv['spec']['local']['path'].startswith('/home/voice-dev/data/'))
            self.assertEqual(pv['spec']['claimRef']['namespace'], 'voice-dev')
            self.assertEqual(pv['spec']['persistentVolumeReclaimPolicy'], 'Retain')
        self.assertTrue(any(x['kind'] == 'ResourceQuota' for x in output))

    def test_existing_dev_marker_required_and_no_staging_mutations(self):
        self.assertEqual(dev.target_args(), ['--namespace', 'voice-dev'])
        with self.assertRaises(ValueError):
            dev.validate_target({'metadata': {'name': 'voice-staging'}})
        with self.assertRaises(ValueError):
            dev.validate_target({'metadata': {'name': 'voice-dev', 'labels': {}}})

    def test_literal_source_credentials_or_cluster_ips_are_rejected(self):
        for value in ('postgres://voice:password@voice-postgres:5432/chat_db', 'http://10.43.1.23:8080'):
            snapshot = source()
            snapshot[0]['spec']['template']['spec']['containers'][0]['env'] = [{'name': 'UPSTREAM', 'value': value}]
            with self.assertRaises(ValueError):
                dev.render(snapshot, 'pmdebook')

    def test_jobs_are_suspended_and_fresh_bootstrap_refs_resolve(self):
        job = source()[0]
        job['kind'] = 'Job'
        job['spec']['template']['spec']['volumes'] = [{'name': 'creds', 'secret': {'secretName': 'voice-nats-bootstrap-contract-3a08869d02b06f22f565ad71'}}]
        result = dev.render([job], 'pmdebook')[0]
        self.assertTrue(result['spec']['suspend'])
        self.assertEqual(result['spec']['template']['spec']['volumes'][0]['secret']['secretName'], 'voice-nats-bootstrap-credentials')

    def test_missing_fresh_secret_or_key_fails_before_apply(self):
        rendered = dev.render(source(), 'pmdebook')
        with self.assertRaises(ValueError):
            dev.validate_references(rendered, [])
        dev.validate_references(rendered, [{'kind': 'Secret', 'metadata': {'name': 'voice-app-secrets', 'namespace': 'voice-dev'}, 'data': {'KEY': 'eA=='}}])


if __name__ == '__main__':
    unittest.main()
