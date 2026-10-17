"""Focused source and issuer tests for the separate CompleteMatch DEV PKI operation."""
import base64
import copy
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

import complete_match_pki as pki

ISSUER_PATH = Path(__file__).with_name('complete-match-pki-issuer') / 'issuer.py'
ISSUER_SPEC = importlib.util.spec_from_file_location('complete_match_pki_issuer', ISSUER_PATH)
issuer = importlib.util.module_from_spec(ISSUER_SPEC)
ISSUER_SPEC.loader.exec_module(issuer)

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

        journal = {'operationId': OPERATION, 'resources': {GATEWAY_SECRET: {'phase': 'planned'}}}
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
                    pki.create_only(row, {'phase': 'planned'}, {'operationId': OPERATION},
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
