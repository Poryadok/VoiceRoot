"""Contract tests for the fixed HTTPS Kubernetes API adapter."""
import json
import base64
import os
import ssl
import hashlib
import copy
import socket
import threading
import tempfile
import unittest
from pathlib import Path
from urllib.error import HTTPError
from unittest.mock import patch

import kube_json_api as kube
import census_stream
import verification_job


def jwt(audience, expiry):
    encode = lambda value: base64.urlsafe_b64encode(json.dumps(value, separators=(',', ':')).encode()).decode().rstrip('=')
    return encode({'alg': 'none'}) + '.' + encode({'aud': audience, 'exp': expiry,
        'sub': 'system:serviceaccount:voice-staging:voice-nats-staging-runner'}) + '.signature'


class Response:
    def __init__(self, body, status=200):
        self.body = body if isinstance(body, bytes) else json.dumps(body).encode()
        self.status = status

    def __enter__(self): return self
    def __exit__(self, *_): return None
    def read(self, limit=-1): return self.body[:limit]
    def getcode(self): return self.status


class TokenFileTests(unittest.TestCase):
    def test_requires_private_runner_owned_file_and_live_fixed_audience(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'token'
            token = jwt('voice-staging-runner', 1600)
            path.write_text(token); path.chmod(0o600)
            source = kube.TokenFile(path, audience='voice-staging-runner', owner_uid=os.geteuid(), clock=lambda: 1000)
            self.assertEqual(source.read(minimum_seconds=300), token)
            with self.assertRaises(kube.KubeAPIError):
                source.read(minimum_seconds=601)
            path.write_text(jwt('other-audience', 1600))
            with self.assertRaises(kube.KubeAPIError):
                source.read(minimum_seconds=1)

    def test_no_symlink_or_group_access_on_credential(self):
        with tempfile.TemporaryDirectory() as directory:
            real = Path(directory) / 'real'; real.write_text(jwt('voice-staging-runner', 1600)); real.chmod(0o600)
            link = Path(directory) / 'link'; link.symlink_to(real)
            with self.assertRaises(kube.KubeAPIError):
                kube.TokenFile(link, audience='voice-staging-runner').read()
            real.chmod(0o640)
            with self.assertRaises(kube.KubeAPIError):
                kube.TokenFile(real, audience='voice-staging-runner').read()


class KubernetesJSONAdapterTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.ca = self.root / 'ca.pem'
        self.ca.write_bytes(Path(ssl.get_default_verify_paths().cafile).read_bytes()); self.ca.chmod(0o444)
        self.token = self.root / 'token'; self.token.write_text(jwt('voice-staging-runner', 9999999999)); self.token.chmod(0o600)
        self.calls = []
        self.deleted = False
        def opener(request, *, context, timeout):
            self.calls.append((request, context, timeout))
            if request.get_method() == 'DELETE':
                self.deleted = True
                return Response({'status': 'Success'})
            if self.deleted and request.full_url.endswith('/jobs/job'):
                raise HTTPError(request.full_url, 404, 'Not Found', {}, None)
            if '/pods?' in request.full_url and 'labelSelector=' in request.full_url:
                return Response({'items': [{'metadata': {'name': 'pod', 'ownerReferences': [
                    {'kind': 'Job', 'name': 'job', 'uid': '33333333-3333-4333-8333-333333333333', 'controller': True}]}}]})
            if '/pods/pod/log?' in request.full_url:
                return Response(b'{"phase":"census"}\n')
            return Response({'items': []})
        self.opener = opener
        self.api = kube.KubernetesJSONAPI('https://10.0.0.1:6443', self.ca, self.token,
            audience='voice-staging-runner', owner_uid=os.geteuid(), trusted_config_uid=os.geteuid(), opener=opener)

    def tearDown(self): self.temp.cleanup()

    def test_resource_methods_use_only_fixed_https_origins_and_encoded_names(self):
        with patch.object(self.api.tokens, 'read', return_value='opaque-test-token'):
            self.api.get_pvc('voice-staging', 'claim.a')
            self.api.get_pv('pv/a')
            self.api.get_node('node-1')
            self.api.get_job_pod('voice-staging', 'job')
        urls = [call[0].full_url for call in self.calls]
        self.assertEqual(urls[0], 'https://10.0.0.1:6443/api/v1/namespaces/voice-staging/persistentvolumeclaims/claim.a')
        self.assertEqual(urls[1], 'https://10.0.0.1:6443/api/v1/persistentvolumes/pv%2Fa')
        self.assertEqual(urls[2], 'https://10.0.0.1:6443/api/v1/nodes/node-1')
        self.assertIn('/api/v1/namespaces/voice-staging/pods?', urls[3])
        self.assertTrue(all(call[0].get_header('Authorization') == 'Bearer opaque-test-token' for call in self.calls))
        self.assertTrue(all(call[0].get_header('Accept') == 'application/json' for call in self.calls))
        self.assertTrue(all(call[1].check_hostname and call[1].verify_mode for call in self.calls))

    def test_native_stage_gets_are_namespaced_and_fixed_kind(self):
        with patch.object(self.api, '_request', return_value={'metadata': {}}) as get:
            self.api.get_service('voice-staging', 'voice-nats')
            self.api.get_pod('voice-staging', 'voice-nats-pvc-candidate-0')
            self.api.get_replicaset('voice-staging', 'voice-nats-pvc-candidate-abc')
            self.api.get_deployment('voice-staging', 'voice-gateway')
        paths = [call.args[1] for call in get.call_args_list]
        self.assertEqual(paths, [
            '/api/v1/namespaces/voice-staging/services/voice-nats',
            '/api/v1/namespaces/voice-staging/pods/voice-nats-pvc-candidate-0',
            '/apis/apps/v1/namespaces/voice-staging/replicasets/voice-nats-pvc-candidate-abc',
            '/apis/apps/v1/namespaces/voice-staging/deployments/voice-gateway'])
        for method, args in ((self.api.get_service, ('default', 'voice-nats')),
                             (self.api.get_pod, ('voice-staging', '../escape')),
                             (self.api.get_replicaset, ('voice-staging', '')),
                             (self.api.get_deployment, ('default', 'voice-gateway'))):
            with self.subTest(args=args), patch.object(self.api, '_request') as request, \
                 self.assertRaises(kube.KubeAPIError):
                method(*args)
            request.assert_not_called()

    def test_native_stage_lists_are_closed_and_fail_on_pagination(self):
        with patch.object(self.api, '_request', return_value={'items': []}) as get:
            self.assertEqual(self.api.list_fence_objects('voice-staging'), {'pods': [], 'replicasets': []})
            self.assertEqual(self.api.list_control_objects('voice-staging'), {
                'deployments': [], 'horizontalpodautoscalers': [], 'jobs': []})
        self.assertEqual(len(get.call_args_list), 5)
        paths = [call.args[1] for call in get.call_args_list]
        self.assertIn('/api/v1/namespaces/voice-staging/pods?', paths[0])
        self.assertIn('/apis/apps/v1/namespaces/voice-staging/replicasets?', paths[1])
        self.assertIn('/apis/apps/v1/namespaces/voice-staging/deployments?', paths[2])
        self.assertIn('/apis/autoscaling/v2/namespaces/voice-staging/horizontalpodautoscalers?', paths[3])
        self.assertIn('/apis/batch/v1/namespaces/voice-staging/jobs?', paths[4])
        with patch.object(self.api, '_request', return_value={'items': [], 'metadata': {'continue': 'next'}}):
            with self.assertRaises(kube.KubeAPIError):
                self.api.list_control_objects('voice-staging')

    def test_hub_pod_list_uses_only_the_fixed_selector_and_rejects_truncation(self):
        item = {'metadata': {'name': 'voice-nats-pvc-candidate-0'}}
        with patch.object(self.api, '_request', return_value={'items': [item]}) as get:
            self.assertEqual(self.api.list_hub_pods('voice-staging', {'app': 'voice-nats-pvc-candidate'}), [item])
        self.assertIn('labelSelector=app%3Dvoice-nats-pvc-candidate', get.call_args.args[1])
        self.assertIn('limit=2048', get.call_args.args[1])
        for args in (('default', {'app': 'voice-nats-pvc-candidate'}),
                     ('voice-staging', {'app': 'other'}),
                     ('voice-staging', {'app': 'voice-nats-pvc-candidate', 'role': 'hub'})):
            with self.subTest(args=args), patch.object(self.api, '_request') as request, \
                 self.assertRaises(kube.KubeAPIError):
                self.api.list_hub_pods(*args)
            request.assert_not_called()
        with patch.object(self.api, '_request', return_value={'items': [], 'metadata': {'continue': 'next'}}):
            with self.assertRaises(kube.KubeAPIError):
                self.api.list_hub_pods('voice-staging', {'app': 'voice-nats-pvc-candidate'})

    def test_hub_jsz_uses_exact_pod_uid_fixed_command_and_bounded_stdout(self):
        pod_uid = '44444444-4444-4444-8444-444444444444'
        pod = {'metadata': {'uid': pod_uid, 'namespace': 'voice-staging'}}
        with patch.object(self.api, 'get_pod', return_value=pod), \
             patch.object(self.api.tokens, 'read', return_value='opaque-test-token'), \
             patch.object(kube, '_websocket_exec', return_value=iter((b'{"account":', b'"A"}'))) as execute:
            self.assertEqual(self.api.exec_hub_jsz('voice-staging', 'voice-nats-pvc-candidate-0', pod_uid, 'nats'),
                b'{"account":"A"}')
        path, token = execute.call_args.args[1:]
        self.assertEqual(token, 'opaque-test-token')
        from urllib.parse import parse_qs, urlsplit
        query = parse_qs(urlsplit(path).query)
        self.assertEqual(query['container'], ['nats'])
        self.assertEqual(query['stdin'], ['false'])
        self.assertEqual(query['stdout'], ['true'])
        self.assertEqual(query['stderr'], ['false'])
        self.assertEqual(query['tty'], ['false'])
        self.assertEqual(query['command'], ['/bin/busybox', 'wget', '-q', '-O', '-',
            'http://127.0.0.1:8222/jsz?accounts=true&streams=true&consumers=true&config=true&limit=2048'])
        self.assertEqual(execute.call_args.kwargs['max_bytes'], 64 << 20)
        with patch.object(self.api, 'get_pod', return_value={'metadata': {'uid': '55555555-5555-4555-8555-555555555555', 'namespace': 'voice-staging'}}), \
             patch.object(self.api, '_operation_token') as token_read, \
             self.assertRaises(kube.KubeAPIError):
            self.api.exec_hub_jsz('voice-staging', 'voice-nats-pvc-candidate-0', pod_uid, 'nats')
        token_read.assert_not_called()

    def test_owned_pod_finalizer_cas_tests_uid_resource_version_and_exact_list(self):
        uid = '44444444-4444-4444-8444-444444444444'
        current = {'metadata': {'uid': uid, 'resourceVersion': '12', 'finalizers': ['voice.io/old']}}
        with patch.object(self.api, 'get_pod', return_value=current), \
             patch.object(self.api, '_request', return_value={'kind': 'Pod'}) as patch_request:
            result = self.api.add_owned_pod_finalizer('voice-staging', 'hub-0', uid, '12',
                ['voice.io/old'], 'voice.io/rollout')
        self.assertEqual(result, {'kind': 'Pod'})
        method, path, patch_doc = patch_request.call_args.args
        self.assertEqual((method, path), ('PATCH', '/api/v1/namespaces/voice-staging/pods/hub-0'))
        self.assertEqual(patch_doc[:3], [
            {'op': 'test', 'path': '/metadata/uid', 'value': uid},
            {'op': 'test', 'path': '/metadata/resourceVersion', 'value': '12'},
            {'op': 'test', 'path': '/metadata/finalizers', 'value': ['voice.io/old']}])
        self.assertEqual(patch_doc[3], {'op': 'replace', 'path': '/metadata/finalizers',
            'value': ['voice.io/old', 'voice.io/rollout']})
        self.assertEqual(patch_request.call_args.kwargs['content_type'], 'application/json-patch+json')
        with patch.object(self.api, 'get_pod', return_value={
                'metadata': {'uid': uid, 'resourceVersion': '13', 'finalizers': ['voice.io/old']}}), \
             patch.object(self.api, '_request') as request, self.assertRaises(kube.KubeAPIError):
            self.api.add_owned_pod_finalizer('voice-staging', 'hub-0', uid, '12',
                ['voice.io/old'], 'voice.io/rollout')
        request.assert_not_called()

    def test_deployment_cas_binds_uid_rv_replicas_and_full_template_digest(self):
        uid = '44444444-4444-4444-8444-444444444444'
        template = {'metadata': {'labels': {'app': 'hub'}}, 'spec': {'volumes': [
            {'name': 'jsdata', 'persistentVolumeClaim': {'claimName': 'old'}}]}}
        digest = hashlib.sha256(json.dumps(template, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
        current = {'metadata': {'uid': uid, 'resourceVersion': '12'},
            'spec': {'replicas': 1, 'template': template}}
        replacement = copy.deepcopy(template)
        replacement['spec']['volumes'][0]['persistentVolumeClaim']['claimName'] = 'new'
        with patch.object(self.api, 'get_deployment', return_value=current), \
             patch.object(self.api, '_request', return_value={'kind': 'Deployment'}) as patch_request:
            self.api.cas_rollout_deployment('voice-staging', 'voice-nats-pvc-candidate', uid, '12', 1, digest,
                new_replicas=0, new_template=replacement, expected_jsdata_claim='old', new_jsdata_claim='new')
        method, path, patch_doc = patch_request.call_args.args
        self.assertEqual((method, path), ('PATCH', '/apis/apps/v1/namespaces/voice-staging/deployments/voice-nats-pvc-candidate'))
        self.assertEqual(patch_doc[:3], [
            {'op': 'test', 'path': '/metadata/uid', 'value': uid},
            {'op': 'test', 'path': '/metadata/resourceVersion', 'value': '12'},
            {'op': 'test', 'path': '/spec/replicas', 'value': 1}])
        self.assertEqual(patch_doc[3], {'op': 'replace', 'path': '/spec/replicas', 'value': 0})
        self.assertEqual(patch_doc[4]['path'], '/spec/template')
        self.assertEqual(patch_doc[4]['value'], replacement)
        with patch.object(self.api, 'get_deployment', return_value=current), \
             patch.object(self.api, '_request') as request, self.assertRaises(kube.KubeAPIError):
            self.api.cas_rollout_deployment('voice-staging', 'voice-nats-pvc-candidate', uid, '12', 1, 'f' * 64,
                new_replicas=0)
        request.assert_not_called()

    def test_rollout_marker_cas_is_closed_and_preserves_claim_identity(self):
        uid = '44444444-4444-4444-8444-444444444444'
        operation = '0123456789ab'
        claim = 'voice-nats-jsdata-d20261017a'
        marker = {'metadata': {'uid': uid, 'resourceVersion': '12'},
            'data': {'phase': 'active', 'dataPVC': claim, 'generation': 'r20261017a'}}
        with patch.object(self.api, 'get_configmap', return_value=marker), \
             patch.object(self.api, '_request', return_value={'kind': 'ConfigMap'}) as patch_request:
            self.api.cas_rollout_marker('voice-nats-generation', operation, uid, '12',
                'active', 'rollout-capturing', expected_claim=claim)
        method, path, patch_doc = patch_request.call_args.args
        self.assertEqual((method, path), ('PATCH',
            '/api/v1/namespaces/voice-staging/configmaps/voice-nats-generation'))
        self.assertEqual(patch_doc[:4], [
            {'op': 'test', 'path': '/metadata/uid', 'value': uid},
            {'op': 'test', 'path': '/metadata/resourceVersion', 'value': '12'},
            {'op': 'test', 'path': '/data/phase', 'value': 'active'},
            {'op': 'test', 'path': '/data/dataPVC', 'value': claim}])
        self.assertEqual(patch_doc[4:], [
            {'op': 'replace', 'path': '/data/phase', 'value': 'rollout-capturing'},
            {'op': 'add', 'path': '/data/knownRolloutOperation', 'value': operation}])
        self.assertEqual(patch_request.call_args.kwargs['content_type'], 'application/json-patch+json')

        owned = {'metadata': {'uid': uid, 'resourceVersion': '13'},
            'data': {'phase': 'rollout-verified', 'dataPVC': claim,
                'knownRolloutOperation': operation, 'generation': 'r20261017a'}}
        with patch.object(self.api, 'get_configmap', return_value=owned), \
             patch.object(self.api, '_request', return_value={'kind': 'ConfigMap'}) as patch_request:
            self.api.cas_rollout_marker('voice-nats-generation', operation, uid, '13',
                'rollout-verified', 'active', expected_claim=claim, release=True)
        patch_doc = patch_request.call_args.args[2]
        self.assertEqual(patch_doc[-2:], [
            {'op': 'replace', 'path': '/data/phase', 'value': 'active'},
            {'op': 'remove', 'path': '/data/knownRolloutOperation'}])

        for phase, target, release, data in (
            ('active', 'rollout-prepared', False, marker['data']),
            ('rollout-prepared', 'rollout-verified', False,
                {'phase': 'rollout-prepared', 'dataPVC': claim, 'knownRolloutOperation': 'f' * 12}),
        ):
            with self.subTest(phase=phase), \
                 patch.object(self.api, 'get_configmap', return_value={
                     'metadata': {'uid': uid, 'resourceVersion': '12'}, 'data': data}), \
                 patch.object(self.api, '_request') as request, self.assertRaises(kube.KubeAPIError):
                self.api.cas_rollout_marker('voice-nats-generation', operation, uid, '12',
                    phase, target, expected_claim=claim, release=release)
            request.assert_not_called()

    def test_user_bootstrap_template_install_and_restore_are_exact(self):
        uid = '44444444-4444-4444-8444-444444444444'
        operation = '0123456789ab'
        original = {'metadata': {'labels': {'app': 'voice-user'}}, 'spec': {'containers': [
            {'name': 'user', 'image': 'example.invalid/user@sha256:' + 'a' * 64,
             'env': [{'name': 'A', 'value': 'B'}]},
            {'name': 'metrics', 'image': 'example.invalid/metrics@sha256:' + 'b' * 64}]}}
        digest = lambda value: hashlib.sha256(json.dumps(value, sort_keys=True,
            separators=(',', ':')).encode()).hexdigest()
        with patch.object(self.api, 'get_configmap', return_value={'data': {'SPACE_GRPC_ADDR': 'voice-space:9090'}}):
            self.api.get_configmap('voice-staging', 'voice-app-config')
        current = {'metadata': {'uid': uid, 'resourceVersion': '12'},
            'spec': {'replicas': 0, 'template': original}}
        with patch.object(self.api, 'get_deployment', return_value=current), \
             patch.object(self.api, '_request', return_value={'kind': 'Deployment'}) as patch_request:
            self.api.cas_user_bootstrap_template('voice-staging', 'voice-user', uid, '12', 0,
                digest(original), operation=operation, action='install')
        installed = patch_request.call_args.args[2][-1]['value']
        self.assertEqual(installed['metadata']['annotations']['voice.io/nats-user-space-bootstrap'], operation)
        self.assertEqual(installed['spec']['containers'][0]['env'][-1],
            {'name': 'SPACE_GRPC_ADDR', 'value': ''})

        current['metadata']['resourceVersion'] = '13'
        current['spec']['template'] = installed
        with patch.object(self.api, 'get_deployment', return_value=current), \
             patch.object(self.api, '_request', return_value={'kind': 'Deployment'}) as patch_request:
            self.api.cas_user_bootstrap_template('voice-staging', 'voice-user', uid, '13', 0,
                digest(installed), operation=operation, original_template=original, action='restore')
        self.assertEqual(patch_request.call_args.args[2][-1]['value'], original)

        altered = copy.deepcopy(installed)
        altered['spec']['containers'][0]['env'][0]['value'] = 'changed'
        current['metadata']['resourceVersion'] = '14'
        current['spec']['template'] = altered
        with patch.object(self.api, 'get_deployment', return_value=current), \
             patch.object(self.api, '_request') as request, self.assertRaises(kube.KubeAPIError):
            self.api.cas_user_bootstrap_template('voice-staging', 'voice-user', uid, '14', 0,
                digest(altered), operation=operation, original_template=original, action='restore')
        request.assert_not_called()

    def test_capture_configmaps_are_fixed_to_canonical_bootstrap_roles(self):
        with patch.object(self.api, '_request', return_value={'kind': 'ConfigMap'}) as request:
            self.api.get_configmap('voice-staging', 'voice-nats-analytics-chat-bootstrap')
        self.assertIn('/configmaps/voice-nats-analytics-chat-bootstrap', request.call_args.args[1])
        for name in ('voice-nats-unknown-bootstrap', 'voice-nats-generation-extra', 'voice-app-config-secret'):
            with self.subTest(name=name), self.assertRaises(kube.KubeAPIError):
                self.api.get_configmap('voice-staging', name)

    def test_generation_secret_reads_are_fixed_and_tls_metadata_drops_secret_data(self):
        secret = {'metadata': {'name': 'voice-nats-hub-tls-r20261017a',
            'namespace': 'voice-staging', 'uid': '11111111-1111-4111-8111-111111111111',
            'resourceVersion': '42'}, 'data': {'tls.key': 'private'}}
        operator = copy.deepcopy(secret)
        operator['metadata']['name'] = 'voice-nats-operator-r20261017a'
        with patch.object(self.api, '_request', side_effect=[operator, secret]) as request:
            row = self.api.get_secret('voice-staging', 'voice-nats-operator-r20261017a')
            metadata = self.api.get_secret_metadata('voice-staging', 'voice-nats-hub-tls-r20261017a')
        self.assertEqual(row, operator)
        self.assertNotIn('data', metadata)
        self.assertEqual(metadata['uid'], secret['metadata']['uid'])
        self.assertEqual(request.call_count, 2)
        for namespace, name in (('default', 'voice-nats-operator-r20261017a'),
                ('voice-staging', 'other-secret'),
                ('voice-staging', 'voice-nats-hub-tls-r20261017a')):
            with self.subTest(namespace=namespace, name=name), self.assertRaises(kube.KubeAPIError):
                self.api.get_secret(namespace, name)

    def test_job_create_read_logs_and_uid_precondition_delete_are_json_contracts(self):
        payload = {'apiVersion': 'batch/v1', 'kind': 'Job', 'metadata': {'name': 'job'}}
        with patch.object(self.api.tokens, 'read', return_value='opaque-test-token'):
            self.api.create_job('voice-staging', payload)
            self.api.get_job('voice-staging', 'job')
            self.api.pod_logs('voice-staging', 'pod')
            self.api.get_job_pod('voice-staging', 'job')
            self.api.delete_job('voice-staging', 'job', '33333333-3333-4333-8333-333333333333')
        request = next(call[0] for call in self.calls if call[0].get_method() == 'DELETE')
        self.assertEqual(request.get_method(), 'DELETE')
        self.assertEqual(json.loads(request.data), {'apiVersion': 'v1', 'kind': 'DeleteOptions', 'preconditions': {'uid': '33333333-3333-4333-8333-333333333333'}})
        self.assertTrue(request.full_url.endswith('/apis/batch/v1/namespaces/voice-staging/jobs/job'))

    def test_verification_claim_create_has_closed_operation_scoped_rwo_spec(self):
        with patch.object(self.api, '_request', return_value={'kind': 'PersistentVolumeClaim'}) as post:
            result = self.api.create_verification_pvc('a' * 12, 'b' * 32,
                48 << 30, 'local-path')
        self.assertEqual(result, {'kind': 'PersistentVolumeClaim'})
        method, path, body = post.call_args.args
        self.assertEqual(method, 'POST')
        self.assertEqual(path, '/api/v1/namespaces/voice-staging/persistentvolumeclaims')
        self.assertEqual(body, {
            'apiVersion': 'v1', 'kind': 'PersistentVolumeClaim',
            'metadata': {'name': 'voice-nats-verify-' + 'a' * 12, 'namespace': 'voice-staging',
                'labels': {'voice.io/nats-operation': 'a' * 12,
                    'voice.io/nats-job-nonce': 'b' * 32,
                    'voice.io/nats-pvc-role': 'verification'}},
            'spec': {'accessModes': ['ReadWriteOnce'], 'volumeMode': 'Filesystem',
                'storageClassName': 'local-path',
                'resources': {'requests': {'storage': str(48 << 30)}}},
        })
        self.assertNotIn('volumeName', body['spec'])
        self.assertNotIn('selector', body['spec'])

    def test_verification_claim_create_rejects_invalid_or_untrusted_configuration(self):
        for operation, nonce, size, storage_class in (
            ('../escape', 'b' * 32, 1, 'local-path'),
            ('a' * 12, 'short', 1, 'local-path'),
            ('a' * 12, 'b' * 32, True, 'local-path'),
            ('a' * 12, 'b' * 32, 0, 'local-path'),
            ('a' * 12, 'b' * 32, 1 << 63, 'local-path'),
            ('a' * 12, 'b' * 32, 1, ''),
            ('a' * 12, 'b' * 32, 1, 'local..path'),
        ):
            with self.subTest(storage_class=storage_class, size=size), \
                 patch.object(self.api, '_request') as post, \
                 self.assertRaises(kube.KubeAPIError):
                self.api.create_verification_pvc(operation, nonce, size, storage_class)
            post.assert_not_called()

    def test_verification_exec_uses_only_producer_vector_and_live_pod_uid(self):
        request = {'operation': 'a' * 12, 'nonce': 'b' * 32, 'phase': 'census'}
        binding = {
            'source_claim': 'voice-nats-jsdata-d20261017a',
            'source_uid': '11111111-1111-4111-8111-111111111111',
            'source_pv_name': 'pv-voice-nats-source',
            'source_pv_uid': '55555555-5555-4555-8555-555555555555',
            'source_node_name': 'k3s-worker-1',
            'verification_claim': 'voice-nats-verify-' + 'a' * 12,
            'verification_uid': '22222222-2222-4222-8222-222222222222',
            'verification_pv_name': 'pv-voice-nats-verify',
            'verification_pv_uid': '33333333-3333-4333-8333-333333333333',
            'verification_node_name': 'k3s-worker-1',
        }
        pod_uid = '44444444-4444-4444-8444-444444444444'
        pod = {'metadata': {'uid': pod_uid, 'namespace': 'voice-staging'}}
        archive = {'sha256': 'a' * 64, 'bytes': 123}
        session_inputs = {'attempt': 'd' * 8, 'sha256': 'e' * 64, 'bytes': 456}
        with patch.object(self.api.tokens, 'read', return_value='opaque-test-token'), \
             patch.object(self.api, '_request', return_value=pod), \
             patch.object(kube, '_websocket_exec', return_value=iter((b'receipt',))) as execute:
            result = list(self.api.exec_pod_verification('voice-staging', 'pod', pod_uid,
                request, binding, 'census_same_pod_readback', archive=archive,
                session_inputs=session_inputs))
        self.assertEqual(result, [b'receipt'])
        path, token = execute.call_args.args[1:]
        self.assertEqual(token, 'opaque-test-token')
        from urllib.parse import parse_qs, urlsplit
        query = parse_qs(urlsplit(path).query)
        self.assertEqual(query['container'], ['nats-phase'])
        self.assertEqual(query['stdin'], ['false'])
        self.assertEqual(query['stdout'], ['true'])
        self.assertEqual(query['stderr'], ['false'])
        self.assertEqual(query['tty'], ['false'])
        self.assertEqual(query['command'], verification_job.exec_vectors(
            request, pod_uid, binding, archive=archive,
            session_inputs=session_inputs)['census_same_pod_readback'])
        self.assertEqual(execute.call_args.kwargs['max_bytes'], census_stream.MAX_TRANSPORT_BYTES)

    def test_census_input_duplex_streams_bounded_file_envelope_and_half_closes(self):
        archive = (self.root / 'archive.tar')
        archive_bytes = bytes(range(251)) * 1100
        archive.write_bytes(archive_bytes); archive.chmod(0o600)
        manifest = b'{"files":[],"schema":"fixture"}'
        digest = hashlib.sha256(archive_bytes).hexdigest()
        client, server = socket.socketpair()
        # Keep a protocol regression from hanging this private fixture forever.
        client.settimeout(10)
        server.settimeout(10)
        received = bytearray()
        largest = 0
        failure = []

        def read_exact(sock, size):
            output = bytearray()
            while len(output) < size:
                chunk = sock.recv(size - len(output))
                if not chunk: raise EOFError('socket closed')
                output.extend(chunk)
            return bytes(output)

        def serve():
            nonlocal largest
            try:
                header = bytearray()
                while not header.endswith(b'\r\n\r\n'):
                    header.extend(server.recv(4096))
                key = next(line.split(b':', 1)[1].strip().decode()
                    for line in header.split(b'\r\n') if line.lower().startswith(b'sec-websocket-key:'))
                accept = base64.b64encode(hashlib.sha1((key +
                    '258EAFA5-E914-47DA-95CA-C5AB0DC85B11').encode()).digest())
                server.sendall(b'HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\n'
                    b'Connection: Upgrade\r\nSec-WebSocket-Protocol: v5.channel.k8s.io\r\n'
                    b'Sec-WebSocket-Accept: ' + accept + b'\r\n\r\n')
                while True:
                    first = read_exact(server, 2)
                    size = first[1] & 0x7f
                    if size == 126: size = int.from_bytes(read_exact(server, 2), 'big')
                    elif size == 127: size = int.from_bytes(read_exact(server, 8), 'big')
                    mask = read_exact(server, 4)
                    payload = read_exact(server, size)
                    payload = bytes(value ^ mask[i % 4] for i, value in enumerate(payload))
                    largest = max(largest, len(payload))
                    if payload == b'\xff\x00': break
                    if not payload or payload[0] != 0: raise AssertionError('not stdin channel')
                    received.extend(payload[1:])
                status = b'\x03' + json.dumps({'status': 'Success'}).encode()
                server.sendall(bytes((0x82, len(status))) + status + b'\x88\x00')
            except BaseException as error:
                failure.append(error)
            finally:
                server.close()

        worker = threading.Thread(target=serve, daemon=True); worker.start()
        fake_context = type('Context', (), {'wrap_socket': staticmethod(lambda raw, server_hostname: raw)})()
        self.api.context = fake_context
        source_fd = kube._open_verified_archive(archive, len(archive_bytes), digest)
        try:
            with patch.object(kube.socket, 'create_connection', return_value=client):
                kube._websocket_exec_input(self.api, '/exec?stdin=true', 'opaque-token',
                    source_fd, manifest, archive_bytes=len(archive_bytes), archive_sha256=digest)
        finally:
            os.close(source_fd)
        worker.join(timeout=5)
        self.assertFalse(worker.is_alive())
        self.assertEqual(failure, [])
        expected = (kube.CENSUS_INPUT_MAGIC + len(manifest).to_bytes(4, 'big') + manifest
            + len(archive_bytes).to_bytes(8, 'big') + archive_bytes)
        self.assertEqual(bytes(received), expected)
        self.assertLessEqual(largest, kube.MAX_EXEC_STDIN_CHUNK + 1)

    def test_verification_inputs_use_raw_bounded_json_stdin(self):
        bundle = b'{"files":{},"nonce":"' + b'b' * 32 + b'","operation":"' + b'a' * 12 + b'","pod_uid":"44444444-4444-4444-8444-444444444444","schema":"voice-nats-verification-inputs-v1"}'
        archive = (self.root / 'input-archive.tar')
        archive.write_bytes(bundle); archive.chmod(0o600)
        digest = hashlib.sha256(bundle).hexdigest()
        client, server = socket.socketpair()
        client.settimeout(10); server.settimeout(10)
        received = bytearray(); failure = []

        def read_exact(sock, size):
            output = bytearray()
            while len(output) < size:
                chunk = sock.recv(size - len(output))
                if not chunk: raise EOFError('socket closed')
                output.extend(chunk)
            return bytes(output)

        def serve():
            try:
                header = bytearray()
                while not header.endswith(b'\r\n\r\n'):
                    header.extend(server.recv(4096))
                key = next(line.split(b':', 1)[1].strip().decode()
                    for line in header.split(b'\r\n') if line.lower().startswith(b'sec-websocket-key:'))
                accept = base64.b64encode(hashlib.sha1((key +
                    '258EAFA5-E914-47DA-95CA-C5AB0DC85B11').encode()).digest())
                server.sendall(b'HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\n'
                    b'Connection: Upgrade\r\nSec-WebSocket-Protocol: v5.channel.k8s.io\r\n'
                    b'Sec-WebSocket-Accept: ' + accept + b'\r\n\r\n')
                while True:
                    first = read_exact(server, 2)
                    size = first[1] & 0x7f
                    if size == 126: size = int.from_bytes(read_exact(server, 2), 'big')
                    elif size == 127: size = int.from_bytes(read_exact(server, 8), 'big')
                    mask = read_exact(server, 4)
                    payload = read_exact(server, size)
                    payload = bytes(value ^ mask[i % 4] for i, value in enumerate(payload))
                    if payload == b'\xff\x00': break
                    if not payload or payload[0] != 0: raise AssertionError('not stdin channel')
                    received.extend(payload[1:])
                status = b'\x03' + json.dumps({'status': 'Success'}).encode()
                server.sendall(bytes((0x82, len(status))) + status + b'\x88\x00')
            except BaseException as error:
                failure.append(error)
            finally:
                server.close()

        worker = threading.Thread(target=serve, daemon=True); worker.start()
        fake_context = type('Context', (), {'wrap_socket': staticmethod(lambda raw, server_hostname: raw)})()
        self.api.context = fake_context
        source_fd = kube._open_verified_archive(archive, len(bundle), digest)
        try:
            with patch.object(kube.socket, 'create_connection', return_value=client):
                kube._websocket_exec_input(self.api, '/exec?stdin=true', 'opaque-token',
                    source_fd, b'', archive_bytes=len(bundle), archive_sha256=digest, raw_stdin=True)
        finally:
            os.close(source_fd)
        worker.join(timeout=5)
        self.assertFalse(worker.is_alive())
        self.assertEqual(failure, [])
        self.assertEqual(bytes(received), bundle)

    def test_verification_input_method_accepts_only_fixed_loader_vector(self):
        request = {'operation': 'a' * 12, 'nonce': 'b' * 32, 'phase': 'census'}
        binding = {
            'source_claim': 'voice-nats-jsdata-d20261017a',
            'source_uid': '11111111-1111-4111-8111-111111111111',
            'source_pv_name': 'pv-voice-nats-source',
            'source_pv_uid': '55555555-5555-4555-8555-555555555555',
            'source_node_name': 'k3s-worker-1',
            'verification_claim': 'voice-nats-verify-' + 'a' * 12,
            'verification_uid': '22222222-2222-4222-8222-222222222222',
            'verification_pv_name': 'pv-voice-nats-verify',
            'verification_pv_uid': '33333333-3333-4333-8333-333333333333',
            'verification_node_name': 'k3s-worker-1',
        }
        pod_uid = '44444444-4444-4444-8444-444444444444'
        files = {name: base64.b64encode(b'x').decode() for name in kube.VERIFICATION_INPUT_FILES}
        bundle = json.dumps({'schema': 'voice-nats-verification-inputs-v1', 'operation': request['operation'],
            'nonce': request['nonce'], 'pod_uid': pod_uid, 'files': files}, sort_keys=True,
            separators=(',', ':'), ensure_ascii=False).encode()
        archive_binding = {'sha256': 'c' * 64, 'bytes': 123}
        input_binding = {'attempt': 'd' * 8, 'sha256': hashlib.sha256(bundle).hexdigest(), 'bytes': len(bundle)}
        pod = {'metadata': {'uid': pod_uid, 'namespace': 'voice-staging'}}
        with patch.object(self.api, '_request', return_value=pod), \
             patch.object(self.api.tokens, 'read', return_value='opaque-token'), \
             patch.object(kube, '_websocket_exec_input') as execute:
            self.api.exec_pod_verification_input('voice-staging', 'pod', pod_uid, request, binding,
                archive_binding, input_binding, bundle)
        path, token = execute.call_args.args[1:3]
        self.assertEqual(token, 'opaque-token')
        self.assertTrue(path.startswith('/api/v1/namespaces/voice-staging/pods/pod/exec?'))
        from urllib.parse import parse_qs, urlsplit
        query = parse_qs(urlsplit(path).query)
        self.assertEqual(query['container'], ['nats-phase'])
        self.assertEqual(query['stdin'], ['true'])
        self.assertEqual(query['stdout'], ['true'])
        self.assertEqual(query['stderr'], ['false'])
        self.assertEqual(query['tty'], ['false'])
        self.assertEqual(query['command'], verification_job.exec_vectors(request, pod_uid, binding,
            archive=archive_binding, session_inputs=input_binding)['census_load_inputs'])
        self.assertEqual(execute.call_args.kwargs, {'archive_bytes': len(bundle),
            'archive_sha256': hashlib.sha256(bundle).hexdigest(), 'raw_stdin': True})

    def test_verification_input_rejects_bad_binding_before_api_io(self):
        request = {'operation': 'a' * 12, 'nonce': 'b' * 32, 'phase': 'census'}
        pod_uid = '44444444-4444-4444-8444-444444444444'
        bundle = b'{}'
        with patch.object(self.api, '_request') as get:
            with self.assertRaises(kube.KubeAPIError):
                self.api.exec_pod_verification_input('voice-staging', 'pod', pod_uid, request, {},
                    {'sha256': 'c' * 64, 'bytes': 123}, {'attempt': 'd' * 8, 'sha256': '0' * 64,
                    'bytes': len(bundle)}, bundle)
        get.assert_not_called()

    def test_census_input_manifest_must_be_canonical_duplicate_free_json(self):
        for raw in (b'{"z":1,"a":2}', b'{"a":1,"a":2}', b'{"x":NaN}'):
            with self.subTest(manifest=raw), self.assertRaises(kube.KubeAPIError):
                kube._validate_canonical_manifest(raw)
        self.assertIsNone(kube._validate_canonical_manifest(b'{"a":2,"z":1}'))

    def test_census_input_archive_requires_private_regular_file_and_exact_digest(self):
        archive = self.root / 'bad-archive.tar'
        archive.write_bytes(b'archive'); archive.chmod(0o600)
        with self.assertRaisesRegex(kube.KubeAPIError, 'census_archive_digest_mismatch'):
            kube._open_verified_archive(archive, 7, '0' * 64)
        archive.chmod(0o640)
        with self.assertRaisesRegex(kube.KubeAPIError, 'census_archive_custody_invalid'):
            kube._open_verified_archive(archive, 7, hashlib.sha256(b'archive').hexdigest())

    def test_read_verification_census_returns_only_validated_same_pod_census(self):
        request = {'operation': 'a' * 12, 'nonce': 'b' * 32, 'phase': 'census'}
        binding = {
            'source_claim': 'voice-nats-jsdata-d20261017a',
            'source_uid': '11111111-1111-4111-8111-111111111111',
            'source_pv_name': 'pv-voice-nats-source',
            'source_pv_uid': '55555555-5555-4555-8555-555555555555',
            'source_node_name': 'k3s-worker-1',
            'verification_claim': 'voice-nats-verify-' + 'a' * 12,
            'verification_uid': '22222222-2222-4222-8222-222222222222',
            'verification_pv_name': 'pv-voice-nats-verify',
            'verification_pv_uid': '33333333-3333-4333-8333-333333333333',
            'verification_node_name': 'k3s-worker-1',
        }
        pod_uid = '44444444-4444-4444-8444-444444444444'
        archive = {'sha256': 'a' * 64, 'bytes': 123}
        session_inputs = {'attempt': 'd' * 8, 'sha256': 'e' * 64, 'bytes': 456}
        with patch.object(self.api.tokens, 'read', return_value='opaque-test-token'), \
             patch.object(self.api, '_request', return_value={'metadata': {
                 'uid': pod_uid, 'namespace': 'voice-staging'}}), \
             patch.object(kube, '_websocket_exec', return_value=iter((b'framed-output',))) as execute, \
             patch.object(census_stream, 'consume',
                 side_effect=lambda chunks, **kwargs: (list(chunks), kwargs)) as consume:
            result = self.api.read_verification_census('voice-staging', 'pod', pod_uid,
                request, binding, archive, session_inputs)
        self.assertEqual(result, ([b'framed-output'], {
            'operation': request['operation'], 'nonce': request['nonce'], 'pod_uid': pod_uid,
            'source_pvc_uid': binding['source_uid'],
            'source_pv_uid': binding['source_pv_uid'],
            'verification_pvc_uid': binding['verification_uid'],
            'verification_pv_uid': binding['verification_pv_uid'],
            'archive_sha256': archive['sha256'], 'archive_bytes': archive['bytes'],
        }))
        self.assertEqual(execute.call_args.kwargs['max_bytes'], census_stream.MAX_TRANSPORT_BYTES)
        self.assertEqual(consume.call_count, 1)

    def test_read_verification_census_rejects_non_census_phase_before_api(self):
        request = {'operation': 'a' * 12, 'nonce': 'b' * 32, 'phase': 'archive'}
        binding = {
            'source_claim': 'voice-nats-jsdata-d20261017a',
            'source_uid': '11111111-1111-4111-8111-111111111111',
            'source_pv_name': 'pv-voice-nats-source',
            'source_pv_uid': '55555555-5555-4555-8555-555555555555',
            'source_node_name': 'k3s-worker-1',
            'verification_claim': 'voice-nats-verify-' + 'a' * 12,
            'verification_uid': '22222222-2222-4222-8222-222222222222',
            'verification_pv_name': 'pv-voice-nats-verify',
            'verification_pv_uid': '33333333-3333-4333-8333-333333333333',
            'verification_node_name': 'k3s-worker-1',
        }
        with patch.object(self.api, '_request') as read, \
             patch.object(kube, '_websocket_exec') as execute, \
             self.assertRaises(kube.KubeAPIError):
            self.api.read_verification_census('voice-staging', 'pod',
                '44444444-4444-4444-8444-444444444444', request, binding,
                {'sha256': 'a' * 64, 'bytes': 123},
                {'attempt': 'd' * 8, 'sha256': 'e' * 64, 'bytes': 456})
        read.assert_not_called()
        execute.assert_not_called()

    def test_verification_exec_rejects_arbitrary_action_and_changed_pod(self):
        request = {'operation': 'a' * 12, 'nonce': 'b' * 32, 'phase': 'census'}
        binding = {
            'source_claim': 'voice-nats-jsdata-d20261017a',
            'source_uid': '11111111-1111-4111-8111-111111111111',
            'source_pv_name': 'pv-voice-nats-source', 'source_pv_uid': '55555555-5555-4555-8555-555555555555',
            'source_node_name': 'k3s-worker-1', 'verification_claim': 'voice-nats-verify-' + 'a' * 12,
            'verification_uid': '22222222-2222-4222-8222-222222222222',
            'verification_pv_name': 'pv-voice-nats-verify', 'verification_pv_uid': '33333333-3333-4333-8333-333333333333',
            'verification_node_name': 'k3s-worker-1'}
        session_inputs = {'attempt': 'd' * 8, 'sha256': 'e' * 64, 'bytes': 456}
        with self.assertRaises(kube.KubeAPIError):
            list(self.api.exec_pod_verification('voice-staging', 'pod',
                '44444444-4444-4444-8444-444444444444', request, binding, 'arbitrary'))
        with patch.object(self.api.tokens, 'read', return_value='opaque-test-token'), \
             patch.object(self.api, '_request', return_value={'metadata': {
                'uid': '99999999-9999-4999-8999-999999999999', 'namespace': 'voice-staging'}}), \
             patch.object(kube, '_websocket_exec') as execute:
            with self.assertRaises(kube.KubeAPIError):
                list(self.api.exec_pod_verification('voice-staging', 'pod',
                    '44444444-4444-4444-8444-444444444444', request, binding,
                    'census_same_pod_readback', archive={'sha256': 'a' * 64, 'bytes': 123},
                    session_inputs=session_inputs))
        execute.assert_not_called()

    def test_rejects_non_https_or_unpinned_server_before_network(self):
        with self.assertRaises(kube.KubeAPIError):
            kube.KubernetesJSONAPI('http://10.0.0.1:6443', self.ca, self.token)
        self.assertEqual(self.calls, [])

    def test_token_refresh_scope_is_fixed_and_persisted_atomically(self):
        new_token = jwt('voice-staging-runner', 9999999999)
        def refresh(request, *, context, timeout):
            self.calls.append((request, context, timeout))
            return Response({'status': {'token': new_token, 'expirationTimestamp': '2030-01-01T00:00:00Z'}})
        self.api.opener = refresh
        with patch.object(self.api.tokens, 'read', return_value='opaque-test-token'):
            token = self.api.request_token_refresh()
            body = json.loads(self.calls[-1][0].data)
        self.assertEqual(token, new_token)
        self.assertEqual(self.calls[-1][0].full_url,
            'https://10.0.0.1:6443/api/v1/namespaces/voice-staging/serviceaccounts/voice-nats-staging-runner/token')
        self.assertEqual(body['spec'], {'audiences': ['voice-staging-runner'], 'expirationSeconds': 3600})
        with self.assertRaises(kube.KubeAPIError): self.api.request_token_refresh(service_account='other')
        target = self.root / 'state'; target.mkdir(mode=0o700)
        self.api.save_refreshed_token(new_token, target / 'token')
        self.assertEqual((target / 'token').read_text(), new_token)
        self.assertEqual((target / 'token').stat().st_mode & 0o777, 0o600)

    def test_low_budget_refreshes_before_resource_read_and_outage_blocks_it(self):
        old_token = jwt('voice-staging-runner', int(__import__('time').time()) + 90)
        new_token = jwt('voice-staging-runner', int(__import__('time').time()) + 3600)
        self.token.write_text(old_token); self.token.chmod(0o600)
        def refresh_then_read(request, *, context, timeout):
            self.calls.append((request, context, timeout))
            if request.full_url.endswith('/serviceaccounts/voice-nats-staging-runner/token'):
                return Response({'status': {'token': new_token, 'expirationTimestamp': '2030-01-01T00:00:00Z'}})
            return Response({'metadata': {'name': 'claim'}})
        self.api.opener = refresh_then_read
        self.api.get_pvc('voice-staging', 'claim')
        self.assertEqual([c[0].get_method() for c in self.calls], ['POST', 'GET'])
        self.assertIn(new_token, self.calls[1][0].get_header('Authorization'))

        self.calls.clear()
        self.token.write_text(old_token); self.token.chmod(0o600)
        def denied(request, *, context, timeout):
            self.calls.append((request, context, timeout))
            raise HTTPError(request.full_url, 403, 'Forbidden', {}, None)
        self.api.opener = denied
        with self.assertRaisesRegex(kube.KubeAPIError, 'transport_failed'):
            self.api.get_pvc('voice-staging', 'claim')
        self.assertEqual([c[0].get_method() for c in self.calls], ['POST'])


class SourceBindingResolutionTests(unittest.TestCase):
    def setUp(self):
        self.api = SourceAPI()

    def test_resolves_hostname_to_actual_node_name_and_binds_live_chain(self):
        result = kube.resolve_source_binding(self.api, operation='a' * 12)
        self.assertEqual(result['binding'], {
            'source_claim': 'voice-nats-jsdata-d20261017a',
            'source_uid': '11111111-1111-4111-8111-111111111111',
            'source_pv_name': 'voice-nats-pv-a',
            'source_pv_uid': '22222222-2222-4222-8222-222222222222',
            'source_node_name': 'node-actual-name'})
        self.assertEqual(result['proof']['node_hostname'], 'node-hostname-label')
        self.assertEqual(result['proof']['node_uid'], '33333333-3333-4333-8333-333333333333')
        self.assertEqual(self.api.reads, ['namespace', 'marker', 'hub', 'pvc', 'pv', 'nodes', 'pods'])
        self.assertEqual(kube.resolve_source_binding(self.api, operation='a' * 12, expected=result), result)

    def test_reconciles_same_binding_after_exact_owned_fence_without_any_hub_pod(self):
        first = kube.resolve_source_binding(self.api, operation='a' * 12)
        self.api.marker['data'] = self.api.marker['data'] | {
            'phase': 'known-baseline-maintenance', 'knownBaselineOperation': 'a' * 12}
        self.api.pods = []
        self.assertEqual(kube.resolve_source_binding(self.api, operation='a' * 12, expected=first), first)

    def test_rejects_ambiguous_hostname_node_before_binding(self):
        self.api.nodes.append(copy.deepcopy(self.api.nodes[0]))
        with self.assertRaisesRegex(kube.KubeAPIError, 'source_node_ambiguous'):
            kube.resolve_source_binding(self.api, operation='a' * 12)

    def test_rejects_hub_pod_on_a_different_actual_node(self):
        self.api.pods[0]['spec']['nodeName'] = 'other-node'
        with self.assertRaisesRegex(kube.KubeAPIError, 'source_hub_pod_node_changed'):
            kube.resolve_source_binding(self.api, operation='a' * 12)

    def test_rejects_any_fresh_identity_drift_during_reconciliation(self):
        first = kube.resolve_source_binding(self.api, operation='a' * 12)
        self.api.marker['metadata']['uid'] = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'
        with self.assertRaisesRegex(kube.KubeAPIError, 'source_binding_changed'):
            kube.resolve_source_binding(self.api, operation='a' * 12, expected=first)


class SourceAPI:
    def __init__(self):
        self.reads = []
        self.namespace = {'metadata': {'name': 'voice-staging', 'uid': 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'}}
        self.marker = {'metadata': {'name': 'voice-nats-generation', 'namespace': 'voice-staging',
            'uid': 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb'},
            'data': {'phase': 'active', 'generation': '123', 'dataPVC': 'voice-nats-jsdata-d20261017a'}}
        self.hub = {'metadata': {'name': 'voice-nats-pvc-candidate', 'namespace': 'voice-staging',
            'uid': 'cccccccc-cccc-4ccc-8ccc-cccccccccccc'},
            'spec': {'selector': {'matchLabels': {'app': 'voice-nats-pvc-candidate'}}, 'template': {'spec': {
                'volumes': [{'name': 'jsdata', 'persistentVolumeClaim': {'claimName': 'voice-nats-jsdata-d20261017a'}}]}}}}
        self.claim = {'metadata': {'name': 'voice-nats-jsdata-d20261017a', 'namespace': 'voice-staging',
            'uid': '11111111-1111-4111-8111-111111111111'}, 'spec': {'volumeName': 'voice-nats-pv-a',
                'accessModes': ['ReadWriteOnce'], 'volumeMode': 'Filesystem'}, 'status': {'phase': 'Bound'}}
        self.pv = {'metadata': {'name': 'voice-nats-pv-a', 'uid': '22222222-2222-4222-8222-222222222222'},
            'spec': {'claimRef': {'namespace': 'voice-staging', 'name': 'voice-nats-jsdata-d20261017a',
                'uid': '11111111-1111-4111-8111-111111111111'}, 'volumeMode': 'Filesystem',
                'nodeAffinity': {'required': {'nodeSelectorTerms': [{'matchExpressions': [
                    {'key': 'kubernetes.io/hostname', 'operator': 'In', 'values': ['node-hostname-label']}]}]}}},
            'status': {'phase': 'Bound'}}
        self.nodes = [{'metadata': {'name': 'node-actual-name', 'uid': '33333333-3333-4333-8333-333333333333',
            'labels': {'kubernetes.io/hostname': 'node-hostname-label'}}, 'status': {'conditions': [
                {'type': 'Ready', 'status': 'True'}]}}]
        self.pods = [{'metadata': {'name': 'voice-nats-pvc-candidate-0', 'uid': '44444444-4444-4444-8444-444444444444'},
            'spec': {'nodeName': 'node-actual-name', 'volumes': [{'name': 'jsdata',
                'persistentVolumeClaim': {'claimName': 'voice-nats-jsdata-d20261017a'}}]}}]

    def get_namespace(self, name): self.reads.append('namespace'); return self.namespace
    def get_configmap(self, namespace, name): self.reads.append('marker'); return self.marker
    def get_deployment(self, namespace, name): self.reads.append('hub'); return self.hub
    def get_pvc(self, namespace, name): self.reads.append('pvc'); return self.claim
    def get_pv(self, name): self.reads.append('pv'); return self.pv
    def list_nodes_by_hostname(self, hostname): self.reads.append('nodes'); return self.nodes
    def list_hub_pods(self, namespace, labels): self.reads.append('pods'); return self.pods


if __name__ == '__main__': unittest.main()
