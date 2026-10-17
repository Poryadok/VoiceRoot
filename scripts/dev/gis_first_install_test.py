"""Offline contracts for the fixed voice-dev GIS first-install helpers."""
import base64
import copy
import hashlib
import io
import json
import tarfile
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import gis_dev_first_install as gis


def encoded(values):
    return {key: base64.b64encode(value).decode('ascii') for key, value in values.items()}


def buildx_artifact(source, run_id, attempt, *, revision=None, context='src/backend',
                    output_digest=None, started='2026-10-17T00:01:33Z',
                    finished='2026-10-17T00:02:08Z'):
    """Build a minimal authenticated Buildx history-record artifact."""
    revision = revision or source
    config = 'd' * 64
    child = json.dumps({'config': {'digest': 'sha256:' + config}}, separators=(',', ':')).encode()
    child_digest = hashlib.sha256(child).hexdigest()
    image_index = json.dumps({'schemaVersion': 2,
        'mediaType': 'application/vnd.oci.image.index.v1+json',
        'manifests': [{'mediaType': 'application/vnd.oci.image.manifest.v1+json',
            'digest': 'sha256:' + child_digest, 'size': len(child),
            'platform': {'os': 'linux', 'architecture': 'amd64'}}]},
        separators=(',', ':')).encode()
    actual_output_digest = hashlib.sha256(image_index).hexdigest()
    output_digest = output_digest or actual_output_digest
    builder = f'https://github.com/Poryadok/VoiceRoot/actions/runs/{run_id}/attempts/{attempt}'
    provenance = json.dumps({'buildDefinition': {'externalParameters': {
        'request': {'root': {'configSource': {'path': 'Dockerfile'},
            'request': {'args': {'vcs:localdir:context': context,
                'vcs:localdir:dockerfile': 'src/backend/gameintegration',
                'vcs:revision': revision,
                'vcs:source': 'https://github.com/Poryadok/VoiceRoot'}}}}}},
        'runDetails': {'builder': {'id': builder},
            'metadata': {'startedOn': started, 'finishedOn': finished}}},
        separators=(',', ':')).encode()
    blobs = {}

    def blob(value):
        digest = hashlib.sha256(value).hexdigest()
        blobs['blobs/sha256/' + digest] = value
        return {'digest': 'sha256:' + digest, 'size': len(value)}

    config_desc = blob(json.dumps({'FrontendAttrs': {
        'attest:provenance': 'type=provenance,builder-id=' + builder,
        'filename': 'Dockerfile', 'vcs:localdir:context': context,
        'vcs:localdir:dockerfile': 'src/backend/gameintegration',
        'vcs:revision': revision, 'vcs:source': 'https://github.com/Poryadok/VoiceRoot'},
        'Exporters': [{'Type': 'image', 'Attrs': {'name': gis.IMAGE + ':' + source,
            'push': 'true'}}],
        'Result': {'Results': {'0': {'media_type': 'application/vnd.oci.image.index.v1+json',
            'digest': 'sha256:' + output_digest, 'size': len(image_index)}}},
        'DefaultPlatform': 'linux/amd64'}, separators=(',', ':')).encode())
    layers = [dict(blob(image_index), mediaType='application/json',
        annotations={'vnd.export-build.descriptor.mediatype': 'application/vnd.oci.image.index.v1+json'}),
        dict(blob(provenance), mediaType='application/vnd.in-toto+json',
            annotations={'in-toto.io/predicate-type': 'https://slsa.dev/provenance/v1'})]
    record_manifest = json.dumps({'schemaVersion': 2,
        'mediaType': 'application/vnd.oci.image.manifest.v1+json',
        'config': dict(config_desc, mediaType='application/vnd.buildkit.historyrecord.v0'),
        'layers': layers}, separators=(',', ':')).encode()
    record_desc = blob(record_manifest)
    archive_index = json.dumps({'schemaVersion': 2,
        'mediaType': 'application/vnd.oci.image.index.v1+json',
        'manifests': [dict(record_desc, mediaType='application/vnd.oci.image.manifest.v1+json')]},
        separators=(',', ':')).encode()
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode='w:gz') as tar:
        for name, value in [('index.json', archive_index), *blobs.items()]:
            entry = tarfile.TarInfo(name)
            entry.size = len(value)
            entry.mode = 0o600
            tar.addfile(entry, io.BytesIO(value))
    return output.getvalue(), actual_output_digest, child, config, image_index


