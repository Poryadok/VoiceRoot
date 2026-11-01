import os
import base64
import subprocess
import sys
import unittest
import json
import hashlib
import tempfile
import shutil
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

import nats_contract_integration as integration
import contract_fixture_entry as capture_entry


class HostedTransportTests(unittest.TestCase):
    @unittest.skipUnless(sys.platform == 'linux', 'Linux custody modules')
    def test_music_probe_only_accepts_success_and_fixed_denial_marker(self):
        sys.path.insert(0, str(Path(__file__).parent.parent/'nats-known-baseline'))
        self.addCleanup(sys.path.pop, 0)
        for status, output, accepted in (('0','FIXTURE_MUSIC_PUBLISH_DENIED\n',True),
                ('1','FIXTURE_MUSIC_PUBLISH_DENIED\n',False),('0','broker timeout',False)):
            with self.subTest(status=status, output=output), tempfile.TemporaryDirectory() as directory:
                runtime=Mock();runtime.base=Path(directory);runtime.create.return_value='probe'
                runtime.inspect.return_value={'State':{'Running':False}}
                runtime.run.side_effect=lambda args, **kwargs: status if args[0]=='wait' else output if args[0]=='logs' else ''
                actor=SimpleNamespace(runtime=runtime,broker='broker',assert_closed_isolation=Mock())
                def copy(_source,target):Path(target).write_bytes(b'fixture')
                with patch.object(integration.shutil,'copyfile',side_effect=copy), patch.object(integration.os,'chown'):
                    if accepted:integration._prove_fixture_music_denied(actor)
                    else:
                        with self.assertRaisesRegex(RuntimeError,'fixture_music_acl_probe_failed'):
                            integration._prove_fixture_music_denied(actor)

    @unittest.skipUnless(sys.platform == 'linux' and os.geteuid() == 0, 'Linux root custody')
    def test_real_bootstrap_prove_checks_fixture_account_before_runtime(self):
        sys.path.insert(0, str(Path(__file__).parent.parent/'nats-known-baseline'))
        self.addCleanup(sys.path.pop, 0)
        import bootstrap_auth
        keys = ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')
        raw = {key: ('generated-' + key).encode() for key in keys}
        operator = {'data': {key: base64.b64encode(value).decode() for key,value in raw.items()}}
        account = hashlib.sha256(raw['account.public']).hexdigest()
        actor = hashlib.sha256(b'generated-actor-public').hexdigest()
        installation = integration.fixture_installation(account, actor, operator)
        foreign = integration.fixture_installation(hashlib.sha256(b'foreign').hexdigest(), actor, operator)
        with tempfile.TemporaryDirectory() as directory, patch.object(bootstrap_auth.guard, 'ROOT', Path(directory)):
            with patch.object(bootstrap_auth, 'DockerRuntime', side_effect=RuntimeError('fixture_reached_runtime')):
                with self.assertRaisesRegex(RuntimeError, 'fixture_reached_runtime'):
                    bootstrap_auth.prove(b'generated-credentials', operator, lambda:None, installation=installation)
                with self.assertRaisesRegex(bootstrap_auth.Blocked, 'bootstrap_auth_account_changed'):
                    bootstrap_auth.prove(b'generated-credentials', operator, lambda:None, installation=foreign)

    @unittest.skipUnless(sys.platform == 'linux', 'Linux custody modules')
    def test_fixture_authority_binds_generated_identities_and_negative_reaches_music(self):
        sys.path.insert(0, str(Path(__file__).parent.parent/'nats-known-baseline'))
        self.addCleanup(sys.path.pop, 0)
        import resource_claim
        import bootstrap_auth
        from nats_contract_actor import Actor
        from nats_contract_plan import ContractError
        operator = {'data': {'account.jwt': 'fixture-account-jwt', 'operator.jwt': 'fixture-operator-jwt'}}
        account = hashlib.sha256(b'generated fixture account public key').hexdigest()
        actor = hashlib.sha256(b'generated fixture actor public key').hexdigest()
        installation = integration.fixture_installation(account, actor, operator)
        self.assertIs(type(installation), resource_claim.InstallationAuthority)
        self.assertEqual(installation.profile, 'owned-isolated-v1')
        self.assertEqual((installation.account_sha256, installation.bootstrap_actor_sha256), (account, actor))
        self.assertNotEqual(installation.genesis_sha256, '0'*64)
        seen = []
        def prover(credentials, operator_arg, unchanged, *, installation):
            seen.append((credentials, installation))
            if credentials == b'old':
                Actor.mutate(None, '$JS.API.CONSUMER.CREATE.file_events.messaging_music_metadata', {})
            return {'verified': True}
        def denied(self, api, payload): raise ContractError('nats_contract_actor_refused')
        with patch.object(bootstrap_auth, 'prove', side_effect=prover), patch.object(integration, '_prove_fixture_music_denied') as probe:
            self.assertTrue(integration.prove_fixture_bootstrap(b'old', b'new', operator, installation)['verified'])
            probe.assert_called_once()
        self.assertEqual(seen, [(b'old', installation), (b'new', installation)])
        def wrong_failure(*args, **kwargs): raise bootstrap_auth.Blocked('bootstrap_auth_account_changed')
        with patch.object(bootstrap_auth, 'prove', side_effect=wrong_failure):
            with self.assertRaisesRegex(bootstrap_auth.Blocked, 'bootstrap_auth_account_changed'):
                integration.prove_fixture_bootstrap(b'old', b'new', operator, installation)
        def other_api(credentials, operator_arg, unchanged, *, installation):
            Actor.mutate(None, '$JS.API.CONSUMER.CREATE.social_events.other', {})
        with patch.object(bootstrap_auth, 'prove', side_effect=other_api), patch.object(Actor, 'mutate', denied):
            with self.assertRaisesRegex(ContractError, 'nats_contract_actor_refused'):
                integration.prove_fixture_bootstrap(b'old', b'new', operator, installation)
        with patch.object(bootstrap_auth, 'prove', side_effect=prover), patch.object(integration, '_prove_fixture_music_denied', side_effect=RuntimeError('fixture_music_acl_probe_failed')):
            with self.assertRaisesRegex(RuntimeError, 'fixture_music_acl_probe_failed'):
                integration.prove_fixture_bootstrap(b'old', b'new', operator, installation)

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


