"""Actual staging rendering must satisfy the implemented media security contract."""
import json
import base64
import subprocess
import re
from pathlib import Path
import unittest
import yaml
import compiler
import source_plan
import nonnats_plan
import source_plan_test as fixtures

SOURCE = Path(__file__).resolve().parents[3]


class MediaProjectionTest(unittest.TestCase):
    def rendered(self):
        return compiler.rendered_source(SOURCE, {
            'generation': 'r20260930a4', 'dataPVC': 'voice-nats-jsdata-d20261014a',
            'registry': 'example.invalid', 'tag': 'a' * 40,
            'release_policy': fixtures.release_fixture(),
            'gateway_host': 'gateway.example.invalid', 's3_signing_endpoint': 'https://storage.example.invalid',
        }, yaml.safe_load)

    def test_canonical_media_tuples_are_projected_with_private_readonly_files(self):
        rows = self.rendered()
        compose = yaml.safe_load((SOURCE / 'docker-compose.voice-space-media.yml').read_text())['services']
        for service in ('auth', 'chat', 'voice', 'role', 'gateway', 'matchmaking'):
            with self.subTest(service=service):
                pod = next(r for r in rows if r['kind'] == 'Deployment' and r['metadata']['name'] == 'voice-' + service)['spec']['template']['spec']
                container = next(c for c in pod['containers'] if c['name'] == service)
                env = {e['name']: e for e in container['env']}
                required = {k for k in compose[service]['environment'] if k != 'LIVEKIT_URL' and not k.startswith('VOICE_SPACE_LIFECYCLE_')}
                self.assertLessEqual(required, set(env), service)
                volumes = {v['name']: v for v in pod['volumes']}
                for key in required:
                    entry = env[key]
                    value = entry.get('value', '')
                    self.assertNotIn('phase0', value)
                    if key.endswith(('_FILE', '_DIR')):
                        mount = next(m for m in container['volumeMounts'] if value == m['mountPath'] or value.startswith(m['mountPath'] + '/'))
                        self.assertTrue(mount['readOnly'])
                        self.assertIn('secret', volumes[mount['name']])
                        self.assertTrue(volumes[mount['name']]['secret']['items'])
                if service == 'auth':
                    self.assertIn('-XX:MaxRAMPercentage=75.0', env['JAVA_TOOL_OPTIONS']['value'])
                    self.assertIn('-XX:InitialRAMPercentage=25.0', env['JAVA_TOOL_OPTIONS']['value'])
                    self.assertIn('-Djavax.net.ssl.trustStore=', env['JAVA_TOOL_OPTIONS']['value'])
                    self.assertIn('-Djavax.net.ssl.trustStorePassword=changeit', env['JAVA_TOOL_OPTIONS']['value'])
                    self.assertNotIn('$', env['JAVA_TOOL_OPTIONS']['value'])
                    self.assertNotIn('MEDIA_TRUSTSTORE_PASSWORD', env)
                if service == 'voice':
                    self.assertEqual(env['VOICE_AUTH_SESSION_FLOOR_GRPC_ADDR']['value'], 'voice-auth:9092')
                    self.assertEqual(env['VOICE_AUTH_SESSION_FLOOR_TLS_SERVER_NAME']['value'], 'voice-auth')
                    self.assertEqual(json.loads(env['S2S_JWKS_URLS_JSON']['value'])['gateway'], 'https://gateway.example.invalid/.well-known/voice-principal-jwks.json')
                    self.assertNotIn('VOICE_SPACE_LIFECYCLE_PRINCIPAL_GRPC_LISTEN', env)
                if service == 'gateway':
                    signing = next(v['secret'] for v in pod['volumes'] if v['secret']['secretName'] == 'voice-gateway-principal-signing')
                    self.assertEqual({i['path'] for i in signing['items']}, {'current.pem', 'next.pem'})
        for service, ports in {'auth': {9091, 9092}, 'chat': {9092}, 'voice': {8443, 9093, 9094, 9095},
                               'role': {9091}, 'matchmaking': {8443}}.items():
            row = next(r for r in rows if r['kind'] == 'Service' and r['metadata']['name'] == 'voice-' + service)
            self.assertLessEqual(ports, {p['port'] for p in row['spec']['ports']})

    def test_missing_media_credential_is_rejected_before_any_mutating_apply(self):
        kube, params = fixtures.SourceTests().canonical_fixture('full')
        # The prerequisite must be explicit even if the current live templates lack it.
        kube.objects.pop(('Secret', 'voice-voice-media-client-tls'), None)
        with self.assertRaisesRegex(source_plan.PlanError, 'voice-voice-media-client-tls'):
            source_plan.compile_plan(SOURCE, params, fixtures.decoder, kube)
        self.assertFalse(any(c[0] == 'run' and c[1][0] == 'apply' and '--dry-run=server' not in c[1] for c in kube.calls))

    def test_binary_truststore_is_explicit_and_other_credentials_remain_text(self):
        raw = b'\x30\x82\xff\x00binary-pkcs12'
        secret = {'data': {'truststore.p12': base64.b64encode(raw).decode()}}
        desired = {'required_keys': ['truststore.p12'], 'binary_keys': ['truststore.p12']}
        nonnats_plan._secret(secret, desired)
        with self.assertRaises(nonnats_plan.PlanError):
            nonnats_plan._secret(secret, {'required_keys': ['truststore.p12']})
        for invalid in ('', '!!!', base64.b64encode(b'').decode()):
            with self.subTest(invalid=invalid), self.assertRaises(nonnats_plan.PlanError):
                nonnats_plan._secret({'data': {'truststore.p12': invalid}}, desired)
        with self.assertRaises(nonnats_plan.PlanError):
            nonnats_plan._secret({'data': {'password': base64.b64encode(raw).decode()}},
                                {'required_keys': ['password'], 'binary_keys': ['password']})

    def test_existing_shell_renderer_resolves_gateway_issuer_host(self):
        script = (SOURCE / 'scripts/staging/apply-app-manifests.sh').read_text()
        function = re.search(r'^render\(\) \{.*?^\}', script, re.M | re.S).group(0)
        command = ('set -eu\nnats_generation_render() { cat "$1"; }\n'
                   'REGISTRY=example.invalid TAG=unit S3_SIGNING_ENDPOINT=https://storage.example.invalid\n'
                   'VOICE_GATEWAY_INGRESS_HOST=gateway.example.invalid\n' + function + '\nrender "$1"')
        raw = subprocess.check_output(['bash', '-c', command, '--', str(SOURCE / 'deploy/staging/services.yaml')], text=True)
        self.assertNotIn('__GATEWAY_INGRESS_HOST__', raw)
        self.assertIn('https://gateway.example.invalid/.well-known/voice-principal-jwks.json', raw)

    def test_private_listener_client_symmetry_and_shared_resolver_secret(self):
        rows = self.rendered()
        envs = {}
        for service in ('auth', 'chat', 'voice', 'role', 'gateway', 'space', 'matchmaking'):
            row = next(r for r in rows if r['kind'] == 'Deployment' and r['metadata']['name'] == 'voice-' + service)
            envs[service] = {e['name']: e for c in row['spec']['template']['spec']['containers'] if c['name'] == service for e in c['env']}
        for client, key, server, port in (
            ('voice', 'VOICE_AUTH_SESSION_FLOOR_GRPC_ADDR', 'auth', 9092),
            ('voice', 'VOICE_ROLE_GRPC_ADDR', 'role', 9091),
            ('gateway', 'GATEWAY_VOICE_USER_GRPC_ADDR', 'voice', 9093),
            ('matchmaking', 'MATCHMAKING_MATCH_SQUAD_CHAT_GRPC_ADDR', 'chat', 9092),
            ('matchmaking', 'MATCHMAKING_MATCH_SQUAD_VOICE_GRPC_ADDR', 'voice', 9094),
            ('gateway', 'GATEWAY_MATCH_SQUAD_MEMBER_GRPC_ADDR', 'voice', 9095)):
            self.assertEqual(envs[client][key]['value'], 'voice-' + server + ':' + str(port))
            service = next(r for r in rows if r['kind'] == 'Service' and r['metadata']['name'] == 'voice-' + server)
            self.assertIn(port, {p['port'] for p in service['spec']['ports']})
        self.assertEqual(envs['space']['SPACE_VOICE_S2S_TOKEN']['valueFrom'], envs['voice']['SPACE_VOICE_S2S_TOKEN']['valueFrom'])
        for service, issuers in (('auth', {'gateway', 'space', 'voice'}), ('role', {'space', 'gameintegration', 'voice'}),
                                 ('voice', {'gateway'}), ('matchmaking', {'gateway', 'space'})):
            urls = json.loads(envs[service]['S2S_JWKS_URLS_JSON']['value'])
            self.assertEqual(set(urls), issuers)
            if 'voice' in urls:
                self.assertEqual(urls['voice'], 'https://voice-voice:8443/internal/v1/principal/jwks.json')
        # Public roots belong to issuer fetch trust, never the inbound mTLS CA.
        self.assertNotEqual(envs['voice']['S2S_JWKS_CA_FILE']['value'], envs['voice']['VOICE_USER_PRINCIPAL_CLIENT_CA_FILE']['value'])
        for service in ('chat', 'voice'):
            prefix = service.upper() + '_MATCH_SQUAD_PRINCIPAL_'
            self.assertEqual(envs[service][prefix + 'JWKS_URL']['value'],
                             'https://voice-matchmaking.voice-staging.svc:8443/internal/v1/principal/jwks.json')
            self.assertNotEqual(envs[service][prefix + 'JWKS_CA_FILE']['value'],
                                envs[service][prefix + 'CLIENT_CA_FILE']['value'])


if __name__ == '__main__':
    unittest.main()