class FakeKube:
    def __init__(self):
        self.namespace = {'metadata': {'name': 'voice-dev', 'uid': '00000000-0000-0000-0000-000000000001'}}
        self.secrets = {
            'voice-app-secrets': {'apiVersion': 'v1', 'kind': 'Secret',
                'metadata': {'name': 'voice-app-secrets', 'namespace': 'voice-dev',
                    'uid': '00000000-0000-0000-0000-000000000002', 'resourceVersion': '12'},
                'type': 'Opaque', 'data': encoded({'POSTGRES_PASSWORD': b'admin', 'KEEP': b'existing'})},
            'voice-media-ca': {'apiVersion': 'v1', 'kind': 'Secret',
                'metadata': {'name': 'voice-media-ca', 'namespace': 'voice-dev',
                    'uid': '00000000-0000-0000-0000-000000000003', 'resourceVersion': '1'},
                'data': encoded({'ca.crt': b'inbound-ca', 'jwks-ca.crt': b'outbound-jwks-ca'})},
            'voice-gameintegration-jwks-tls': {'apiVersion': 'v1', 'kind': 'Secret',
                'metadata': {'name': 'voice-gameintegration-jwks-tls', 'namespace': 'voice-dev',
                    'uid': '00000000-0000-0000-0000-000000000004', 'resourceVersion': '2'},
                'data': encoded({'tls.crt': b'cert', 'tls.key': b'tls-key'})},
        }
        self.writes = []
        self.conflict = False

    def get_namespace(self):
        return copy.deepcopy(self.namespace)

    def get_secret(self, name):
        if name not in self.secrets:
            return None
        return copy.deepcopy(self.secrets[name])

    def create_secret(self, body):
        self.writes.append(('create', body['metadata']['name']))
        if body['metadata']['name'] in self.secrets:
            raise RuntimeError('already exists')
        self.secrets[body['metadata']['name']] = copy.deepcopy(body)

    def replace_secret(self, body, *, expected_uid, expected_resource_version):
        name = body['metadata']['name']
        live = self.secrets[name]
        if (self.conflict or live['metadata']['uid'] != expected_uid
                or live['metadata']['resourceVersion'] != expected_resource_version):
            raise RuntimeError('conflict')
        self.writes.append(('replace', name))
        body = copy.deepcopy(body)
        body['metadata']['resourceVersion'] = str(int(expected_resource_version) + 1)
        self.secrets[name] = body


def materials():
    return {
        'GAME_INTEGRATION_DB_PASSWORD': b'runtime-password',
        'GAME_INTEGRATION_DATABASE_URL': b'postgres://gameintegration_runtime:runtime-password@voice-postgres:5432/game_integration_db?sslmode=disable',
        'voice-gameintegration-principal-signing': {
            'current.pem': b'private-current', 'next.pem': b'private-next', 'active-kid': b'current'},
        'voice-gameintegration-workload-credentials': {
            'GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64': base64.b64encode(b'n'*32),
            'GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64': base64.b64encode(b'r'*32)},
    }