class FixtureIsolationTest(unittest.TestCase):
    def test_story_probe_is_compact_and_rejects_oversized_capture(self):
        with tempfile.TemporaryDirectory() as directory:
            bundle = Path(directory)
            commands = []
            def build(args, **kwargs):
                commands.append(args)
                output = Path(args[args.index('-o') + 1])
                if output.name == 'story-current4-probe':
                    output.write_bytes(b'\x7fELF' + b'X' * (16 << 20))
                else:
                    output.write_bytes(b'\x7fELFfixture')
            with patch.object(integration.fixture, 'call', side_effect=build):
                with self.assertRaisesRegex(RuntimeError, 'fixture_capture_binary_too_large'):
                    integration.build_fixture_records(bundle)
            story = next(args for args in commands if args[-1] == './cmd/owned-story-probe')
            self.assertIn('-ldflags=-s -w', story)

    def test_capture_is_production_bundle_and_drivers_are_separate(self):
        with tempfile.TemporaryDirectory() as directory:
            bundle = Path(directory)
            def build(args, **kwargs):
                Path(args[args.index('-o') + 1]).write_bytes(b'\x7fELFfixture')
            with patch.object(integration.fixture, 'call', side_effect=build):
                integration.build_fixture_records(bundle)
            manifest = json.loads((bundle / 'capture/capture-manifest.json').read_bytes())
            self.assertEqual(len(manifest), 99)
            for name, digest in manifest.items():
                self.assertEqual(hashlib.sha256((bundle/'capture'/name).read_bytes()).hexdigest(), digest)
            for name in ('contract_fixture_entry.py', 'nats_contract_integration.py',
                         'nats_a3_roster_integration.py', 'rollout_census_integration.py'):
                self.assertNotIn('nats-rollout-preservation/' + name, manifest)
                self.assertTrue((bundle / name).is_file())
            helper = (bundle / 'helper.go').read_text(encoding='utf-8')
            acl_probe = helper.split('func musicACLDenial()',1)[1].split('func main()',1)[0]
            self.assertIn('nats.UserCredentials("/inputs/bootstrap.creds")', acl_probe)
            self.assertIn('errors.Is(err,nats.ErrPermissionViolation)', acl_probe)
            self.assertIn('FIXTURE_MUSIC_PUBLISH_DENIED', acl_probe)
            for source_bound_seed in ('SocialMaxAge int64', 'SocialDuplicateWindow int64',
                    'social.Config.Duplicates=time.Duration(baseline.SocialDuplicateWindow)',
                    'js.AddConsumer(expected.Stream,&nats.ConsumerConfig',
                    'js.ConsumerInfo(expected.Stream,expected.Name)'):
                self.assertIn(source_bound_seed, helper)

    @unittest.skipUnless(sys.platform == 'linux' and os.geteuid() == 0, 'Linux root custody')
    def test_binding_rejects_spoof_and_corruption_before_driver(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bundle = root / 'bundle'
            bundle.mkdir()
            def build(args, **kwargs):
                Path(args[args.index('-o') + 1]).write_bytes(b'\x7fELFfixture')
            with patch.object(integration.fixture, 'call', side_effect=build):
                integration.build_fixture_records(bundle)
            source = bundle / 'capture'
            entry = Path(__file__).with_name('contract_fixture_entry.py')
            spoof = root / 'spoof'
            spoof.mkdir()
            (spoof / 'root_cli.py').write_text("raise RuntimeError('spoof_executed')\r\n")
            (spoof / 'guard.py').write_text("raise RuntimeError('spoof_executed')\n")
            (spoof / 'fixture_driver.py').write_text("import guard\ndef run(): print('DRIVER_EXECUTED')\n")
            script = '''
import importlib.util, pathlib, sys, types
entry, source, base, spoof, preload = sys.argv[1:]
spec = importlib.util.spec_from_file_location('entry', entry)
module = importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
module._CAPTURE_SOURCE = pathlib.Path(source)
sys.path.insert(0, spoof)
if preload != 'no': sys.modules[preload] = types.ModuleType(preload)
module._materialize_capture(base)
import root_cli, guard
assert pathlib.Path(root_cli.__file__) == pathlib.Path(base)/'scripts/staging/nats-rollout-preservation/root_cli.py'
assert pathlib.Path(guard.__file__) == pathlib.Path(base)/'scripts/staging/nats-rollout-preservation/guard.py'
print('REAL_BINDING_COMPLETED')
import fixture_driver
fixture_driver.run()
'''
            def run(capture, name, preload='no'):
                return subprocess.run([sys.executable, '-I', '-c', script, str(entry), str(capture),
                    str(root/name), str(spoof), preload], capture_output=True, text=True)
            good = run(source, 'good')
            self.assertEqual(good.returncode, 0, good.stderr)
            self.assertIn('REAL_BINDING_COMPLETED', good.stdout)
            self.assertIn('DRIVER_EXECUTED', good.stdout)
            for name in ('root_cli', 'guard'):
                collision = run(source, 'collision-'+name, name)
                self.assertNotEqual(collision.returncode, 0)
                self.assertIn('captured_module_collision', collision.stderr)
                self.assertNotIn('DRIVER_EXECUTED', collision.stdout)
            def change_manifest(path, change):
                index = path/'capture-manifest.json'
                manifest = json.loads(index.read_text())
                change(manifest)
                index.write_text(json.dumps(manifest, sort_keys=True))
            def remove_member(manifest):
                manifest.pop(next(iter(manifest)))
            def add_member(manifest):
                manifest['nats-rollout-preservation/extra.py'] = '0'*64
            def traverse(manifest):
                manifest['../escape.py'] = manifest.pop(next(iter(manifest)))
            def link_member(path):
                member = path/'nats-rollout-preservation/root_cli.py'
                member.unlink()
                member.symlink_to(source/'nats-rollout-preservation/root_cli.py')
            for name, mutate in (
                ('member', lambda p: (p/'nats-rollout-preservation/root_cli.py').write_text('spoof')),
                ('manifest', lambda p: (p/'capture-manifest.json').write_text('{}')),
                ('duplicate', lambda p: (p/'capture-manifest.json').write_text('{"x":1,"x":2}')),
                ('order', lambda p: (p/'capture-manifest.json').write_text(json.dumps(
                    dict(reversed(list(json.loads((p/'capture-manifest.json').read_text()).items())))))),
                ('omitted', lambda p: change_manifest(p, remove_member)),
                ('extra', lambda p: change_manifest(p, add_member)),
                ('traversal', lambda p: change_manifest(p, traverse)),
                ('symlink', link_member),
            ):
                variant = root / name
                shutil.copytree(source, variant)
                mutate(variant)
                result = run(variant, name+'-base')
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertNotIn('REAL_BINDING_COMPLETED', result.stdout)
                self.assertNotIn('DRIVER_EXECUTED', result.stdout)
                self.assertNotIn('spoof_executed', result.stderr)
            (root/'occupied/scripts/staging').mkdir(parents=True)
            occupied = run(source, 'occupied')
            self.assertNotEqual(occupied.returncode, 0)
            self.assertIn('captured_code_destination_exists', occupied.stderr)


if __name__ == '__main__':
    unittest.main()
