import base64
import copy
import json
import unittest
import nonnats_plan as subject

NS = 'voice-staging'

def secret():
    values = {'AUTH_RESEND_API_KEY': 'private-mail-token', 'AUTH_RESEND_FROM': 'Voice <sender@example.invalid>'}
    return {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {'name': 'voice-app-secrets', 'namespace': NS, 'uid': 's1', 'resourceVersion': '1'},
            'data': {k: base64.b64encode(v.encode()).decode() for k, v in values.items()}}

def fixture():
    live = secret()
    config = {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'voice-app-config', 'namespace': NS, 'uid': 'c1', 'resourceVersion': '2'}, 'data': {'S3_SIGNING_ENDPOINT': 'https://storage.example.invalid'}}
    mail = {'kind': 'Secret', 'name': 'voice-app-secrets', 'namespace': NS, 'desired': {'required_keys': ['AUTH_RESEND_API_KEY', 'AUTH_RESEND_FROM'], 'predicates': {'auth_mail': True}}}
    signing = {'kind': 'ConfigMap', 'name': 'voice-app-config', 'namespace': NS, 'desired': {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {}, 'data': config['data']}}
    plan = {'schema': 'nats-rollout-nonnats-v1', 'mode': 'images-only', 'checks': [
        {'id': 'auth-mail', 'disposition': 'preserve', 'objects': [mail]},
        {'id': 'signing-endpoint', 'disposition': 'preserve', 'objects': [signing]}], 'actions': []}
    return Fake([live, config]), plan

class Fake:
    def __init__(self, objects):
        self.objects = {(o['kind'].lower(), o['metadata']['name']): copy.deepcopy(o) for o in objects}
        self.calls = []
    def get(self, kind, name):
        self.calls.append((kind, name))
        return copy.deepcopy(self.objects[(kind.lower(), name)])