class SourceAuthorityTests(unittest.TestCase):
    def test_build_receipt_binds_clean_source_and_immutable_image(self):
        source = 'a' * 40
        image = 'ghcr.io/poryadok/voiceroot/gameintegration@sha256:' + 'b' * 64
        receipt = {'schema': gis.BUILD_RECEIPT_SCHEMA, 'source_sha': source,
            'image_ref': image, 'build_context_sha256': gis.hashlib.sha256(('e' * 40).encode()).hexdigest(), 'image_config_sha256': 'd' * 64,
            'working_tree_clean': True, 'builder_kind': 'github-actions',
            'builder_run': 11, 'builder_job': 12,
            'builder_sha': source, 'builder_tree': 'e' * 40,
            'builder_artifact_id': 13, 'builder_artifact_sha256': 'f' * 64,
            'builder_output_index_sha256': 'b' * 64, 'builder_provenance_sha256': '1' * 64}
        self.assertTrue(gis.validate_build_receipt(source, image, receipt, clean=True))
        with self.assertRaises(gis.Blocked):
            gis.validate_build_receipt(source, image, receipt, clean=False)
        with self.assertRaises(gis.Blocked):
            gis.validate_build_receipt(source, image, dict(receipt, working_tree_clean=False), clean=True)
        with self.assertRaises(gis.Blocked):
            gis.validate_build_receipt(source, image.replace('b'*64, 'e'*64), receipt, clean=True)

    def test_source_build_receipt_is_produced_from_clean_source_and_inspected_oci(self):
        source = 'a' * 40
        tree = 'c' * 40
        config = 'd' * 64
        manifest_raw = json.dumps({'config': {'digest': 'sha256:' + config}}).encode()
        image_digest = gis.hashlib.sha256(manifest_raw).hexdigest()
        image = gis.IMAGE + '@sha256:' + image_digest
        calls = []
        artifact_raw, image_digest, child_raw, _, image_index = buildx_artifact(source, 11, 1)
        image = gis.IMAGE + '@sha256:' + image_digest
        child_digest = gis.hashlib.sha256(child_raw).hexdigest()
        api_run = {'id': 11, 'status': 'completed', 'conclusion': 'success',
            'event': 'push', 'head_branch': 'master', 'head_sha': source, 'workflow_id': 9,
            'head_repository': {'full_name': 'Poryadok/VoiceRoot'}, 'run_attempt': 1}
        api_workflow = {'path': '.github/workflows/ci.yml'}
        api_jobs = {'jobs': [{'id': 12, 'status': 'completed', 'conclusion': 'success',
            'name': 'staging-images-push (gameintegration)',
            'steps': [{'name': 'Set up job', 'conclusion': 'success'},
                {'name': 'Build and push gameintegration image', 'conclusion': 'success',
                    'started_at': '2026-10-17T00:01:32Z', 'completed_at': '2026-10-17T00:02:22Z'}]}]}
        api_artifacts = {'total_count': 1, 'artifacts': [{'id': 13, 'name': 'VoiceRoot.dockerbuild',
            'expired': False, 'size_in_bytes': len(artifact_raw)}]}
        workflow_source = ('if grep -qx "$S" scripts/ci/backend-docker-context.txt\n'
            'echo "context=./src/backend"\n  staging-images-push:\n'
            '- name: Build and push ${{ matrix.service }} image\n'
            'docker/build-push-action@v6\ncontext: ${{ steps.dockerctx.outputs.context }}\n'
            'file: ${{ steps.dockerctx.outputs.file }}\npush: true\n'
            '  staging-images-promote:\n')

        def runner(argv, **_kwargs):
            calls.append(argv)
            if argv[0] == 'git' and 'rev-parse' in argv:
                return SimpleNamespace(returncode=0, stdout=source if argv[-1] == 'HEAD' else tree, stderr='')
            if argv[0] == 'git' and 'status' in argv:
                return SimpleNamespace(returncode=0, stdout='', stderr='')
            if argv[0] == 'git' and 'show' in argv:
                content = 'gameintegration\n' if argv[-1].endswith('backend-docker-context.txt') else workflow_source
                return SimpleNamespace(returncode=0, stdout=content, stderr='')
            if argv[:2] == ['gh', 'api']:
                endpoint = argv[2]
                if endpoint.endswith('/artifacts/13/zip'):
                    return SimpleNamespace(returncode=0, stdout=artifact_raw, stderr=b'')
                value = (api_run if endpoint.endswith('/runs/11') else
                    api_workflow if endpoint.endswith('/workflows/9') else
                    api_artifacts if endpoint.endswith('/artifacts?per_page=100') else api_jobs)
                return SimpleNamespace(returncode=0, stdout=json.dumps(value), stderr='')
            if argv[:5] == ['docker', 'buildx', 'imagetools', 'inspect', '--raw']:
                raw = child_raw if argv[-1].endswith('@sha256:' + child_digest) else image_index
                return SimpleNamespace(returncode=0, stdout=raw, stderr=b'')
            if argv[:2] == ['docker', 'pull']:
                return SimpleNamespace(returncode=0, stdout=b'', stderr=b'')
            if argv[:3] == ['docker', 'image', 'inspect']:
                return SimpleNamespace(returncode=0, stdout=json.dumps([{
                    'Id': 'sha256:' + config,
                    'RepoDigests': [image],
                    'Config': {'Labels': {
                        'org.opencontainers.image.revision': source,
                        'io.voice.build.context.sha256': gis.hashlib.sha256(tree.encode()).hexdigest(),
                    }},
                }]).encode(), stderr=b'')
            raise AssertionError(argv)

        actual_sha, receipt = gis.source_build_receipt(Path('.'), image,
            builder_run=11, builder_job=12, runner=runner)
        self.assertEqual(actual_sha, source)
        self.assertEqual(receipt['image_config_sha256'], config)
        self.assertEqual(receipt['builder_tree'], tree)
        self.assertEqual(receipt['builder_artifact_id'], 13)
        self.assertEqual(receipt['builder_artifact_sha256'], gis.hashlib.sha256(artifact_raw).hexdigest())
        self.assertEqual(receipt['builder_output_index_sha256'], image_digest)
        self.assertTrue(gis._valid_sha(receipt['builder_provenance_sha256']))
        self.assertIn('docker', calls[-1][0])

        # The producer workflow template is matrix-expanded by GitHub at run
        # time. Keep both sides bound: the API step must name gameintegration,
        # and source must contain the reviewed matrix template.
        self.assertEqual(api_jobs['jobs'][0]['steps'][1]['name'],
            'Build and push gameintegration image')
        workflow_source = workflow_source.replace(
            '- name: Build and push ${{ matrix.service }} image',
            '- name: Build and push ${{ matrix.other }} image')
        with self.assertRaises(gis.Blocked) as blocked:
            gis.source_build_receipt(Path('.'), image,
                builder_run=11, builder_job=12, runner=runner)
        self.assertEqual(str(blocked.exception), 'gis_dev_builder_source_context_mismatch')

    def test_buildx_record_rejects_wrong_run_revision_context_and_output_digest(self):
        source = 'a' * 40
        image = gis.IMAGE + '@sha256:' + 'b' * 64
        step = {'started_at': '2026-10-17T00:01:32Z', 'completed_at': '2026-10-17T00:02:22Z'}
        for run_id, revision, context, digest in [
            (99, source, 'src/backend', None),
            (11, 'c' * 40, 'src/backend', None),
            (11, source, 'untrusted/context', None),
            (11, source, 'src/backend', 'f' * 64),
        ]:
            raw, actual_digest, _child, _config, _image_index = buildx_artifact(source, run_id, 1,
                revision=revision, context=context, output_digest=digest)
            target = gis.IMAGE + '@sha256:' + (digest or actual_digest)
            with self.assertRaises(gis.Blocked):
                gis._verify_buildx_artifact(raw, 13, len(raw), target, 11, 1,
                    source, step, None)

    def test_dirty_source_blocks_before_image_inspection(self):
        source = 'a' * 40
        tree = 'c' * 40
        image = gis.IMAGE + '@sha256:' + 'b' * 64
        calls = []

        def runner(argv, **_kwargs):
            calls.append(argv)
            output = source if argv[-1] == 'HEAD' else tree
            if 'status' in argv:
                output = ' M scripts/dev/gis_dev_first_install.py'
            return SimpleNamespace(returncode=0, stdout=output, stderr='')

        with self.assertRaises(gis.Blocked):
            gis.source_build_receipt(Path('.'), image, builder_run=11, builder_job=12, runner=runner)
        self.assertFalse(any(argv[0] == 'docker' for argv in calls))

    def test_tls_validator_requires_explicit_exact_dns_san_and_matching_key(self):
        outputs = [
            SimpleNamespace(stdout=b''),
            SimpleNamespace(stdout='X509v3 Subject Alternative Name:\n    DNS:voice-gameintegration\n'),
            SimpleNamespace(stdout=b'public-key\n'),
            SimpleNamespace(stdout=b'public-key\n'),
        ]
        calls = []

        def run(argv, **_kwargs):
            calls.append(argv)
            return outputs.pop(0)

        with patch.object(gis.subprocess, 'run', side_effect=run):
            self.assertTrue(gis.validate_tls_leaf(b'certificate', b'private-key', b'ca'))
        self.assertEqual(calls[1][-2:], ['-ext', 'subjectAltName'])

        with patch.object(gis.subprocess, 'run', side_effect=[
            SimpleNamespace(stdout=b''),
            SimpleNamespace(stdout='X509v3 Subject Alternative Name:\n    DNS:other-host\n'),
        ]) as runner:
            with self.assertRaises(gis.Blocked):
                gis.validate_tls_leaf(b'certificate', b'private-key', b'ca')
        self.assertEqual(runner.call_count, 2)

    def test_secret_material_producer_emits_exact_signer_contract(self):
        outputs = []

        def keygen(argv, **_kwargs):
            path = Path(argv[argv.index("-out") + 1])
            value = b"generated-current" if path.name == "current.pem" else b"generated-next"
            path.write_bytes(value)
            outputs.append(path.name)
            return type("Result", (), {"stdout": b"", "stderr": b"", "returncode": 0})()

        with patch.object(gis.subprocess, "run", side_effect=keygen):
            material = gis.generate_secret_material()
        signer = material[gis.SIGNER_SECRET]
        self.assertEqual(set(signer), {"current.pem", "next.pem", "active-kid"})
        self.assertEqual(signer["active-kid"], b"current")
        self.assertEqual(outputs, ["current.pem", "next.pem"])
        self.assertEqual(set(material[gis.WORKLOAD_SECRET]), {
            "GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64", "GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64"})
        plan = gis.build_secret_plan(FakeKube(), tls_validator=lambda *_: None,
            signer_validator=lambda _values: None, material_factory=lambda: material)
        signer_operation = next(row for row in plan['operations'] if row['name'] == gis.SIGNER_SECRET)
        self.assertEqual(signer_operation['body']['data']['active-kid'], base64.b64encode(b'current').decode())


