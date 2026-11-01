"""Offline contracts for privately generated staging GIS credentials."""
import base64
import copy
import hashlib
import importlib
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch


def generate():
    # A missing implementation is represented as a missing capability so this
    # test fails at the observable contract, not at module collection.
    try:
        module = importlib.import_module('staging_gis_material')
    except ModuleNotFoundError as error:
        if error.name != 'staging_gis_material': raise
        return None
    function = getattr(module, 'generate_runtime_material', None)
    return function() if callable(function) else None


def captured_source(root):
    checkout = Path(__file__).resolve().parents[3]
    paths = ['deploy/staging/services.yaml', 'scripts/dev/migrations.py',
        'docker/postgres/provision-gameintegration-runtime-role.sql', 'docker/postgres/verify-gameintegration-runtime-role.sql',
        'deploy/templates/provision-gameintegration-runtime-role-job.yaml', 'deploy/templates/migrate-game-integration-db-job.yaml',
        'deploy/templates/verify-game-integration-db-job.yaml']
    paths += [p.relative_to(checkout).as_posix() for p in sorted((checkout/'src/backend/migrations/game_integration_db').glob('*.sql'))]
    capture = {}
    for relative in paths:
        dest = root / relative; dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(checkout / relative, dest)
        capture[relative] = hashlib.sha256(dest.read_bytes()).hexdigest()
    return {'source_sha':'a'*40,'source_files':capture}


class GeneratedMaterialTest(unittest.TestCase):
    def test_source_contract_uses_real_captured_sql_and_caller_wiring(self):
        module = importlib.import_module('staging_gis_material')
        checkout = Path(__file__).resolve().parents[3]
        paths = ['deploy/staging/services.yaml', 'scripts/dev/migrations.py',
            'docker/postgres/provision-gameintegration-runtime-role.sql',
            'docker/postgres/verify-gameintegration-runtime-role.sql',
            'deploy/templates/provision-gameintegration-runtime-role-job.yaml',
            'deploy/templates/migrate-game-integration-db-job.yaml',
            'deploy/templates/verify-game-integration-db-job.yaml']
        paths += [p.relative_to(checkout).as_posix() for p in sorted(
            (checkout/'src/backend/migrations/game_integration_db').glob('*.sql'))]
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); capture = {}
            for relative in paths:
                dest = root / relative; dest.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(checkout / relative, dest)
                capture[relative] = hashlib.sha256(dest.read_bytes()).hexdigest()
            approved = {'source_sha':'a'*40,'source_files':capture}
            contract = getattr(module, 'source_contract', None)
            self.assertTrue(callable(contract), 'source-bound GIS material contract is required')
            result, digest = contract(approved, root)
            self.assertEqual(result['schema'], 'voice-staging-gis-material-source-contract-v1')
            self.assertEqual(digest, hashlib.sha256(json.dumps(result, sort_keys=True,
                separators=(',', ':'), ensure_ascii=True).encode()).hexdigest())
            verify_sql = root/'docker/postgres/verify-gameintegration-runtime-role.sql'
            original_verifier = verify_sql.read_bytes()
            changed_verifier = original_verifier.replace(b'migration_version <> 33',
                b'migration_version != 30 -- migration_version <> 33')
            self.assertNotEqual(changed_verifier, original_verifier)
            verify_sql.write_bytes(changed_verifier)
            approved['source_files']['docker/postgres/verify-gameintegration-runtime-role.sql'] = hashlib.sha256(changed_verifier).hexdigest()
            with self.assertRaises(module.MaterialError): contract(approved, root)
            verify_sql.write_bytes(original_verifier)
            approved['source_files']['docker/postgres/verify-gameintegration-runtime-role.sql'] = hashlib.sha256(original_verifier).hexdigest()
            service = root/'deploy/staging/services.yaml'; raw = service.read_bytes()
            service.write_bytes(raw.replace(b'key: GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64',
                                            b'key: GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64'))
            approved['source_files']['deploy/staging/services.yaml'] = hashlib.sha256(service.read_bytes()).hexdigest()
            with self.assertRaises(module.MaterialError): contract(approved, root)

            for old, new in (
                (b'https://voice-gameintegration:8443/internal/v1/principal/jwks.json', b'https://foreign:8443/internal/v1/principal/jwks.json'),
                (b'secretName: voice-gameintegration-jwks-tls', b'secretName: voice-foreign-tls'),
                (b'{name: principal-jwks, port: 8443}', b'{name: principal-jwks, port: 9443}'),
                (b'{name: principal-jwks, port: 8443}', b'{name: principal-jwks, port: 8443, targetPort: 9999}'),
                (b'{name: principal-jwks, port: 8443}', b'{name: principal-jwks, port: 8443, protocol: UDP}'),
                (b'ports: [{name: http, port: 8080}, {name: principal-jwks, port: 8443}]',
                 b'ports: [{name: http, port: 8080}, {name: principal-jwks, port: 8443}, {name: http, port: 8080}]'),
                (b'matchLabels: {app: voice-gameintegration}', b'matchLabels: {app: foreign}'),
                (b'matchLabels: {app: voice-gameintegration}',
                 b'matchLabels: {app: voice-gameintegration}\n    matchExpressions: [{key: app, operator: NotIn, values: [voice-gameintegration]}]'),
                (b'GAME_INTEGRATION_PRINCIPAL_JWKS_LISTEN_ADDR, value: ":8443"',
                 b'GAME_INTEGRATION_PRINCIPAL_JWKS_LISTEN_ADDR, value: ":9443"'),
                (b'"gameintegration":"https://voice-gameintegration:8443/internal/v1/principal/jwks.json"',
                 b'"gameintegration":"https://foreign","gameintegration":"https://voice-gameintegration:8443/internal/v1/principal/jwks.json"'),
                (b'- {name: principal-tls, mountPath: /etc/voice/principal/tls, readOnly: true}',
                 b'- {name: principal-tls, mountPath: /etc/voice/principal/tls, readOnly: true}\n            - {name: other, mountPath: /etc/voice/principal/tls, readOnly: true}'),
                (b'- name: principal-signing\n          secret: {secretName: voice-gameintegration-principal-signing',
                 b'- name: principal-signing\n          secret: {secretName: voice-gameintegration-principal-signing, items: []}\n        - name: principal-signing\n          secret: {secretName: voice-gameintegration-principal-signing')):
                changed = raw.replace(old,new)
                self.assertNotEqual(changed,raw)
                service.write_bytes(changed)
                approved['source_files']['deploy/staging/services.yaml'] = hashlib.sha256(changed).hexdigest()
                with self.assertRaises(module.MaterialError): contract(approved, root)
            service.write_bytes(raw); approved['source_files']['deploy/staging/services.yaml'] = hashlib.sha256(raw).hexdigest()
            extra = root/'src/backend/migrations/game_integration_db/foreign.sql'; extra.write_text('SELECT 1;')
            with self.assertRaises(module.MaterialError): contract(approved, root)

    def test_source_contract_blocks_migrations_newer_than_the_fixed_verifier(self):
        module = importlib.import_module('staging_gis_material')
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); approved = captured_source(root)
            self.assertTrue(any('000032_' in name for name in approved['source_files']))
            for suffix in ('up','down'):
                relative='src/backend/migrations/game_integration_db/000033_fixture.'+suffix+'.sql'
                path=root/relative; path.write_text('SELECT 1;\n')
                approved['source_files'][relative]=hashlib.sha256(path.read_bytes()).hexdigest()
            with self.assertRaises(module.MaterialError): module.source_contract(approved,root)
    def test_private_keys_and_partner_credentials_are_fresh_and_compatible(self):
        values = generate()
        self.assertIsInstance(values, dict, 'GIS material generation is required')
        self.assertEqual(set(values), {'GAME_INTEGRATION_DB_PASSWORD', 'GAME_INTEGRATION_DATABASE_URL',
            'voice-gameintegration-principal-signing', 'voice-gameintegration-workload-credentials'})
        password = values['GAME_INTEGRATION_DB_PASSWORD']
        self.assertIsInstance(password, bytes)
        self.assertGreaterEqual(len(password), 32)
        self.assertTrue(values['GAME_INTEGRATION_DATABASE_URL'] ==
            b'postgres://gameintegration_runtime:' + password +
            b'@voice-postgres:5432/game_integration_db?sslmode=disable')
        signers = values['voice-gameintegration-principal-signing']
        self.assertEqual(set(signers), {'current.pem', 'next.pem', 'active-kid'})
        self.assertEqual(signers['active-kid'], b'current')
        self.assertTrue(signers['current.pem'] != signers['next.pem'])
        for name in ('current.pem', 'next.pem'):
            self.assertIsInstance(signers[name], bytes)
            self.assertTrue(signers[name].startswith(b'-----BEGIN PRIVATE KEY-----'))
            result = subprocess.run(['openssl', 'rsa', '-check', '-noout'], input=signers[name],
                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=15)
            self.assertEqual(result.returncode, 0)
            public = subprocess.run(['openssl', 'pkey', '-pubout'], input=signers[name],
                check=True, capture_output=True, timeout=15).stdout
            details = subprocess.run(['openssl', 'rsa', '-pubin', '-text', '-noout'], input=public,
                check=True, capture_output=True, timeout=15).stdout
            self.assertIn(b'Public-Key: (3072 bit)', details)
        workload = values['voice-gameintegration-workload-credentials']
        notification = 'GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64'
        realtime = 'GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64'
        self.assertEqual(set(workload), {notification, realtime})
        left = base64.b64decode(workload[notification], validate=True)
        right = base64.b64decode(workload[realtime], validate=True)
        self.assertEqual((len(left), len(right)), (32, 32))
        self.assertTrue(left != right)
        other = generate()
        self.assertTrue(other['GAME_INTEGRATION_DB_PASSWORD'] != password)
        self.assertTrue(other['voice-gameintegration-principal-signing']['current.pem'] != signers['current.pem'])

    def test_duplicate_workload_entropy_is_rejected(self):
        try:
            module = importlib.import_module('staging_gis_material')
        except ModuleNotFoundError as error:
            if error.name != 'staging_gis_material': raise
            self.fail('GIS material generation is required')
        with patch.object(module.secrets, 'token_bytes', return_value=b'x' * 32):
            with self.assertRaises(RuntimeError):
                module.generate_runtime_material()

    def test_duplicate_or_failed_signer_does_not_expose_child_output(self):
        module = importlib.import_module('staging_gis_material')
        with patch.object(module, '_signer', return_value=b'identical-private-material'):
            with self.assertRaisesRegex(module.MaterialError, 'staging_gis_signer_generation_failed'):
                module.generate_runtime_material()
        error = subprocess.CalledProcessError(1, ['openssl'], stderr=b'private-material-must-not-escape')
        with patch.object(module.subprocess, 'run', side_effect=error):
            with self.assertRaises(module.MaterialError) as caught:
                module.generate_runtime_material()
        self.assertNotIn('private-material-must-not-escape', str(caught.exception))

    def test_invalid_database_password_entropy_is_rejected_before_key_generation(self):
        module = importlib.import_module('staging_gis_material')
        with patch.object(module.secrets, 'token_urlsafe', return_value='!' * 43), \
                patch.object(module, '_signer') as signer:
            with self.assertRaisesRegex(module.MaterialError, 'staging_gis_entropy_invalid'):
                module.generate_runtime_material()
            signer.assert_not_called()


