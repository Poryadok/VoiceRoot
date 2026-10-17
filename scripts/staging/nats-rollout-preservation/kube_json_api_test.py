"""Contract tests for the fixed HTTPS Kubernetes API adapter."""
import json
import base64
import os
import ssl
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
                    'voice.io/nats-job-nonce': 'b' * 32}},
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
        with patch.object(self.api.tokens, 'read', return_value='opaque-test-token'), \
             patch.object(self.api, '_request', return_value=pod), \
             patch.object(kube, '_websocket_exec', return_value=iter((b'receipt',))) as execute:
            result = list(self.api.exec_pod_verification('voice-staging', 'pod', pod_uid,
                request, binding, 'census_receipt_readback'))
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
            request, pod_uid, binding)['census_receipt_readback'])
        self.assertEqual(execute.call_args.kwargs['max_bytes'], census_stream.MAX_TRANSPORT_BYTES)

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
        with patch.object(self.api.tokens, 'read', return_value='opaque-test-token'), \
             patch.object(self.api, '_request', return_value={'metadata': {
                 'uid': pod_uid, 'namespace': 'voice-staging'}}), \
             patch.object(kube, '_websocket_exec', return_value=iter((b'framed-output',))) as execute, \
             patch.object(census_stream, 'consume',
                 side_effect=lambda chunks, **kwargs: (list(chunks), kwargs)) as consume:
            result = self.api.read_verification_census('voice-staging', 'pod', pod_uid,
                request, binding)
        self.assertEqual(result, ([b'framed-output'], {
            'operation': request['operation'], 'nonce': request['nonce'], 'pod_uid': pod_uid,
            'source_pvc_uid': binding['source_uid'],
            'verification_pvc_uid': binding['verification_uid'],
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
                '44444444-4444-4444-8444-444444444444', request, binding)
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
        with self.assertRaises(kube.KubeAPIError):
            list(self.api.exec_pod_verification('voice-staging', 'pod',
                '44444444-4444-4444-8444-444444444444', request, binding, 'arbitrary'))
        with patch.object(self.api.tokens, 'read', return_value='opaque-test-token'), \
             patch.object(self.api, '_request', return_value={'metadata': {
                'uid': '99999999-9999-4999-8999-999999999999', 'namespace': 'voice-staging'}}), \
             patch.object(kube, '_websocket_exec') as execute:
            with self.assertRaises(kube.KubeAPIError):
                list(self.api.exec_pod_verification('voice-staging', 'pod',
                    '44444444-4444-4444-8444-444444444444', request, binding, 'census_receipt_readback'))
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


if __name__ == '__main__': unittest.main()