class SecretPreparationTests(unittest.TestCase):
    def test_tls_failure_precedes_material_generation_and_any_write(self):
        kube = FakeKube()
        events = []
        def reject(_cert, _key, _ca):
            events.append('tls')
            raise gis.Blocked('gis_dev_tls_invalid')
        with self.assertRaises(gis.Blocked):
            gis.build_secret_plan(kube, tls_validator=reject,
                material_factory=lambda: events.append('generate') or materials())
        self.assertEqual(events, ['tls'])
        self.assertEqual(kube.writes, [])

    def test_valid_tls_plan_preserves_app_keys_and_never_writes_until_commit(self):
        kube = FakeKube()
        calls = []
        plan = gis.build_secret_plan(kube,
            tls_validator=lambda cert, key, ca: calls.append((cert, key, ca)),
            signer_validator=lambda _values: None,
            material_factory=materials)
        self.assertEqual(calls, [(b'cert', b'tls-key', b'outbound-jwks-ca')])
        self.assertEqual(kube.writes, [])
        app = next(row['body'] for row in plan['operations'] if row['name'] == 'voice-app-secrets')
        self.assertEqual(app['data']['KEEP'], encoded({'KEEP': b'existing'})['KEEP'])
        self.assertIn('POSTGRES_PASSWORD', app['data'])
        gis.apply_secret_plan(kube, plan)
        self.assertEqual(kube.writes[0], ('replace', 'voice-app-secrets'))
        self.assertEqual(kube.secrets['voice-app-secrets']['data']['KEEP'], encoded({'KEEP': b'existing'})['KEEP'])

    def test_missing_jwks_ca_key_blocks_without_material_generation_or_write(self):
        kube = FakeKube()
        kube.secrets['voice-media-ca']['data'] = encoded({'ca.crt': b'inbound-ca'})
        events = []
        with self.assertRaises(gis.Blocked):
            gis.build_secret_plan(kube, tls_validator=lambda *_: events.append('tls'),
                material_factory=lambda: events.append('generate') or materials())
        self.assertEqual(events, [])
        self.assertEqual(kube.writes, [])

    def test_partial_secret_identity_and_namespace_mismatch_fail_before_write(self):
        kube = FakeKube()
        kube.secrets['voice-gameintegration-principal-signing'] = {
            'metadata': {'name': 'voice-gameintegration-principal-signing', 'namespace': 'voice-dev',
                'uid': '00000000-0000-0000-0000-000000000005', 'resourceVersion': '1'},
            'data': encoded({'current.pem': b'existing'})}
        with self.assertRaises(gis.Blocked):
            gis.build_secret_plan(kube, tls_validator=lambda *_: None,
                signer_validator=lambda _values: None, material_factory=materials)
        self.assertEqual(kube.writes, [])

        wrong = FakeKube()
        wrong.namespace['metadata']['name'] = 'voice-staging'
        with self.assertRaises(gis.Blocked):
            gis.build_secret_plan(wrong, tls_validator=lambda *_: None, material_factory=materials)
        self.assertEqual(wrong.writes, [])

    def test_resource_version_conflict_does_not_retry_or_overwrite(self):
        kube = FakeKube()
        plan = gis.build_secret_plan(kube, tls_validator=lambda *_: None,
            signer_validator=lambda _values: None, material_factory=materials)
        kube.conflict = True
        with self.assertRaises(gis.Blocked):
            gis.apply_secret_plan(kube, plan)

    def test_secret_mutations_carry_same_operation_marker(self):
        kube = FakeKube()
        plan = gis.build_secret_plan(kube, operation='123456abcdef',
            tls_validator=lambda *_: None, signer_validator=lambda _values: None,
            material_factory=materials)
        self.assertEqual({row['body']['metadata']['annotations'][gis.OPERATION_ANNOTATION]
            for row in plan['operations']}, {'123456abcdef'})


