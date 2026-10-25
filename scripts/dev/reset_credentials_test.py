import base64
import copy
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock
import json

import reset_credentials as credentials


def secret(name, **values):
    return {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {'name': name, 'namespace': 'voice-dev'},
            'data': {key: base64.b64encode(value.encode()).decode() for key, value in values.items()}}


def workload():
    return {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'metadata': {'name': 'voice-test', 'namespace': 'voice-dev'},
            'spec': {'template': {'spec': {'containers': [{'name': 'test', 'env': [
                {'name': 'TLS_CERT_FILE', 'value': '/etc/voice/tls/tls.crt'}],
                'volumeMounts': [{'name': 'tls', 'mountPath': '/etc/voice/tls', 'readOnly': True}]}],
                'volumes': [{'name': 'tls', 'secret': {'secretName': 'voice-tls'}}]}}}}


class ReferenceTests(unittest.TestCase):
    def test_private_destination_freshness_symlink_owner_and_mode(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch).resolve()
            root.chmod(0o700)
            with mock.patch.object(credentials.sys, 'platform', 'linux'), mock.patch.object(credentials.os, 'geteuid', return_value=123, create=True):
                with mock.patch.object(credentials, '_directory_info', return_value=type('Info', (), {'st_mode': 0o40700, 'st_uid': 123})()):
                    credentials.private_destination(root / 'fresh', root)
                    with self.assertRaisesRegex(ValueError, 'destination'):
                        credentials.private_destination(root, root)
                with mock.patch.object(credentials, '_directory_info', return_value=type('Info', (), {'st_mode': 0o40755, 'st_uid': 123})()):
                    with self.assertRaisesRegex(ValueError, 'private_workdir'):
                        credentials.private_destination(root / 'fresh', root)

    def test_replay_requires_actual_noauth_redis_and_empty_all_client_passwords(self):
        redis = workload()
        redis['metadata']['name'] = 'voice-redis'
        container = redis['spec']['template']['spec']['containers'][0]
        container['name'] = 'redis'
        container['args'] = ['redis-server', '--appendonly', 'yes']
        replay = [secret(name, password='') for name in credentials.REPLAY_NAMES]
        credentials.verify_replay([redis], replay)
        replay[0]['data']['password'] = base64.b64encode(b'unconfigured').decode()
        with self.assertRaisesRegex(ValueError, 'replay_password'):
            credentials.verify_replay([redis], replay)
        replay[0]['data']['password'] = ''
        container['args'].extend(['--requirepass', 'hidden'])
        with self.assertRaisesRegex(ValueError, 'redis_contract'):
            credentials.verify_replay([redis], replay)

    def test_identity_issuer_replaces_connected_families_and_separates_complete_client_root(self):
        material = mock.Mock()
        material.items = []
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch)
            root.joinpath('ca.crt').write_bytes(b'public fixture')
            material.ca.return_value = (root / 'ca.key', root / 'ca.crt')
            credentials.issue_identities(material, b'public roots')
        leaves = {call.args[0]: call.args for call in material.tls.call_args_list}
        self.assertEqual(leaves['voice-realtime-p3-tls'][2:], ('voice-realtime.voice-dev.svc', 'serverAuth,clientAuth'))
        self.assertEqual(leaves['voice-chat-realtime-p3-tls'][2:], ('voice-chat.voice-dev.svc', 'serverAuth'))
        self.assertEqual(leaves['voice-p3-principal-jwks-tls'][2], 'voice-p3-principal-jwks.voice-dev.svc')
        self.assertIn('voice-space-media-client-tls', leaves)
        self.assertIn('voice-notification-principal-tls', leaves)
        calls = {call.args[0]: call.kwargs for call in material.signer.call_args_list}
        self.assertEqual(calls['voice-realtime-principal-signing'], {'active': False})
        self.assertEqual(calls['voice-space-principal-signing'], {'active': True})
        ca_calls = [call.args[0] for call in material.ca.call_args_list]
        self.assertIn('match-squad', ca_calls)
        self.assertIn('matchmaking-complete-client', ca_calls)

    def test_fresh_database_completion_uses_same_fresh_password_and_documented_databases(self):
        source = secret('voice-app-secrets', POSTGRES_PASSWORD='fresh-password')
        completed = credentials.complete_application(source)
        data = credentials.values(completed)
        self.assertEqual(data['REALTIME_DATABASE_URL'], b'postgres://voice:fresh-password@voice-postgres:5432/realtime_db?sslmode=disable')
        self.assertEqual(data['NOTIFICATION_DATABASE_URL'], b'postgres://voice:fresh-password@voice-postgres:5432/notification_db?sslmode=disable')
        self.assertEqual(credentials.values(source), {'POSTGRES_PASSWORD': b'fresh-password'})
        bad = secret('voice-app-secrets', POSTGRES_PASSWORD='fresh-password', REALTIME_DATABASE_URL='postgres://stale')
        with self.assertRaisesRegex(ValueError, 'database_generation'):
            credentials.complete_application(bad)

    def test_all_application_databases_and_object_store_are_coherent(self):
        class Material:
            def __init__(self):
                self.items = []
            add = credentials.canonical.Material.add
        material = Material()
        app = credentials.canonical.application_values(b'private fixture', 'api-dev.comrade.click')
        for service in ('USER', 'FILE'):
            app[service + '_R2_ACCESS_KEY_ID'] = 'dev-user'
            app[service + '_R2_SECRET_ACCESS_KEY'] = 'dev-pass'
        material.add('voice-app-secrets', app)
        material.items[0] = credentials.complete_application(material.items[0])
        material.add('voice-minio-credentials', {'MINIO_ROOT_USER': 'dev-user', 'MINIO_ROOT_PASSWORD': 'dev-pass'})
        credentials.verify_application(material.items)
        material.items[0]['data']['SOCIAL_DATABASE_URL'] = base64.b64encode(b'postgres://stale').decode()
        with self.assertRaisesRegex(ValueError, 'application_database'):
            credentials.verify_application(material.items)

    def test_union_is_explicit_and_never_merges_identity_generations_by_name(self):
        retained = secret('voice-tls', **{'tls.crt': 'retained'})
        retained['metadata'].update(uid='old', resourceVersion='8', managedFields=[{}])
        fresh = [secret('voice-app-secrets', POSTGRES_PASSWORD='fresh')]
        result = credentials.build_union(fresh, [retained], {'voice-tls'})
        self.assertNotIn('uid', result[-1]['metadata'])
        self.assertEqual(credentials.values(result[-1])['tls.crt'], b'retained')
        with self.assertRaisesRegex(ValueError, 'generation_collision'):
            credentials.build_union(fresh + [secret('voice-tls', **{'tls.crt': 'new-generation'})], [retained], {'voice-tls'})
        with self.assertRaisesRegex(ValueError, 'missing_retained'):
            credentials.build_union(fresh, [], {'voice-tls'})

    def test_whole_secret_mount_requires_actual_runtime_files(self):
        obj = workload()
        with self.assertRaisesRegex(ValueError, 'runtime_file'):
            credentials.references([obj], [secret('voice-tls', unrelated='private-sentinel')])
        rows = credentials.references([obj], [secret('voice-tls', **{'tls.crt': 'private-sentinel'})])
        self.assertIn('tls.crt', rows[0]['keys'])
        self.assertNotIn('private-sentinel', str(rows))

    def test_projected_items_subpath_envfrom_and_pull_secret_are_checked(self):
        obj = workload()
        pod = obj['spec']['template']['spec']
        pod['volumes'] = [{'name': 'tls', 'projected': {'sources': [{'secret': {
            'name': 'voice-tls', 'items': [{'key': 'crt', 'path': 'tls.crt'}]}}]}}]
        pod['containers'][0]['envFrom'] = [{'secretRef': {'name': 'voice-extra'}}]
        pod['imagePullSecrets'] = [{'name': 'voice-pull'}]
        material = [secret('voice-tls', crt='private-sentinel'), secret('voice-extra', value='private-sentinel'),
                    secret('voice-pull', **{'.dockerconfigjson': '{"auths":{}}'})]
        rows = credentials.references([obj], material)
        self.assertEqual({row['name'] for row in rows}, {'voice-tls', 'voice-extra', 'voice-pull'})
        bad = copy.deepcopy(obj)
        bad['spec']['template']['spec']['containers'][0]['volumeMounts'][0]['subPath'] = 'missing'
        with self.assertRaisesRegex(ValueError, 'subpath'):
            credentials.references([bad], material)
        with self.assertRaisesRegex(ValueError, 'missing_object'):
            credentials.references([obj], material[:-1])

    def test_duplicate_and_foreign_objects_reject_without_payload_errors(self):
        obj = secret('voice-tls', **{'tls.crt': 'private-sentinel'})
        for material in ([obj, obj], [{**obj, 'metadata': {'name': 'voice-tls', 'namespace': 'voice-staging'}}]):
            with self.assertRaises(ValueError) as error:
                credentials.references([workload()], material)
            self.assertNotIn('private-sentinel', str(error.exception))


