import base64
from contextlib import ExitStack, nullcontext
import importlib.util
import json
from pathlib import Path
import os
import tempfile
import unittest
from unittest import mock

import media_pki as pki

ISSUER_PATH = Path(__file__).parent / 'media-pki-issuer' / 'issuer.py'
SPEC = importlib.util.spec_from_file_location('voice_media_pki_issuer', ISSUER_PATH)
issuer = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(issuer)


def secret_list():
    items = [{'apiVersion': 'v1', 'kind': 'Secret',
              'metadata': {'name': pki.CA_SECRET, 'namespace': pki.NAMESPACE},
              'type': 'Opaque',
              'data': {'ca.crt': base64.b64encode(b'public ca').decode()}}]
    items.append({'apiVersion': 'v1', 'kind': 'Secret',
                  'metadata': {'name': pki.MATCHMAKING_CA_SECRET, 'namespace': pki.NAMESPACE},
                  'type': 'Opaque',
                  'data': {'ca.crt': base64.b64encode(b'public ca').decode()}})
    for name in pki.LEAF_SECRETS:
        items.append({'apiVersion': 'v1', 'kind': 'Secret',
                      'metadata': {'name': name, 'namespace': pki.NAMESPACE},
                      'type': 'kubernetes.io/tls',
                      'data': {'tls.crt': base64.b64encode(b'public cert').decode(),
                               'tls.key': base64.b64encode(b'new private key').decode()}})
    return json.dumps({'apiVersion': 'v1', 'kind': 'List', 'items': items}).encode()


def role_deployment():
    return {'apiVersion': 'apps/v1', 'kind': 'Deployment',
            'metadata': {'name': 'voice-role', 'namespace': 'voice-dev'},
            'spec': {'template': {'spec': {
                'containers': [{'name': 'role', 'env': [
                    {'name': 'S2S_JWKS_CA_FILE', 'value': '/etc/voice/media/ca/ca.crt'},
                    {'name': 'ROLE_PRINCIPAL_CLIENT_CA_FILE', 'value': '/etc/voice/media/ca/ca.crt'}]}],
                'volumes': [{'name': 'media-ca', 'secret': {'secretName': 'voice-media-ca',
                            'items': [{'key': 'ca.crt', 'path': 'ca.crt'}]}}]}}}}