class ReadOnlyDatabasePreflightTests(unittest.TestCase):
    def test_database_presence_query_is_read_only_and_uses_no_credential_argument(self):
        identity = {'namespace_uid': 'n', 'service_uid': 's',
            'service_spec_sha256': 'a' * 64, 'endpoints': [{'uid': 'e', 'pod_uids': ['p']}]}

        class Kube:
            def __init__(self): self.calls = []
            def run(self, args, timeout=30):
                self.calls.append(args)
                if args[:2] == ['get', 'endpointslices']:
                    return {'items': [{'endpoints': [{'conditions': {'ready': True},
                        'targetRef': {'kind': 'Pod', 'name': 'voice-postgres-0', 'uid': 'p'}}]}]}
                return 't\n'

        kube = Kube()
        with patch('gis_prerequisite._endpoint_identity', return_value=identity):
            self.assertEqual(gis._database_exists_readonly(kube, identity), identity)
        query = kube.calls[-1]
        self.assertEqual(query[:2], ['exec', 'voice-postgres-0'])
        self.assertIn('pg_database', query[-1])
        self.assertNotIn('POSTGRES_PASSWORD', ' '.join(query))
        self.assertNotIn('PGPASSWORD=', ' '.join(query))

    def test_missing_database_fails_closed_before_any_write(self):
        identity = {'namespace_uid': 'n', 'service_uid': 's',
            'service_spec_sha256': 'a' * 64, 'endpoints': [{'uid': 'e', 'pod_uids': ['p']}]}

        class Kube:
            writes = []
            def run(self, args, timeout=30):
                if args[:2] == ['get', 'endpointslices']:
                    return {'items': [{'endpoints': [{'conditions': {'ready': True},
                        'targetRef': {'kind': 'Pod', 'name': 'voice-postgres-0', 'uid': 'p'}}]}]}
                return 'f\n'

        with patch('gis_prerequisite._endpoint_identity', return_value=identity):
            with self.assertRaises(gis.Blocked):
                gis._database_exists_readonly(Kube(), identity)