class AppSecretObservationTest(unittest.TestCase):
    def secret(self):
        return {'apiVersion': 'v1', 'kind': 'Secret', 'type': 'Opaque',
            'metadata': {'name': 'voice-app-secrets', 'namespace': 'voice-staging',
                         'uid': '00000000-0000-0000-0000-000000000101', 'resourceVersion': '7'},
            'data': {'POSTGRES_PASSWORD': base64.b64encode(b'other-private-value').decode()}}

    def test_absent_pair_is_reported_without_disclosing_or_changing_other_data(self):
        module = importlib.import_module('staging_gis_material')
        row = self.secret()
        before = copy.deepcopy(row)
        observe = getattr(module, 'inspect_app_secret', lambda _row: None)
        result = observe(row)
        self.assertIsInstance(result, dict, 'read-only app Secret observation is required')
        self.assertEqual(result, {'uid': row['metadata']['uid'], 'resourceVersion': '7',
            'data_sha256': hashlib.sha256(json.dumps(row['data'], sort_keys=True,
                separators=(',', ':')).encode()).hexdigest(), 'credentials': 'absent'})
        self.assertEqual(row, before)
        self.assertNotIn('other-private-value', json.dumps(result))

    def test_partial_or_inconsistent_pair_and_foreign_identity_are_rejected(self):
        module = importlib.import_module('staging_gis_material')
        observe = getattr(module, 'inspect_app_secret', lambda _row: None)
        for fault in ('partial', 'wrong_dsn', 'unsafe_password', 'namespace', 'terminating', 'bad_encoding', 'immutable'):
            with self.subTest(fault=fault):
                row = self.secret()
                if fault in ('partial', 'wrong_dsn'):
                    row['data']['GAME_INTEGRATION_DB_PASSWORD'] = base64.b64encode(b'credential').decode()
                if fault == 'wrong_dsn':
                    row['data']['GAME_INTEGRATION_DATABASE_URL'] = base64.b64encode(b'postgres://foreign').decode()
                if fault == 'unsafe_password':
                    password = b'bad@foreign'
                    row['data']['GAME_INTEGRATION_DB_PASSWORD'] = base64.b64encode(password).decode()
                    row['data']['GAME_INTEGRATION_DATABASE_URL'] = base64.b64encode(
                        b'postgres://gameintegration_runtime:' + password +
                        b'@voice-postgres:5432/game_integration_db?sslmode=disable').decode()
                if fault == 'namespace': row['metadata']['namespace'] = 'voice-dev'
                if fault == 'terminating': row['metadata']['deletionTimestamp'] = '2026-10-29T00:00:00Z'
                if fault == 'bad_encoding': row['data']['POSTGRES_PASSWORD'] = 'not-base64!'
                if fault == 'immutable': row['immutable'] = True
                with self.assertRaisesRegex(module.MaterialError, 'staging_gis_app_secret_invalid'):
                    observe(row)

    def test_matching_present_pair_is_observed_not_adopted(self):
        module = importlib.import_module('staging_gis_material')
        row = self.secret()
        password = b'private-fixture-password'
        row['data']['GAME_INTEGRATION_DB_PASSWORD'] = base64.b64encode(password).decode()
        row['data']['GAME_INTEGRATION_DATABASE_URL'] = base64.b64encode(
            b'postgres://gameintegration_runtime:' + password +
            b'@voice-postgres:5432/game_integration_db?sslmode=disable').decode()
        result = module.inspect_app_secret(row)
        self.assertEqual(result['credentials'], 'present')
        self.assertNotIn('authorized', result)
        self.assertNotIn('private-fixture-password', json.dumps(result))


