"""Protected Story/File deployment template wiring; no live environment access."""
from pathlib import Path
import json
import unittest

import yaml


class StoryMediaTemplateTest(unittest.TestCase):
    def test_messaging_signer_uses_two_regular_subpath_mounts_and_mtls_jwks(self):
        root = Path(__file__).resolve().parents[2]
        obj = yaml.safe_load((root / 'deploy/templates/story-media-messaging-patch.yaml').read_text())
        pod = obj['spec']['template']['spec']
        container = next(c for c in pod['containers'] if c['name'] == 'messaging')
        env = {e['name']: e['value'] for e in container['env']}
        self.assertEqual(env['MESSAGING_PURGE_PRINCIPAL_SIGNING_KEYS_DIR'], '/run/messaging-principal')
        self.assertEqual(env['MESSAGING_PURGE_PRINCIPAL_ACTIVE_KID'], 'current')
        self.assertEqual(env['MESSAGING_PURGE_PRINCIPAL_JWKS_LISTEN'], ':8443')
        self.assertEqual(env['MESSAGING_PURGE_PRINCIPAL_TLS_CERT_FILE'], '/run/messaging-jwks/tls.crt')
        self.assertEqual(env['MESSAGING_PURGE_PRINCIPAL_TLS_KEY_FILE'], '/run/messaging-jwks/tls.key')
        self.assertEqual(env['MESSAGING_PURGE_PRINCIPAL_CLIENT_CA_FILE'], '/run/messaging-jwks-client-ca/ca.crt')
        self.assertFalse(any(k.startswith(('FILE_MANAGED_CHAT_PURGE_', 'SEARCH_MANAGED_CHAT_PURGE_')) for k in env))
        mounts = {m['mountPath']: m for m in container['volumeMounts']}
        for kid in ('current', 'next'):
            mount = mounts['/run/messaging-principal/' + kid + '.pem']
            self.assertEqual(mount['name'], 'messaging-signing')
            self.assertEqual(mount['subPath'], kid + '.pem')
            self.assertTrue(mount['readOnly'])
        volumes = {v['name']: v['secret'] for v in pod['volumes']}
        self.assertEqual(volumes['messaging-signing']['secretName'], '__MESSAGING_SIGNING_SECRET__')
        self.assertEqual(volumes['messaging-signing']['defaultMode'], 0o444)
        self.assertEqual(volumes['messaging-jwks-tls']['secretName'], 'voice-messaging-jwks-tls')
        self.assertEqual(volumes['messaging-jwks-client-ca']['secretName'], 'voice-messaging-jwks-client-ca')
        self.assertEqual(mounts['/run/messaging-jwks']['name'], 'messaging-jwks-tls')
        self.assertEqual(mounts['/run/messaging-jwks-client-ca']['name'], 'messaging-jwks-client-ca')
        self.assertEqual([c['name'] for c in pod['containers']], ['messaging'])

    def test_messaging_jwks_has_separate_service_port(self):
        root = Path(__file__).resolve().parents[2]
        service = yaml.safe_load((root / 'deploy/templates/story-media-messaging-service.yaml').read_text())
        self.assertEqual(service['metadata']['name'], 'voice-messaging-principal')
        self.assertEqual(service['spec']['selector'], {'app': 'voice-messaging'})
        self.assertEqual(service['spec']['ports'], [{'name': 'principal-jwks', 'port': 8443, 'targetPort': 8443}])

    def test_story_private_jwks_service_and_listener(self):
        root = Path(__file__).resolve().parents[2]
        service = yaml.safe_load((root / 'deploy/templates/story-media-story-jwks-service.yaml').read_text())
        self.assertEqual(service['metadata']['name'], 'voice-story-principal')
        self.assertEqual(service['spec']['selector'], {'app': 'voice-story'})
        self.assertEqual(service['spec']['ports'], [{'name': 'principal-jwks', 'port': 8443, 'targetPort': 8443}])
        patch = yaml.safe_load((root / 'deploy/templates/story-media-story-jwks-patch.yaml').read_text())
        pod = patch['spec']['template']['spec']
        container = next(c for c in pod['containers'] if c['name'] == 'story')
        env = {e['name']: e['value'] for e in container['env']}
        self.assertEqual(env['STORY_PRINCIPAL_JWKS_HTTPS_LISTEN'], ':8443')
        self.assertEqual(env['STORY_PRINCIPAL_JWKS_TLS_CERT_FILE'], '/run/story-jwks/tls.crt')
        self.assertEqual(env['STORY_PRINCIPAL_JWKS_TLS_KEY_FILE'], '/run/story-jwks/tls.key')
        self.assertEqual(pod['volumes'][0]['secret']['secretName'], 'voice-story-jwks-tls')

    def test_file_listener_uses_separate_server_and_client_trust_secrets(self):
        root = Path(__file__).resolve().parents[2]
        template = yaml.safe_load((root / 'deploy/templates/story-media-file-patch.yaml').read_text())
        pod = template['spec']['template']['spec']
        file_container = next(c for c in pod['containers'] if c['name'] == 'file')
        env = {e['name']: e.get('value') for e in file_container['env']}
        mounts = {m['name']: m['mountPath'] for m in file_container['volumeMounts']}
        volumes = {v['name']: v['secret']['secretName'] for v in pod['volumes']}
        self.assertEqual(env['FILE_PRINCIPAL_TLS_CERT_FILE'], '/run/file-principal-grpc/tls.crt')
        self.assertEqual(env['FILE_PRINCIPAL_TLS_KEY_FILE'], '/run/file-principal-grpc/tls.key')
        self.assertEqual(env['FILE_PRINCIPAL_CLIENT_CA_FILE'], '/run/file-principal-client-ca/ca.crt')
        self.assertEqual(json.loads(env['S2S_JWKS_URLS_JSON']), {
            'story': 'https://__STORY_JWKS_HOST__/.well-known/jwks.json',
            'messaging': '__MESSAGING_JWKS_URL__'})
        self.assertEqual(env['S2S_JWKS_CLIENT_CERT_FILE'], '/run/file-jwks-client/tls.crt')
        self.assertEqual(env['S2S_JWKS_CLIENT_KEY_FILE'], '/run/file-jwks-client/tls.key')
        self.assertEqual(mounts['file-principal-grpc-tls'], '/run/file-principal-grpc')
        self.assertEqual(mounts['file-principal-client-ca'], '/run/file-principal-client-ca')
        self.assertEqual(volumes['file-principal-grpc-tls'], 'voice-file-principal-grpc-tls')
        self.assertEqual(volumes['file-principal-client-ca'], 'voice-file-principal-client-ca')
        self.assertEqual(mounts['file-jwks-client-tls'], '/run/file-jwks-client')
        self.assertEqual(volumes['file-jwks-client-tls'], 'voice-file-jwks-client-tls')
        self.assertNotIn('file-principal-tls', volumes)


if __name__ == '__main__':
    unittest.main()
