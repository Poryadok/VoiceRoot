"""Focused source and issuer tests for the separate CompleteMatch DEV PKI operation."""
import base64
import copy
import contextlib
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

sys.dont_write_bytecode = True
import complete_match_pki as pki

ISSUER_PATH = Path(__file__).with_name('complete-match-pki-issuer') / 'issuer.py'
ISSUER_SPEC = importlib.util.spec_from_file_location('complete_match_pki_issuer', ISSUER_PATH)
issuer = importlib.util.module_from_spec(ISSUER_SPEC)
_WRITE_BYTECODE = sys.dont_write_bytecode
sys.dont_write_bytecode = True
try:
    ISSUER_SPEC.loader.exec_module(issuer)
finally:
    sys.dont_write_bytecode = _WRITE_BYTECODE

OPERATION = 'cmp-1234567890abcdef1234567890abcdef'
SOURCE_SHA = 'a' * 40
SERVER_SECRET = 'voice-matchmaking-complete-server-tls'
CLIENT_CA = 'voice-matchmaking-complete-client-ca'
GATEWAY_SECRET = 'voice-gateway-matchmaking-complete-client-tls'
TARGETS = {SERVER_SECRET, CLIENT_CA, GATEWAY_SECRET}


def run_openssl(args, *, input_bytes=None):
    result = subprocess.run(['openssl', *args], input=input_bytes, capture_output=True, check=False)
    if result.returncode:
        raise AssertionError('synthetic OpenSSL fixture command failed')
    return result.stdout


def synthetic_ca(folder):
    key = folder / 'fixture-root.key'
    cert = folder / 'fixture-root.crt'
    run_openssl(['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-sha256', '-days', '30',
                 '-subj', '/CN=CompleteMatch synthetic fixture',
                 '-addext', 'basicConstraints=critical,CA:TRUE,pathlen:0',
                 '-addext', 'keyUsage=critical,keyCertSign,cRLSign',
                 '-addext', 'nameConstraints=critical,permitted;DNS:voice-matchmaking.voice-dev.svc',
                 '-keyout', str(key), '-out', str(cert)])
    key.chmod(0o600)
    return key, cert


def decoded(item):
    return {key: base64.b64decode(value, validate=True) for key, value in item['data'].items()}


class CompleteMatchOperatorTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='complete-match-pki-test-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.ca_key, self.ca_cert = synthetic_ca(self.root)
        self.raw, self.client_ca_key = issuer.build_packet(OPERATION, SOURCE_SHA, self.ca_key, self.ca_cert)
        self.items = pki.validate_export(self.raw, OPERATION, SOURCE_SHA, self.ca_cert.read_bytes())

    def test_issuer_emits_exact_three_create_only_targets_and_leaf_profiles(self):
        self.assertEqual(set(self.items), TARGETS)
        self.assertEqual(self.items[CLIENT_CA]['kind'], 'ConfigMap')
        self.assertEqual(self.items[CLIENT_CA]['immutable'], True)
        self.assertEqual(set(self.items[CLIENT_CA]['data']), {'ca.crt'})
        for name in (SERVER_SECRET, GATEWAY_SECRET):
            item = self.items[name]
            self.assertEqual(item['kind'], 'Secret')
            self.assertEqual(item['type'], 'kubernetes.io/tls')
            self.assertEqual(item['immutable'], True)
            self.assertEqual(set(item['data']), {'tls.crt', 'tls.key'})
        issuer.verify_packet_profiles(self.raw, self.ca_cert.read_bytes())
        server_cert = decoded(self.items[SERVER_SECRET])['tls.crt']
        retained_cert = self.items[SERVER_SECRET]['metadata']['annotations'][pki.RETAINED_CA_FINGERPRINT_ANNOTATION]
        self.assertEqual(retained_cert, pki.certificate_fingerprint(self.ca_cert.read_bytes()))
        self.assertIn(b'voice-matchmaking.voice-dev.svc', run_openssl(['x509', '-in', '/dev/stdin',
                                                                       '-noout', '-ext', 'subjectAltName'],
                                                                      input_bytes=server_cert))

    def test_export_rejects_extra_objects_wrong_namespace_type_keys_and_foreign_marker(self):
        value = json.loads(self.raw)
        cases = []
        extra = copy.deepcopy(value)
        extra['items'].append(copy.deepcopy(extra['items'][0]))
        extra['items'][-1]['metadata']['name'] = 'voice-match-squad-ca'
        cases.append(extra)
        wrong_namespace = copy.deepcopy(value)
        wrong_namespace['items'][0]['metadata']['namespace'] = 'voice-staging'
        cases.append(wrong_namespace)
        wrong_type = copy.deepcopy(value)
        next(row for row in wrong_type['items'] if row['metadata']['name'] == SERVER_SECRET)['type'] = 'Opaque'
        cases.append(wrong_type)
        wrong_keys = copy.deepcopy(value)
        next(row for row in wrong_keys['items'] if row['metadata']['name'] == GATEWAY_SECRET)['data']['extra'] = 'eA=='
        cases.append(wrong_keys)
        foreign_marker = copy.deepcopy(value)
        next(row for row in foreign_marker['items'] if row['metadata']['name'] == CLIENT_CA)['metadata']['annotations'][pki.OPERATION_ANNOTATION] = 'cmp-other'
        cases.append(foreign_marker)
        for case in cases:
            with self.subTest(case=case['items'][0]['metadata']):
                with self.assertRaises(ValueError):
                    pki.validate_export(json.dumps(case), OPERATION, SOURCE_SHA, self.ca_cert.read_bytes())

    def test_public_receipt_contains_only_public_identity_and_certificate_evidence(self):
        receipt = pki.public_receipt(self.items, {
            'operationId': OPERATION, 'sourceSha': SOURCE_SHA, 'phase': 'complete',
            'namespace': 'voice-dev', 'namespaceUid': 'namespace-uid',
            'issuerImageId': 'sha256:' + 'b' * 64, 'consumerContractSha256': 'c' * 64,
            'retained': {'caUid': 'retained-ca-uid', 'caResourceVersion': '101',
                         'fingerprint': 'd' * 64},
            'resources': {name: {'uid': 'uid-' + str(index), 'resourceVersion': str(101 + index)}
                          for index, name in enumerate(TARGETS)},
        })
        rendered = json.dumps(receipt, sort_keys=True)
        self.assertNotIn('current.pem', rendered)
        self.assertNotIn('password', rendered)
        self.assertNotIn('caKeySha256', rendered)
        for name in (SERVER_SECRET, GATEWAY_SECRET):
            self.assertNotIn(base64.b64decode(self.items[name]['data']['tls.key']).decode('ascii'), rendered)
            self.assertIn('tls.key', receipt['resources'][name]['keyNames'])
        self.assertEqual(set(receipt['resources']), TARGETS)
        self.assertEqual(receipt['resources'][CLIENT_CA]['keyNames'], ['ca.crt'])

    def test_retained_ca_requires_exact_legacy_source_template_binding(self):
        expected_binding = hashlib.sha256(
            f'{pki.OLD_SOURCE_SHA}:{pki.OLD_TEMPLATE_SHA256}'.encode('ascii')).hexdigest()
        row = {
            'metadata': {
                'uid': pki.OLD_CA_UID,
                'resourceVersion': pki.OLD_CA_RV,
                'namespace': pki.NAMESPACE,
                'labels': copy.deepcopy(pki.MANAGED_LABELS),
                'annotations': {
                    'voice.dev/match-squad-pki-operation': pki.OLD_OPERATION,
                    'voice.dev/match-squad-pki-source-sha256': expected_binding,
                },
            },
            'immutable': True,
            'data': {'ca.crt': self.ca_cert.read_text(encoding='ascii')},
        }
        kube = mock.Mock()
        kube.get.return_value = row
        with mock.patch.object(pki, 'certificate_fingerprint', return_value=pki.OLD_CA_FINGERPRINT):
            self.assertEqual(pki._retained_ca_object(kube)['fingerprint'], pki.OLD_CA_FINGERPRINT)
            wrong_bindings = (
                pki.OLD_SOURCE_SHA,
                hashlib.sha256(f"{'b' * 40}:{pki.OLD_TEMPLATE_SHA256}".encode('ascii')).hexdigest(),
                hashlib.sha256(f"{pki.OLD_SOURCE_SHA}:{'c' * 64}".encode('ascii')).hexdigest(),
            )
            for binding in wrong_bindings:
                with self.subTest(binding=binding):
                    candidate = copy.deepcopy(row)
                    candidate['metadata']['annotations'][
                        'voice.dev/match-squad-pki-source-sha256'] = binding
                    kube.get.return_value = candidate
                    with self.assertRaisesRegex(pki.Blocked, 'retained_public_ca_identity_changed'):
                        pki._retained_ca_object(kube)

    def test_create_only_server_dryrun_precedes_create_and_requires_exact_readback(self):
        events = []

        class Kube:
            def get(self, kind, name):
                events.append(('get', kind, name))
                return None if name not in persisted else copy.deepcopy(persisted[name])

            def server_dry_run(self, value):
                events.append(('dry-run', value['metadata']['name']))
                return copy.deepcopy(value)

            def create(self, value):
                events.append(('create', value['metadata']['name']))
                row = copy.deepcopy(value)
                row['metadata'].update(uid='01234567-89ab-cdef-0123-456789abcdef', resourceVersion='102')
                persisted[row['metadata']['name']] = row
                return {'uid': '01234567-89ab-cdef-0123-456789abcdef', 'resourceVersion': '102'}

        persisted = {}
        row = copy.deepcopy(self.items[SERVER_SECRET])
        journal = {'operationId': OPERATION, 'resources': {SERVER_SECRET: {'phase': 'planned'}}}
        receipts = []
        result = pki.create_only(row, journal['resources'][SERVER_SECRET], journal,
                                 lambda value: receipts.append(copy.deepcopy(value)), Kube())
        self.assertEqual(result, {'uid': '01234567-89ab-cdef-0123-456789abcdef', 'resourceVersion': '102'})
        self.assertIn(('dry-run', SERVER_SECRET), events)
        self.assertIn(('create', SERVER_SECRET), events)
        self.assertLess(events.index(('dry-run', SERVER_SECRET),), events.index(('create', SERVER_SECRET)))
        self.assertEqual(receipts[-1]['resources'][SERVER_SECRET]['phase'], 'created')

    def test_server_dryrun_admission_change_stops_before_create(self):
        row = copy.deepcopy(self.items[SERVER_SECRET])
        events = []

        class MutatingKube:
            def get(self, kind, name):
                events.append('get')
                return None

            def server_dry_run(self, value):
                changed = copy.deepcopy(value)
                changed['metadata']['annotations']['admission.example/mutated'] = 'yes'
                return changed

            def create(self, value):
                events.append('create')

        with self.assertRaisesRegex(ValueError, 'dry_run.*differs|admission'):
            pki.create_only(row, {'phase': 'planned'}, {'operationId': OPERATION, 'resources': {}},
                            lambda _: None, MutatingKube())
        self.assertNotIn('create', events)

    def test_created_object_readback_mutation_fails_before_receipt(self):
        row = copy.deepcopy(self.items[SERVER_SECRET])
        persisted = {}

        class MutatingKube:
            def get(self, kind, name):
                value = persisted.get(name)
                return copy.deepcopy(value) if value is not None else None

            def server_dry_run(self, value):
                return copy.deepcopy(value)

            def create(self, value):
                changed = copy.deepcopy(value)
                changed['metadata']['labels']['admission.example/mutated'] = 'true'
                changed['metadata'].update(uid='11111111-1111-1111-1111-111111111111', resourceVersion='104')
                persisted[changed['metadata']['name']] = changed
                return {'uid': '11111111-1111-1111-1111-111111111111', 'resourceVersion': '104'}

        journal = {'operationId': OPERATION, 'resources': {SERVER_SECRET: {'phase': 'planned'}}}
        receipts = []
        with self.assertRaisesRegex(ValueError, 'readback_mismatch'):
            pki.create_only(row, journal['resources'][SERVER_SECRET], journal,
                            lambda value: receipts.append(copy.deepcopy(value)), MutatingKube())
        self.assertEqual(len(receipts), 1)
        self.assertEqual(receipts[0]['resources'][SERVER_SECRET]['phase'], 'intent')
        self.assertEqual(journal['resources'][SERVER_SECRET]['phase'], 'intent')

    def test_planned_resource_cannot_adopt_preexisting_exact_object(self):
        row = copy.deepcopy(self.items[SERVER_SECRET])
        existing = copy.deepcopy(row)
        existing['metadata'].update(uid='01234567-89ab-cdef-0123-456789abcdef', resourceVersion='102')
        persisted = []

        class Kube:
            def get(self, kind, name):
                return copy.deepcopy(existing)

        journal_row = {'phase': 'planned'}
        with self.assertRaisesRegex(ValueError, 'planned_target_already_exists'):
            pki.create_only(row, journal_row, {'operationId': OPERATION},
                            lambda _: persisted.append('write'), Kube())
        self.assertEqual(journal_row['phase'], 'planned')
        self.assertEqual(persisted, [])

    def test_kubectl_uses_pinned_private_config_with_sanitized_environment(self):
        with tempfile.TemporaryDirectory(prefix='complete-match-kubeconfig-test-') as temp:
            directory = Path(temp) / '.kube'
            directory.mkdir(mode=0o700)
            config = directory / 'config'
            config.write_bytes(b'synthetic kubeconfig metadata fixture')
            os.chmod(config, 0o600)
            completed = subprocess.CompletedProcess([], 0, stdout=b'{}', stderr=b'')
            with mock.patch.object(pki, 'KUBECONFIG_PATH', config), \
                    mock.patch.object(pki.subprocess, 'run', return_value=completed) as run, \
                    mock.patch.dict(os.environ, {'KUBECONFIG': '/untrusted/ambient/config'}):
                result = pki.kubectl_result(['get', 'namespace', 'voice-dev'])
            self.assertEqual(result.stdout, b'{}')
            args = run.call_args.args[0]
            self.assertEqual(args[:5], ['kubectl', '--kubeconfig', str(config),
                                        '--context', 'default'])
            self.assertEqual(args[5:], ['get', 'namespace', 'voice-dev'])
            env = run.call_args.kwargs['env']
            self.assertEqual(env, {'PATH': '/usr/local/bin:/usr/bin:/bin', 'HOME': '/nonexistent'})
            self.assertNotIn('KUBECONFIG', env)

    def test_kubeconfig_custody_rejects_permissive_file_and_symlink(self):
        with tempfile.TemporaryDirectory(prefix='complete-match-kubeconfig-custody-') as temp:
            directory = Path(temp) / '.kube'
            directory.mkdir(mode=0o700)
            config = directory / 'config'
            config.write_bytes(b'synthetic kubeconfig metadata fixture')
            os.chmod(config, 0o640)
            with mock.patch.object(pki, 'KUBECONFIG_PATH', config):
                with self.assertRaisesRegex(ValueError, 'kubeconfig_custody_invalid'):
                    pki._pinned_kubeconfig()
                os.chmod(config, 0o600)
                symlink = directory / 'link'
                symlink.symlink_to(config)
                with mock.patch.object(pki, 'KUBECONFIG_PATH', symlink):
                    with self.assertRaisesRegex(ValueError, 'kubeconfig_custody_invalid'):
                        pki._pinned_kubeconfig()

    def test_prepare_apply_and_resume_all_require_explicit_confirmation(self):
        common = ['--issuer-image', 'sha256:' + 'b' * 64,
                  '--expected-source-sha', SOURCE_SHA,
                  '--expected-consumer-contract-sha256', 'c' * 64,
                  '--expected-namespace-uid', '15950fc8-0392-4053-9156-e95e78b596f9']
        for command in ('prepare', 'apply', 'resume'):
            with self.subTest(command=command), contextlib.redirect_stderr(io.StringIO()):
                with self.assertRaises(SystemExit):
                    pki.main([command, *common])

    def test_source_binding_requires_exact_commit_and_clean_checkout(self):
        head = subprocess.CompletedProcess([], 0, stdout=(SOURCE_SHA + '\n').encode(), stderr=b'')
        clean = subprocess.CompletedProcess([], 0, stdout=b'', stderr=b'')
        with mock.patch.object(pki, 'run_result', side_effect=[head, clean]), \
                mock.patch.object(pki, 'consumer_contract', return_value='c' * 64):
            self.assertEqual(pki._verify_source(SOURCE_SHA, 'c' * 64), {
                'sourceSha': SOURCE_SHA, 'consumerContractSha256': 'c' * 64})
        dirty = subprocess.CompletedProcess([], 0, stdout=b' M scripts/dev/issuer.py\n', stderr=b'')
        with mock.patch.object(pki, 'run_result', side_effect=[head, dirty]), \
                mock.patch.object(pki, 'consumer_contract', return_value='c' * 64):
            with self.assertRaisesRegex(ValueError, 'source_worktree_not_clean'):
                pki._verify_source(SOURCE_SHA, 'c' * 64)

    @unittest.skipUnless(shutil.which('git'), 'requires a real Git checkout for source-pinned CLI proof')
    def test_preflight_cli_uses_real_source_and_consumer_pins_with_fake_external_receipts(self):
        root = Path(__file__).resolve().parents[2]
        source = subprocess.run(['git', '-C', str(root), 'rev-parse', 'HEAD'],
                                capture_output=True, check=True, text=True).stdout.strip()
        contract = pki.consumer_contract(root)
        namespace = {'name': pki.NAMESPACE, 'uid': pki.NAMESPACE_UID}
        retained_journal = {'operationId': pki.OLD_OPERATION,
                            'journalSha256': pki.OLD_JOURNAL_SHA256}
        retained_ca = {'uid': pki.OLD_CA_UID, 'resourceVersion': pki.OLD_CA_RV,
                       'fingerprint': pki.OLD_CA_FINGERPRINT, 'certificate': b'synthetic-public-only'}
        stdout = io.StringIO()
        with (mock.patch.object(pki, '_check_local_image'),
              mock.patch.object(pki, '_namespace_identity', return_value=namespace),
              mock.patch.object(pki, '_load_retained_journal', return_value=retained_journal),
              mock.patch.object(pki, '_verify_retained_volume'),
              mock.patch.object(pki, '_retained_ca_object', return_value=retained_ca),
              mock.patch.object(pki, '_targets_absent'),
              mock.patch.object(pki, '_volume_list', return_value=[]),
              contextlib.redirect_stdout(stdout)):
            result = pki.main([
                'preflight', '--issuer-image', 'sha256:' + 'b' * 64,
                '--expected-source-sha', source,
                '--expected-consumer-contract-sha256', contract,
                '--expected-namespace-uid', pki.NAMESPACE_UID,
            ])
        self.assertEqual(result, 0)
        receipt = json.loads(stdout.getvalue())
        self.assertEqual(receipt['sourceSha'], source)
        self.assertEqual(receipt['consumerContractSha256'], contract)
        self.assertEqual(receipt['targets'], sorted(TARGETS))
        self.assertNotIn('certificate', receipt)

    def test_cli_startup_does_not_dirty_a_clean_git_checkout(self):
        with tempfile.TemporaryDirectory(prefix='complete-match-clean-startup-') as temp:
            root = Path(temp)
            dev = root / 'scripts' / 'dev'
            issuer_dir = dev / 'complete-match-pki-issuer'
            issuer_dir.mkdir(parents=True)
            helper = dev / 'complete_match_pki.py'
            test_module = dev / 'complete_match_pki_test.py'
            shutil.copy2(pki.__file__, helper)
            shutil.copy2(__file__, test_module)
            shutil.copy2(ISSUER_PATH, issuer_dir / 'issuer.py')
            subprocess.run(['git', 'init', '--quiet', str(root)], check=True,
                           capture_output=True)
            subprocess.run(['git', '-C', str(root), 'add', 'scripts/dev'], check=True,
                           capture_output=True)
            subprocess.run(['git', '-C', str(root), '-c', 'user.name=Test',
                            '-c', 'user.email=test@example.invalid', 'commit', '--quiet',
                            '-m', 'baseline'], check=True, capture_output=True)
            env = {'PATH': os.environ.get('PATH', ''), 'HOME': str(root)}
            result = subprocess.run([sys.executable, str(helper), '--help'], cwd=root,
                                    env=env, capture_output=True, text=True, check=False)
            self.assertEqual(result.returncode, 0, result.stderr)
            status = subprocess.run(['git', '-C', str(root), 'status', '--porcelain',
                                     '--untracked-files=all'], capture_output=True,
                                    text=True, check=True)
            self.assertEqual(status.stdout, '')
            result = subprocess.run([sys.executable, str(test_module), '--help'], cwd=root,
                                    env=env, capture_output=True, text=True, check=False)
            self.assertEqual(result.returncode, 0, result.stderr)
            status = subprocess.run(['git', '-C', str(root), 'status', '--porcelain',
                                     '--untracked-files=all'], capture_output=True,
                                    text=True, check=True)
            self.assertEqual(status.stdout, '')

    def test_journal_loader_rejects_unexpected_resource_fields(self):
        state = self.root / 'operator-state'
        state.mkdir(mode=0o700)
        resources = {name: {'phase': 'planned'} for name in sorted(TARGETS)}
        journal = {
            'schema': 'voice-dev-complete-match-pki-operation-v1', 'operationId': OPERATION,
            'namespace': 'voice-dev', 'namespaceUid': '15950fc8-0392-4053-9156-e95e78b596f9',
            'sourceSha': SOURCE_SHA, 'consumerContractSha256': 'c' * 64,
            'issuerImageId': 'sha256:' + 'b' * 64, 'volume': pki.NEW_VOLUME,
            'retained': {'operationId': pki.OLD_OPERATION, 'journalSha256': pki.OLD_JOURNAL_SHA256,
                         'sourceSha': pki.OLD_SOURCE_SHA, 'templateSha256': pki.OLD_TEMPLATE_SHA256,
                         'issuerImageId': pki.OLD_IMAGE_ID, 'volume': pki.OLD_VOLUME,
                         'namespaceUid': pki.NAMESPACE_UID, 'caUid': pki.OLD_CA_UID,
                         'caResourceVersion': pki.OLD_CA_RV, 'fingerprint': pki.OLD_CA_FINGERPRINT},
            'phase': 'stage-intent', 'resources': resources, 'stageReceipt': None,
        }
        pki.write_private_json(state / 'operation.json', journal)
        self.assertEqual(pki._load_journal(state)['phase'], 'stage-intent')
        journal['resources'][SERVER_SECRET]['uid'] = 'unexpected-before-create'
        pki.write_private_json(state / 'operation.json', journal)
        with self.assertRaisesRegex(ValueError, 'resource_state_invalid'):
            pki._load_journal(state)
        journal['resources'] = {name: {'phase': 'planned'} for name in sorted(TARGETS)}
        journal['retained']['unexpected'] = 'foreign'
        pki.write_private_json(state / 'operation.json', journal)
        with self.assertRaisesRegex(ValueError, 'journal_binding_invalid'):
            pki._load_journal(state)
        journal['retained'].pop('unexpected')
        journal['phase'] = 'complete'
        journal['stageReceipt'] = {'state': 'staged', 'resourceNames': sorted(TARGETS)}
        pki.write_private_json(state / 'operation.json', journal)
        with self.assertRaisesRegex(ValueError, 'completion_receipt_invalid'):
            pki._load_journal(state)

    def test_lost_create_receipt_adopts_exact_same_staged_resource(self):
        row = copy.deepcopy(self.items[GATEWAY_SECRET])
        persisted = copy.deepcopy(row)
        persisted['metadata'].update(uid='aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee', resourceVersion='103')

        class Kube:
            gets = 0

            def get(self, kind, name):
                self.gets += 1
                return None if self.gets == 1 else copy.deepcopy(persisted)

            def server_dry_run(self, value):
                return copy.deepcopy(value)

            def create(self, value):
                raise ValueError('create result was lost')

        journal = {'operationId': OPERATION, 'resources': {GATEWAY_SECRET: {'phase': 'intent'}}}
        journal_row = journal['resources'][GATEWAY_SECRET]
        result = pki.create_only(row, journal_row, journal, lambda _: None, Kube())
        self.assertEqual(result, {'uid': 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee', 'resourceVersion': '103'})
        self.assertEqual(journal_row['phase'], 'created')

    def test_payload_or_identity_drift_and_external_collision_fail_closed(self):
        row = copy.deepcopy(self.items[CLIENT_CA])

        class ExistingKube:
            def __init__(self, existing):
                self.existing = existing

            def get(self, kind, name):
                return copy.deepcopy(self.existing)

            def server_dry_run(self, value):
                return copy.deepcopy(value)

            def create(self, value):
                raise AssertionError('existing object must not be overwritten')

        for existing in ({'metadata': {'name': CLIENT_CA, 'namespace': 'voice-dev'}},
                         dict(row, data={'ca.crt': base64.b64encode(b'changed').decode()}),
                         dict(row, metadata={**row['metadata'], 'uid': 'foreign-uid'})):
            with self.subTest(existing=existing.get('metadata', {})):
                with self.assertRaises(ValueError):
                    pki.create_only(row, {'phase': 'intent'}, {'operationId': OPERATION},
                                    lambda _: None, ExistingKube(existing))


class CompleteMatchIssuerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='complete-match-issuer-test-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.retained_key, self.retained_cert = synthetic_ca(self.root)
        self.state = self.root / 'state'
        self.state.mkdir(mode=0o700)

    def test_stage_is_same_material_idempotent_and_emits_no_private_output(self):
        stdout = issuer.stage(self.state, OPERATION, SOURCE_SHA,
                              self.retained_key, self.retained_cert)
        self.assertEqual(json.loads(stdout), {
            'operationId': OPERATION, 'sourceSha256': SOURCE_SHA,
            'resourceNames': sorted(TARGETS), 'state': 'staged'})
        self.assertNotIn('PRIVATE KEY', stdout)
        self.assertNotIn('caKeySha256', stdout)
        first = issuer.export(self.state, OPERATION, SOURCE_SHA,
                              self.retained_key, self.retained_cert)
        again = issuer.stage(self.state, OPERATION, SOURCE_SHA,
                             self.retained_key, self.retained_cert)
        self.assertEqual(first, issuer.export(self.state, OPERATION, SOURCE_SHA,
                                              self.retained_key, self.retained_cert))
        self.assertEqual(json.loads(stdout), json.loads(again))

    def test_wrong_retained_signer_and_public_certificate_mismatch_fail_before_staging(self):
        other = self.root / 'other'
        other.mkdir()
        wrong_key, _ = synthetic_ca(other)
        with self.assertRaises(ValueError):
            issuer.stage(self.state, OPERATION, SOURCE_SHA, wrong_key, self.retained_cert)
        partial = self.state / (OPERATION + '.building')
        self.assertEqual([path.name for path in self.state.iterdir()], [partial.name])
        self.assertEqual(list(partial.iterdir()), [])

    def test_nonempty_building_and_foreign_entries_are_preserved_and_rejected(self):
        partial = self.state / (OPERATION + '.building')
        partial.mkdir(mode=0o700)
        marker = partial / 'preserve-me'
        marker.write_text('synthetic partial marker', encoding='ascii')
        with self.assertRaises(ValueError):
            issuer.stage(self.state, OPERATION, SOURCE_SHA, self.retained_key, self.retained_cert)
        self.assertTrue(marker.exists())
        marker.unlink()
        partial.rmdir()
        (self.state / 'foreign-entry').write_text('fixture', encoding='ascii')
        with self.assertRaises(ValueError):
            issuer.stage(self.state, OPERATION, SOURCE_SHA, self.retained_key, self.retained_cert)

    def test_public_ca_and_leaves_are_separate_trusts_with_exact_san_eku_and_pairing(self):
        raw, _ = issuer.build_packet(OPERATION, SOURCE_SHA, self.retained_key, self.retained_cert)
        issuer.verify_packet_profiles(raw, self.retained_cert.read_bytes())
        items = {item['metadata']['name']: item for item in json.loads(raw)['items']}
        server = decoded(items[SERVER_SECRET])
        gateway = decoded(items[GATEWAY_SECRET])
        new_ca = items[CLIENT_CA]['data']['ca.crt'].encode('ascii')
        self.assertNotEqual(pki.certificate_fingerprint(new_ca), pki.certificate_fingerprint(self.retained_cert.read_bytes()))
        self.assertNotEqual(run_openssl(['x509', '-in', '/dev/stdin', '-pubkey', '-noout'], input_bytes=server['tls.crt']),
                            run_openssl(['x509', '-in', '/dev/stdin', '-pubkey', '-noout'], input_bytes=gateway['tls.crt']))
        self.assertNotEqual(server['tls.key'], gateway['tls.key'])


if __name__ == '__main__':
    unittest.main()
