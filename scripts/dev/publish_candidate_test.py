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


C = '31a04a471c604488bedd5b442039dfdf0434f9c3'
PREVIOUS_C = '4e88e14e64300667e3349b08e75c4c31bf50b61e'
P = 'a' * 40
A = 'f1946355a084bed4c62a602ec1dc04e2d57e52ee'
FILE = '0d75a990dae5d9672f1b13e2ed346f8850e3c036'
DIGEST = 'sha256:' + 'b' * 64


class PublisherTests(unittest.TestCase):
    def test_story_file_pair_requires_selected_exact_product_and_outputs(self):
        candidate = 'e' * 40
        with mock.patch.dict(publisher.PRODUCT_COMPONENTS, {candidate: ('file', 'story')}):
            plan = publisher.build_plan(candidate, P, '12345', '2')
            self.assertEqual(set(plan['components']), {'file', 'story'})
            for name in ('file', 'story'):
                row = plan['components'][name]
                self.assertEqual(row['context'], 'src/backend')
                self.assertEqual(row['dockerfile'], 'src/backend/' + name + '/Dockerfile')
                self.assertEqual(row['tag'], publisher.REGISTRY + '/' + name + ':dev-' + candidate + '-12345-2')
                publisher.verify_output(name, {'service': b'\x7fELFbinary'}, candidate)
                with self.assertRaises(publisher.Refused):
                    publisher.verify_output(name, {'service': b'invalid'}, candidate)
            with self.assertRaises(publisher.Refused):
                publisher.build_plan('f' * 40, P, '12345', '2')

    def test_selected_messaging_dependency_uses_its_source_image(self):
        candidate = 'e' * 40
        with mock.patch.dict(publisher.PRODUCT_COMPONENTS, {candidate: ('file', 'story', 'messaging')}):
            plan = publisher.build_plan(candidate, P, '12345', '2')
            self.assertEqual(set(plan['components']), {'file', 'story', 'messaging'})
            row = plan['components']['messaging']
            self.assertEqual((row['context'], row['dockerfile']),
                             ('src/backend', 'src/backend/messaging/Dockerfile'))
            self.assertEqual(row['tag'], publisher.REGISTRY + '/messaging:dev-' + candidate + '-12345-2')
            publisher.verify_output('messaging', {'service': b'\x7fELFbinary'}, candidate)
            with self.assertRaises(publisher.Refused):
                publisher.verify_output('messaging', {'service': b'invalid'}, candidate)

    def test_media_cohort_publication_preserves_distinct_source_and_receipts(self):
        product = 'e' * 40
        env = {'PRODUCT_SHA': product, 'PUBLISHER_SHA': P, 'GITHUB_RUN_ID': '12345',
               'GITHUB_RUN_ATTEMPT': '2', 'GITHUB_REPOSITORY': publisher.REPOSITORY,
               'GITHUB_REF': 'refs/heads/master', 'GITHUB_ACTOR': 'fixture', 'GH_TOKEN': 'fixture'}
        run = {'id': 12345, 'run_attempt': 2, 'head_sha': P, 'event': 'workflow_dispatch',
               'path': publisher.WORKFLOW, 'repository': {'full_name': publisher.REPOSITORY}}
        jobs = {'jobs': [{'id': 67890, 'run_id': 12345, 'head_sha': P, 'name': publisher.JOB_NAME}]}
        manifest = json.dumps({'config': {'digest': DIGEST}}).encode()
        platform = 'sha256:' + hashlib.sha256(manifest).hexdigest()
        index = json.dumps({'manifests': [{'digest': platform,
            'platform': {'os': 'linux', 'architecture': 'amd64'}}]}).encode()
        digest = 'sha256:' + hashlib.sha256(index).hexdigest()
        registry = mock.Mock()
        registry.get.side_effect = lambda repository, path, absent=False: (
            None if absent else manifest if path == 'manifests/' + platform else index)
        builds = []
        def command(args, **kwargs):
            if args[0] == 'gh':
                return json.dumps(jobs if '/jobs?' in args[-1] else run).encode()
            if args[:3] == ['docker', 'buildx', 'build']:
                self.assertIn('org.opencontainers.image.revision=' + product, args)
                Path(args[args.index('--metadata-file') + 1]).write_text(json.dumps({'containerimage.digest': digest}))
                builds.append(args)
            return b''
        with tempfile.TemporaryDirectory() as folder, \
                mock.patch.dict(publisher.PRODUCT_COMPONENTS, {product: ('file', 'story', 'messaging')}), \
                mock.patch.object(publisher, 'source_snapshot', side_effect=lambda root, sha: {
                    'source_sha': sha, 'files': {
                        'src/backend/migrations/file_db/000001_initial.up.sql': 'a' * 64,
                        'src/backend/migrations/file_db/000001_initial.down.sql': 'b' * 64,
                        'src/backend/migrations/story_db/000001_initial.up.sql': 'c' * 64,
                        'src/backend/migrations/story_db/000001_initial.down.sql': 'd' * 64,
                        'src/backend/migrations/messaging_db/000001_initial.up.sql': 'a' * 64,
                        'src/backend/migrations/messaging_db/000001_initial.down.sql': 'b' * 64}}), \
                mock.patch.object(publisher, 'Registry', return_value=registry), \
                mock.patch.object(publisher, 'command', side_effect=command), \
                mock.patch.object(publisher, 'pulled_output', return_value={'verified': True}):
            output = Path(folder) / 'evidence'
            publisher.publish(Path(folder), Path(folder), output, env)
            receipt = json.loads((output / 'published-images.json').read_bytes())
            self.assertEqual(receipt['product_sha'], product)
            self.assertEqual(receipt['publisher_sha'], P)
            self.assertEqual({row['component'] for row in receipt['images']}, {'file', 'story', 'messaging'})
            self.assertTrue(all(row['product_sha'] == product and row['publisher_sha'] == P for row in receipt['images']))
            self.assertFalse(receipt['runtime_qualified'])
            self.assertFalse(receipt['deployment_performed'])
            self.assertEqual(len(builds), 3)
            migrations = json.loads((output / 'story-file-migrations.json').read_bytes())
            self.assertEqual(migrations['source_sha'], product)
            self.assertEqual(len(migrations['files']), 6)

    def test_story_file_migration_inventory_rejects_missing_or_malformed(self):
        with self.assertRaises(publisher.Refused):
            publisher.story_file_migrations({'source_sha': 'e' * 40, 'files': {}})
        with self.assertRaises(publisher.Refused):
            publisher.story_file_migrations({'source_sha': 'e' * 40, 'files': {
                'src/backend/migrations/file_db/000001_initial.up.sql': 'bad',
                'src/backend/migrations/story_db/000001_initial.up.sql': 'a' * 64}})
        with self.assertRaisesRegex(publisher.Refused, '^migration_direction_missing$'):
            publisher.story_file_migrations({'source_sha': 'e' * 40, 'files': {
                'src/backend/migrations/file_db/000001_initial.up.sql': 'a' * 64,
                'src/backend/migrations/story_db/000001_initial.up.sql': 'b' * 64,
                'src/backend/migrations/story_db/000001_initial.down.sql': 'c' * 64}})

    def test_messaging_dependency_includes_its_migration_inventory(self):
        files = {f'src/backend/migrations/{database}/000001_init.{direction}.sql': 'a' * 64
                 for database in ('file_db', 'story_db', 'messaging_db') for direction in ('up', 'down')}
        inventory = publisher.story_file_migrations({'source_sha': 'e' * 40, 'files': files},
                                                    include_messaging=True)
        self.assertEqual(inventory['files'], files)
        self.assertFalse(inventory['live_schema_qualified'])
        del files['src/backend/migrations/messaging_db/000001_init.up.sql']
        with self.assertRaises(publisher.Refused):
            publisher.story_file_migrations({'source_sha': 'e' * 40, 'files': files},
                                            include_messaging=True)

    def test_frozen_web_route_is_separate_and_source_bound(self):
        source = 'df00a393e03feb176439a50bfe5605f32c16b322'
        plan = publisher.build_plan(source, P, '12345', '2', route='web')
        self.assertEqual(set(plan['components']), {'web'})
        row = plan['components']['web']
        self.assertEqual((row['context'], row['dockerfile']), ('src/frontend', 'src/frontend/Dockerfile'))
        self.assertEqual(row['build_args'], {'VOICE_RUNTIME_CONFIG_REQUIRED': 'true', 'SOURCE_REVISION': source})
        self.assertEqual(row['tag'], publisher.REGISTRY + '/web:source-' + source + '-12345-2')
        with self.assertRaises(publisher.Refused):
            publisher.build_plan(source, P, '12345', '2')
        with self.assertRaises(publisher.Refused):
            publisher.build_plan(C, P, '12345', '2', route='web')

    def test_frozen_web_output_requires_runtime_loader_and_no_environment_urls(self):
        source = 'df00a393e03feb176439a50bfe5605f32c16b322'
        def fixture(main=b'voice-runtime-config.json', *, bootstrap_binds_main=True):
            main_sha = hashlib.sha256(main).hexdigest()
            main_name = 'main.' + main_sha + '.dart.js'
            boot = main_name.encode() if bootstrap_binds_main else b'unrelated bootstrap'
            boot_sha = hashlib.sha256(boot).hexdigest()
            boot_name = 'flutter_bootstrap.' + boot_sha + '.js'
            manifest = {'schema': 'voice-web-assets-v1',
                        'main': {'path': main_name, 'sha256': main_sha, 'bytes': len(main)},
                        'bootstrap': {'path': boot_name, 'sha256': boot_sha, 'bytes': len(boot)}}
            return {'main.dart.js': main, main_name: main, boot_name: boot,
                    'index.html': boot_name.encode(), 'web-asset-manifest.json': json.dumps(manifest).encode()}

        publisher.verify_output('web', fixture(), source)
        bad_scripts = (b'other', *(b'voice-runtime-config.json ' + url for url in (
            b'https://api-dev.comrade.click', b'https://api-staging.comrade.click',
            b'wss://livekit-dev.comrade.click', b'wss://livekit-staging.comrade.click')))
        for script in bad_scripts:
            with self.subTest(script=script), self.assertRaisesRegex(publisher.Refused, '^web_runtime_config_output$'):
                publisher.verify_output('web', fixture(script), source)

        files = fixture()
        manifest = json.loads(files['web-asset-manifest.json'])
        files[manifest['main']['path']] += b'corrupt'
        with self.assertRaisesRegex(publisher.Refused, '^web_asset_identity$'):
            publisher.verify_output('web', files, source)
        files = fixture()
        files['index.html'] = b'unrelated index'
        with self.assertRaisesRegex(publisher.Refused, '^web_index_binding$'):
            publisher.verify_output('web', files, source)
        with self.assertRaisesRegex(publisher.Refused, '^web_bootstrap_binding$'):
            publisher.verify_output('web', fixture(bootstrap_binds_main=False), source)

    def test_frozen_file_candidate_is_single_source_bound_image(self):
        plan = publisher.build_plan(FILE, P, '12345', '2')
        self.assertEqual(set(plan['components']), {'file'})
        row = plan['components']['file']
        self.assertEqual(row['context'], 'src/backend')
        self.assertEqual(row['dockerfile'], 'src/backend/file/Dockerfile')
        self.assertEqual(row['platform'], 'linux/amd64')
        self.assertEqual(row['build_args'], {})
        self.assertEqual(row['tag'], publisher.REGISTRY + '/file:dev-' + FILE + '-12345-2')
        self.assertEqual(plan['publisher_sha'], P)
        self.assertEqual(publisher.verify_output('file', {'service': b'\x7fELFbinary'}, FILE)['files']['service'],
                         {'sha256': hashlib.sha256(b'\x7fELFbinary').hexdigest(), 'bytes': 10})
        for bad in ({'service': b'wrong'}, {'other': b'\x7fELFbinary'}):
            with self.assertRaises(publisher.Refused):
                publisher.verify_output('file', bad, FILE)
        with self.assertRaises(publisher.Refused):
            publisher.verify_output('file', {'service': b'\x7fELFbinary'}, A)

    def test_frozen_file_builder_enforces_cgroup_caps_and_cleans_up(self):
        calls = []
        def command(args, **kwargs):
            calls.append(args)
            return b''
        with mock.patch.object(publisher, 'command', side_effect=command):
            with publisher.file_builder(12345, 2) as builder:
                self.assertEqual(builder, 'voice-file-12345-2')
        self.assertEqual(calls, [
            ['docker', 'buildx', 'create', '--driver', 'docker-container', '--name', builder],
            ['docker', 'buildx', 'inspect', '--builder', builder, '--bootstrap'],
            ['docker', 'update', '--cpus', '2', '--memory', '4g', '--memory-swap', '4g',
             'buildx_buildkit_' + builder + '0'],
            ['docker', 'buildx', 'rm', builder],
        ])
        calls.clear()
        with mock.patch.object(publisher, 'command', side_effect=command):
            with self.assertRaisesRegex(RuntimeError, 'fixture'):
                with publisher.file_builder(12345, 2):
                    raise RuntimeError('fixture')
        self.assertEqual(calls[-1], ['docker', 'buildx', 'rm', builder])

    def test_frozen_file_publication_retains_product_publisher_and_registry_receipt(self):
        env = {'PRODUCT_SHA': FILE, 'PUBLISHER_SHA': P, 'GITHUB_RUN_ID': '12345',
               'GITHUB_RUN_ATTEMPT': '2', 'GITHUB_REPOSITORY': publisher.REPOSITORY,
               'GITHUB_REF': 'refs/heads/master', 'GITHUB_ACTOR': 'fixture', 'GH_TOKEN': 'fixture'}
        run = {'id': 12345, 'run_attempt': 2, 'head_sha': P, 'event': 'workflow_dispatch',
               'path': publisher.WORKFLOW, 'repository': {'full_name': publisher.REPOSITORY}}
        jobs = {'jobs': [{'id': 67890, 'run_id': 12345, 'head_sha': P, 'name': publisher.JOB_NAME}]}
        manifest = json.dumps({'config': {'digest': DIGEST}}).encode()
        platform = 'sha256:' + hashlib.sha256(manifest).hexdigest()
        index = json.dumps({'manifests': [{'digest': platform,
            'platform': {'os': 'linux', 'architecture': 'amd64'}}]}).encode()
        digest = 'sha256:' + hashlib.sha256(index).hexdigest()
        registry = mock.Mock()
        registry.get.side_effect = lambda repository, path, absent=False: (
            None if absent else manifest if path == 'manifests/' + platform else index)
        calls = []
        def command(args, **kwargs):
            calls.append(args)
            if args[0] == 'gh':
                return json.dumps(jobs if '/jobs?' in args[-1] else run).encode()
            if args[:3] == ['docker', 'buildx', 'build']:
                self.assertEqual(args[3:5], ['--builder', 'voice-file-12345-2'])
                self.assertIn('org.opencontainers.image.revision=' + FILE, args)
                self.assertNotIn('--build-arg', args)
                Path(args[args.index('--metadata-file') + 1]).write_text(
                    json.dumps({'containerimage.digest': digest}))
            return b''
        with tempfile.TemporaryDirectory() as folder, \
                mock.patch.object(publisher, 'source_snapshot', side_effect=lambda root, sha: {'source_sha': sha}) as snapshot, \
                mock.patch.object(publisher, 'Registry', return_value=registry), \
                mock.patch.object(publisher, 'command', side_effect=command), \
                mock.patch.object(publisher, 'pulled_output', return_value={'verified': True}):
            output = Path(folder) / 'evidence'
            publisher.publish(Path(folder), Path(folder), output, env)
            receipt = json.loads((output / 'published-images.json').read_bytes())
            self.assertEqual(receipt['product_sha'], FILE)
            self.assertEqual(receipt['publisher_sha'], P)
            self.assertEqual(len(receipt['images']), 1)
            image = receipt['images'][0]
            self.assertEqual(image['component'], 'file')
            self.assertEqual(image['config_digest'], DIGEST)
            self.assertEqual(image['platform_manifest_digest'], platform)
            self.assertEqual(image['reference'], publisher.REGISTRY + '/file@' + platform)
            self.assertEqual([call.args[1] for call in snapshot.call_args_list], [P, FILE, FILE, P])
            self.assertIn(['docker', 'buildx', 'rm', 'voice-file-12345-2'], calls)

    def test_auth_user_adoption_candidate_is_exact_and_component_scoped(self):
        plan = publisher.build_plan(A, P, '12345', '2')
        self.assertEqual(set(plan['components']), {'dev-auth', 'dev-user'})
        for row in plan['components'].values():
            self.assertEqual(row['build_args'], {'VOICE_SOURCE_SHA': A})
            self.assertIn(':dev-' + A + '-12345-2', row['tag'])
        publisher.verify_config_source({'config': {'Labels': {'org.opencontainers.image.revision': A}}}, A)
        with self.assertRaises(publisher.Refused):
            publisher.verify_config_source({'config': {'Labels': {'org.opencontainers.image.revision': C}}}, A)
        stream = io.BytesIO()
        with zipfile.ZipFile(stream, 'w') as archive:
            archive.writestr('BOOT-INF/classes/voice-build-source.sha', A)
        publisher.verify_output('dev-auth', {'auth.jar': stream.getvalue()}, A)
        publisher.verify_output('dev-user', {'service': b'\x7fELF:' + A.encode()}, A)
        with self.assertRaises(publisher.Refused):
            publisher.verify_output('dev-user', {'service': b'\x7fELF:' + C.encode()}, A)
        with self.assertRaises(publisher.Refused):
            publisher.verify_output('dev-auth', {'auth.jar': self._jar(C)}, A)
        for component in ('dev-web', 'dev-notification', 'dev-realtime', 'dev-space-native-migrate'):
            with self.subTest(component=component), self.assertRaises(publisher.Refused):
                publisher.verify_output(component, {'service': b'\x7fELF:' + A.encode(),
                    'main.dart.js': b'https://api-dev.comrade.click wss://livekit-dev.comrade.click'}, A)

    @staticmethod
    def _jar(source):
        stream = io.BytesIO()
        with zipfile.ZipFile(stream, 'w') as archive:
            archive.writestr('BOOT-INF/classes/voice-build-source.sha', source)
        return stream.getvalue()

    def test_adoption_publication_threads_source_through_build_pull_and_receipt(self):
        env = {'PRODUCT_SHA': A, 'PUBLISHER_SHA': P, 'GITHUB_RUN_ID': '12345',
               'GITHUB_RUN_ATTEMPT': '2', 'GITHUB_REPOSITORY': publisher.REPOSITORY,
               'GITHUB_REF': 'refs/heads/master', 'GITHUB_ACTOR': 'fixture', 'GH_TOKEN': 'fixture'}
        run = {'id': 12345, 'run_attempt': 2, 'head_sha': P, 'event': 'workflow_dispatch',
               'path': publisher.WORKFLOW, 'repository': {'full_name': publisher.REPOSITORY}}
        jobs = {'jobs': [{'id': 67890, 'run_id': 12345, 'head_sha': P, 'name': publisher.JOB_NAME}]}
        raw_manifest = json.dumps({'config': {'digest': DIGEST}}).encode()
        manifest_digest = 'sha256:' + hashlib.sha256(raw_manifest).hexdigest()
        raw_index = json.dumps({'manifests': [{'digest': manifest_digest,
            'platform': {'os': 'linux', 'architecture': 'amd64'}}]}).encode()
        index_digest = 'sha256:' + hashlib.sha256(raw_index).hexdigest()
        registry = mock.Mock()
        registry.get.side_effect = lambda repository, path, absent=False: (
            None if absent else raw_manifest if path == 'manifests/' + manifest_digest else raw_index)
        builds = []
        def command(args, **kwargs):
            if args[0] == 'gh':
                return json.dumps(jobs if '/jobs?' in args[-1] else run).encode()
            self.assertEqual(args[:3], ['docker', 'buildx', 'build'])
            self.assertIn('org.opencontainers.image.revision=' + A, args)
            self.assertIn('VOICE_SOURCE_SHA=' + A, args)
            Path(args[args.index('--metadata-file') + 1]).write_text(
                json.dumps({'containerimage.digest': index_digest}))
            builds.append(args)
            return b''
        with tempfile.TemporaryDirectory() as folder, \
                mock.patch.object(publisher, 'source_snapshot', side_effect=lambda root, sha: {'source_sha': sha}) as snapshot, \
                mock.patch.object(publisher, 'Registry', return_value=registry), \
                mock.patch.object(publisher, 'command', side_effect=command), \
                mock.patch.object(publisher, 'pulled_output', return_value={'verified': True}) as pulled:
            output = Path(folder) / 'evidence'
            publisher.publish(Path(folder), Path(folder), output, env)
            receipt = json.loads((output / 'published-images.json').read_bytes())
            self.assertTrue(receipt['publication_complete'])
            self.assertEqual(receipt['product_sha'], A)
            self.assertFalse(receipt['runtime_qualified'])
            self.assertFalse(receipt['deployment_performed'])
            self.assertEqual({row['component'] for row in receipt['images']}, {'dev-auth', 'dev-user'})
            self.assertTrue(all(row['product_sha'] == A for row in receipt['images']))
            self.assertEqual(len(builds), 2)
            self.assertEqual([call.kwargs['product'] for call in pulled.call_args_list], [A, A])
            self.assertEqual([call.args[1] for call in snapshot.call_args_list], [P, A, A, P])
            for component in ('dev-auth', 'dev-user'):
                self.assertEqual(json.loads((output / (component + '-invocation.json')).read_bytes())['product_sha'], A)

    def test_adoption_pulled_bytes_use_selected_source_and_cleanup_on_rejection(self):
        for component in ('dev-auth', 'dev-user'):
            for embedded in (A, C):
                raw = json.dumps({'config': {'Labels': {'org.opencontainers.image.revision': A}}}).encode()
                config_digest = 'sha256:' + hashlib.sha256(raw).hexdigest()
                registry = mock.Mock()
                registry.get.return_value = raw
                calls = []
                def command(args, **kwargs):
                    calls.append(args)
                    if args[:3] == ['docker', 'image', 'inspect']:
                        return config_digest.encode()
                    if args[:2] == ['docker', 'create']:
                        return b'c' * 64
                    if args[:2] == ['docker', 'cp']:
                        destination = Path(args[-1])
                        if destination.name == 'auth.jar':
                            destination.write_bytes(self._jar(embedded))
                        elif destination.name == 'service':
                            destination.write_bytes(b'\x7fELF:' + embedded.encode())
                        else:
                            destination.write_bytes(b'public-trust-fixture')
                    return b''
                with self.subTest(component=component, embedded=embedded), tempfile.TemporaryDirectory() as folder, \
                        mock.patch.object(publisher, 'command', side_effect=command):
                    args = (component, 'ghcr.io/fixture@' + DIGEST, config_digest, Path(folder),
                            registry, 'fixture', Path(folder))
                    if embedded == A:
                        self.assertTrue(publisher.pulled_output(*args, product=A)['verified'])
                    else:
                        with self.assertRaises(publisher.Refused):
                            publisher.pulled_output(*args, product=A)
                    self.assertEqual(calls[-1], ['docker', 'rm', 'c' * 64])

    def test_selected_current_product_rejects_previous_candidate(self):
        self.assertEqual(publisher.build_plan(C, P, '12345', '2')['product_sha'], C)
        with self.assertRaisesRegex(publisher.Refused, 'unselected_product'):
            publisher.build_plan(PREVIOUS_C, P, '12345', '2')
        with self.assertRaises(publisher.Refused):
            publisher.verify_config_source(
                {'config': {'Labels': {'org.opencontainers.image.revision': PREVIOUS_C}}}, C)

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

    def test_registry_refreshes_once_on_actual401_without_waiving_denial(self):
        class Response(io.BytesIO):
            def __enter__(self):
                return self

            def __exit__(self, *unused):
                self.close()

        def denied(code):
            return urllib.error.HTTPError('https://ghcr.io/v2/fixture/manifests/tag', code, 'fixture', {}, None)

        for final, expected_calls in ((Response(b'actual-manifest'), 4), (denied(401), 4), (denied(403), 2)):
            registry = publisher.Registry('fixture', 'not-a-real-token')
            responses = ([Response(b'{"token":"first"}'), denied(401), Response(b'{"token":"second"}'), final]
                         if expected_calls == 4 else [Response(b'{"token":"first"}'), final])
            with mock.patch.object(registry.opener, 'open', side_effect=responses) as request:
                if isinstance(final, Exception):
                    with self.assertRaises(publisher.Refused):
                        registry.get('fixture', 'manifests/tag')
                else:
                    self.assertEqual(registry.get('fixture', 'manifests/tag'), b'actual-manifest')
                    self.assertEqual(request.call_args_list[1].args[0].get_header('Authorization'), 'Bearer first')
                    self.assertEqual(request.call_args_list[3].args[0].get_header('Authorization'), 'Bearer second')
                self.assertEqual(request.call_count, expected_calls)

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
    def test_web_route_is_isolated_and_authenticates_its_own_job(self):
        path = Path(__file__).resolve().parents[2] / publisher.WEB_WORKFLOW
        workflow = yaml.load(path.read_text(encoding='utf-8'), Loader=yaml.BaseLoader)
        self.assertEqual(set(workflow['on']), {'pull_request', 'workflow_dispatch'})
        self.assertEqual(workflow['concurrency'], {'group': 'source-web-image-publication', 'cancel-in-progress': 'false'})
        job = workflow['jobs']['publish']
        self.assertEqual(job['name'], publisher.WEB_JOB_NAME)
        self.assertIn("github.event_name == 'workflow_dispatch'", job['if'])
        self.assertIn("github.ref == 'refs/heads/master'", job['if'])
        self.assertEqual(job['permissions'], {'actions': 'read', 'contents': 'read', 'packages': 'write'})
        self.assertIn('--route web', str(job))
        run = {'id': 12345, 'run_attempt': 2, 'head_sha': P, 'event': 'workflow_dispatch',
               'path': publisher.WEB_WORKFLOW, 'repository': {'full_name': publisher.REPOSITORY}}
        jobs = {'jobs': [{'id': 67890, 'run_id': 12345, 'head_sha': P, 'name': publisher.WEB_JOB_NAME}]}
        self.assertEqual(publisher.validate_run(run, jobs, P, '12345', '2', route='web')['workflow'], publisher.WEB_WORKFLOW)
        with self.assertRaises(publisher.Refused):
            publisher.validate_run(run, jobs, P, '12345', '2')

    def test_publication_is_manual_only_with_no_deployment_consumer(self):
        path = Path(__file__).resolve().parents[2] / publisher.WORKFLOW
        workflow = yaml.load(path.read_text(encoding='utf-8'), Loader=yaml.BaseLoader)
        self.assertEqual(workflow['on']['workflow_dispatch']['inputs']['product_sha']['default'], C)
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
