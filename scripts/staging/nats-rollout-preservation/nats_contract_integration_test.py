import os
import sys
import unittest
import json
import hashlib
import tempfile
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

import nats_contract_integration as integration
import contract_fixture_entry as entry


class HostedTransportTests(unittest.TestCase):
    def test_existing_fixture_credentials_do_not_already_grant_music_consumer(self):
        contract=json.loads((Path(__file__).parents[1]/'nats-known-baseline/deployed-contract.json').read_text())
        grants=integration.fixture_bootstrap_grants(contract)
        self.assertNotIn('$JS.API.CONSUMER.CREATE.file_events.messaging_music_metadata',grants)
        self.assertNotIn('$JS.API.CONSUMER.INFO.file_events.messaging_music_metadata',grants)
        self.assertIn('$JS.API.CONSUMER.CREATE.chat_events.voice_space_media_chat',grants)
        self.assertIn('$JS.API.STREAM.CREATE.file_events',grants)

    def test_postmigration_plan_uses_authoritative_stream_configs_and_only_declared_consumer_info(self):
        import copy
        from nats_contract_plan import CHAT_SUBJECTS_17, ROLE_SUBJECTS_10, DECLARATIONS, ContractError, compile_plan

        source = Path(__file__).resolve().parents[3]
        social = json.loads((Path(__file__).parent / 'testdata/nats-9ae-predecessor.json').read_text(encoding='utf-8'))
        configs = {
            'chat_events': {'name':'chat_events','storage':'file','retention':'limits','max_age':604800000000000,
                'subjects':sorted(CHAT_SUBJECTS_17)},
            'social_events': {'name':'social_events','storage':'file','retention':'limits','max_age':604800000000000,
                'duplicate_window':86400000000000,'subjects':social['social_subjects']},
            'role_events': {'name':'role_events','storage':'file','retention':'limits','max_age':604800000000000,
                'subjects':sorted(ROLE_SUBJECTS_10)},
            'file_events': {'name':'file_events','storage':'file','retention':'limits','max_age':604800000000000,
                'subjects':['file.processed']},
            'rollout_extra': {'name':'rollout_extra','storage':'file','retention':'limits','subjects':['rollout.extra']},
        }
        details=[]
        normalized={}
        for stream, declaration in DECLARATIONS:
            durable=declaration['durable_name']
            config={**copy.deepcopy(declaration),'deliver_policy':'new'}
            normalized[(stream,durable)]=config
            details.append({'name':stream,'config':copy.deepcopy(configs[stream]),'state':{},
                'consumer_detail':[{'name':durable,'durable_name':durable,'config':{'monitor':'raw'}}]})
        details.append({'name':'rollout_extra','config':copy.deepcopy(configs['rollout_extra']),
            'state':{},'consumer_detail':[]})

        class DeclaredInfoOnly:
            def __init__(self,drift=False): self.calls=[];self.drift=drift
            def info_config(self, obj):
                self.calls.append(obj)
                matches=[(stream,config) for stream,config in DECLARATIONS if stream+'/'+config['durable_name']==obj]
                if len(matches)!=1: raise AssertionError('unexpected Actor.INFO request: '+obj)
                stream,config=matches[0]
                result=copy.deepcopy(normalized[(stream,config['durable_name'])])
                if self.drift:result['ack_wait']=999999999
                return result

        actor=DeclaredInfoOnly()
        streams,consumers=integration._postmigration_plan_inputs(actor,details)
        self.assertEqual(actor.calls,[stream+'/'+config['durable_name'] for stream,config in DECLARATIONS])
        self.assertEqual(streams['rollout_extra']['config'],configs['rollout_extra'])
        self.assertEqual(set(consumers),set(normalized))
        self.assertEqual({key:row['config'] for key,row in consumers.items()},normalized)
        plan=compile_plan(source,streams,consumers,normalized)
        self.assertEqual(plan['actions'],[])
        drift_actor=DeclaredInfoOnly(drift=True)
        drift_streams,drift_consumers=integration._postmigration_plan_inputs(drift_actor,details)
        with self.assertRaises(ContractError):compile_plan(source,drift_streams,drift_consumers,normalized)

    def test_private_archive_base_uses_required_root_group_and_mode(self):
        base = Path('/synthetic-fixture-base')
        with patch.object(integration.os, 'chown', create=True) as chown, patch.object(Path, 'chmod') as chmod:
            integration._set_archive_base_custody(base)
        chown.assert_called_once_with(base, 0, 65532)
        chmod.assert_called_once_with(0o750)

    def test_private_store_descriptor_requires_bound_path_and_fence(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / 'base'
            root.mkdir()
            store = root / 'store'
            store.mkdir()
            escaped = Path(temporary) / 'outside'
            escaped.mkdir()
            fenced = Mock()
            self.assertIsNone(integration._private_fixture_store_descriptor(root, store, fenced))
            fenced.assert_called_once_with()
            with self.assertRaisesRegex(RuntimeError, 'fixture_private_store_binding_invalid'):
                integration._private_fixture_store_descriptor(root, escaped, fenced)
            fenced.assert_called_once_with()

    def test_default_transport_remains_the_existing_pmd_host(self):
        self.assertEqual(integration.select_transport([], {}, 'linux'), 'pmdebook')

    def test_explicit_hosted_transport_requires_all_runner_facts(self):
        accepted = {'RUNNER_ENVIRONMENT': 'github-hosted', 'GITHUB_ACTIONS': 'true'}
        self.assertEqual(integration.select_transport(['--github-hosted'], accepted, 'linux'), 'github-hosted')
        for env, platform in (
            ({}, 'linux'),
            ({'RUNNER_ENVIRONMENT': 'self-hosted', 'GITHUB_ACTIONS': 'true'}, 'linux'),
            ({'RUNNER_ENVIRONMENT': 'github-hosted', 'GITHUB_ACTIONS': 'false'}, 'linux'),
            (accepted, 'win32'),
        ):
            with self.subTest(env=env, platform=platform):
                with self.assertRaises(SystemExit):
                    integration.select_transport(['--github-hosted'], env, platform)

    def test_unknown_arguments_are_rejected(self):
        for args in (['--hosted'], ['--github-hosted', '--other'], ['--pmdebook']):
            with self.subTest(args=args), self.assertRaises(SystemExit):
                integration.select_transport(args, {}, 'linux')

    def test_rejected_hosted_mode_fails_before_build_or_transport(self):
        with patch.dict(os.environ, {}, clear=True), patch.object(sys, 'platform', 'linux'):
            with patch.object(integration, 'build_fixture_records') as build:
                with patch.object(integration.fixture, 'remote_run') as remote:
                    with self.assertRaises(SystemExit):
                        integration.main(['--github-hosted'])
        build.assert_not_called()
        remote.assert_not_called()

    def test_hosted_dispatch_uses_owned_runner_not_ssh(self):
        with patch.object(integration.fixture, 'remote_run') as remote:
            with patch.object(integration.subprocess, 'run') as ssh:
                integration.run_fixture('github-hosted', Path('/synthetic-bundle'),
                    {'RUNNER_ENVIRONMENT': 'github-hosted', 'GITHUB_ACTIONS': 'true'})
        remote.assert_called_once_with(Path('/synthetic-bundle'), transport='github-hosted')
        ssh.assert_not_called()

    def test_predecessor_fixture_is_the_reviewed_9ae_stream_and_consumer_contract(self):
        from nats_contract_plan import CHAT_SUBJECTS_14, ROLE_SUBJECTS_9

        path = Path(__file__).parent / 'testdata/nats-9ae-predecessor.json'
        baseline = json.loads(path.read_text(encoding='utf-8'))
        self.assertEqual(baseline['schema'], 'voice-nats-reviewed-predecessor-v1')
        self.assertEqual(baseline['source_commit'], '9ae2e0c259e5e039109f7331aaf62b3607e549dd')
        self.assertEqual(set(baseline['chat_subjects']), CHAT_SUBJECTS_14)
        self.assertEqual(set(baseline['role_subjects']), ROLE_SUBJECTS_9)
        self.assertEqual(len(set(baseline['chat_subjects'])), 14)
        self.assertEqual(len(set(baseline['role_subjects'])), 9)
        self.assertEqual(len(baseline['chat_subjects']), 14)
        self.assertEqual(len(baseline['role_subjects']), 9)
        self.assertEqual(baseline['social_subjects'], [
            'social.friend_request', 'social.friend_accepted', 'social.friend_removed',
            'social.user_blocked', 'social.contacts_synced'])
        self.assertEqual(baseline['social_max_age_ns'], 604800000000000)
        self.assertEqual(baseline['social_duplicate_window_ns'], 86400000000000)
        self.assertEqual(baseline['friend_removed_consumer'], {
            'stream': 'social_events', 'name': 'rt_realtime1_friend_removed',
            'filter_subject': 'social.friend_removed',
            'deliver_subject': '_INBOX.voice.realtime1.friend_removed',
            'deliver_policy': 'new', 'ack_policy': 'explicit'})

    def test_bundle_contains_complete_captured_import_closure_and_manifest(self):
        with tempfile.TemporaryDirectory() as temporary:
            bundle = Path(temporary)

            def fake_build(args, **kwargs):
                output = Path(args[args.index('-o') + 1])
                output.write_bytes(('synthetic:' + output.name).encode())
                return SimpleNamespace(returncode=0, stdout=b'', stderr=b'')

            with patch.object(integration.fixture, 'call', side_effect=fake_build):
                integration.build_fixture_records(bundle)

            manifest_path = bundle / 'capture/capture-manifest.json'
            manifest = json.loads(manifest_path.read_text(encoding='utf-8'))
            source = Path(__file__).resolve().parents[3]
            captured = integration._captured_python_sources(source)
            self.assertEqual(set(manifest), set(captured) | {'nats-known-baseline/kernel'})
            self.assertLessEqual(len(manifest), 101)
            self.assertIn('nats-rollout-preservation/contract_fixture_entry.py', manifest)
            for relative, wanted in manifest.items():
                content = (bundle / 'capture' / relative).read_bytes()
                self.assertEqual(hashlib.sha256(content).hexdigest(), wanted)
                if relative in captured:
                    self.assertEqual(content, captured[relative].read_bytes())
            for relative in (
                'nats-rollout-preservation/closed_archive.py',
                'nats-rollout-preservation/closed_durable.py',
                'nats-rollout-preservation/root_cli.py',
                'nats-rollout-preservation/nats_contract_integration.py',
                'nats-known-baseline/kernel',
            ):
                content = (bundle / 'capture' / relative).read_bytes()
                self.assertEqual(hashlib.sha256(content).hexdigest(), manifest[relative])
            helper = (bundle / 'helper.go').read_text(encoding='utf-8')
            for source_bound_seed in (
                'SocialMaxAge int64',
                'SocialDuplicateWindow int64',
                'social.Config.Duplicates=time.Duration(baseline.SocialDuplicateWindow)',
                'js.AddConsumer(expected.Stream,&nats.ConsumerConfig',
                'js.ConsumerInfo(expected.Stream,expected.Name)',
            ):
                self.assertIn(source_bound_seed, helper)
            self.assertEqual(
                (bundle / 'contract_fixture_entry.py').read_bytes(),
                (bundle / 'capture/nats-rollout-preservation/contract_fixture_entry.py').read_bytes(),
            )
            transport = (bundle / 'rollout_census_integration.py').read_text(encoding='utf-8')
            self.assertIn("__import__('contract_fixture_entry').inner(sys.argv[2])", transport)
            self.assertNotIn('contract_fixture.py', manifest)
            entry = (bundle / 'contract_fixture_entry.py').read_text(encoding='utf-8')
            self.assertLess(entry.index('root_cli.code_binding(code)'),
                            entry.index('from nats_contract_integration import inner'))

    def test_hosted_runner_rejection_precedes_docker(self):
        with patch.object(integration.sys, 'platform', 'linux'):
            with patch.dict(os.environ, {}, clear=True):
                with patch.object(integration.fixture, 'call') as docker:
                    with self.assertRaisesRegex(RuntimeError, 'github_hosted_fixture_runner_required'):
                        integration.fixture.remote_run(Path('/synthetic'), transport='github-hosted')
        docker.assert_not_called()

    def test_hosted_prefetches_exact_pinned_images_before_fixture_allocation(self):
        images = integration.fixture._pinned_runtime_images()
        calls = []

        def docker(args):
            calls.append(args)
            if args[:3] == ['docker', 'image', 'inspect']:
                image = args[3]
                return json.dumps([{'RepoDigests': [image]}])
            return ''

        integration.fixture._prefetch_pinned_images(images, docker)
        expected = []
        for image in images:
            self.assertRegex(image, r'.+@sha256:[a-f0-9]{64}\Z')
            expected.extend((['docker', 'pull', image], ['docker', 'image', 'inspect', image]))
        self.assertEqual(calls, expected)
        self.assertEqual(len(images), 2)

    def test_hosted_prefetch_rejects_wrong_digest_before_any_fixture_creation(self):
        images = integration.fixture._pinned_runtime_images()
        calls = []

        def docker(args):
            calls.append(args)
            if args[:2] == ['docker', 'image']:
                return json.dumps([{'RepoDigests': ['example.invalid/wrong@sha256:' + '0' * 64]}])
            return ''

        with self.assertRaisesRegex(RuntimeError, 'fixture_pinned_image_digest_invalid'):
            integration.fixture._prefetch_pinned_images(images, docker)
        self.assertEqual(calls, [
            ['docker', 'pull', images[0]],
            ['docker', 'image', 'inspect', images[0]],
        ])

    def test_fixture_failure_reports_only_bounded_safe_exception_labels(self):
        stderr = (b'Traceback (most recent call last):\n'
                  b'  File "/bundle/contract_fixture.py", line 123, in inner\n'
                  b'RuntimeError: owned_census_fixture_command_failed: RuntimeError: baseline_record_changed\n'
                  b'RuntimeError: owned_census_fixture_command_failed: RuntimeError: identifier_shaped_secret123\n'
                  b'AssertionError: known_record_changed\n'
                  b'ValueError: nats_census_incomplete_or_invalid\n'
                  b'RuntimeError: identifier_shaped_secret123\n'
                  b'RuntimeError: raw credential material must not appear\n')
        result = SimpleNamespace(returncode=1, stdout=b'', stderr=stderr)
        with patch.object(integration.fixture.subprocess, 'run', return_value=result):
            with self.assertRaises(RuntimeError) as raised:
                integration.fixture.call(['docker', 'run', 'synthetic'])
        self.assertEqual(str(raised.exception),
            'owned_census_fixture_command_failed: frames=contract_fixture.py:123 '
            'errors=RuntimeError: owned_census_fixture_command_failed;RuntimeError: baseline_record_changed;'
            'RuntimeError;AssertionError: known_record_changed;ValueError: nats_census_incomplete_or_invalid')
        self.assertNotIn('raw credential material', str(raised.exception))
        self.assertNotIn('identifier_shaped_secret123', str(raised.exception))

    def test_failed_before_outer_creation_still_removes_exact_owned_volume(self):
        operation = 'voice-nats-rollout-fixture-' + '1' * 16
        label = 'voice.rollout.census=' + operation
        images = integration.fixture._pinned_runtime_images()
        volume = {'Name': operation, 'Labels': {'voice.rollout.census': operation},
                  'Mountpoint': '/var/lib/docker/volumes/' + operation + '/_data'}
        calls = []

        def docker(args):
            calls.append(args)
            if args[:3] == ['docker', 'info', '--format']:
                return 'linux'
            if args[:2] == ['docker', 'pull']:
                return ''
            if args[:3] == ['docker', 'image', 'inspect']:
                return json.dumps([{'RepoDigests': [args[3]]}])
            if args[:3] == ['docker', 'volume', 'create']:
                return operation
            if args[:3] == ['docker', 'volume', 'inspect']:
                return json.dumps([volume])
            if args[:3] == ['docker', 'run', '--name']:
                raise RuntimeError('synthetic_runner_create_failure')
            if args[:3] == ['docker', 'ps', '-a']:
                return ''
            if args[:3] == ['docker', 'volume', 'rm']:
                return ''
            if args[:3] == ['docker', 'volume', 'ls'] or args[:3] == ['docker', 'network', 'ls']:
                return ''
            raise AssertionError('unexpected bounded Docker command: ' + repr(args[:4]))

        with patch.object(integration.fixture, 'call', side_effect=docker):
            with patch.object(integration.uuid, 'uuid4', return_value=type('UUID', (), {'hex': '1' * 32})()):
                with patch.object(integration.sys, 'platform', 'linux'):
                    with patch.dict(os.environ, {'RUNNER_ENVIRONMENT': 'github-hosted', 'GITHUB_ACTIONS': 'true'}):
                        with self.assertRaisesRegex(RuntimeError, 'synthetic_runner_create_failure'):
                            integration.fixture.remote_run(Path('/synthetic'), transport='github-hosted')
        volume_create = calls.index(['docker', 'volume', 'create', '--label', label, operation])
        self.assertLess(max(calls.index(['docker', 'pull', image]) for image in images), volume_create)
        self.assertLess(max(calls.index(['docker', 'image', 'inspect', image]) for image in images), volume_create)

    def test_hosted_pull_failure_precedes_fixture_volume_creation(self):
        calls = []

        def docker(args):
            calls.append(args)
            if args[:3] == ['docker', 'info', '--format']:
                return 'linux'
            if args[:2] == ['docker', 'pull']:
                raise RuntimeError('synthetic_pinned_image_pull_failure')
            raise AssertionError('fixture allocation started before image prefetch: ' + repr(args[:3]))

        with patch.object(integration.fixture, 'call', side_effect=docker):
            with patch.object(integration.uuid, 'uuid4', return_value=type('UUID', (), {'hex': '1' * 32})()):
                with patch.object(integration.sys, 'platform', 'linux'):
                    with patch.dict(os.environ, {'RUNNER_ENVIRONMENT': 'github-hosted', 'GITHUB_ACTIONS': 'true'}):
                        with self.assertRaisesRegex(RuntimeError, 'synthetic_pinned_image_pull_failure'):
                            integration.fixture.remote_run(Path('/synthetic'), transport='github-hosted')
        self.assertEqual(calls[:2], [
            ['docker', 'info', '--format', '{{.OSType}}'],
            ['docker', 'pull', integration.fixture._pinned_runtime_images()[0]],
        ])


class CaptureEntryBoundsTests(unittest.TestCase):
    def exercise(self, count, *, changed=False):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); source = root / 'capture'; source.mkdir()
            base = root / 'materialized'; base.mkdir()
            names = ['nats-rollout-preservation/contract_fixture_entry.py',
                     'nats-rollout-preservation/root_cli.py', 'nats-known-baseline/kernel']
            names += ['nats-rollout-preservation/fixture-%03d.py' % index for index in range(count - len(names))]
            manifest = {}
            for name in names:
                path = source / name; path.parent.mkdir(parents=True, exist_ok=True)
                content = ('fixture:' + name).encode(); path.write_bytes(content)
                manifest[name] = hashlib.sha256(content).hexdigest()
            (source / 'capture-manifest.json').write_text(json.dumps(manifest), encoding='utf-8')
            if changed:
                (source / names[0]).write_bytes(b'changed')
            binding = Mock()
            def resolve(value):
                return source if str(value) == '/bundle/capture' else Path(value)
            real_stat = Path.stat
            kernel = base / 'scripts/staging/nats-known-baseline/kernel'
            def owned_stat(path, **kwargs):
                info = real_stat(path, **kwargs)
                if path == kernel and kwargs.get('follow_symlinks') is False:
                    return SimpleNamespace(st_mode=info.st_mode, st_uid=0)
                return info
            with patch.object(entry, 'Path', side_effect=resolve), patch.object(entry.os, 'chown'), \
                 patch.object(Path, 'stat', owned_stat), \
                 patch.object(sys, 'path', list(sys.path)), patch.dict(sys.modules, {'root_cli': SimpleNamespace(code_binding=binding)}):
                if count > 101:
                    with self.assertRaisesRegex(RuntimeError, 'captured_code_manifest_invalid'):
                        entry._materialize_capture(base)
                    self.assertFalse((base / 'scripts/staging').exists()); binding.assert_not_called()
                elif changed:
                    with self.assertRaisesRegex(RuntimeError, 'captured_code_source_changed'):
                        entry._materialize_capture(base)
                    binding.assert_not_called()
                else:
                    code = entry._materialize_capture(base)
                    self.assertEqual(json.loads((code / 'capture-manifest.json').read_text(encoding='utf-8')), manifest)
                    for name in names: self.assertEqual((code / name).read_bytes(), (source / name).read_bytes())
                    binding.assert_called_once_with(code)

    def test_accepts_current_size_and_bound(self):
        for count in (94, 101):
            with self.subTest(count=count): self.exercise(count)

    def test_rejects_over_bound_before_materialization(self):
        self.exercise(102)

    def test_rejects_changed_bytes(self):
        self.exercise(94, changed=True)


if __name__ == '__main__':
    unittest.main()
