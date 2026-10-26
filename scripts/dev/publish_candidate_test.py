"""Offline publisher boundaries; no registry, Docker build or environment calls."""
import copy
import hashlib
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock
import urllib.error
import urllib.request
import zipfile

import yaml
import publish_candidate as publisher


C = '4e88e14e64300667e3349b08e75c4c31bf50b61e'
P = 'a' * 40
DIGEST = 'sha256:' + 'b' * 64


class PublisherTests(unittest.TestCase):
    def test_plan_has_only_eight_required_variants_and_distinct_identities(self):
        plan = publisher.build_plan(C, P, '12345', '2')
        self.assertEqual(plan['product_sha'], C)
        self.assertEqual(plan['publisher_sha'], P)
        self.assertEqual(set(plan['components']), {
            'dev-web', 'dev-admin', 'dev-developer-portal', 'dev-auth',
            'dev-user', 'dev-notification', 'dev-realtime', 'dev-space-native-migrate'})
        self.assertEqual(len({r['tag'] for r in plan['components'].values()}), 8)
        for name, row in plan['components'].items():
            self.assertIn(C + '-12345-2', row['tag'])
            self.assertNotIn(':latest', row['tag'])
            self.assertEqual(row['platform'], 'linux/amd64')
        for name in ('auth', 'user', 'notification', 'realtime'):
            self.assertEqual(plan['components']['dev-' + name]['build_args'], {'VOICE_SOURCE_SHA': C})
        self.assertEqual(plan['components']['dev-space-native-migrate']['build_args'], {'SOURCE_REVISION': C})
        self.assertEqual(plan['components']['dev-web']['build_args'], {
            'VOICE_API_BASE_URL': 'https://api-dev.comrade.click',
            'VOICE_LIVEKIT_URL': 'wss://livekit-dev.comrade.click'})

    def test_wrong_product_source_and_unvalidated_execution_identity_reject(self):
        for args in ((P, P, '1', '1'), (C + '~1', P, '1', '1'),
                     (C, 'master', '1', '1'), (C, P, '../1', '1'),
                     (C, P, '1', '0'), (C, P, '1', '1;echo bad')):
            with self.subTest(args=args), self.assertRaises(publisher.Refused):
                publisher.build_plan(*args)

    def test_authenticated_run_and_attempt_are_not_inferred_from_product(self):
        run = {'id': 12345, 'run_attempt': 2, 'head_sha': P,
               'event': 'workflow_dispatch', 'path': publisher.WORKFLOW,
               'repository': {'full_name': publisher.REPOSITORY}}
        jobs = {'jobs': [{'id': 67890, 'run_id': 12345, 'head_sha': P,
                          'name': publisher.JOB_NAME}]}
        proof = publisher.validate_run(run, jobs, P, '12345', '2')
        self.assertEqual(proof['job_id'], 67890)
        for key, wrong in (('head_sha', C), ('run_attempt', 1), ('id', 987),
                           ('event', 'pull_request'), ('path', '.github/workflows/ci.yml')):
            changed = copy.deepcopy(run)
            changed[key] = wrong
            with self.subTest(key=key), self.assertRaises(publisher.Refused):
                publisher.validate_run(changed, jobs, P, '12345', '2')
        jobs['jobs'].append(copy.deepcopy(jobs['jobs'][0]))
        with self.assertRaises(publisher.Refused):
            publisher.validate_run(run, jobs, P, '12345', '2')

    def test_tag_conflict_stops_actual_orchestrator_before_any_build(self):
        env = {'PRODUCT_SHA': C, 'PUBLISHER_SHA': P, 'GITHUB_RUN_ID': '12345',
               'GITHUB_RUN_ATTEMPT': '2', 'GITHUB_REPOSITORY': publisher.REPOSITORY,
               'GITHUB_REF': 'refs/heads/master', 'GITHUB_ACTOR': 'fixture', 'GH_TOKEN': 'not-a-real-token'}
        run = {'id': 12345, 'run_attempt': 2, 'head_sha': P, 'event': 'workflow_dispatch',
               'path': publisher.WORKFLOW, 'repository': {'full_name': publisher.REPOSITORY}}
        jobs = {'jobs': [{'id': 67890, 'run_id': 12345, 'head_sha': P, 'name': publisher.JOB_NAME}]}
        registry = mock.Mock()
        registry.get.return_value = b'existing-manifest'
        with tempfile.TemporaryDirectory() as folder, \
                mock.patch.object(publisher, 'source_snapshot', return_value={}), \
                mock.patch.object(publisher, 'Registry', return_value=registry), \
                mock.patch.object(publisher, 'command', side_effect=[json.dumps(run).encode(), json.dumps(jobs).encode()]) as command:
            with self.assertRaisesRegex(publisher.Refused, 'publication_tag_conflict'):
                publisher.publish(Path(folder), Path(folder), Path(folder) / 'fresh-evidence', env)
            self.assertEqual(len(command.call_args_list), 2)
            self.assertTrue(all(call.args[0][0] == 'gh' for call in command.call_args_list))

    def test_registry_only_authenticated_HTTP404_is_absence(self):
        registry = publisher.Registry('fixture', 'not-a-real-token')
        request = urllib.request.Request('https://ghcr.io/v2/fixture/manifests/tag')
        for code in (401, 403, 429, 500):
            with self.subTest(code=code), mock.patch.object(registry.opener, 'open', side_effect=urllib.error.HTTPError(request.full_url, code, 'fixture', {}, None)):
                with self.assertRaisesRegex(publisher.Refused, 'registry_HTTP_' + str(code)):
                    registry._read(request, absent=True)
        with mock.patch.object(registry.opener, 'open', side_effect=urllib.error.HTTPError(request.full_url, 404, 'fixture', {}, None)):
            self.assertIsNone(registry._read(request, absent=True))
            with self.assertRaises(publisher.Refused):
                registry._read(request)

    def test_registry_redirect_does_not_forward_cross_origin_bearer(self):
        request = urllib.request.Request('https://ghcr.io/blob', headers={'Authorization': 'Bearer fixture'})
        redirect = publisher.SafeRedirect().redirect_request(request, None, 302, 'fixture', {}, 'https://pkg-containers.githubusercontent.com/blob?opaque=fixture')
        self.assertFalse(redirect.has_header('Authorization'))
        with self.assertRaises(publisher.Refused):
            publisher.SafeRedirect().redirect_request(request, None, 302, 'fixture', {}, 'http://untrusted/blob')

    def test_real_git_checkout_rejects_dirty_and_wrong_source(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            subprocess.run(['git', 'init', '-q', str(root)], check=True)
            (root / 'input.txt').write_bytes(b'exact-input')
            subprocess.run(['git', '-C', str(root), 'add', 'input.txt'], check=True)
            subprocess.run(['git', '-C', str(root), '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'Synthetic publisher input'], check=True)
            source = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD']).decode().strip()
            snapshot = publisher.source_snapshot(root, source)
            self.assertEqual(snapshot['files'], {'input.txt': hashlib.sha256(b'exact-input').hexdigest()})
            with self.assertRaisesRegex(publisher.Refused, 'checkout_identity'):
                publisher.source_snapshot(root, P)
            (root / 'input.txt').write_bytes(b'changed')
            with self.assertRaisesRegex(publisher.Refused, 'checkout_dirty'):
                publisher.source_snapshot(root, source)

    def test_registry_manifest_bytes_and_exact_single_platform_are_verified(self):
        raw = json.dumps({'manifests': [
            {'digest': DIGEST, 'platform': {'os': 'linux', 'architecture': 'amd64'}},
            {'digest': 'sha256:' + 'c' * 64, 'platform': {'os': 'unknown', 'architecture': 'unknown'}}]}).encode()
        digest = 'sha256:' + hashlib.sha256(raw).hexdigest()
        value = publisher.decode_manifest(raw, digest)
        self.assertEqual(publisher.platform_manifest(value), DIGEST)
        with self.assertRaises(publisher.Refused):
            publisher.decode_manifest(raw + b' ', digest)
        for platform in ({'os': 'linux', 'architecture': 'arm64'}, {'os': 'unknown', 'architecture': 'unknown'}):
            changed = copy.deepcopy(value)
            changed['manifests'][0]['platform'] = platform
            with self.assertRaises(publisher.Refused):
                publisher.platform_manifest(changed)
        value['manifests'].append(value['manifests'][0])
        with self.assertRaises(publisher.Refused):
            publisher.platform_manifest(value)

    def test_tag_conflict_or_unknown_is_never_treated_as_absence(self):
        publisher.require_absent(None)
        for value in (b'{}', b'', {'status': 403}, {'unavailable': True}):
            with self.assertRaises(publisher.Refused):
                publisher.require_absent(value)

    def test_pulled_config_identity_must_match_registry_manifest(self):
        publisher.validate_config_identity(DIGEST, DIGEST)
        with self.assertRaises(publisher.Refused):
            publisher.validate_config_identity(DIGEST, 'sha256:' + 'd' * 64)

    def test_pulled_image_source_label_cannot_substitute_publisher_for_product(self):
        publisher.verify_config_source({'config': {'Labels': {'org.opencontainers.image.revision': C}}}, C)
        for label in (P, '', None):
            with self.subTest(label=label), self.assertRaises(publisher.Refused):
                publisher.verify_config_source({'config': {'Labels': {'org.opencontainers.image.revision': label}}}, C)

    def test_actual_jar_and_binary_must_bind_selected_product(self):
        def jar(source):
            stream = io.BytesIO()
            with zipfile.ZipFile(stream, 'w') as archive:
                archive.writestr('BOOT-INF/classes/voice-build-source.sha', source)
            return stream.getvalue()
        publisher.verify_output('dev-auth', {'auth.jar': jar(C)}, C)
        with self.assertRaises(publisher.Refused):
            publisher.verify_output('dev-auth', {'auth.jar': jar(P)}, C)
        for name in ('dev-user', 'dev-notification', 'dev-realtime'):
            publisher.verify_output(name, {'service': b'\x7fELFfixture:' + C.encode()}, C)
            with self.assertRaises(publisher.Refused):
                publisher.verify_output(name, {'service': b'\x7fELFfixture:unbound'}, C)

    def test_native_sql_and_dev_frontend_urls_are_checked_on_pulled_bytes(self):
        files = {'migrations/000001.up.sql': b'SELECT 1;', 'native-migrate': b'\x7fELF:' + C.encode()}
        publisher.verify_output('dev-space-native-migrate', files, C, {'000001.up.sql': b'SELECT 1;'})
        with self.assertRaises(publisher.Refused):
            publisher.verify_output('dev-space-native-migrate', files, C, {'000001.up.sql': b'SELECT 2;'})
        with self.assertRaises(publisher.Refused):
            publisher.verify_output('dev-space-native-migrate', {**files, 'native-migrate': b'\x7fELF:unbound'}, C, {'000001.up.sql': b'SELECT 1;'})
        publisher.verify_output('dev-web', {'main.dart.js': b'https://api-dev.comrade.click wss://livekit-dev.comrade.click'}, C)
        with self.assertRaises(publisher.Refused):
            publisher.verify_output('dev-web', {'main.dart.js': b'https://api-staging.comrade.click'}, C)
        for name in ('dev-admin', 'dev-developer-portal'):
            publisher.verify_output(name, {'assets/app.js': b'https://api-dev.comrade.click'}, C)


class WorkflowTests(unittest.TestCase):
    def test_publication_is_manual_only_with_no_deployment_consumer(self):
        path = Path(__file__).resolve().parents[2] / publisher.WORKFLOW
        workflow = yaml.load(path.read_text(encoding='utf-8'), Loader=yaml.BaseLoader)
        self.assertEqual(set(workflow['on']), {'pull_request', 'workflow_dispatch'})
        self.assertEqual(workflow['permissions'], {})
        self.assertEqual(set(workflow['jobs']), {'validate', 'publish'})
        job = workflow['jobs']['publish']
        self.assertIn("github.event_name == 'workflow_dispatch'", job['if'])
        self.assertIn("github.ref == 'refs/heads/master'", job['if'])
        self.assertEqual(job['permissions'], {'actions': 'read', 'contents': 'read', 'packages': 'write'})
        self.assertEqual(workflow['jobs']['validate']['permissions'], {'contents': 'read'})
        checkouts = [step for step in job['steps'] if step.get('uses', '').startswith('actions/checkout@')]
        self.assertEqual({step['with']['path'] for step in checkouts}, {'publisher', 'product'})
        self.assertTrue(all(step['with']['persist-credentials'] == 'false' for step in checkouts))
        run = next(step for step in job['steps'] if step.get('name') == 'Publish and verify immutable candidate')
        self.assertEqual(run['env']['PRODUCT_SHA'], '${{ inputs.product_sha }}')
        self.assertEqual(run['env']['PUBLISHER_SHA'], '${{ github.sha }}')
        self.assertIn('publish_candidate.py', run['run'])
        self.assertNotIn('kubectl', str(job))
        self.assertNotIn('ssh', str(job))


if __name__ == '__main__':
    unittest.main()
