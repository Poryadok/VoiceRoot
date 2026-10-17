import base64
import copy
import hashlib
import io
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock
from contextlib import redirect_stderr, redirect_stdout

import match_squad_pki as pki

ISSUER_PATH = Path(__file__).with_name('match-squad-pki-issuer') / 'issuer.py'
ISSUER_SPEC = importlib.util.spec_from_file_location('match_squad_pki_issuer', ISSUER_PATH)
issuer = importlib.util.module_from_spec(ISSUER_SPEC)
ISSUER_SPEC.loader.exec_module(issuer)


OPERATION = 'msq-1234567890abcdef1234567890abcdef'
SOURCE_SHA = 'a' * 64
CA_OBJECT = 'voice-match-squad-ca'
SECRET_NAMES = {
    'voice-matchmaking-principal-signing',
    'voice-matchmaking-client-tls',
    'voice-chat-match-squad-server-tls',
    'voice-voice-match-squad-server-tls',
    'voice-match-squad-replay',
    'voice-match-squad-member-replay',
}
ANNOTATION = 'voice.dev/match-squad-pki-operation'
SOURCE_SHA_ANNOTATION = 'voice.dev/match-squad-pki-source-sha256'


def encoded(value):
    return base64.b64encode(value).decode('ascii')


def manifest():
    objects = []
    objects.append({
        'apiVersion': 'v1', 'kind': 'ConfigMap',
        'metadata': {'name': CA_OBJECT, 'namespace': 'voice-dev',
                     'labels': {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'},
                     'annotations': {ANNOTATION: OPERATION, SOURCE_SHA_ANNOTATION: SOURCE_SHA}},
        'immutable': True,
        'data': {'ca.crt': '-----BEGIN CERTIFICATE-----\npublic-root\n-----END CERTIFICATE-----\n'},
    })
    secrets = {
        'voice-matchmaking-principal-signing': {'current.pem': b'current', 'next.pem': b'next', 'active-kid': b'current'},
        'voice-matchmaking-client-tls': {'tls.crt': b'mm-client-cert', 'tls.key': b'mm-client-key'},
        'voice-chat-match-squad-server-tls': {'tls.crt': b'chat-cert', 'tls.key': b'chat-key'},
        'voice-voice-match-squad-server-tls': {'tls.crt': b'voice-cert', 'tls.key': b'voice-key'},
        'voice-match-squad-replay': {'password': b'provider-password'},
        'voice-match-squad-member-replay': {'password': b'member-password'},
    }
    for name, data in secrets.items():
        tls = name.endswith('-tls')
        objects.append({
            'apiVersion': 'v1', 'kind': 'Secret',
            'metadata': {'name': name, 'namespace': 'voice-dev',
                         'labels': {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'},
                         'annotations': {ANNOTATION: OPERATION, SOURCE_SHA_ANNOTATION: SOURCE_SHA}},
            'type': 'kubernetes.io/tls' if tls else 'Opaque',
            'immutable': True,
            'data': {key: encoded(value) for key, value in data.items()},
        })
    return {'apiVersion': 'v1', 'kind': 'List', 'items': objects}


class FakeKube:
    def __init__(self):
        self.objects = {}
        self.create_calls = []
        self.next_uid = 1

    def get(self, kind, name):
        value = self.objects.get((kind.lower(), name))
        return copy.deepcopy(value) if value is not None else None

    def exists(self, kind, name):
        return (kind.lower(), name) in self.objects

    def create(self, value):
        key = (value['kind'].lower(), value['metadata']['name'])
        self.create_calls.append(key)
        if key in self.objects:
            raise RuntimeError('AlreadyExists')
        created = copy.deepcopy(value)
        created['metadata']['uid'] = f'uid-{self.next_uid}'
        created['metadata']['resourceVersion'] = f'rv-{self.next_uid}'
        self.next_uid += 1
        self.objects[key] = created
        return f"{created['metadata']['uid']}|{created['metadata']['resourceVersion']}"


class MatchSquadPKITests(unittest.TestCase):
    def test_export_uses_exact_dev_objects_and_top_level_secret_types(self):
        value = pki.validate_export(json.dumps(manifest()), OPERATION, SOURCE_SHA)
        self.assertEqual(set(value), SECRET_NAMES | {CA_OBJECT})
        for name in SECRET_NAMES:
            self.assertEqual(value[name]['metadata']['namespace'], 'voice-dev')
            self.assertNotIn('type', value[name]['metadata'])
            self.assertEqual(value[name]['metadata']['annotations'][ANNOTATION], OPERATION)

    def test_export_rejects_extra_object_secret_key_wrong_namespace_or_marker(self):
        for mutate in (
            lambda v: v['items'].append(copy.deepcopy(v['items'][0])),
            lambda v: v['items'][1]['metadata'].__setitem__('namespace', 'voice-staging'),
            lambda v: v['items'][1]['data'].__setitem__('unexpected', encoded(b'x')),
            lambda v: v['items'][1]['metadata']['annotations'].__setitem__(ANNOTATION, 'other'),
            lambda v: v['items'][1]['metadata'].__setitem__('type', 'Opaque'),
            lambda v: v['items'][1]['metadata']['annotations'].__setitem__(SOURCE_SHA_ANNOTATION, 'b' * 64),
            lambda v: v['items'][1]['metadata'].__setitem__('finalizers', ['unexpected']),
        ):
            value = manifest()
            mutate(value)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                pki.validate_export(json.dumps(value), OPERATION, SOURCE_SHA)

    def test_signing_and_replay_material_are_separate_and_active_id_is_valid(self):
        value = pki.validate_export(json.dumps(manifest()), OPERATION, SOURCE_SHA)
        signing = value['voice-matchmaking-principal-signing']['data']
        self.assertNotEqual(signing['current.pem'], signing['next.pem'])
        self.assertEqual(base64.b64decode(signing['active-kid']), b'current')
        provider = value['voice-match-squad-replay']['data']['password']
        member = value['voice-match-squad-member-replay']['data']['password']
        self.assertNotEqual(provider, member)
        for name in ('voice-matchmaking-client-tls', 'voice-chat-match-squad-server-tls',
                     'voice-voice-match-squad-server-tls'):
            self.assertEqual(value[name]['type'], 'kubernetes.io/tls')

    def test_missing_get_requires_zero_exit_and_empty_text(self):
        missing = subprocess.CompletedProcess(['kubectl'], 0, stdout='', stderr='')
        self.assertTrue(pki.is_explicitly_missing(missing))
        present = subprocess.CompletedProcess(['kubectl'], 0, stdout='secret/name', stderr='')
        self.assertFalse(pki.is_explicitly_missing(present))
        failed = subprocess.CompletedProcess(['kubectl'], 1, stdout='', stderr='NotFound')
        with self.assertRaises(ValueError):
            pki.is_explicitly_missing(failed)

    def test_real_kubectl_adapter_consumes_completed_process_text(self):
        args = ['kubectl', '--namespace', 'voice-dev', 'create', '-f', '-',
                '-o', 'go-template={{.metadata.uid}}|{{.metadata.resourceVersion}}']
        body = manifest()['items'][1]
        response = subprocess.CompletedProcess(args, 0, stdout='uid-7|rv-8', stderr='')
        with mock.patch.object(pki, 'run_result', return_value=response) as run, \
                mock.patch.object(pki, '_namespace_identity', return_value={'uid': 'ns-123'}):
            stdout = pki.KubectlCreateOnly('ns-123').create(body)
        self.assertEqual(pki.parse_create_identity(stdout), ('uid-7', 'rv-8'))
        self.assertEqual(run.call_args.kwargs['input_text'], json.dumps(body, separators=(',', ':')))
        self.assertIn('--namespace', run.call_args.args[0])
        self.assertIn('create', run.call_args.args[0])

    def test_issuer_errors_never_echo_secret_stderr(self):
        response = subprocess.CompletedProcess(['docker'], 1, stdout='', stderr='PRIVATE-KEY-DO-NOT-PRINT')
        with mock.patch.object(pki, 'run_result', return_value=response):
            with self.assertRaises(ValueError) as error:
                pki._issuer_output('sha256:' + 'a' * 64, 'voice-dev-match-squad-pki',
                                   'export', OPERATION, 'b' * 64)
        self.assertNotIn('PRIVATE-KEY-DO-NOT-PRINT', str(error.exception))

    def test_create_output_is_parsed_from_real_cli_text(self):
        self.assertEqual(pki.parse_create_identity('uid-1|rv-2'), ('uid-1', 'rv-2'))
        for value in ({'uid': 'uid-1', 'resourceVersion': 'rv-2'}, '', 'uid-only', 'uid-1|rv-2|extra'):
            with self.subTest(value=value), self.assertRaises(ValueError):
                pki.parse_create_identity(value)

    def test_lost_receipt_write_resumes_same_material_without_duplicate_create(self):
        expected = manifest()['items'][1]
        resource = {'phase': 'planned', 'uid': None, 'resourceVersion': None}
        kube = FakeKube()
        initial = {'operationId': OPERATION, 'resources': {expected['metadata']['name']: copy.deepcopy(resource)}}
        persisted = {'raw': json.dumps(initial)}
        writes = 0

        def persist(journal):
            nonlocal writes
            writes += 1
            if writes == 2:
                raise OSError('simulated lost receipt write')
            persisted['raw'] = json.dumps(journal)

        with self.assertRaises(OSError):
            pki.create_only(expected, initial['resources'][expected['metadata']['name']], initial, persist, kube)
        durable_after_crash = json.loads(persisted['raw'])
        self.assertEqual(durable_after_crash['resources'][expected['metadata']['name']]['phase'], 'create-intent')
        count_after_lost_write = len(kube.create_calls)
        pki.create_only(expected, durable_after_crash['resources'][expected['metadata']['name']], durable_after_crash,
                        lambda journal: persisted.update(raw=json.dumps(journal)), kube)
        self.assertEqual(len(kube.create_calls), count_after_lost_write)
        self.assertEqual(durable_after_crash['resources'][expected['metadata']['name']]['phase'], 'created')
        self.assertTrue(durable_after_crash['resources'][expected['metadata']['name']]['uid'])

    def test_successful_create_requires_exact_persisted_content_and_metadata(self):
        expected = manifest()['items'][1]
        for changed in ('content', 'metadata'):
            kube = FakeKube()
            create = kube.create

            def mutate_after_create(value, *, change=changed):
                receipt = create(value)
                actual = kube.objects[(value['kind'].lower(), value['metadata']['name'])]
                if change == 'content':
                    actual['data']['tls.key'] = encoded(b'admission-mutated-key')
                else:
                    actual['metadata']['labels']['admission.example/changed'] = 'true'
                return receipt

            kube.create = mutate_after_create
            resource = {'phase': 'create-intent', 'uid': None, 'resourceVersion': None}
            journal = {'operationId': OPERATION}
            with self.subTest(changed=changed), self.assertRaisesRegex(
                    ValueError, 'created resource differs from the exact staged object'):
                pki.create_only(expected, resource, journal, lambda _journal: None, kube)
            self.assertEqual(resource['phase'], 'create-intent')
            self.assertIsNone(resource['uid'])
            self.assertEqual(len(kube.create_calls), 1)

    def test_external_collision_and_same_marker_content_drift_fail_closed(self):
        expected = manifest()['items'][1]
        for external in ('unmarked', 'same-marker-drift'):
            kube = FakeKube()
            record = {'phase': 'create-intent', 'uid': None, 'resourceVersion': None}
            journal = {'operationId': OPERATION}
            actual = copy.deepcopy(expected)
            actual['metadata']['uid'] = 'external-uid'
            actual['metadata']['resourceVersion'] = 'external-rv'
            if external == 'unmarked':
                actual['metadata']['annotations'][ANNOTATION] = 'external-operation'
            else:
                actual['data']['tls.key'] = encoded(b'drifted-key')
            kube.objects[('secret', expected['metadata']['name'])] = actual
            with self.subTest(external=external), self.assertRaises(ValueError):
                pki.create_only(expected, record, journal, lambda _journal: None, kube)
            self.assertEqual(kube.create_calls, [])

    def test_public_receipt_has_no_secret_data_or_private_fingerprints(self):
        value = pki.validate_export(json.dumps(manifest()), OPERATION, SOURCE_SHA)
        with mock.patch.object(pki, '_certificate_fingerprint', return_value='AA:BB'):
            receipt = pki.public_receipt(value, {'operationId': OPERATION, 'sourceSha': 'a' * 40,
                                                 'issuerImageId': 'sha256:' + 'b' * 64,
                                                 'resources': {}})
        rendered = json.dumps(receipt)
        self.assertNotIn(encoded(b'current'), rendered)
        self.assertNotIn(encoded(b'mm-client-key'), rendered)
        self.assertNotIn(encoded(b'provider-password'), rendered)
        self.assertNotIn('privateKeySha256', rendered)
        self.assertNotIn('passwordSha256', rendered)

    def test_no_write_cli_rejects_unconfirmed_mutating_commands(self):
        calls = []

        def cli(args, *, input_text=None):
            calls.append((args, input_text))
            return subprocess.CompletedProcess(args, 0, stdout='', stderr='')

        with redirect_stderr(io.StringIO()), mock.patch.object(pki, 'run_result', side_effect=cli), \
                mock.patch.object(pki, 'validate_source_pins', return_value={
                    'sourceSha': 'a' * 40, 'templateSha256': 'b' * 64}), \
                mock.patch.object(pki, 'state_dir', return_value=None):
            for command in ('prepare', 'apply', 'resume'):
                with self.subTest(command=command), self.assertRaises(SystemExit) as exit_code:
                    pki.main([command, '--issuer-image', 'sha256:' + 'c' * 64,
                              '--expected-source-sha', 'a' * 40,
                              '--expected-namespace-uid', 'namespace-uid',
                              '--expected-template-sha256', 'b' * 64])
                self.assertNotEqual(exit_code.exception.code, 0)
        self.assertEqual(calls, [])
        self.assertFalse(any('create' in args for args, _ in calls))

    def test_preflight_cli_only_uses_read_commands_and_explicit_empty_stdout(self):
        image = 'sha256:' + 'c' * 64
        namespace = {'kind': 'Namespace', 'metadata': {
            'name': 'voice-dev', 'uid': 'ns-123',
            'labels': {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'}}}
        responses = [
            subprocess.CompletedProcess(['docker'], 0, stdout=image, stderr=''),
            subprocess.CompletedProcess(['kubectl'], 0, stdout=json.dumps(namespace), stderr=''),
        ] + [subprocess.CompletedProcess(['kubectl'], 0, stdout='', stderr='') for _ in range(7)]
        with redirect_stdout(io.StringIO()) as output, \
                mock.patch.object(pki, 'run_result', side_effect=responses) as run, \
                mock.patch.object(pki, 'validate_source_pins', return_value={
                    'sourceSha': 'a' * 40, 'templateSha256': 'b' * 64}):
            status = pki.main(['preflight', '--issuer-image', image,
                               '--expected-source-sha', 'a' * 40,
                               '--expected-template-sha256', 'b' * 64,
                               '--expected-namespace-uid', 'ns-123'])
        self.assertEqual(status, 0)
        self.assertEqual(len(run.call_args_list), 9)
        commands = [call.args[0] for call in run.call_args_list]
        self.assertTrue(all('create' not in args and 'apply' not in args and 'replace' not in args
                            and 'delete' not in args for args in commands))
        self.assertTrue(all(args[0] != 'kubectl' or 'get' in args for args in commands))
        self.assertEqual(json.loads(output.getvalue())['namespace']['uid'], 'ns-123')

    @unittest.skipUnless(sys.platform == 'linux', 'operator custody is Linux-only')
    def test_private_journal_is_atomic_owner_only_and_lock_is_exclusive(self):
        with tempfile.TemporaryDirectory() as temp:
            os.chmod(temp, 0o700)
            path = Path(temp) / 'journal.json'
            pki.write_private_json(path, {'operationId': OPERATION, 'phase': 'staged'})
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertEqual(json.loads(path.read_text()), {'operationId': OPERATION, 'phase': 'staged'})
            lock = Path(temp) / 'operator.lock'
            with pki.invocation_lock(lock):
                with self.assertRaises(BlockingIOError):
                    with pki.invocation_lock(lock):
                        pass

    @unittest.skipUnless(sys.platform == 'linux', 'operator custody is Linux-only')
    def test_private_journal_rejects_symlinked_temp_and_insecure_directory(self):
        with tempfile.TemporaryDirectory() as temp:
            os.chmod(temp, 0o700)
            path = Path(temp) / 'journal.json'
            (Path(temp) / 'journal.json.tmp').symlink_to(Path(temp) / 'target')
            with self.assertRaises(OSError):
                pki.write_private_json(path, {'phase': 'staged'})
        with tempfile.TemporaryDirectory() as temp:
            os.chmod(temp, 0o755)
            with self.assertRaises(ValueError):
                pki.write_private_json(Path(temp) / 'journal.json', {'phase': 'staged'})


class MatchSquadIssuerTests(unittest.TestCase):
    def test_issuer_rejects_malformed_operation_identity_before_staging(self):
        malformed = (
            'msq-' + 'g' * 32,
            'msq-' + 'a' * 31,
            'msq-' + 'a' * 33,
            'other-' + 'a' * 30,
        )
        for operation in malformed:
            with self.subTest(operation=operation), tempfile.TemporaryDirectory() as temp:
                with self.assertRaisesRegex(ValueError, 'invalid operation identity'):
                    issuer.stage(temp, operation, 'a' * 64)
                self.assertEqual(list(Path(temp).iterdir()), [])

    def test_real_issuer_profiles_and_secret_contract(self):
        with tempfile.TemporaryDirectory() as temp:
            operation = 'msq-1234567890abcdef1234567890abcdef'
            source_sha = 'a' * 64
            raw = issuer.stage(temp, operation, source_sha)
            items = pki.validate_export(raw.decode('utf-8'), operation, source_sha)
            ca_pem = items[CA_OBJECT]['data']['ca.crt'].encode('utf-8')
            self.assertNotIn(b'PRIVATE KEY', ca_pem)
            profiles = {
                'voice-matchmaking-client-tls': ('voice-matchmaking.voice-dev.svc', 'sslclient'),
                'voice-chat-match-squad-server-tls': ('voice-chat', 'sslserver'),
                'voice-voice-match-squad-server-tls': ('voice-voice', 'sslserver'),
            }
            with tempfile.TemporaryDirectory() as certdir:
                ca_path = Path(certdir) / 'ca.crt'
                ca_path.write_bytes(ca_pem)
                constraints = issuer.openssl(['x509', '-in', str(ca_path), '-noout', '-ext', 'nameConstraints'])
                self.assertIn(b'voice-matchmaking.voice-dev.svc', constraints)
                self.assertIn(b'voice-chat', constraints)
                self.assertIn(b'voice-voice', constraints)
                with tempfile.TemporaryDirectory() as wrong_temp:
                    wrong_raw = issuer.stage(wrong_temp, 'msq-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'b' * 64)
                    wrong_ca = Path(certdir) / 'wrong-ca.crt'
                    wrong_items = json.loads(wrong_raw.decode('utf-8'))['items']
                    wrong_ca.write_text(next(item['data']['ca.crt'] for item in wrong_items
                                              if item['kind'] == 'ConfigMap'), encoding='ascii')
                for name, (dns_name, purpose) in profiles.items():
                    cert_path = Path(certdir) / (name + '.crt')
                    cert_path.write_bytes(base64.b64decode(items[name]['data']['tls.crt']))
                    result = issuer.openssl(['verify', '-CAfile', str(ca_path), '-purpose', purpose,
                                             '-verify_hostname', dns_name, str(cert_path)])
                    self.assertIn(b'OK', result)
                    wrong_purpose = 'sslserver' if purpose == 'sslclient' else 'sslclient'
                    with self.assertRaises(ValueError):
                        issuer.openssl(['verify', '-CAfile', str(ca_path), '-purpose', wrong_purpose,
                                        '-verify_hostname', dns_name, str(cert_path)])
                    with self.assertRaises(ValueError):
                        issuer.openssl(['verify', '-CAfile', str(ca_path), '-purpose', purpose,
                                        '-verify_hostname', 'wrong.example', str(cert_path)])
                    with self.assertRaises(ValueError):
                        issuer.openssl(['verify', '-CAfile', str(wrong_ca), '-purpose', purpose,
                                        '-verify_hostname', dns_name, str(cert_path)])
            signing = pki.decoded_secrets(items['voice-matchmaking-principal-signing'])
            self.assertNotEqual(signing['current.pem'], signing['next.pem'])
            self.assertEqual(signing['active-kid'], b'current')
            with tempfile.TemporaryDirectory() as keydir:
                for label in ('current.pem', 'next.pem'):
                    key_path = Path(keydir) / label
                    key_path.write_bytes(signing[label])
                    self.assertIn(b'RSA key ok', issuer.openssl(['rsa', '-in', str(key_path), '-check', '-noout']))
            self.assertNotEqual(pki.decoded_secrets(items[issuer.REPLAY_SECRETS[0]])['password'],
                                pki.decoded_secrets(items[issuer.REPLAY_SECRETS[1]])['password'])

    def test_issuer_refuses_replacement_and_tampered_stage(self):
        with tempfile.TemporaryDirectory() as temp:
            operation = 'msq-1234567890abcdef1234567890abcdef'
            source_sha = 'a' * 64
            issuer.stage(temp, operation, source_sha)
            with self.assertRaises(ValueError):
                issuer.stage(temp, operation, source_sha)
            path = Path(temp) / operation / 'stage.json'
            path.chmod(0o600)
            ca_key_path = Path(temp) / operation / 'ca.key'
            self.assertEqual(ca_key_path.stat().st_mode & 0o777, 0o600)
            payload = json.loads(path.read_text(encoding='utf-8'))
            original = path.read_text(encoding='utf-8')
            payload['manifestSha256'] = '0' * 64
            path.write_text(json.dumps(payload), encoding='utf-8')
            with self.assertRaises(ValueError):
                issuer.load_stage(temp, operation, source_sha)
            path.write_text(original, encoding='utf-8')
            ca_key_path.write_bytes(ca_key_path.read_bytes()[:-1] + b'x')
            with self.assertRaises(ValueError):
                issuer.load_stage(temp, operation, source_sha)

    def test_journal_binding_rejects_changed_source_namespace_or_image(self):
        journal = {'sourceSha': 'a' * 40, 'templateSha256': 'b' * 64,
                   'issuerImageId': 'sha256:' + 'c' * 64, 'namespaceUid': 'ns-1',
                   'sourceBinding': pki._source_binding({'sourceSha': 'a' * 40,
                                                        'templateSha256': 'b' * 64})}
        pins = {'sourceSha': 'a' * 40, 'templateSha256': 'b' * 64}
        pki._verify_journal_pins(journal, pins, 'sha256:' + 'c' * 64, {'uid': 'ns-1'})
        for changed_pins, image, namespace in (
                ({'sourceSha': 'd' * 40, 'templateSha256': 'b' * 64}, 'sha256:' + 'c' * 64, {'uid': 'ns-1'}),
                ({'sourceSha': 'a' * 40, 'templateSha256': 'e' * 64}, 'sha256:' + 'c' * 64, {'uid': 'ns-1'}),
                (pins, 'sha256:' + 'f' * 64, {'uid': 'ns-1'}),
                (pins, 'sha256:' + 'c' * 64, {'uid': 'ns-2'})):
            with self.subTest(pins=changed_pins, image=image, namespace=namespace), self.assertRaises(ValueError):
                pki._verify_journal_pins(journal, changed_pins, image, namespace)

    def test_stage_receipt_and_resume_export_bind_same_manifest(self):
        raw = json.dumps(manifest())
        journal = {'operationId': OPERATION, 'sourceBinding': SOURCE_SHA,
                   'issuerImageId': 'sha256:' + 'c' * 64,
                   'volume': 'voice-dev-match-squad-pki', 'phase': 'stage-intent'}
        receipt = json.dumps({'operationId': OPERATION, 'sourceSha256': SOURCE_SHA,
                              'manifestSha256': hashlib.sha256(raw.encode()).hexdigest()})
        with mock.patch.object(pki, '_issuer_output', side_effect=[receipt, raw]) as issuer_output:
            items = pki._read_staged_manifest(journal, stage_first=True)
        self.assertEqual(set(items), pki.EXPECTED_OBJECTS)
        self.assertEqual(journal['phase'], 'staged')
        self.assertEqual(journal['manifestSha256'], hashlib.sha256(raw.encode()).hexdigest())
        self.assertEqual([call.args[2] for call in issuer_output.call_args_list], ['stage', 'export'])
        with mock.patch.object(pki, '_issuer_output', return_value=raw):
            pki._read_staged_manifest(journal)
        changed = json.dumps(manifest()).replace(SOURCE_SHA, 'd' * 64)
        with mock.patch.object(pki, '_issuer_output', return_value=changed), self.assertRaises(ValueError):
            pki._read_staged_manifest(journal)

    def test_stage_command_emits_public_receipt_without_private_manifest(self):
        with tempfile.TemporaryDirectory() as temp, redirect_stdout(io.StringIO()) as output:
            result = issuer.main(['stage', '--state-dir', temp,
                                  '--operation-id', 'msq-1234567890abcdef1234567890abcdef',
                                  '--source-sha256', 'a' * 64])
        self.assertEqual(result, 0)
        receipt = json.loads(output.getvalue())
        self.assertIn('manifestSha256', receipt)
        self.assertNotIn('manifest', receipt)
        self.assertNotIn('PRIVATE KEY', output.getvalue())
        self.assertNotIn('password', output.getvalue())


if __name__ == '__main__':
    unittest.main()
