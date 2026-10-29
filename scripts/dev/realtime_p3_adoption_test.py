import copy
import unittest

import realtime_p3_adoption as adoption


class PlanTest(unittest.TestCase):
    def setUp(self):
        self.original = {
            'apiVersion': 'apps/v1', 'kind': 'Deployment',
            'metadata': {'name': 'voice-realtime', 'namespace': 'voice-dev',
                         'uid': 'deployment-uid', 'resourceVersion': '123'},
            'spec': {'replicas': 1, 'selector': {'matchLabels': {'app': 'voice-realtime'}},
                     'template': {'metadata': {'labels': {'app': 'voice-realtime'}},
                                  'spec': {'containers': [
                                      {'name': 'realtime', 'image': 'old', 'env': [{'name': 'NATS_URL', 'value': 'nats://existing'}]},
                                      {'name': 'sidecar', 'image': 'other'}]}}}}
        self.receipts = {
            'assets': {name: {'uid': 'uid-' + name, 'resourceVersion': '4', 'namespace': 'voice-dev',
                              'kind': 'ConfigMap' if name in ('voice-app-config', 'voice-p3-principal-public') else 'Secret',
                              'keys': sorted(keys)} for name, keys in adoption.ASSETS.items()},
            'schema': {'database': 'realtime_db', 'migration': '000001_admission',
                       'evidence_sha256': 'a' * 64},
            'transport_evidence_sha256': 'b' * 64,
            'public_hashes': {key: 'c' * 64 for key in adoption.PUBLIC_HASHES},
            'service': {'name': 'voice-realtime', 'namespace': 'voice-dev', 'uid': 'service-uid', 'resourceVersion': '5', 'type': 'ClusterIP', 'clusterIP': '10.0.0.1', 'selector': {'app': 'voice-realtime'}, 'ports': [{'name': 'realtime-lifecycle', 'port': 9091}]},
        }
        self.receipts['assets']['voice-p3-principal-public']['immutable'] = True

    def test_preserves_original_and_fences_full_spec(self):
        before = copy.deepcopy(self.original)
        patch, target, digest = adoption.plan(self.original, 'registry/realtime@sha256:' + 'd' * 64, self.receipts)
        self.assertEqual(before, self.original)
        self.assertEqual(patch[:3], [
            {'op': 'test', 'path': '/metadata/uid', 'value': 'deployment-uid'},
            {'op': 'test', 'path': '/metadata/resourceVersion', 'value': '123'},
            {'op': 'test', 'path': '/spec', 'value': before['spec']}])
        self.assertEqual(patch[3], {'op': 'replace', 'path': '/spec', 'value': target})
        container = target['template']['spec']['containers'][0]
        self.assertIn({'name': 'NATS_URL', 'value': 'nats://existing'}, container['env'])
        self.assertEqual(target['template']['spec']['containers'][1], before['spec']['template']['spec']['containers'][1])
        self.assertEqual(target['strategy'], {'type': 'Recreate'})
        self.assertEqual(target['template']['spec']['securityContext']['fsGroup'], 65532)
        values = {item['name']: item for item in container['env']}
        self.assertEqual(values['DATABASE_URL']['valueFrom']['secretKeyRef'], {'name': 'voice-app-secrets', 'key': 'REALTIME_DATABASE_URL'})
        self.assertEqual(values['REALTIME_SPACE_PRINCIPAL_JWKS_URL']['value'], 'https://voice-space.voice-dev.svc:8443/.well-known/jwks.json')
        trust = next(v for v in target['template']['spec']['volumes'] if v['name'] == 'p3-trust')
        self.assertEqual({i['key'] for i in trust['secret']['items']}, {'chat-server-ca.crt', 'space-client-ca.crt', 'space-jwks-ca.crt'})
        self.assertRegex(digest, r'^[0-9a-f]{64}$')

    def test_rejects_stale_or_incomplete_inputs(self):
        for mutation in ('missing_asset', 'wrong_keys', 'empty_uid', 'missing_schema', 'collision', 'wrong_namespace', 'mutable_image', 'envfrom_collision', 'mount_collision', 'replicas', 'nonimmutable', 'secret_value', 'service_port', 'service_selector', 'jwks_client', 'external_service'):
            with self.subTest(mutation=mutation):
                original, receipts = copy.deepcopy(self.original), copy.deepcopy(self.receipts)
                image = 'registry/realtime@sha256:' + 'd' * 64
                if mutation == 'missing_asset': receipts['assets'].pop('voice-realtime-p3-tls')
                if mutation == 'wrong_keys': receipts['assets']['voice-realtime-principal-signing']['keys'] = ['current.pem']
                if mutation == 'empty_uid': receipts['assets']['voice-realtime-p3-tls']['uid'] = ''
                if mutation == 'missing_schema': receipts['schema']['evidence_sha256'] = ''
                if mutation == 'collision': original['spec']['template']['spec']['containers'][0]['env'].append({'name': 'DATABASE_URL', 'value': 'unexpected'})
                if mutation == 'wrong_namespace': original['metadata']['namespace'] = 'voice-staging'
                if mutation == 'mutable_image': image = 'registry/realtime:latest'
                if mutation == 'envfrom_collision':
                    original['spec']['template']['spec']['containers'][0]['envFrom'] = [{'configMapRef': {'name': 'voice-app-config'}}]
                    receipts['assets']['voice-app-config']['keys'].append('REALTIME_PRINCIPAL_ACTIVE_KID')
                if mutation == 'mount_collision': original['spec']['template']['spec']['containers'][0]['volumeMounts'] = [{'name': 'other', 'mountPath': '/etc/voice/p3'}]
                if mutation == 'replicas': original['spec']['replicas'] = 2
                if mutation == 'nonimmutable': receipts['assets']['voice-p3-principal-public']['immutable'] = False
                if mutation == 'secret_value': receipts['assets']['voice-realtime-p3-tls']['data'] = 'secret'
                if mutation == 'service_port': receipts['service']['ports'][0]['targetPort'] = 8080
                if mutation == 'service_selector': receipts['service']['selector']['app'] = 'other'
                if mutation == 'jwks_client': original['spec']['template']['spec']['containers'][0]['env'].append({'name': 'REALTIME_SPACE_PRINCIPAL_JWKS_CLIENT_CERT_FILE', 'value': '/unverified'})
                if mutation == 'external_service': receipts['service']['type'] = 'ExternalName'
                with self.assertRaises(ValueError): adoption.plan(original, image, receipts)

    def test_operation_owned_credential_is_fixed_and_immutable(self):
        name = 'voice-realtime-db-' + 'a'*32
        receipts = copy.deepcopy(self.receipts)
        receipts['assets'].pop('voice-app-secrets')
        receipts['assets'][name] = {'uid':'uid', 'resourceVersion':'7', 'namespace':'voice-dev',
                                     'kind':'Secret', 'keys':['DATABASE_URL'], 'immutable':True}
        _, target, _ = adoption.plan(self.original, 'registry/realtime@sha256:'+'d'*64, receipts,
                                     operation_id='media-c1-'+'a'*32)
        env = target['template']['spec']['containers'][0]['env']
        self.assertEqual(next(e for e in env if e['name']=='DATABASE_URL')['valueFrom']['secretKeyRef'],
                         {'name':name,'key':'DATABASE_URL'})
        _, second, _ = adoption.plan(self.original, 'registry/realtime@sha256:'+'d'*64, receipts,
                                      operation_id='media-c2-'+'a'*32)
        second_env = second['template']['spec']['containers'][0]['env']
        self.assertEqual(next(e for e in second_env if e['name']=='DATABASE_URL')['valueFrom']['secretKeyRef'],
                         {'name':name,'key':'DATABASE_URL'})
        receipts['assets'][name]['immutable'] = False
        with self.assertRaises(ValueError):
            adoption.plan(self.original, 'registry/realtime@sha256:'+'d'*64, receipts, operation_id='media-c1-'+'a'*32)
        with self.assertRaises(ValueError):
            adoption.plan(self.original, 'registry/realtime@sha256:'+'d'*64, receipts, operation_id='media-c3-'+'a'*32)

    def test_patch_rejects_intervening_identity_and_spec_changes(self):
        patch, target, _ = adoption.plan(self.original, 'registry/realtime@sha256:' + 'd' * 64, self.receipts)
        # Model JSON Patch's atomic test-before-replace semantics on owned fixtures.
        def apply(obj):
            obj = copy.deepcopy(obj)
            for operation in patch:
                keys = operation['path'].strip('/').split('/')
                parent = obj
                for key in keys[:-1]: parent = parent[key]
                if operation['op'] == 'test' and parent[keys[-1]] != operation['value']:
                    raise ValueError('CAS conflict')
                if operation['op'] == 'replace': parent[keys[-1]] = copy.deepcopy(operation['value'])
            return obj
        self.assertEqual(apply(self.original)['spec'], target)
        for path in ('uid', 'resourceVersion', 'spec'):
            changed = copy.deepcopy(self.original)
            if path == 'spec': changed['spec']['replicas'] = 2
            else: changed['metadata'][path] += '-changed'
            with self.subTest(path=path), self.assertRaises(ValueError): apply(changed)
        self.original['spec']['replicas'] = 9
        self.assertEqual(patch[2]['value']['replicas'], 1)

    def test_service_addition_is_fenced_and_preserves_http(self):
        service = {'apiVersion': 'v1', 'kind': 'Service',
                   'metadata': {'name': 'voice-realtime', 'namespace': 'voice-dev', 'uid': 'u', 'resourceVersion': '10'},
                   'spec': {'selector': {'app': 'voice-realtime'}, 'clusterIP': '10.0.0.1',
                            'ports': [{'name': 'http', 'port': 8080, 'targetPort': 8080, 'protocol': 'TCP'}]}}
        before = copy.deepcopy(service)
        patch, target = adoption.plan_service(service, {'app': 'voice-realtime'})
        self.assertEqual(service, before)
        self.assertEqual(patch[2], {'op': 'test', 'path': '/spec', 'value': before['spec']})
        self.assertEqual(target['ports'][0], before['spec']['ports'][0])
        self.assertEqual(target['ports'][1], {'name': 'realtime-lifecycle', 'port': 9091, 'targetPort': 9091, 'protocol': 'TCP'})
        service['spec']['ports'].append({'name': 'conflict', 'port': 9091})
        with self.assertRaises(ValueError): adoption.plan_service(service, {'app': 'voice-realtime'})


if __name__ == '__main__':
    unittest.main()