@unittest.skipUnless(shutil.which('openssl'), 'canonical OpenSSL verifier required')
class CertificateTests(unittest.TestCase):
    def test_actual_issued_identity_graph_and_cross_family_tampering(self):
        with tempfile.TemporaryDirectory() as scratch:
            material = credentials.ResetMaterial(Path(scratch))
            original = credentials.canonical.run
            def fixture_run(*args):
                return original(*(str(arg).replace('rsa_keygen_bits:3072', 'rsa_keygen_bits:2048') for arg in args))
            with mock.patch.object(credentials.canonical, 'run', side_effect=fixture_run):
                credentials.issue_identities(material, b'')
            signer = next(o for o in material.items if o['metadata']['name'] == 'voice-realtime-principal-signing')
            material.items.append(credentials.public_config('voice-p3-principal-public',
                {'realtime.json': json.dumps(credentials.signer_jwks(signer))}, True))
            receipts = credentials.verify_identities(material.items)
            self.assertIn('voice-realtime-p3-tls/clientAuth', receipts)
            self.assertIn('voice-space-media-server-tls/serverAuth', receipts)
            tampered = copy.deepcopy(material.items)
            index = credentials.objects_by_identity(tampered)
            index['Secret', 'voice-p3-principal-trust']['data']['space-jwks-ca.crt'] = index['Secret', 'voice-principal-ca']['data']['ca.crt']
            with self.assertRaisesRegex(ValueError, 'p3_trust_generation'):
                credentials.verify_identities(tampered)
            tampered = copy.deepcopy(material.items)
            index = credentials.objects_by_identity(tampered)
            index['Secret', 'voice-voice-media-server-tls']['data'] = index['Secret', 'voice-voice-match-squad-server-tls']['data']
            with self.assertRaisesRegex(ValueError, 'certificate'):
                credentials.verify_identities(tampered)

    def test_p3_jwks_is_derived_from_two_distinct_pkcs8_keys_and_rejects_tamper(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch)
            pairs = {}
            for kid in ('current', 'next'):
                result = subprocess.run(['openssl', 'genpkey', '-algorithm', 'RSA', '-pkeyopt',
                                         'rsa_keygen_bits:2048', '-out', str(root / kid)], capture_output=True)
                self.assertEqual(result.returncode, 0)
                pairs[kid + '.pem'] = (root / kid).read_text()
            source = secret('voice-realtime-principal-signing', **pairs)
            document = credentials.signer_jwks(source)
            self.assertEqual([key['kid'] for key in document['keys']], ['current', 'next'])
            self.assertNotEqual(document['keys'][0]['n'], document['keys'][1]['n'])
            public = credentials.public_config('voice-p3-principal-public', {'realtime.json': json.dumps(document)}, True)
            credentials.verify_p3_public(source, public)
            public['data']['realtime.json'] = '{"keys":[]}'
            with self.assertRaisesRegex(ValueError, 'jwks_generation'):
                credentials.verify_p3_public(source, public)
            duplicate = secret('voice-realtime-principal-signing', **{'current.pem': pairs['current.pem'], 'next.pem': pairs['current.pem']})
            with self.assertRaisesRegex(ValueError, 'duplicate_rotation'):
                credentials.signer_jwks(duplicate)
            source['data']['active-kid'] = base64.b64encode(b'current').decode()
            with self.assertRaisesRegex(ValueError, 'realtime_signer_keys'):
                credentials.signer_jwks(source)

    def test_chain_key_pair_hostname_and_directional_eku_are_verified(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch)
            def run(*args):
                result = subprocess.run(['openssl', *args], capture_output=True, cwd=root)
                self.assertEqual(result.returncode, 0, 'synthetic certificate fixture failed')
            run('req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '2', '-subj', '/CN=fixture-root',
                '-addext', 'basicConstraints=critical,CA:TRUE', '-keyout', 'ca.key', '-out', 'ca.crt')
            run('req', '-new', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=voice-test', '-keyout', 'tls.key', '-out', 'tls.csr')
            (root / 'extensions').write_text('basicConstraints=critical,CA:FALSE\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:voice-test\n')
            run('x509', '-req', '-in', 'tls.csr', '-CA', 'ca.crt', '-CAkey', 'ca.key', '-CAcreateserial',
                '-days', '2', '-extfile', 'extensions', '-out', 'tls.crt')
            cert, key, ca = [(root / name).read_bytes() for name in ('tls.crt', 'tls.key', 'ca.crt')]
            result = credentials.verify_certificate(cert, key, ca, 'voice-test', 'serverAuth')
            self.assertEqual(set(result), {'certificate_sha256', 'public_key_sha256'})
            for bad_key, host, purpose in ((ca and (root / 'ca.key').read_bytes(), 'voice-test', 'serverAuth'),
                                            (key, 'foreign-host', 'serverAuth'), (key, 'voice-test', 'clientAuth')):
                with self.assertRaisesRegex(ValueError, 'certificate'):
                    credentials.verify_certificate(cert, bad_key, ca, host, purpose)
            for extension in ('basicConstraints=critical,CA:FALSE\nextendedKeyUsage=serverAuth\n',
                              'basicConstraints=critical,CA:FALSE\nsubjectAltName=DNS:voice-test\n',
                              'basicConstraints=critical,CA:TRUE\nsubjectAltName=DNS:voice-test\nextendedKeyUsage=serverAuth\n'):
                (root / 'extensions').write_text(extension)
                run('x509', '-req', '-in', 'tls.csr', '-CA', 'ca.crt', '-CAkey', 'ca.key', '-CAcreateserial',
                    '-days', '2', '-extfile', 'extensions', '-out', 'tls.crt')
                with self.assertRaisesRegex(ValueError, 'certificate'):
                    credentials.verify_certificate((root / 'tls.crt').read_bytes(), key, ca, 'voice-test', 'serverAuth')


if __name__ == '__main__':
    unittest.main()