class InstallerInvocationTests(unittest.TestCase):
    def test_callable_flow_orders_database_check_before_secrets_and_runs_shared_owner(self):
        import gis_first_install as shared_gis
        import gis_prerequisite

        image = gis.IMAGE + '@sha256:' + 'b' * 64
        source = 'a' * 40
        build = {'schema': gis.BUILD_RECEIPT_SCHEMA, 'source_sha': source,
            'image_ref': image, 'build_context_sha256': gis.hashlib.sha256(('e' * 40).encode()).hexdigest(),
            'image_config_sha256': 'd' * 64, 'working_tree_clean': True,
            'builder_kind': 'github-actions', 'builder_run': 11, 'builder_job': 12,
            'builder_sha': source, 'builder_tree': 'e' * 40,
            'builder_artifact_id': 13, 'builder_artifact_sha256': 'f' * 64,
            'builder_output_index_sha256': 'b' * 64, 'builder_provenance_sha256': '1' * 64}
        events = []
        operation = '123456abcdef'
        namespace_uid = '00000000-0000-0000-0000-000000000001'
        database_identity = {'namespace_uid': namespace_uid,
            'service_uid': '00000000-0000-0000-0000-000000000002',
            'service_spec_sha256': 'e' * 64,
            'endpoints': [{'uid': '00000000-0000-0000-0000-000000000003',
                'pod_uids': ['00000000-0000-0000-0000-000000000004']}]}
        inputs = {'schema': 'voice-gis-prerequisite-v1',
            'configmaps': {'provision': 'voice-provision-runtime-role-sql',
                'migrations': 'voice-gameintegration-migrations',
                'verify': 'voice-gameintegration-db-verifier-sql'},
            'jobs': {'provision': 'voice-provision-gameintegration-runtime-role',
                'migrate': 'voice-migrate-gameintegration-db',
                'verify': 'voice-verify-gameintegration-db'},
            'job_images': {role: {'gameintegration': logical} for role, logical in
                (('provision', 'postgres'), ('migrate', 'migrate'), ('verify', 'postgres'))},
            'images': {'postgres': 'postgres@sha256:' + 'c' * 64,
                'migrate': 'migrate@sha256:' + 'd' * 64}}
        service = {'apiVersion': 'v1', 'kind': 'Service',
            'metadata': {'name': 'voice-gameintegration', 'namespace': 'voice-dev'}, 'spec': {}}
        deployment = {'apiVersion': 'apps/v1', 'kind': 'Deployment',
            'metadata': {'name': 'voice-gameintegration', 'namespace': 'voice-dev'},
            'spec': {'replicas': 1, 'template': {'spec': {'containers': [
                {'name': 'gameintegration', 'image': image}]}}}}
        source_plan = {'schema': shared_gis.SOURCE_SCHEMA, 'objects': [service, deployment]}
        shared_prerequisite_execute = gis_prerequisite.execute

        class Kube:
            def get_namespace(self):
                events.append('namespace')
                return {'metadata': {'name': 'voice-dev', 'uid': namespace_uid}}

        def save(value):
            events.append('journal:' + str(value.get('phase')))

        def prerequisite(kube, plan, root, capture, save_plan):
            self.assertEqual(plan['source_build_receipt'], build)
            self.assertTrue(shared_gis.validate_dev_source_build_receipt(
                plan['source_build_receipt'], source, image))
            plan['secret_metadata'] = {'uid': '00000000-0000-0000-0000-000000000005',
                'resourceVersion': '5'}
            plan['config_metadata'] = {'uid': '00000000-0000-0000-0000-000000000006',
                'resourceVersion': '6'}
            config_docs = {role: object() for role in gis_prerequisite.JOB_ROLES}
            job_docs = {role: object() for role in gis_prerequisite.JOB_ROLES}
            configmaps = {role: {'name': name,
                'uid': '00000000-0000-0000-0000-000000000010', 'spec_sha256': '3' * 64}
                for role, name in (('provision', inputs['configmaps']['provision']),
                    ('migrate', inputs['configmaps']['migrations']),
                    ('verify', inputs['configmaps']['verify']))}
            jobs = {}
            for role, name in inputs['jobs'].items():
                jobs[role] = {'name': name,
                    'uid': '00000000-0000-0000-0000-000000000011', 'spec_sha256': '4' * 64,
                    'pod_uid': '00000000-0000-0000-0000-000000000012',
                    'image_ids': {container: inputs['images'][logical]
                        for container, logical in inputs['job_images'][role].items()},
                    'complete': True, 'succeeded': 1, 'completion_time': '2026-10-17T00:00:00Z'}
            created_records = iter([
                (kind, row) for role in gis_prerequisite.JOB_ROLES
                for kind, row in (('configmap', configmaps[role]), ('job', jobs[role]))])

            def create_once(_kube, kind, _obj, active_plan, _saved, _save):
                expected_kind, row = next(created_records)
                self.assertEqual(kind, expected_kind)
                record = {'uid': row['uid'], 'spec_sha256': row['spec_sha256']}
                active_plan['created'][f"{kind}/{row['name']}"] = record
                return None, record

            with (
                patch.object(gis_prerequisite, '_documents', return_value=(config_docs, job_docs)),
                patch.object(gis_prerequisite, 'preflight', return_value=database_identity),
                patch.object(gis_prerequisite, '_create_once', side_effect=create_once),
                patch.object(gis_prerequisite, '_verify_job', side_effect=[
                    (jobs[role], gis_prerequisite.OUTPUT) for role in gis_prerequisite.JOB_ROLES]),
                patch.object(gis_prerequisite, 'source_inputs', return_value=inputs),
            ):
                receipt = shared_prerequisite_execute(kube, plan, root, capture, save_plan)
            self.assertTrue(shared_gis.validate_prerequisite_receipt(receipt, plan))
            events.append('prerequisite')
            plan['prerequisite_receipt'] = receipt
            save_plan(plan)
            return receipt

        def create_pair(kube, plan, save_plan, *, start):
            self.assertFalse(start)
            events.append('paused-pair')
            plan['phase'] = 'CREATED'
            save_plan(plan)
            return {}

        def start(kube, plan, save_plan):
            events.append('guarded-start')
            plan['phase'] = 'STARTED'
            plan['receipt'] = {'schema': 'voice-gis-first-install-resource-v1'}
            save_plan(plan)
            return plan['receipt']

        ready = {'schema': 'voice-gis-first-install-ready-v1',
            'deployment_uid': 'deployment-uid'}
        with tempfile.TemporaryDirectory() as directory:
            services = Path(directory) / 'deploy' / 'staging' / 'services.yaml'
            services.parent.mkdir(parents=True)
            services.write_text('apiVersion: v1\nkind: Service\n', encoding='utf-8')
            state_path = Path(directory) / 'state.json'
            with (
                patch.object(gis, '_source_pair', return_value=source_plan),
                patch.object(gis, '_capture_prerequisite_sources', return_value={'source': 'hash'}),
                patch.object(gis_prerequisite, 'source_inputs', return_value=inputs),
                patch.object(gis_prerequisite, '_endpoint_identity', return_value=database_identity),
                patch.object(gis, '_database_exists_readonly', side_effect=lambda *_args:
                    events.append('database-readonly')),
                patch.object(gis, 'apply_secret_plan', side_effect=lambda *_args:
                    events.append('secret-write')),
                patch.object(gis, 'secret_write_receipt', return_value={}),
                patch.object(gis_prerequisite, 'execute', side_effect=prerequisite),
                patch.object(shared_gis, 'execute', side_effect=create_pair),
                patch.object(shared_gis, 'start_owned', side_effect=start),
                patch.object(shared_gis, 'verify_ready', side_effect=lambda *_args:
                    events.append('readiness') or ready),
            ):
                result = gis.install(Path(directory), image, state_path, kube=Kube(),
                    apply=True, operation=operation,
                    build_receipt_factory=lambda *_args, **_kwargs: (source, build),
                    source_verifier=lambda *_args: None,
                    secret_plan_factory=lambda *_args, **_kwargs:
                        events.append('tls') or {'operations': [], 'namespace_uid': namespace_uid, 'operation': operation})

            self.assertEqual(result, ready)
            ordered = ['tls', 'database-readonly', 'secret-write', 'prerequisite',
                'paused-pair', 'guarded-start', 'readiness']
            positions = [events.index(item) for item in ordered]
            self.assertEqual(positions, sorted(positions))
            saved = json.loads(state_path.read_text(encoding='utf-8'))
            self.assertEqual(saved['phase'], 'READY')
            self.assertNotIn('runtime-password', state_path.read_text(encoding='utf-8'))

    def test_forged_build_receipt_blocks_before_namespace_or_secret_access(self):
        image = gis.IMAGE + '@sha256:' + 'b' * 64
        source = 'a' * 40
        tree = 'e' * 40
        forged = {'schema': gis.BUILD_RECEIPT_SCHEMA, 'source_sha': source,
            'image_ref': image, 'build_context_sha256': 'f' * 64,
            'image_config_sha256': 'd' * 64, 'working_tree_clean': True,
            'builder_kind': 'github-actions', 'builder_run': 11, 'builder_job': 12,
            'builder_sha': source, 'builder_tree': tree,
            'builder_artifact_id': 13, 'builder_artifact_sha256': 'f' * 64,
            'builder_output_index_sha256': 'b' * 64, 'builder_provenance_sha256': '1' * 64}
        calls = []

        class Kube:
            def get_namespace(self):
                calls.append('namespace')
                return {'metadata': {'name': 'voice-dev', 'uid': 'namespace-uid'}}

        with tempfile.TemporaryDirectory() as directory:
            with patch.object(gis, 'verify_current_source', return_value=None):
                with self.assertRaises(gis.Blocked):
                    gis.install(Path(directory), image, Path(directory) / 'state.json',
                        kube=Kube(), apply=True,
                        build_receipt_factory=lambda *_args, **_kwargs: (source, forged))
        self.assertEqual(calls, [])


if __name__ == '__main__':
    unittest.main()