class JWKSTLSMaterialTest(unittest.TestCase):
    def create_ca(self, root, name, days='120'):
        key, cert = root / (name + '.key'), root / (name + '.crt')
        subprocess.run(['openssl', 'genpkey', '-algorithm', 'RSA', '-pkeyopt', 'rsa_keygen_bits:2048',
                        '-out', str(key)], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=30)
        key.chmod(0o600)
        subprocess.run(['openssl', 'req', '-x509', '-new', '-key', str(key), '-subj', '/CN=fixture-ca',
                        '-days', days, '-out', str(cert)], check=True, stdout=subprocess.DEVNULL,
                       stderr=subprocess.DEVNULL, timeout=30)
        return cert.read_bytes(), key.read_bytes()

    def test_leaf_has_matching_key_hostname_server_usage_and_trusted_chain(self):
        module = importlib.import_module('staging_gis_material')
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            ca, key = self.create_ca(root, 'first')
            issue = getattr(module, 'issue_jwks_leaf', lambda *_args: None)
            leaf = issue(ca, key, ca)
            self.assertIsInstance(leaf, dict, 'GIS JWKS leaf generation is required')
            self.assertEqual(set(leaf), {'tls.crt', 'tls.key'})
            cert_path, key_path = root / 'leaf.crt', root / 'leaf.key'
            cert_path.write_bytes(leaf['tls.crt']); key_path.write_bytes(leaf['tls.key'])
            cert_path.chmod(0o600); key_path.chmod(0o600)
            result = subprocess.run(['openssl', 'verify', '-CAfile', str(root / 'first.crt'),
                '-verify_hostname', 'voice-gameintegration', '-purpose', 'sslserver', str(cert_path)],
                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=15)
            self.assertEqual(result.returncode, 0)
            cert_public = subprocess.run(['openssl', 'x509', '-in', str(cert_path), '-pubkey', '-noout'],
                check=True, capture_output=True, timeout=15).stdout
            key_public = subprocess.run(['openssl', 'pkey', '-in', str(key_path), '-pubout'],
                check=True, capture_output=True, timeout=15).stdout
            self.assertTrue(cert_public == key_public)

    def test_foreign_trust_or_mismatched_ca_key_fails_closed(self):
        module = importlib.import_module('staging_gis_material')
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            ca, key = self.create_ca(root, 'first')
            other_ca, other_key = self.create_ca(root, 'second')
            for candidate_ca, candidate_key, trust in ((ca, key, other_ca), (ca, other_key, ca)):
                with self.subTest(fault='foreign-trust' if trust == other_ca else 'mismatched-key'):
                    with self.assertRaisesRegex(module.MaterialError, 'staging_gis_tls_generation_failed'):
                        module.issue_jwks_leaf(candidate_ca, candidate_key, trust)

    def test_issuer_must_cover_the_fixed_leaf_lifetime(self):
        module = importlib.import_module('staging_gis_material')
        with tempfile.TemporaryDirectory() as tmp:
            ca, key = self.create_ca(Path(tmp), 'short', days='2')
            with self.assertRaisesRegex(module.MaterialError, 'staging_gis_tls_generation_failed'):
                module.issue_jwks_leaf(ca, key, ca)

    def test_verifier_rejects_leaf_without_exact_generated_profile(self):
        module = importlib.import_module('staging_gis_material')
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); ca, ca_key = self.create_ca(root, 'issuer')
            key = module._signer(); (root / 'leaf.key').write_bytes(key)
            subprocess.run(['openssl', 'req', '-new', '-key', str(root / 'leaf.key'), '-subj', '/CN=voice-gameintegration',
                '-out', str(root / 'leaf.csr')], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            for extension in ('basicConstraints=critical,CA:TRUE\nextendedKeyUsage=serverAuth\n',
                              'basicConstraints=critical,CA:FALSE\n'):
                (root / 'extensions').write_text(extension + 'keyUsage=critical,digitalSignature,keyEncipherment\nsubjectAltName=DNS:voice-gameintegration\n')
                subprocess.run(['openssl', 'x509', '-req', '-in', str(root / 'leaf.csr'), '-CA', str(root / 'issuer.crt'),
                    '-CAkey', str(root / 'issuer.key'), '-set_serial', '1', '-days', '1', '-extfile', str(root / 'extensions'),
                    '-out', str(root / 'leaf.crt')], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                with self.assertRaises((ValueError, subprocess.SubprocessError)):
                    module._verify_tls_material((root / 'leaf.crt').read_bytes(), key, ca, ca_key, ca)

    def test_private_create_payloads_keep_shared_data_and_fixed_owned_secrets(self):
        module = importlib.import_module('staging_gis_material')
        before = AppSecretObservationTest().secret(); original = copy.deepcopy(before)
        with tempfile.TemporaryDirectory() as tmp:
            ca, key = self.create_ca(Path(tmp), 'issuer')
            builder = getattr(module, 'build_create_payloads', None)
            self.assertTrue(callable(builder), 'private GIS payload builder is required')
            payloads = builder('123456abcdef', before, ca, key, ca)
        self.assertEqual(before, original)
        self.assertEqual(set(payloads), {'gis-signer', 'gis-workload', 'gis-tls', 'gis-app-credentials'})
        for item in ('gis-signer', 'gis-workload', 'gis-tls'):
            obj = json.loads(payloads[item])
            self.assertEqual(obj['metadata']['annotations']['voice.poryadok.io/prerequisites-operation'], '123456abcdef')
            self.assertEqual(obj['metadata']['namespace'], 'voice-staging')
        app = json.loads(payloads['gis-app-credentials'])
        self.assertEqual(app['before'], original)
        self.assertEqual(app['after_data']['POSTGRES_PASSWORD'], original['data']['POSTGRES_PASSWORD'])
        self.assertEqual(set(app['after_data']) - set(original['data']),
                         {'GAME_INTEGRATION_DB_PASSWORD', 'GAME_INTEGRATION_DATABASE_URL'})

    def test_builder_refuses_unowned_existing_or_partial_app_credentials(self):
        module = importlib.import_module('staging_gis_material')
        builder = getattr(module, 'build_create_payloads', None)
        self.assertTrue(callable(builder), 'private GIS payload builder is required')
        for both in (False, True):
            row = AppSecretObservationTest().secret()
            password = b'fixture-pass'
            row['data']['GAME_INTEGRATION_DB_PASSWORD'] = base64.b64encode(password).decode()
            if both:
                row['data']['GAME_INTEGRATION_DATABASE_URL'] = base64.b64encode(
                    b'postgres://gameintegration_runtime:' + password + b'@voice-postgres:5432/game_integration_db?sslmode=disable').decode()
            with patch.object(module, 'generate_runtime_material') as generate:
                with self.assertRaises(module.MaterialError): builder('123456abcdef', row, b'ca', b'key', b'ca')
                generate.assert_not_called()


class ClosedPlanTest(unittest.TestCase):
    def plan(self):
        uid = '00000000-0000-0000-0000-000000000101'
        names = [('gis-signer', 'voice-gameintegration-principal-signing'),
                 ('gis-workload', 'voice-gameintegration-workload-credentials'),
                 ('gis-tls', 'voice-gameintegration-jwks-tls'),
                 ('gis-app-credentials', 'voice-app-secrets')]
        effects = []
        for effect_id, name in names:
            before = ({'uid': uid, 'resourceVersion': '7', 'data_sha256': 'a' * 64}
                      if effect_id == 'gis-app-credentials' else {'absent': True, 'list_rv': '5'})
            effects.append({'id': effect_id, 'kind': 'Secret', 'name': name, 'namespace': 'voice-staging',
                            'before': before, 'desired_sha256': 'b' * 64, 'payload_sha256': 'c' * 64})
        return {'schema': 'voice-staging-gis-material-plan-v1', 'operation': '123456abcdef',
            'namespace_uid': uid, 'source_capture_sha256': 'd' * 64, 'source_contract_sha256': 'e' * 64,
            'issuer': {'input_receipt_sha256': '1' * 64, 'ca_secret_uid': uid,
                'ca_public_sha256': '2' * 64, 'issuer_certificate_sha256': '3' * 64,
                'private_input_sha256': '4' * 64}, 'effects': effects, 'preserved': {}}

    def test_valid_plan_has_only_closed_fixed_effects(self):
        module = importlib.import_module('staging_gis_material')
        validate = getattr(module, 'validate_plan', lambda _value: None)
        self.assertTrue(validate(self.plan()), 'closed GIS child plan validation is required')
        preserved = self.plan()
        effect = preserved['effects'].pop()
        preserved['preserved'][effect['id']] = {'authority': {'operation': 'abcdef123456',
            'receipt_sha256': 'f' * 64}, 'observed': {'kind': effect['kind'], 'name': effect['name'],
            'namespace': effect['namespace'], 'uid': '00000000-0000-0000-0000-000000000202',
            'resourceVersion': '8', 'semantic_sha256': '9' * 64}}
        self.assertTrue(validate(preserved))

    def test_plan_builder_hashes_actual_fixed_payloads_and_keeps_preserved_refs(self):
        module = importlib.import_module('staging_gis_material')
        base = self.plan()
        prior = base['effects'].pop(0)
        preserved = {prior['id']: {'authority': {'operation': 'abcdef123456', 'receipt_sha256': 'f' * 64},
            'observed': {'kind': 'Secret', 'name': prior['name'], 'namespace': 'voice-staging',
                'uid': '00000000-0000-0000-0000-000000000202', 'resourceVersion': '5', 'semantic_sha256': '9' * 64}}}
        before = AppSecretObservationTest().secret()
        added = {'GAME_INTEGRATION_DB_PASSWORD': base64.b64encode(b'password').decode(),
            'GAME_INTEGRATION_DATABASE_URL': base64.b64encode(
                b'postgres://gameintegration_runtime:password@voice-postgres:5432/game_integration_db?sslmode=disable').decode()}
        payloads = {}
        for effect in base['effects'][:-1]:
            row = {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {'name': effect['name'],
                'namespace': 'voice-staging', 'annotations': {'voice.poryadok.io/prerequisites-operation': base['operation']}},
                'type': 'kubernetes.io/tls' if effect['id'] == 'gis-tls' else 'Opaque', 'data': {'fixture': 'YQ=='}}
            payloads[effect['id']] = json.dumps(row, sort_keys=True, separators=(',', ':')).encode()
        payloads['gis-app-credentials'] = json.dumps({'before': before, 'after_data': {**before['data'], **added}},
                                                      sort_keys=True, separators=(',', ':')).encode()
        before_map = {item['id']: item['before'] for item in base['effects']}
        before_map['gis-app-credentials'] = {key: module.inspect_app_secret(before)[key]
                                              for key in ('uid', 'resourceVersion', 'data_sha256')}
        builder = getattr(module, 'build_plan', None)
        self.assertTrue(callable(builder), 'closed private GIS plan builder is required')
        plan, retained = builder(base['operation'], base['namespace_uid'], base['source_capture_sha256'],
                                 base['source_contract_sha256'], base['issuer'], before_map, preserved, payloads)
        self.assertEqual(retained, payloads)
        self.assertTrue(module.validate_plan(plan))
        self.assertEqual(plan['preserved'], preserved)
        for effect in plan['effects']:
            self.assertEqual(effect['payload_sha256'], hashlib.sha256(payloads[effect['id']]).hexdigest())
        bad = dict(before_map); bad['foreign'] = {'absent': True, 'list_rv': '3'}
        with self.assertRaises(module.MaterialError):
            builder(base['operation'], base['namespace_uid'], base['source_capture_sha256'],
                    base['source_contract_sha256'], base['issuer'], bad, preserved, payloads)

    def test_unknown_missing_duplicate_or_unbound_effects_are_rejected(self):
        module = importlib.import_module('staging_gis_material')
        validate = getattr(module, 'validate_plan', lambda _value: None)
        for fault in ('missing', 'duplicate', 'order', 'foreign_name', 'namespace', 'extra_field',
                      'weak_hash', 'unbound_ca', 'untrusted_preserved'):
            with self.subTest(fault=fault):
                plan = self.plan()
                if fault == 'missing': plan['effects'].pop()
                elif fault == 'duplicate': plan['effects'].append(copy.deepcopy(plan['effects'][0]))
                elif fault == 'order': plan['effects'].reverse()
                elif fault == 'foreign_name': plan['effects'][0]['name'] = 'voice-other-secret'
                elif fault == 'namespace': plan['effects'][0]['namespace'] = 'voice-dev'
                elif fault == 'extra_field': plan['effects'][0]['action'] = 'replace'
                elif fault == 'weak_hash': plan['effects'][0]['payload_sha256'] = 'bad'
                elif fault == 'unbound_ca': plan['issuer']['private_input_sha256'] = None
                else:
                    effect = plan['effects'].pop()
                    plan['preserved'][effect['id']] = {'observed': {'uid': 'unowned'}}
                with self.assertRaisesRegex(module.MaterialError, 'staging_gis_plan_invalid'):
                    validate(plan)

    def test_execution_uses_parent_fixed_effect_dispatch_in_order(self):
        module = importlib.import_module('staging_gis_material')
        events = []
        parent = SimpleNamespace(
            require_child=Mock(return_value=self.plan()),
            record_pending=Mock(side_effect=lambda _s, slot, item: events.append(('pending', slot, item))),
            submit_effect=Mock(side_effect=lambda _s, slot, item: events.append(('submit', slot, item)) or {'id': item}),
            record_done=Mock(side_effect=lambda _s, slot, item, _observation: events.append(('done', slot, item))))
        execute = getattr(module, 'execute', lambda _session: None)
        session = SimpleNamespace(check=Mock())
        with patch.dict(sys.modules, {'staging_prerequisites': parent}):
            execute(session)
        parent.require_child.assert_called_once_with(session, 'gis_material')
        self.assertEqual(events, [(phase, 'gis_material', effect['id']) for effect in self.plan()['effects']
                                  for phase in ('pending', 'submit', 'done')])

    def test_unknown_effect_outcome_stops_before_done_and_later_effects(self):
        module = importlib.import_module('staging_gis_material')
        parent = SimpleNamespace(require_child=Mock(return_value=self.plan()), record_pending=Mock(),
            submit_effect=Mock(side_effect=OSError('unknown response')), record_done=Mock())
        execute = getattr(module, 'execute', lambda _session: None)
        with patch.dict(sys.modules, {'staging_prerequisites': parent}):
            with self.assertRaisesRegex(OSError, 'unknown response'): execute(object())
        parent.record_pending.assert_called_once()
        parent.submit_effect.assert_called_once()
        parent.record_done.assert_not_called()


class AppEffectVerificationTest(ClosedPlanTest):
    def test_preflight_validates_every_private_payload_before_dispatch(self):
        module = importlib.import_module('staging_gis_material')
        import guard
        base = self.plan(); before = AppSecretObservationTest().secret()
        with tempfile.TemporaryDirectory() as tmp:
            source = Path(tmp)/'installed'/'sources'/base['operation']
            approved = captured_source(source)
            _, base['source_contract_sha256'] = module.source_contract(approved, source)
            ca, key = JWKSTLSMaterialTest().create_ca(Path(tmp), 'issuer')
            base['issuer']['ca_public_sha256'] = hashlib.sha256(ca).hexdigest()
            base['issuer']['issuer_certificate_sha256'] = hashlib.sha256(ca).hexdigest()
            issuer_ca = {'apiVersion': 'v1', 'kind': 'Secret', 'type': 'Opaque', 'metadata': {
                'name': 'voice-media-ca', 'namespace': 'voice-staging', 'uid': base['issuer']['ca_secret_uid'],
                'resourceVersion': '11'}, 'data': {'jwks-ca.crt': base64.b64encode(ca).decode()}}
            issuer = {'schema': 'voice-staging-gis-issuer-input-v1', 'receipt': {}, 'ca_certificate_pem': ca.decode(),
                'ca_private_key_pem': key.decode(), 'jwks_ca_bundle_pem': ca.decode(), 'ca_secret': issuer_ca}
            payloads = module.build_create_payloads(base['operation'], before, ca, key, ca)
            before_map = {effect['id']: effect['before'] for effect in base['effects']}
            before_map['gis-app-credentials'] = {key: module.inspect_app_secret(before)[key]
                                                  for key in ('uid', 'resourceVersion', 'data_sha256')}
            plan, payloads = module.build_plan(base['operation'], base['namespace_uid'], base['source_capture_sha256'],
                                               base['source_contract_sha256'], base['issuer'], before_map, {}, payloads)
            namespace = {'metadata': {'name': 'voice-staging', 'uid': base['namespace_uid']}}
            kube = SimpleNamespace(get=Mock(side_effect=lambda _kind, name: before if name == 'voice-app-secrets' else namespace if name == 'voice-staging' else issuer_ca),
                run=Mock(return_value={'apiVersion': 'v1', 'kind': 'SecretList', 'metadata': {'resourceVersion': '5'}, 'items': []}))
            parent = SimpleNamespace(require_child=Mock(return_value=plan), load_payload=Mock(side_effect=lambda _s, _slot, name: payloads[name]),
                load_issuer=Mock(return_value=issuer), load_authority=Mock(return_value={
                    'approved':approved,'execution':{'fixture':'protected'},'source':str(source)}), submit_effect=Mock())
            preflight = getattr(module, 'preflight_payloads', None)
            self.assertTrue(callable(preflight), 'fixed GIS preflight is required')
            with patch.dict(sys.modules, {'staging_prerequisites': parent, 'stage_runtime': SimpleNamespace(Kube=lambda: kube)}), patch.object(guard, 'ROOT', Path(tmp)):
                preflight(SimpleNamespace(operation='123456abcdef',check=Mock()))
                parent.submit_effect.assert_not_called()
                service=source/'deploy/staging/services.yaml'; source_bytes=service.read_bytes()
                service.write_bytes(source_bytes+b'\n')
                with self.assertRaises(module.MaterialError):
                    preflight(SimpleNamespace(operation='123456abcdef',check=Mock()))
                service.write_bytes(source_bytes)
                guarded=SimpleNamespace(operation='123456abcdef',check=Mock(side_effect=[None, RuntimeError('claim changed')]))
                with self.assertRaisesRegex(RuntimeError,'claim changed'): preflight(guarded)
                self.assertEqual(guarded.check.call_count,2)
                valid_listing = {'apiVersion': 'v1', 'kind': 'SecretList', 'metadata': {'resourceVersion': '5'}, 'items': []}
                for fault in ('present', 'rv', 'continued', 'wrong_kind'):
                    listing = copy.deepcopy(valid_listing)
                    if fault == 'present': listing['items'] = [{'metadata': {'name': 'foreign'}}]
                    elif fault == 'rv': listing['metadata']['resourceVersion'] = ''
                    elif fault == 'continued': listing['metadata']['continue'] = 'next'
                    else: listing['kind'] = 'List'
                    kube.run.return_value = listing
                    with self.subTest(fault=fault):
                        with self.assertRaises(module.MaterialError): preflight(SimpleNamespace(operation='123456abcdef',check=Mock()))
                kube.run.return_value = valid_listing
                changed_rv = copy.deepcopy(valid_listing); changed_rv['metadata']['resourceVersion'] = '6'
                kube.run.return_value = changed_rv
                preflight(SimpleNamespace(operation='123456abcdef',check=Mock()))
                kube.run.return_value = valid_listing
                workload = json.loads(payloads['gis-workload'])
                workload['metadata']['annotations']['voice.poryadok.io/prerequisites-operation'] = 'abcdef123456'
                workload_live = copy.deepcopy(workload)
                workload_live['metadata'].update(uid='00000000-0000-0000-0000-000000000202', resourceVersion='15')
                digest = hashlib.sha256(json.dumps(workload, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
                observed = {'kind': 'Secret', 'name': 'voice-gameintegration-workload-credentials', 'namespace': 'voice-staging',
                    'uid': workload_live['metadata']['uid'], 'resourceVersion': '15', 'semantic_sha256': digest}
                authority = {'operation': 'abcdef123456', 'receipt_sha256': 'f' * 64}
                retained = {'gis-workload': {'authority': authority, 'observed': observed}}
                remaining = {k: v for k, v in payloads.items() if k != 'gis-workload'}
                mixed_before = {k: v for k, v in before_map.items() if k != 'gis-workload'}
                mixed, _ = module.build_plan(base['operation'], base['namespace_uid'], base['source_capture_sha256'],
                                             base['source_contract_sha256'], base['issuer'], mixed_before, retained, remaining)
                parent.require_child.return_value = mixed
                parent.load_preserved = Mock(return_value={'authority': authority, 'semantic': workload, 'uid': observed['uid']})
                kube.get.side_effect = lambda _kind, name: (before if name == 'voice-app-secrets' else
                    namespace if name == 'voice-staging' else workload_live if name == observed['name'] else issuer_ca)
                preflight(SimpleNamespace(operation='123456abcdef',check=Mock()))
                parent.load_preserved.assert_called_once()
                parent.require_child.return_value = plan
                paths = {'/api/v1/namespaces/voice-staging/secrets?fieldSelector=metadata.name%3D' +
                         effect['name'] + '&limit=2' for effect in plan['effects'][:-1]}
                self.assertTrue(all(call.args[0][0:2] == ['get', '--raw'] and call.args[0][2] in paths
                                    for call in kube.run.call_args_list))
                original_workload = payloads['gis-workload']
                broken = json.loads(original_workload); keys = list(broken['data']); broken['data'][keys[1]] = broken['data'][keys[0]]
                payloads['gis-workload'] = json.dumps(broken, sort_keys=True, separators=(',', ':')).encode()
                plan, payloads = module.build_plan(base['operation'], base['namespace_uid'], base['source_capture_sha256'],
                                                   base['source_contract_sha256'], base['issuer'], before_map, {}, payloads)
                parent.require_child.return_value = plan
                with self.assertRaises(module.MaterialError): preflight(SimpleNamespace(operation='123456abcdef',check=Mock()))
                payloads['gis-workload'] = original_workload
                broken_app = json.loads(payloads['gis-app-credentials'])
                broken_app['after_data']['GAME_INTEGRATION_DB_PASSWORD'] = base64.b64encode(b'x').decode()
                broken_app['after_data']['GAME_INTEGRATION_DATABASE_URL'] = base64.b64encode(
                    b'postgres://gameintegration_runtime:x@voice-postgres:5432/game_integration_db?sslmode=disable').decode()
                payloads['gis-app-credentials'] = json.dumps(broken_app, sort_keys=True, separators=(',', ':')).encode()
                plan, payloads = module.build_plan(base['operation'], base['namespace_uid'], base['source_capture_sha256'],
                                                   base['source_contract_sha256'], base['issuer'], before_map, {}, payloads)
                parent.require_child.return_value = plan
                with self.assertRaises(module.MaterialError): preflight(SimpleNamespace(operation='123456abcdef',check=Mock()))
                parent.submit_effect.assert_not_called()
    def test_preserved_effect_requires_protected_ancestry_and_fresh_exact_observation(self):
        module = importlib.import_module('staging_gis_material')
        plan = self.plan(); effect = plan['effects'].pop()
        secret = AppSecretObservationTest().secret()
        password = b'private-fixture-password'
        secret['data']['GAME_INTEGRATION_DB_PASSWORD'] = base64.b64encode(password).decode()
        secret['data']['GAME_INTEGRATION_DATABASE_URL'] = base64.b64encode(
            b'postgres://gameintegration_runtime:' + password + b'@voice-postgres:5432/game_integration_db?sslmode=disable').decode()
        semantic = {key: copy.deepcopy(secret[key]) for key in ('apiVersion', 'kind', 'type', 'data')}
        semantic['metadata'] = {'name': 'voice-app-secrets', 'namespace': 'voice-staging'}
        digest = hashlib.sha256(json.dumps(semantic, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
        observation = {'kind': 'Secret', 'name': effect['name'], 'namespace': effect['namespace'],
            'uid': secret['metadata']['uid'], 'resourceVersion': secret['metadata']['resourceVersion'], 'semantic_sha256': digest}
        authority = {'operation': 'abcdef123456', 'receipt_sha256': 'f' * 64}
        plan['preserved'][effect['id']] = {'authority': authority, 'observed': observation}
        parent = SimpleNamespace(require_child=Mock(return_value=plan), load_preserved=Mock(return_value={
            'authority': authority, 'semantic': semantic, 'uid': observation['uid']}))
        kube = SimpleNamespace(get=Mock(return_value=secret)); session = SimpleNamespace(check=Mock())
        with patch.dict(sys.modules, {'staging_prerequisites': parent, 'stage_runtime': SimpleNamespace(Kube=lambda: kube)}):
            self.assertEqual(module.verify_effect(session, effect['id'], observation), observation)
            parent.load_preserved.assert_called_with(session, 'gis_material', effect['id'])
            changed = copy.deepcopy(secret); changed['metadata']['resourceVersion'] = '9'; kube.get.return_value = changed
            with self.assertRaises(module.MaterialError): module.verify_effect(session, effect['id'], observation)
            parent.load_preserved.side_effect = RuntimeError('untrusted ancestry')
            with self.assertRaises(RuntimeError): module.verify_effect(session, effect['id'], observation)

    def test_fresh_response_binds_the_complete_app_secret_and_only_two_new_keys(self):
        module = importlib.import_module('staging_gis_material')
        before = AppSecretObservationTest().secret()
        before['metadata']['annotations'] = {'existing': 'unchanged'}
        password = b'p' * 43
        added = {'GAME_INTEGRATION_DB_PASSWORD': base64.b64encode(password).decode(),
                 'GAME_INTEGRATION_DATABASE_URL': base64.b64encode(
                     b'postgres://gameintegration_runtime:' + password + b'@voice-postgres:5432/game_integration_db?sslmode=disable').decode()}
        after = {**before['data'], **added}
        live = copy.deepcopy(before)
        live['metadata']['resourceVersion'] = '8'
        live['data'] = after
        semantic = {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {
            'name': 'voice-app-secrets', 'namespace': 'voice-staging', 'annotations': {'existing': 'unchanged'}},
            'type': 'Opaque', 'data': after}
        digest = lambda x: hashlib.sha256(json.dumps(x, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
        payload = json.dumps({'before': before, 'after_data': after}, sort_keys=True, separators=(',', ':')).encode()
        plan = self.plan()
        effect = plan['effects'][-1]
        effect['before'] = {'uid': before['metadata']['uid'], 'resourceVersion': '7', 'data_sha256': digest(before['data'])}
        effect['payload_sha256'] = hashlib.sha256(payload).hexdigest()
        effect['desired_sha256'] = digest(semantic)
        observation = {'kind': 'Secret', 'name': 'voice-app-secrets', 'namespace': 'voice-staging',
                       'uid': before['metadata']['uid'], 'resourceVersion': '8', 'semantic_sha256': digest(semantic)}
        kube = SimpleNamespace(get=Mock(return_value=live))
        parent = SimpleNamespace(require_child=Mock(return_value=plan), load_payload=Mock(return_value=payload))
        verify = getattr(module, 'verify_effect', None)
        self.assertTrue(callable(verify), 'fresh GIS effect verifier is required')
        session = SimpleNamespace(check=Mock())
        with patch.dict(sys.modules, {'staging_prerequisites': parent, 'stage_runtime': SimpleNamespace(Kube=lambda: kube)}):
            self.assertEqual(verify(session, 'gis-app-credentials', observation), observation)
            guarded = SimpleNamespace(check=Mock(side_effect=[None, RuntimeError('claim changed')]))
            with self.assertRaisesRegex(RuntimeError, 'claim changed'):
                verify(guarded, 'gis-app-credentials', observation)
            self.assertEqual(guarded.check.call_count, 2)
            changed_plan = copy.deepcopy(plan); changed_plan['source_capture_sha256'] = 'a' * 64
            parent.require_child.side_effect = [plan, changed_plan]
            with self.assertRaises(module.MaterialError):
                verify(SimpleNamespace(check=Mock()), 'gis-app-credentials', observation)
            parent.require_child.side_effect = None
            for fault in ('uid', 'resourceVersion', 'data', 'metadata'):
                with self.subTest(fault=fault):
                    changed = copy.deepcopy(live)
                    if fault == 'uid': changed['metadata']['uid'] = '00000000-0000-0000-0000-000000000202'
                    elif fault == 'resourceVersion': changed['metadata']['resourceVersion'] = '9'
                    elif fault == 'data': changed['data']['POSTGRES_PASSWORD'] = base64.b64encode(b'changed').decode()
                    else: changed['metadata']['annotations']['existing'] = 'changed'
                    kube.get.return_value = changed
                    with self.assertRaises(module.MaterialError): verify(session, 'gis-app-credentials', observation)
            kube.get.assert_any_call('secret', 'voice-app-secrets')
            for fault in ('extra_key', 'overwrite_original', 'bad_dsn', 'weak_password'):
                with self.subTest(fault=fault):
                    broken = copy.deepcopy(after)
                    if fault == 'extra_key': broken['UNRELATED'] = base64.b64encode(b'new').decode()
                    elif fault == 'overwrite_original': broken['POSTGRES_PASSWORD'] = base64.b64encode(b'new').decode()
                    elif fault == 'bad_dsn': broken['GAME_INTEGRATION_DATABASE_URL'] = base64.b64encode(b'postgres://elsewhere').decode()
                    else:
                        broken['GAME_INTEGRATION_DB_PASSWORD'] = base64.b64encode(b'x').decode()
                        broken['GAME_INTEGRATION_DATABASE_URL'] = base64.b64encode(
                            b'postgres://gameintegration_runtime:x@voice-postgres:5432/game_integration_db?sslmode=disable').decode()
                    candidate_payload = json.dumps({'before': before, 'after_data': broken}, sort_keys=True, separators=(',', ':')).encode()
                    effect['payload_sha256'] = hashlib.sha256(candidate_payload).hexdigest()
                    candidate_semantic = copy.deepcopy(semantic); candidate_semantic['data'] = broken
                    effect['desired_sha256'] = digest(candidate_semantic)
                    candidate_live = copy.deepcopy(live); candidate_live['data'] = broken
                    candidate_observation = {**observation, 'semantic_sha256': effect['desired_sha256']}
                    parent.load_payload.return_value = candidate_payload
                    kube.get.return_value = candidate_live
                    with self.assertRaises(module.MaterialError): verify(session, 'gis-app-credentials', candidate_observation)

    def test_workload_create_is_response_bound_and_uses_two_distinct_keys(self):
        module = importlib.import_module('staging_gis_material')
        plan = self.plan()
        effect = plan['effects'][1]
        secret = {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {
            'name': effect['name'], 'namespace': 'voice-staging',
            'annotations': {'voice.poryadok.io/prerequisites-operation': plan['operation']}}, 'type': 'Opaque',
            'data': {key: base64.b64encode(base64.b64encode(value)).decode() for key, value in (
                ('GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64', b'a' * 32),
                ('GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64', b'b' * 32))}}
        digest = lambda x: hashlib.sha256(json.dumps(x, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
        raw = json.dumps(secret, sort_keys=True, separators=(',', ':')).encode()
        effect['payload_sha256'] = hashlib.sha256(raw).hexdigest()
        effect['desired_sha256'] = digest(secret)
        live = copy.deepcopy(secret); live['metadata'].update(uid='00000000-0000-0000-0000-000000000202', resourceVersion='8')
        observation = {'kind': 'Secret', 'name': effect['name'], 'namespace': effect['namespace'],
            'uid': live['metadata']['uid'], 'resourceVersion': '8', 'semantic_sha256': effect['desired_sha256']}
        kube = SimpleNamespace(get=Mock(return_value=live))
        parent = SimpleNamespace(require_child=Mock(return_value=plan), load_payload=Mock(return_value=raw))
        with patch.dict(sys.modules, {'staging_prerequisites': parent, 'stage_runtime': SimpleNamespace(Kube=lambda: kube)}):
            self.assertEqual(module.verify_effect(SimpleNamespace(check=Mock()), effect['id'], observation), observation)
            broken = copy.deepcopy(secret)
            keys = list(broken['data'])
            broken['data'][keys[1]] = broken['data'][keys[0]]
            raw = json.dumps(broken, sort_keys=True, separators=(',', ':')).encode()
            effect['payload_sha256'], effect['desired_sha256'] = hashlib.sha256(raw).hexdigest(), digest(broken)
            parent.load_payload.return_value = raw
            candidate = copy.deepcopy(broken); candidate['metadata'].update(uid=observation['uid'], resourceVersion='8')
            kube.get.return_value = candidate
            with self.assertRaises(module.MaterialError):
                module.verify_effect(SimpleNamespace(check=Mock()), effect['id'], {**observation, 'semantic_sha256': effect['desired_sha256']})

    def test_signer_create_requires_two_valid_distinct_3072_bit_private_keys(self):
        module = importlib.import_module('staging_gis_material')
        plan = self.plan(); effect = plan['effects'][0]
        keys = module.generate_runtime_material()['voice-gameintegration-principal-signing']
        secret = {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {
            'name': effect['name'], 'namespace': 'voice-staging',
            'annotations': {'voice.poryadok.io/prerequisites-operation': plan['operation']}}, 'type': 'Opaque',
            'data': {key: base64.b64encode(value).decode() for key, value in keys.items()}}
        digest = lambda x: hashlib.sha256(json.dumps(x, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
        raw = json.dumps(secret, sort_keys=True, separators=(',', ':')).encode()
        effect['payload_sha256'], effect['desired_sha256'] = hashlib.sha256(raw).hexdigest(), digest(secret)
        live = copy.deepcopy(secret); live['metadata'].update(uid='00000000-0000-0000-0000-000000000202', resourceVersion='8')
        observation = {'kind': 'Secret', 'name': effect['name'], 'namespace': effect['namespace'],
            'uid': live['metadata']['uid'], 'resourceVersion': '8', 'semantic_sha256': effect['desired_sha256']}
        kube = SimpleNamespace(get=Mock(return_value=live))
        parent = SimpleNamespace(require_child=Mock(return_value=plan), load_payload=Mock(return_value=raw))
        with patch.dict(sys.modules, {'staging_prerequisites': parent, 'stage_runtime': SimpleNamespace(Kube=lambda: kube)}):
            self.assertEqual(module.verify_effect(SimpleNamespace(check=Mock()), effect['id'], observation), observation)
            broken = copy.deepcopy(secret); broken['data']['next.pem'] = broken['data']['current.pem']
            raw = json.dumps(broken, sort_keys=True, separators=(',', ':')).encode()
            effect['payload_sha256'], effect['desired_sha256'] = hashlib.sha256(raw).hexdigest(), digest(broken)
            parent.load_payload.return_value = raw
            candidate = copy.deepcopy(broken); candidate['metadata'].update(uid=observation['uid'], resourceVersion='8')
            kube.get.return_value = candidate
            with self.assertRaises(module.MaterialError):
                module.verify_effect(SimpleNamespace(check=Mock()), effect['id'], {**observation, 'semantic_sha256': effect['desired_sha256']})

    def test_tls_create_requires_protected_issuer_and_fresh_ca_secret(self):
        module = importlib.import_module('staging_gis_material')
        plan = self.plan(); effect = plan['effects'][2]
        with tempfile.TemporaryDirectory() as tmp:
            ca, key = JWKSTLSMaterialTest().create_ca(Path(tmp), 'issuer')
            leaf = module.issue_jwks_leaf(ca, key, ca)
            digest = lambda x: hashlib.sha256(json.dumps(x, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
            ca_secret = {'apiVersion': 'v1', 'kind': 'Secret', 'type': 'Opaque', 'metadata': {
                'name': 'voice-media-ca', 'namespace': 'voice-staging', 'uid': plan['issuer']['ca_secret_uid'],
                'resourceVersion': '11'}, 'data': {'jwks-ca.crt': base64.b64encode(ca).decode()}}
            plan['issuer']['ca_public_sha256'] = hashlib.sha256(ca).hexdigest()
            plan['issuer']['issuer_certificate_sha256'] = hashlib.sha256(ca).hexdigest()
            issuer = {'schema': 'voice-staging-gis-issuer-input-v1', 'receipt': {}, 'ca_certificate_pem': ca.decode(),
                      'ca_private_key_pem': key.decode(), 'jwks_ca_bundle_pem': ca.decode(), 'ca_secret': ca_secret}
            secret = {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {
                'name': effect['name'], 'namespace': 'voice-staging',
                'annotations': {'voice.poryadok.io/prerequisites-operation': plan['operation']}},
                'type': 'kubernetes.io/tls', 'data': {k: base64.b64encode(v).decode() for k, v in leaf.items()}}
            raw = json.dumps(secret, sort_keys=True, separators=(',', ':')).encode()
            effect['payload_sha256'], effect['desired_sha256'] = hashlib.sha256(raw).hexdigest(), digest(secret)
            live = copy.deepcopy(secret); live['metadata'].update(uid='00000000-0000-0000-0000-000000000202', resourceVersion='8')
            observation = {'kind': 'Secret', 'name': effect['name'], 'namespace': effect['namespace'],
                'uid': live['metadata']['uid'], 'resourceVersion': '8', 'semantic_sha256': effect['desired_sha256']}
            objects = {effect['name']: live, 'voice-media-ca': ca_secret}
            kube = SimpleNamespace(get=Mock(side_effect=lambda _kind, name: objects[name]))
            parent = SimpleNamespace(require_child=Mock(return_value=plan), load_payload=Mock(return_value=raw),
                                     load_issuer=Mock(return_value=issuer))
            with patch.dict(sys.modules, {'staging_prerequisites': parent, 'stage_runtime': SimpleNamespace(Kube=lambda: kube)}):
                self.assertEqual(module.verify_effect(SimpleNamespace(check=Mock()), effect['id'], observation), observation)
                changed_issuer=copy.deepcopy(issuer);changed_issuer['receipt']={'different':True}
                parent.load_issuer.side_effect=[issuer,changed_issuer]
                with self.assertRaises(module.MaterialError):
                    module.verify_effect(SimpleNamespace(check=Mock()), effect['id'], observation)
                parent.load_issuer.side_effect=None
                ca_calls = []
                def race(_kind, name):
                    if name == 'voice-media-ca':
                        ca_calls.append(name)
                        if len(ca_calls) == 3:
                            changed = copy.deepcopy(ca_secret); changed['metadata']['resourceVersion'] = '12'
                            return changed
                    return objects[name]
                kube.get.side_effect = race
                with self.assertRaises(module.MaterialError): module.verify_effect(SimpleNamespace(check=Mock()), effect['id'], observation)
                self.assertEqual(len(ca_calls), 3)
                kube.get.side_effect = lambda _kind, name: objects[name]
                prior = {'operation': 'abcdef123456', 'receipt_sha256': 'f' * 64}
                plan['effects'].remove(effect)
                plan['preserved'][effect['id']] = {'authority': prior, 'observed': observation}
                parent.load_preserved = Mock(return_value={'authority': prior, 'semantic': secret, 'uid': observation['uid']})
                self.assertEqual(module.verify_effect(SimpleNamespace(check=Mock()), effect['id'], observation), observation)
                objects['voice-media-ca'] = copy.deepcopy(ca_secret)
                objects['voice-media-ca']['metadata']['resourceVersion'] = '12'
                with self.assertRaises(module.MaterialError): module.verify_effect(SimpleNamespace(check=Mock()), effect['id'], observation)
                plan['preserved'].clear(); plan['effects'].insert(2, effect)
                objects['voice-media-ca'] = ca_secret
                objects['voice-media-ca'] = copy.deepcopy(ca_secret)
                objects['voice-media-ca']['metadata']['uid'] = '00000000-0000-0000-0000-000000000303'
                with self.assertRaises(module.MaterialError): module.verify_effect(SimpleNamespace(check=Mock()), effect['id'], observation)
                objects['voice-media-ca'] = ca_secret
                broken = copy.deepcopy(secret)
                broken['data']['tls.key'] = base64.b64encode(key).decode()
                broken_raw = json.dumps(broken, sort_keys=True, separators=(',', ':')).encode()
                effect['payload_sha256'], effect['desired_sha256'] = hashlib.sha256(broken_raw).hexdigest(), digest(broken)
                parent.load_payload.return_value = broken_raw
                objects[effect['name']] = copy.deepcopy(broken)
                objects[effect['name']]['metadata'].update(uid=observation['uid'], resourceVersion='8')
                with self.assertRaises(module.MaterialError):
                    module.verify_effect(SimpleNamespace(check=Mock()), effect['id'], {**observation, 'semantic_sha256': effect['desired_sha256']})


if __name__ == '__main__':
    unittest.main()