class PlanTests(unittest.TestCase):
    def test_private_semantic_hash_rejects_forgery_without_serializing_data(self):
        kube, plan = fixture()
        obj = kube.objects[('configmap', 'voice-app-config')]
        obj['data'] = {'livekit.yaml': 'private-inline-livekit-key'}
        plan['checks'][1]['objects'][0]['desired'] = {'private_semantic_sha256': subject.digest(subject.semantic(obj))}
        binding = subject.preflight(kube, plan, 'images-only')
        self.assertNotIn('private-inline-livekit-key', json.dumps(binding))
        self.assertTrue(subject.revalidate(kube, binding))
        plan['checks'][1]['objects'][0]['desired']['private_semantic_sha256'] = '0' * 64
        with self.assertRaises(subject.PlanError): subject.preflight(kube, plan, 'images-only')

    def test_shared_action_across_identical_target_rows(self):
        kube, plan = self.full_fixture()
        manifest = {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'voice-app-config', 'namespace': NS}, 'data': {'S3_SIGNING_ENDPOINT': 'new'}}
        for row in plan['checks']:
            if row['objects'][0]['kind'] != 'ConfigMap': continue
            row['disposition'] = 'action'
            row['objects'][0]['desired'] = subject.semantic(manifest)
            row['objects'][0]['action_id'] = 'config'
        plan['actions'] = [{'id': 'config', 'row': 'domains', 'manifest': manifest}]
        self.assertTrue(subject.revalidate(kube, subject.preflight(kube, plan, 'full')))

    def full_fixture(self):
        kube, plan = fixture()
        plan['mode'] = 'full'
        for row_id in subject.ROW_IDS:
            if row_id in {'auth-mail', 'signing-endpoint'}: continue
            objects = copy.deepcopy(plan['checks'][1]['objects'])
            if row_id in subject.SECRET_CONTRACTS:
                objects = []
                for name, keys in subject.SECRET_CONTRACTS[row_id].items():
                    objects.append({'kind': 'Secret', 'name': name, 'namespace': NS, 'desired': {'required_keys': sorted(keys)}})
                    kube.objects[('secret', name)] = {'kind': 'Secret', 'metadata': {'name': name, 'namespace': NS, 'uid': name, 'resourceVersion': '1'},
                        'data': {key: base64.b64encode(b'private-unchanged-material').decode() for key in keys}}
            plan['checks'].append({'id': row_id, 'disposition': 'preserve', 'objects': objects})
        return kube, plan

    def test_full_compatible_target_is_supported(self):
        kube, plan = self.full_fixture()
        self.assertTrue(subject.revalidate(kube, subject.preflight(kube, plan, 'full')))

    def test_required_secret_rows_cannot_be_replaced_by_configmap(self):
        for row_id in ('minio-credentials', 'principal-secrets', 'user-search-cursor'):
            kube, plan = self.full_fixture()
            next(row for row in plan['checks'] if row['id'] == row_id)['objects'] = copy.deepcopy(plan['checks'][1]['objects'])
            with self.subTest(row_id=row_id):
                with self.assertRaises(subject.PlanError): subject.preflight(kube, plan, 'full')

    def test_bounded_action_binds_baseline_and_rejects_nats_action(self):
        kube, plan = self.full_fixture()
        row = next(row for row in plan['checks'] if row['id'] == 'domains')
        baseline = copy.deepcopy(kube.objects[('configmap', 'voice-app-config')])
        baseline['metadata']['name'] = 'voice-domain-config'
        kube.objects[('configmap', 'voice-domain-config')] = baseline
        row['objects'][0]['name'] = 'voice-domain-config'
        row['disposition'] = 'action'
        row['objects'][0]['action_id'] = 'config'
        row['objects'][0]['desired']['data'] = {'S3_SIGNING_ENDPOINT': 'https://changed.example.invalid'}
        manifest = {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'voice-domain-config', 'namespace': NS}, 'data': row['objects'][0]['desired']['data']}
        plan['actions'] = [{'id': 'config', 'row': 'domains', 'manifest': manifest}]
        binding = subject.preflight(kube, plan, 'full')
        self.assertTrue(subject.revalidate(kube, binding))
        manifest['metadata']['name'] = 'voice-nats-config'
        kube.calls.clear()
        with self.assertRaises(subject.PlanError): subject.preflight(kube, plan, 'full')
        self.assertEqual(kube.calls, [])

    def test_conflicting_row_targets_veto_before_read(self):
        kube, plan = self.full_fixture()
        row = next(row for row in plan['checks'] if row['id'] == 'domains')
        row['disposition'] = 'action'
        row['objects'][0]['action_id'] = 'config'
        row['objects'][0]['desired']['data'] = {'S3_SIGNING_ENDPOINT': 'https://changed.example.invalid'}
        plan['actions'] = [{'id': 'config', 'row': 'domains', 'manifest': {
            'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'voice-app-config', 'namespace': NS},
            'data': row['objects'][0]['desired']['data']}}]
        with self.assertRaises(subject.PlanError): subject.preflight(kube, plan, 'full')
        self.assertEqual(kube.calls, [])

    def test_auth_mail_cannot_be_skipped(self):
        kube, plan = fixture()
        plan['checks'][0] = {'id': 'auth-mail', 'disposition': 'skip', 'objects': [], 'enabled': False, 'reason': 'not-requested'}
        with self.assertRaises(subject.PlanError): subject.preflight(kube, plan, 'images-only')

    def test_supplied_secret_values_rejected_without_echo(self):
        kube, plan = fixture()
        plan['checks'][0]['objects'][0]['desired']['stringData'] = {'AUTH_RESEND_API_KEY': 'private-new-token'}
        with self.assertRaises(subject.PlanError) as caught: subject.preflight(kube, plan, 'images-only')
        self.assertNotIn('private-new-token', str(caught.exception))

    def test_valid_binding_is_private_and_revalidates(self):
        kube, plan = fixture()
        binding = subject.preflight(kube, plan, 'images-only')
        self.assertEqual(len(binding['objects']), 2)
        self.assertNotIn('private-mail-token', json.dumps(binding))
        self.assertNotIn(base64.b64encode(b'private-mail-token').decode(), json.dumps(binding))
        self.assertTrue(subject.revalidate(kube, binding))
    def test_missing_mail_key(self):
        kube, plan = fixture()
        del kube.objects[('secret', 'voice-app-secrets')]['data']['AUTH_RESEND_API_KEY']
        with self.assertRaises(subject.PlanError): subject.preflight(kube, plan, 'images-only')
    def test_invalid_sender(self):
        kube, plan = fixture()
        kube.objects[('secret', 'voice-app-secrets')]['data']['AUTH_RESEND_FROM'] = base64.b64encode(b'not-an-address').decode()
        with self.assertRaises(subject.PlanError): subject.preflight(kube, plan, 'images-only')
    def test_semantic_mismatch_same_identity(self):
        kube, plan = fixture()
        plan['checks'][1]['objects'][0]['desired']['data'] = {'S3_SIGNING_ENDPOINT': 'https://changed.example.invalid'}
        with self.assertRaises(subject.PlanError): subject.preflight(kube, plan, 'images-only')
    def test_revalidation_rejects_uid_reuse_and_rv_drift(self):
        for field in ('uid', 'resourceVersion'):
            kube, plan = fixture()
            binding = subject.preflight(kube, plan, 'images-only')
            kube.objects[('secret', 'voice-app-secrets')]['metadata'][field] = 'changed'
            with self.assertRaises(subject.PlanError): subject.revalidate(kube, binding)
    def test_revalidation_rejects_semantic_change_even_same_rv(self):
        kube, plan = fixture()
        binding = subject.preflight(kube, plan, 'images-only')
        kube.objects[('configmap', 'voice-app-config')]['data']['S3_SIGNING_ENDPOINT'] = 'changed'
        with self.assertRaises(subject.PlanError): subject.revalidate(kube, binding)
    def test_unsupported_action_veto_before_live_read(self):
        kube, plan = fixture()
        plan['actions'] = [{'id': 'reseed', 'row': 'signing-endpoint', 'manifest': {'kind': 'Secret', 'metadata': {'name': 'voice-nats-auth'}}}]
        with self.assertRaises(subject.PlanError): subject.preflight(kube, plan, 'images-only')
        self.assertEqual(kube.calls, [])
    def test_full_requires_all_sixteen_rows(self):
        kube, plan = fixture()
        plan['mode'] = 'full'
        with self.assertRaises(subject.PlanError): subject.preflight(kube, plan, 'full')

if __name__ == '__main__': unittest.main()