class MediaPkiTests(unittest.TestCase):
    def test_namespace_must_have_dev_ownership_markers(self):
        metadata = {'name': 'voice-dev', 'uid': 'uid-1', 'resourceVersion': '1',
                    'labels': dict(pki.NAMESPACE_LABELS)}
        self.assertEqual(pki.namespace_metadata(json.dumps(metadata).encode())['uid'], 'uid-1')
        metadata['labels']['voice.environment'] = 'staging'
        with self.assertRaises(ValueError):
            pki.namespace_metadata(json.dumps(metadata).encode())

    def test_secret_metadata_uses_selected_fields_without_payload_reads(self):
        snapshots = [
            (pki.MATCHMAKING_SECRET + '|' + pki.NAMESPACE + '|tls-uid|tls-rv|kubernetes.io/tls|dev|voice-dev|operation-1').encode(),
            (pki.MATCHMAKING_SECRET + '|' + pki.NAMESPACE + '|replacement-uid|replacement-rv|Opaque|other|other|other').encode(),
        ]

        def select(*args):
            self.assertEqual(args[:3], ('get', 'secret', pki.MATCHMAKING_SECRET))
            self.assertIn('-o', args)
            self.assertNotIn('json', args)
            self.assertNotIn('.data', args[-1])
            self.assertIn('{{.metadata.resourceVersion}}|{{.type}}|', args[-1])
            self.assertNotIn('{{.metadata.type}}', args[-1])
            self.assertIn('{{if .metadata.labels}}', args[-1])
            self.assertIn('{{if .metadata.annotations}}', args[-1])
            return snapshots.pop(0)

        with mock.patch.object(pki, 'kubectl', side_effect=select) as query:
            value, secret_type = pki.metadata_snapshot('secret', pki.MATCHMAKING_SECRET)
        self.assertEqual(value['uid'], 'tls-uid')
        self.assertEqual(value['resourceVersion'], 'tls-rv')
        self.assertNotIn('type', value)
        self.assertEqual(secret_type, 'kubernetes.io/tls')
        self.assertEqual(value['labels'], pki.NAMESPACE_LABELS)
        self.assertEqual(value['annotations'], {'voice.io/media-pki-operation': 'operation-1'})
        self.assertEqual(query.call_count, 1)
        self.assertEqual(len(snapshots), 1)

    def test_namespace_metadata_query_uses_supported_go_template(self):
        def select(args):
            template = args[-1]
            self.assertIn('{{if .metadata.labels}}', template)
            self.assertIn('{{if .metadata.annotations}}', template)
            return (pki.NAMESPACE + '||namespace-uid|namespace-rv|dev|voice-dev|').encode()

        with mock.patch.object(pki, 'run', side_effect=select) as query:
            value = pki.verify_namespace()
        self.assertEqual(value['uid'], 'namespace-uid')
        self.assertEqual(value['resourceVersion'], 'namespace-rv')
        self.assertEqual(query.call_count, 1)
        self.assertTrue(all('go-template=' in call.args[0][-1] for call in query.call_args_list))

    def test_legacy_resume_pins_opaque_type_only_for_unchanged_live_leaf(self):
        expected = {'uid': 'voice-uid', 'beforeResourceVersion': 'voice-rv',
                    'oldLeafSha256': 'old-leaf-fingerprint', 'newLeafSha256': 'new-leaf-fingerprint'}
        live = {'uid': 'voice-uid', 'resourceVersion': 'voice-rv'}
        self.assertEqual(pki.voice_secret_type_for_resume(
            expected, live, 'Opaque', 'old-leaf-fingerprint'),
                         ('Opaque', True))
        self.assertEqual(expected['secretType'], 'Opaque')
        with self.assertRaisesRegex(ValueError, 'only be pinned before'):
            pki.voice_secret_type_for_resume(
                {'uid': 'voice-uid', 'beforeResourceVersion': 'voice-rv',
                 'oldLeafSha256': 'old-leaf-fingerprint', 'newLeafSha256': 'new-leaf-fingerprint'},
                {'uid': 'voice-uid', 'resourceVersion': 'changed-rv'},
                'Opaque', 'old-leaf-fingerprint')

    def test_export_is_exactly_dev_ca_and_three_tls_leaves(self):
        result = pki.safe_secret_list(secret_list())
        self.assertEqual(set(result), {pki.CA_SECRET, pki.MATCHMAKING_CA_SECRET, *pki.LEAF_SECRETS})

    def test_export_rejects_staging_or_extra_secret_material(self):
        value = json.loads(secret_list())
        value['items'][1]['metadata']['namespace'] = 'voice-staging'
        with self.assertRaises(ValueError):
            pki.safe_secret_list(json.dumps(value).encode())
        value = json.loads(secret_list())
        value['items'][1]['data']['client.key'] = 'not-read'
        with self.assertRaises(ValueError):
            pki.safe_secret_list(json.dumps(value).encode())
        value = json.loads(secret_list())
        value['items'][1]['data']['ca.crt'] = base64.b64encode(b'other public ca').decode()
        with self.assertRaisesRegex(ValueError, 'public CA copies do not match'):
            pki.safe_secret_list(json.dumps(value).encode())

    def test_role_outbound_bundle_does_not_change_inbound_mtls_trust(self):
        result = pki.role_outbound_bundle(role_deployment())
        role = result['spec']['template']['spec']['containers'][0]
        env = {item['name']: item['value'] for item in role['env']}
        self.assertEqual(env['S2S_JWKS_CA_FILE'], '/etc/voice/media/ca/jwks-ca.crt')
        self.assertEqual(env['ROLE_PRINCIPAL_CLIENT_CA_FILE'], '/etc/voice/media/ca/ca.crt')
        self.assertEqual(result['spec']['template']['spec']['volumes'][0]['secret']['items'], [
            {'key': 'ca.crt', 'path': 'ca.crt'}, {'key': 'jwks-ca.crt', 'path': 'jwks-ca.crt'}])

    def test_role_refuses_unknown_trust_source(self):
        deployment = role_deployment()
        deployment['spec']['template']['spec']['volumes'][0]['secret']['secretName'] = 'other-secret'
        with self.assertRaises(ValueError):
            pki.role_outbound_bundle(deployment)

    def test_issuer_image_must_be_immutable_local_image_id(self):
        self.assertEqual(pki.image_id('sha256:' + 'a' * 64), 'sha256:' + 'a' * 64)
        for value in ('alpine:latest', 'registry/issuer:tag', 'sha256:abc'):
            with self.assertRaises(ValueError):
                pki.image_id(value)

    def test_ca_append_preserves_every_existing_bundle_byte(self):
        for bundle in (b'old-root\n', b'old-root\r\n\n', b'old-root'):
            appended = pki.append_public_ca(bundle, b'new-root\n')
            self.assertTrue(appended.startswith(bundle))
            self.assertEqual(appended.count(b'new-root'), 1)
        self.assertEqual(pki.append_public_ca(b'old-root\nnew-root\n', b'new-root\n'),
                         b'old-root\nnew-root\n')

    def test_reentry_after_leaf_cutover_preserves_forward_only_journal(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'last-apply.json'
            raw = json.dumps({'namespace': pki.NAMESPACE, 'volume': pki.VOLUME,
                              'phase': 'voice-leaf-rotated'}).encode()
            path.write_bytes(raw)
            os.chmod(path, 0o600)
            with self.assertRaisesRegex(ValueError, 'recovery must continue forward'):
                pki.reject_existing_apply_journal(Path(directory))
            self.assertEqual(path.read_bytes(), raw)

    def test_gis_create_intent_is_forward_only_even_when_voice_leaf_is_unchanged(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'last-apply.json'
            temp = Path(directory) / '.receipt.tmp'
            receipt = {'phase': 'role-trust-switched', 'voiceLeafChanged': False,
                       'operationId': 'operation-1'}
            resource = {'createOnly': True, 'uid': None, 'afterResourceVersion': None}
            item = {'metadata': {'name': pki.GIS_SECRET, 'namespace': pki.NAMESPACE}}
            phases = []

            def persist(_path, current, _temp):
                phases.append(current['phase'])

            def create(*_args, **_kwargs):
                self.assertEqual(receipt['phase'], 'gis-create-intent')
                return b'gis-uid rv-3'

            with mock.patch.object(pki, 'write_receipt', side_effect=persist), \
                    mock.patch.object(pki, 'kubectl', side_effect=create):
                pki.create_operation_secret(pki.GIS_SECRET, item, receipt, resource,
                                            'gis', path, temp)
            self.assertEqual(phases, ['gis-create-intent', 'gis-created'])
            with self.assertRaisesRegex(ValueError, 'resume forward'):
                pki.require_rollback_allowed(receipt['phase'])

    def test_create_only_secret_records_intent_before_create_and_pins_identity(self):
        item = {'metadata': {'name': pki.MATCHMAKING_CA_SECRET, 'namespace': pki.NAMESPACE}}
        receipt = {'operationId': 'operation-1'}
        resource = {'createOnly': True, 'uid': None, 'afterResourceVersion': None}
        phases = []

        def persist(_path, current, _temp):
            phases.append(current['phase'])

        def create(*_args, **_kwargs):
            self.assertEqual(receipt['phase'], 'matchmaking-ca-create-intent')
            return b'uid-1 rv-2'

        with mock.patch.object(pki, 'write_receipt', side_effect=persist), \
                mock.patch.object(pki, 'kubectl', side_effect=create):
            pki.create_operation_secret(pki.MATCHMAKING_CA_SECRET, item, receipt, resource,
                                        'matchmaking-ca', Path('receipt'), Path('temp'))
        self.assertEqual(phases, ['matchmaking-ca-create-intent', 'matchmaking-ca-created'])
        self.assertEqual(resource, {'createOnly': True, 'uid': 'uid-1', 'afterResourceVersion': 'rv-2'})
        self.assertEqual(item['metadata']['annotations']['voice.io/media-pki-operation'], 'operation-1')

    @unittest.skipUnless(hasattr(os, 'geteuid'), 'operator is Linux-only')
    def test_apply_completes_ca_role_voice_gis_and_matchmaking_cutover(self):
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory)
            old_ca = b'old public ca'
            old_bundle = b'old public jwks bundle\n'
            old_voice = b'old public Voice certificate'
            ca_object = {'apiVersion': 'v1', 'kind': 'Secret',
                         'metadata': {'name': pki.CA_SECRET, 'namespace': pki.NAMESPACE,
                                      'uid': 'ca-uid', 'resourceVersion': 'ca-rv'},
                         'type': 'Opaque',
                         'data': {'ca.crt': base64.b64encode(old_ca).decode(),
                                  'jwks-ca.crt': base64.b64encode(old_bundle).decode()}}
            role = role_deployment()
            role['metadata'].update({'uid': 'role-uid', 'resourceVersion': 'role-rv'})
            voice_deployment = {'apiVersion': 'apps/v1', 'kind': 'Deployment',
                                'metadata': {'name': 'voice-voice', 'namespace': pki.NAMESPACE,
                                             'uid': 'voice-deploy-uid', 'resourceVersion': 'voice-deploy-rv'},
                                'spec': {'template': {'metadata': {'annotations': {}},
                                                      'spec': {'containers': [], 'volumes': []}}}}
            metadata_values = {
                ('secret', pki.CA_SECRET): {'name': pki.CA_SECRET, 'namespace': pki.NAMESPACE,
                                            'uid': 'ca-uid', 'resourceVersion': 'ca-rv', 'type': 'Opaque'},
                ('deployment', pki.ROLE): {'name': pki.ROLE, 'namespace': pki.NAMESPACE,
                                           'uid': 'role-uid', 'resourceVersion': 'role-rv'},
                ('secret', pki.VOICE_SECRET): {'name': pki.VOICE_SECRET, 'namespace': pki.NAMESPACE,
                                               'uid': 'voice-tls-uid', 'resourceVersion': 'voice-tls-rv', 'type': 'Opaque'},
                ('deployment', 'voice-voice'): {'name': 'voice-voice', 'namespace': pki.NAMESPACE,
                                                'uid': 'voice-deploy-uid', 'resourceVersion': 'voice-deploy-rv'},
            }
            created = []
            replaced = []

            def fake_kubectl(*args, input_bytes=None):
                if args[:4] == ('get', 'secret', pki.CA_SECRET, '-o'):
                    return json.dumps(ca_object).encode()
                if args[:3] == ('get', 'secret', pki.VOICE_SECRET):
                    return base64.b64encode(old_voice)
                if args[:3] == ('get', 'secret', pki.MATCHMAKING_CA_SECRET) or \
                        args[:3] == ('get', 'secret', pki.GIS_SECRET) or \
                        args[:3] == ('get', 'secret', pki.MATCHMAKING_SECRET):
                    return b''
                if args[:3] == ('get', 'deployment', pki.ROLE):
                    return json.dumps(role).encode()
                if args[:3] == ('get', 'deployment', 'voice-voice'):
                    return json.dumps(voice_deployment).encode()
                if args[:1] == ('create',):
                    item = json.loads(input_bytes)
                    name = item['metadata']['name']
                    created.append(name)
                    return f'uid-{name} rv-{len(created)}'.encode()
                return b''

            def replace(kind, metadata_value, body):
                if kind == 'deployment' and body.get('metadata', {}).get('name') == 'voice-voice':
                    persisted = json.loads((state / 'last-apply.json').read_text(encoding='utf-8'))
                    deployment_expected = persisted['resources']['voice-voice-deployment']
                    self.assertEqual(deployment_expected['targetTemplateSha256'],
                                     pki.deployment_template_sha256(body))
                replaced.append((kind, json.loads(json.dumps(body))))
                return 'replacement-rv-' + str(len(replaced))

            fingerprints = {old_ca: 'old-ca-fingerprint', b'public ca': 'new-ca-fingerprint',
                            old_voice: 'old-voice-fingerprint', b'public cert': 'new-leaf-fingerprint'}
            with mock.patch.object(pki, 'operator_lock', return_value=nullcontext()), \
                    mock.patch.object(pki, 'verify_namespace', return_value={'uid': 'namespace-uid'}), \
                    mock.patch.object(pki, 'issuer', return_value=secret_list()), \
                    mock.patch.object(pki, 'metadata_snapshot', side_effect=lambda kind, name: (
                        {key: value for key, value in metadata_values[(kind, name)].items() if key != 'type'},
                        metadata_values[(kind, name)].get('type'))), \
                    mock.patch.object(pki, 'kubectl', side_effect=fake_kubectl), \
                    mock.patch.object(pki, 'replace_object', side_effect=replace), \
                    mock.patch.object(pki, 'fingerprint_pem', side_effect=lambda pem: fingerprints[pem]):
                pki.apply('sha256:' + 'a' * 64, state)

            self.assertCountEqual(created, [pki.MATCHMAKING_CA_SECRET, pki.GIS_SECRET, pki.MATCHMAKING_SECRET])
            self.assertEqual([kind for kind, _ in replaced], ['secret', 'deployment', 'secret', 'deployment'])
            changed_ca = replaced[0][1]
            changed_role = replaced[1][1]
            changed_voice_tls = replaced[2][1]
            changed_voice_deployment = replaced[3][1]
            self.assertEqual(base64.b64decode(changed_ca['data']['jwks-ca.crt']),
                             old_bundle + b'public ca')
            self.assertEqual(changed_voice_tls['type'], 'Opaque')
            self.assertNotIn('type', changed_voice_tls['metadata'])
            role_env = {item['name']: item['value'] for item in
                        changed_role['spec']['template']['spec']['containers'][0]['env']}
            self.assertEqual(role_env['S2S_JWKS_CA_FILE'], '/etc/voice/media/ca/jwks-ca.crt')
            receipt = json.loads((state / 'last-apply.json').read_text(encoding='utf-8'))
            self.assertEqual(receipt['phase'], 'leaf-cutover-complete')
            self.assertEqual(changed_voice_deployment['spec']['template']['metadata']['annotations'][
                'voice.io/media-pki-operation'], receipt['operationId'])
            self.assertTrue(receipt['resources'][pki.MATCHMAKING_CA_SECRET]['uid'])
            self.assertTrue(receipt['resources'][pki.GIS_SECRET]['uid'])
            self.assertTrue(receipt['resources'][pki.MATCHMAKING_SECRET]['uid'])
            self.assertEqual(receipt['resources'][pki.VOICE_SECRET]['secretType'], 'Opaque')

    @unittest.skipUnless(hasattr(os, 'geteuid'), 'operator is Linux-only')
    def test_resume_adopts_gis_create_after_unchanged_voice_and_finishes_matchmaking(self):
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory)
            state.chmod(0o700)
            old_bundle = b'previous public bundle\n'
            desired_bundle = old_bundle + b'public ca'
            operation_id = 'operation-unchanged-voice'
            receipt = {
                'namespace': pki.NAMESPACE, 'namespaceUid': 'namespace-uid', 'volume': pki.VOLUME,
                'operationId': operation_id, 'issuerImage': 'sha256:' + 'a' * 64,
                'phase': 'gis-create-intent',
                'resources': {
                    pki.CA_SECRET: {'uid': 'ca-uid', 'beforeResourceVersion': 'ca-before',
                                    'afterResourceVersion': 'ca-after', 'oldCaSha256': 'old-ca-fp',
                                    'newCaSha256': 'new-ca-fp'},
                    pki.ROLE: {'uid': 'role-uid', 'beforeResourceVersion': 'role-before',
                               'afterResourceVersion': 'role-after'},
                    pki.VOICE_SECRET: {'uid': 'voice-tls-uid', 'beforeResourceVersion': 'voice-rv',
                                       'afterResourceVersion': 'voice-rv', 'oldLeafSha256': 'leaf-fp',
                                       'newLeafSha256': 'leaf-fp'},
                    pki.MATCHMAKING_CA_SECRET: {'createOnly': True, 'uid': 'mm-ca-uid',
                                                'afterResourceVersion': 'mm-ca-rv',
                                                'publicCaSha256': 'new-ca-fp'},
                    pki.GIS_SECRET: {'createOnly': True, 'uid': None, 'afterResourceVersion': None},
                    pki.MATCHMAKING_SECRET: {'createOnly': True, 'uid': None, 'afterResourceVersion': None},
                    'voice-voice-deployment': {'uid': 'voice-deploy-uid',
                                               'beforeResourceVersion': 'voice-deploy-before',
                                               'afterResourceVersion': 'voice-deploy-after'},
                },
                'newGisLeafSha256': 'leaf-fp', 'newMatchmakingLeafSha256': 'leaf-fp',
                'rollback': {'oldJwksCaBundleBase64': base64.b64encode(old_bundle).decode()},
            }
            (state / 'last-apply.json').write_text(json.dumps(receipt), encoding='utf-8')
            os.chmod(state / 'last-apply.json', 0o600)
            ca_object = {'apiVersion': 'v1', 'kind': 'Secret',
                         'metadata': {'name': pki.CA_SECRET, 'namespace': pki.NAMESPACE,
                                      'uid': 'ca-uid', 'resourceVersion': 'ca-after'},
                         'data': {'ca.crt': base64.b64encode(b'old public ca').decode(),
                                  'jwks-ca.crt': base64.b64encode(desired_bundle).decode()}}
            role = role_deployment()
            role['metadata'].update({'uid': 'role-uid', 'resourceVersion': 'role-after'})
            role = pki.role_outbound_bundle(role)
            voice_deployment = {'apiVersion': 'apps/v1', 'kind': 'Deployment',
                                'metadata': {'name': 'voice-voice', 'namespace': pki.NAMESPACE,
                                             'uid': 'voice-deploy-uid', 'resourceVersion': 'voice-deploy-after'},
                                'spec': {'template': {'metadata': {'annotations': {
                                            'voice.io/media-jwks-cert-sha256': 'leaf-fp'}},
                                                      'spec': {'containers': [], 'volumes': []}}}}
            labels = dict(pki.NAMESPACE_LABELS)
            metas = {
                ('secret', pki.CA_SECRET): {'uid': 'ca-uid', 'resourceVersion': 'ca-after'},
                ('deployment', pki.ROLE): {'uid': 'role-uid', 'resourceVersion': 'role-after'},
                ('secret', pki.MATCHMAKING_CA_SECRET): {
                    'uid': 'mm-ca-uid', 'resourceVersion': 'mm-ca-rv', 'labels': labels,
                    'annotations': {'voice.io/media-pki-operation': operation_id}},
                ('secret', pki.VOICE_SECRET): {'uid': 'voice-tls-uid', 'resourceVersion': 'voice-rv', 'type': 'Opaque'},
                ('secret', pki.GIS_SECRET): {
                    'uid': 'gis-uid', 'resourceVersion': 'gis-rv', 'labels': labels,
                    'annotations': {'voice.io/media-pki-operation': operation_id}},
                ('deployment', 'voice-voice'): {'uid': 'voice-deploy-uid',
                                                'resourceVersion': 'voice-deploy-after'},
            }
            existing = {pki.MATCHMAKING_CA_SECRET, pki.GIS_SECRET}
            created = []
            mismatch_deployment_snapshot = {'next': False}

            def fake_kubectl(*args, input_bytes=None):
                if args[:4] == ('get', 'secret', pki.CA_SECRET, '-o'):
                    return json.dumps(ca_object).encode()
                if args[:3] == ('get', 'secret', pki.VOICE_SECRET):
                    return base64.b64encode(b'public cert')
                if args[:3] == ('get', 'deployment', pki.ROLE):
                    return json.dumps(role).encode()
                if args[:3] == ('get', 'deployment', 'voice-voice'):
                    observed = json.loads(json.dumps(voice_deployment))
                    if mismatch_deployment_snapshot['next']:
                        mismatch_deployment_snapshot['next'] = False
                        observed['metadata']['resourceVersion'] = 'interleaved-rv'
                        observed['spec']['template']['spec']['restartPolicy'] = 'Never'
                    return json.dumps(observed).encode()
                if args[:2] == ('get', 'secret'):
                    name = args[2]
                    if '--ignore-not-found' in args:
                        return ('secret/' + name).encode() if name in existing else b''
                    template = next((item for item in args if item.startswith('go-template=')), '')
                    if '{{range $key' in template:
                        return b'ca.crt' if name == pki.MATCHMAKING_CA_SECRET else b'tls.crt tls.key'
                    if '{{.type}}' in template:
                        return b'Opaque' if name == pki.MATCHMAKING_CA_SECRET else b'kubernetes.io/tls'
                    public = b'public ca' if name == pki.MATCHMAKING_CA_SECRET else b'public cert'
                    return base64.b64encode(public)
                if args[:1] == ('create',):
                    item = json.loads(input_bytes)
                    name = item['metadata']['name']
                    created.append(name)
                    return f'new-{name}-uid new-{name}-rv'.encode()
                return b''

            def fake_metadata(kind, name):
                value = {'name': name, 'namespace': pki.NAMESPACE, **metas[(kind, name)]}
                return value

            fingerprints = {b'old public ca': 'old-ca-fp', b'public ca': 'new-ca-fp',
                            b'public cert': 'leaf-fp'}
            with mock.patch.object(pki, 'operator_lock', return_value=nullcontext()), \
                    mock.patch.object(pki, 'verify_namespace', return_value={'uid': 'namespace-uid'}), \
                    mock.patch.object(pki, 'issuer', return_value=secret_list()), \
                    mock.patch.object(pki, 'metadata_snapshot', side_effect=lambda kind, name: (
                        {key: value for key, value in fake_metadata(kind, name).items() if key != 'type'},
                        fake_metadata(kind, name).get('type'))), \
                    mock.patch.object(pki, 'kubectl', side_effect=fake_kubectl), \
                    mock.patch.object(pki, 'replace_object') as replace_object, \
                    mock.patch.object(pki, 'fingerprint_pem', side_effect=lambda pem: fingerprints[pem]):
                journal = state / 'last-apply.json'
                original_journal = journal.read_bytes()
                mismatch_deployment_snapshot['next'] = True
                with self.assertRaisesRegex(ValueError, 'changed between metadata and full-object reads'):
                    pki.resume('sha256:' + 'a' * 64, state)
                self.assertEqual(journal.read_bytes(), original_journal)
                replace_object.assert_not_called()
                pki.resume('sha256:' + 'a' * 64, state)

            self.assertEqual(created, [pki.MATCHMAKING_SECRET])
            result = json.loads((state / 'last-apply.json').read_text(encoding='utf-8'))
            self.assertEqual(result['phase'], 'leaf-cutover-complete')
            self.assertEqual(result['resources'][pki.GIS_SECRET]['uid'], 'gis-uid')
            self.assertEqual(result['resources'][pki.MATCHMAKING_SECRET]['uid'],
                             'new-' + pki.MATCHMAKING_SECRET + '-uid')

    @unittest.skipUnless(hasattr(os, 'geteuid'), 'operator is Linux-only')
    def test_resume_migrates_legacy_journal_recovers_lost_deployment_receipt_and_rejects_drift(self):
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory)
            state.chmod(0o700)
            image = 'sha256:' + 'a' * 64
            old_bundle = b'previous public bundle\n'
            new_bundle = pki.append_public_ca(old_bundle, b'public ca')
            old_voice = b'old public Voice certificate'
            operation_id = 'operation-legacy-leaf-cutover'
            receipt = {
                'namespace': pki.NAMESPACE, 'namespaceUid': 'namespace-uid', 'volume': pki.VOLUME,
                'operationId': operation_id, 'issuerImage': image,
                'phase': 'leaf-cutover-started',
                'resources': {
                    pki.CA_SECRET: {'uid': 'ca-uid', 'beforeResourceVersion': 'ca-before',
                                    'afterResourceVersion': 'ca-after', 'oldCaSha256': 'old-ca-fp',
                                    'newCaSha256': 'new-ca-fp'},
                    pki.ROLE: {'uid': 'role-uid', 'beforeResourceVersion': 'role-before',
                               'afterResourceVersion': 'role-after'},
                    pki.VOICE_SECRET: {'uid': 'voice-tls-uid', 'beforeResourceVersion': 'voice-rv',
                                       'oldLeafSha256': 'old-leaf-fp', 'newLeafSha256': 'new-leaf-fp'},
                    pki.MATCHMAKING_CA_SECRET: {'createOnly': True, 'uid': 'mm-ca-uid',
                                                'afterResourceVersion': 'mm-ca-rv',
                                                'publicCaSha256': 'new-ca-fp'},
                    pki.GIS_SECRET: {'createOnly': True, 'uid': None, 'afterResourceVersion': None},
                    pki.MATCHMAKING_SECRET: {'createOnly': True, 'uid': None, 'afterResourceVersion': None},
                    'voice-voice-deployment': {'uid': 'voice-deploy-uid',
                                               'beforeResourceVersion': 'voice-deploy-rv'},
                },
                'newGisLeafSha256': 'new-leaf-fp', 'newMatchmakingLeafSha256': 'new-leaf-fp',
                'rollback': {'oldJwksCaBundleBase64': base64.b64encode(old_bundle).decode()},
            }
            journal = state / 'last-apply.json'
            journal.write_text(json.dumps(receipt), encoding='utf-8')
            os.chmod(journal, 0o600)
            ca_object = {'apiVersion': 'v1', 'kind': 'Secret',
                         'metadata': {'name': pki.CA_SECRET, 'namespace': pki.NAMESPACE,
                                      'uid': 'ca-uid', 'resourceVersion': 'ca-after'},
                         'data': {'ca.crt': base64.b64encode(b'old public ca').decode(),
                                  'jwks-ca.crt': base64.b64encode(new_bundle).decode()}}
            role = role_deployment()
            role['metadata'].update({'uid': 'role-uid', 'resourceVersion': 'role-after'})
            role = pki.role_outbound_bundle(role)
            voice_deployment = {
                'apiVersion': 'apps/v1', 'kind': 'Deployment',
                'metadata': {'name': 'voice-voice', 'namespace': pki.NAMESPACE,
                             'uid': 'voice-deploy-uid', 'resourceVersion': 'voice-deploy-rv'},
                'spec': {'template': {'metadata': {'annotations': {
                            'voice.io/media-jwks-cert-sha256': 'old-leaf-fp'}},
                      'spec': {'containers': [], 'volumes': []}}}}
            current_voice = {'certificate': old_voice}
            metadata_values = {
                ('secret', pki.CA_SECRET): {'name': pki.CA_SECRET, 'namespace': pki.NAMESPACE,
                                            'uid': 'ca-uid', 'resourceVersion': 'ca-after', 'type': 'Opaque'},
                ('deployment', pki.ROLE): {'name': pki.ROLE, 'namespace': pki.NAMESPACE,
                                           'uid': 'role-uid', 'resourceVersion': 'role-after'},
                ('secret', pki.VOICE_SECRET): {'name': pki.VOICE_SECRET, 'namespace': pki.NAMESPACE,
                                               'uid': 'voice-tls-uid', 'resourceVersion': 'voice-rv',
                                               'type': 'Opaque'},
                ('deployment', 'voice-voice'): {'name': 'voice-voice', 'namespace': pki.NAMESPACE,
                                                'uid': 'voice-deploy-uid',
                                                'resourceVersion': 'voice-deploy-rv'},
            }
            replaced = []

            def metadata_snapshot(kind, name):
                value = metadata_values[(kind, name)]
                return ({key: item for key, item in value.items() if key != 'type'},
                        value.get('type'))

            def fake_kubectl(*args, input_bytes=None):
                if args[:4] == ('get', 'secret', pki.CA_SECRET, '-o'):
                    return json.dumps(ca_object).encode()
                if args[:3] == ('get', 'secret', pki.VOICE_SECRET):
                    return base64.b64encode(current_voice['certificate'])
                if args[:3] == ('get', 'deployment', pki.ROLE):
                    return json.dumps(role).encode()
                if args[:3] == ('get', 'deployment', 'voice-voice'):
                    return json.dumps(voice_deployment).encode()
                if args[:2] == ('rollout', 'status'):
                    return b''
                raise AssertionError('unexpected Kubernetes call: ' + repr(args[:3]))

            def replace(kind, _metadata_value, body):
                if kind == 'secret':
                    persisted = json.loads(journal.read_text(encoding='utf-8'))
                    self.assertEqual(persisted['resources'][pki.VOICE_SECRET]['secretType'], 'Opaque')
                    current_voice['certificate'] = base64.b64decode(body['data']['tls.crt'])
                    metadata_values[('secret', pki.VOICE_SECRET)]['resourceVersion'] = 'replacement-rv-1'
                else:
                    voice_deployment.clear()
                    voice_deployment.update(json.loads(json.dumps(body)))
                    voice_deployment['metadata']['resourceVersion'] = 'replacement-rv-2'
                    metadata_values[('deployment', 'voice-voice')]['resourceVersion'] = 'replacement-rv-2'
                replaced.append((kind, json.loads(json.dumps(body))))
                return 'replacement-rv-' + str(len(replaced))

            fingerprints = {b'old public ca': 'old-ca-fp', b'public ca': 'new-ca-fp',
                            old_voice: 'old-leaf-fp', b'public cert': 'new-leaf-fp'}
            real_write_receipt = pki.write_receipt
            receipt_write_lost = {'done': False}

            def lose_deployment_receipt_once(path, candidate, temp_path):
                deployment_expected = candidate['resources']['voice-voice-deployment']
                if (not receipt_write_lost['done'] and
                        deployment_expected.get('afterResourceVersion') == 'replacement-rv-2'):
                    receipt_write_lost['done'] = True
                    raise OSError('simulated lost receipt write after Deployment replace')
                return real_write_receipt(path, candidate, temp_path)

            common_patches = [
                mock.patch.object(pki, 'operator_lock', return_value=nullcontext()),
                mock.patch.object(pki, 'verify_namespace', return_value={'uid': 'namespace-uid'}),
                mock.patch.object(pki, 'issuer', return_value=secret_list()),
                mock.patch.object(pki, 'metadata_snapshot', side_effect=metadata_snapshot),
                mock.patch.object(pki, 'kubectl', side_effect=fake_kubectl),
                mock.patch.object(pki, 'replace_object', side_effect=replace),
                mock.patch.object(pki, 'write_receipt', side_effect=lose_deployment_receipt_once),
                mock.patch.object(pki, 'reconcile_operation_secret'),
                mock.patch.object(pki, 'fingerprint_pem', side_effect=lambda pem: fingerprints[pem]),
            ]
            with ExitStack() as stack:
                for patcher in common_patches:
                    stack.enter_context(patcher)
                with self.assertRaisesRegex(OSError, 'simulated lost receipt write'):
                    pki.resume(image, state)
                interrupted = json.loads(journal.read_text(encoding='utf-8'))
                self.assertNotIn('afterResourceVersion',
                                 interrupted['resources']['voice-voice-deployment'])
                target_template_sha = interrupted['resources']['voice-voice-deployment'][
                    'targetTemplateSha256']
                self.assertEqual(target_template_sha,
                                 pki.deployment_template_sha256(replaced[1][1]))
                self.assertEqual(interrupted['phase'], 'voice-leaf-rotated')

                # Drift inside the receipt-write window is rejected even when
                # both operation annotations remain unchanged.
                voice_deployment['spec']['template']['spec']['restartPolicy'] = 'Never'
                voice_deployment['metadata']['resourceVersion'] = 'drift-before-replay-rv'
                metadata_values[('deployment', 'voice-voice')]['resourceVersion'] = 'drift-before-replay-rv'
                with self.assertRaisesRegex(ValueError, 'template differs from the recorded operation target'):
                    pki.resume(image, state)
                refused = json.loads(journal.read_text(encoding='utf-8'))
                self.assertNotIn('afterResourceVersion',
                                 refused['resources']['voice-voice-deployment'])
                self.assertEqual(refused['phase'], 'voice-leaf-rotated')

                # Exact same-operation template replay adopts the returned RV.
                voice_deployment['spec']['template'] = json.loads(json.dumps(
                    replaced[1][1]['spec']['template']))
                voice_deployment['metadata']['resourceVersion'] = 'recovered-rv'
                metadata_values[('deployment', 'voice-voice')]['resourceVersion'] = 'recovered-rv'
                pki.resume(image, state)
                recovered = json.loads(journal.read_text(encoding='utf-8'))
                self.assertEqual(recovered['resources']['voice-voice-deployment']['afterResourceVersion'],
                                 'recovered-rv')

                # A later edit preserving the operation annotations still fails CAS.
                metadata_values[('deployment', 'voice-voice')]['resourceVersion'] = 'drift-rv'
                voice_deployment['metadata']['resourceVersion'] = 'drift-rv'
                with self.assertRaisesRegex(ValueError, 'changed after the recorded operation restart'):
                    pki.resume(image, state)
                self.assertEqual(json.loads(journal.read_text(encoding='utf-8'))['phase'],
                                 'leaf-cutover-complete')

            self.assertEqual([kind for kind, _ in replaced], ['secret', 'deployment'])
            voice_replacement = replaced[0][1]
            self.assertEqual(voice_replacement['type'], 'Opaque')
            self.assertNotIn('type', voice_replacement['metadata'])
            self.assertEqual(voice_replacement['metadata']['annotations'][
                'voice.io/media-pki-operation'], operation_id)
            deployment_annotations = replaced[1][1]['spec']['template']['metadata']['annotations']
            self.assertEqual(deployment_annotations['voice.io/media-jwks-cert-sha256'], 'new-leaf-fp')
            self.assertEqual(deployment_annotations['voice.io/media-pki-operation'], operation_id)
            migrated = json.loads(journal.read_text(encoding='utf-8'))
            self.assertEqual(migrated['resources'][pki.VOICE_SECRET]['secretType'], 'Opaque')
            self.assertEqual(migrated['resources'][pki.VOICE_SECRET]['afterResourceVersion'],
                             'replacement-rv-1')
            self.assertEqual(migrated['phase'], 'leaf-cutover-complete')

    def test_resume_accepts_unchanged_voice_leaf_only_at_recorded_resource_version(self):
        expected = {'oldLeafSha256': 'a' * 64, 'newLeafSha256': 'a' * 64,
                    'beforeResourceVersion': '17', 'afterResourceVersion': '17'}
        self.assertTrue(pki.accept_unchanged_voice_leaf('a' * 64, '17', expected))
        self.assertEqual(expected['afterResourceVersion'], '17')
        self.assertFalse(pki.accept_unchanged_voice_leaf('a' * 64, '18', expected))
        self.assertFalse(pki.accept_unchanged_voice_leaf('b' * 64, '17', expected))

    def test_trust_mutation_phases_are_forward_only(self):
        for phase in ('trust-added', 'role-trust-switched', 'leaf-cutover-started',
                      'voice-leaf-rotated', 'gis-create-intent', 'gis-created'):
            with self.subTest(phase=phase), self.assertRaisesRegex(ValueError, 'resume forward'):
                pki.require_rollback_allowed(phase)

    @unittest.skipUnless(hasattr(os, 'geteuid'), 'operator is Linux-only')
    def test_apply_reentry_stops_before_namespace_or_cluster_access(self):
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory)
            journal = state / 'last-apply.json'
            journal.write_text(json.dumps({'namespace': pki.NAMESPACE, 'volume': pki.VOLUME,
                                           'phase': 'voice-leaf-rotated'}), encoding='utf-8')
            os.chmod(journal, 0o600)
            with mock.patch.object(pki, 'operator_lock', return_value=nullcontext()), \
                    mock.patch.object(pki, 'verify_namespace') as verify_namespace, \
                    mock.patch.object(pki, 'issuer') as issuer, \
                    mock.patch.object(pki, 'kubectl') as kubectl:
                with self.assertRaisesRegex(ValueError, 'recovery must continue forward'):
                    pki.apply('sha256:' + 'a' * 64, state)
            verify_namespace.assert_not_called()
            issuer.assert_not_called()
            kubectl.assert_not_called()

    @unittest.skipUnless(hasattr(os, 'geteuid'), 'operator is Linux-only')
    def test_renew_is_locked_and_refuses_to_mutate_receipt_pinned_issuer(self):
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory)
            state.chmod(0o700)
            with mock.patch.object(pki, 'operator_lock', return_value=nullcontext()) as lock, \
                    mock.patch.object(pki, 'issuer') as issuer:
                pki.renew('sha256:' + 'a' * 64, state)
            lock.assert_called_once_with(state / 'apply.lock')
            issuer.assert_called_once_with('sha256:' + 'a' * 64, 'renew-leaves')
            (state / 'last-apply.json').write_text('{}', encoding='utf-8')
            with mock.patch.object(pki, 'operator_lock', return_value=nullcontext()), \
                    mock.patch.object(pki, 'issuer') as issuer:
                with self.assertRaisesRegex(ValueError, 'disabled after the one-time cutover'):
                    pki.renew('sha256:' + 'a' * 64, state)
            issuer.assert_not_called()

    def test_issuer_profiles_are_name_constrained_server_only_and_fixed(self):
        self.assertIn('CA:TRUE,pathlen:0', issuer.CA_EXTENSIONS)
        self.assertIn('nameConstraints=critical,permitted;DNS:voice-voice,permitted;DNS:voice-gameintegration',
                      issuer.CA_EXTENSIONS)
        self.assertIn('permitted;DNS:voice-matchmaking.voice-dev.svc', issuer.CA_EXTENSIONS)
        self.assertNotIn('voice-matchmaking.voice-staging.svc', issuer.CA_EXTENSIONS)
        for name in issuer.LEAVES:
            profile = issuer.leaf_extensions(name)
            self.assertIn('extendedKeyUsage=serverAuth', profile)
            self.assertIn('subjectAltName=DNS:' + name, profile)
            self.assertNotIn('clientAuth', profile)
        for name in ('voice-space', 'voice-gateway', 'voice-gameintegration.example',
                     'voice-matchmaking', 'voice-matchmaking.voice-staging.svc'):
            with self.assertRaises(ValueError):
                issuer.leaf_extensions(name)
    def test_runtime_chain_validation_rejects_space_and_gateway_hostnames(self):
        with mock.patch.object(issuer, 'run') as verify, mock.patch.object(issuer, 'must_fail') as reject:
            issuer.verify_leaf(Path('/state/generation'), 'voice-gameintegration', Path('/leaf.crt'))
        self.assertEqual(verify.call_count, 1)
        rejected_names = [call.args[-2] for call in reject.call_args_list]
        self.assertEqual(rejected_names, ['voice-space', 'voice-gateway', 'voice-gateway.example',
                                         'voice-matchmaking.voice-staging.svc'])

    def test_role_rejects_ambiguous_outbound_projection(self):
        deployment = role_deployment()
        deployment['spec']['template']['spec']['volumes'][0]['secret']['items'].append(
            {'key': 'jwks-ca.crt', 'path': 'other.crt'})
        with self.assertRaises(ValueError):
            pki.role_outbound_bundle(deployment)


if __name__ == '__main__':
    unittest.main()
