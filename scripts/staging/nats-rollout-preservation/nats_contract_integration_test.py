import os
import sys
import unittest
import json
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import nats_contract_integration as integration


class HostedTransportTests(unittest.TestCase):
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

    def test_predecessor_fixture_is_the_reviewed_9ae_chat14_role9_contract(self):
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

    def test_hosted_runner_rejection_precedes_docker(self):
        with patch.object(integration.sys, 'platform', 'linux'):
            with patch.dict(os.environ, {}, clear=True):
                with patch.object(integration.fixture, 'call') as docker:
                    with self.assertRaisesRegex(RuntimeError, 'github_hosted_fixture_runner_required'):
                        integration.fixture.remote_run(Path('/synthetic'), transport='github-hosted')
        docker.assert_not_called()

    def test_fixture_failure_reports_only_bounded_safe_exception_labels(self):
        stderr = (b'Traceback (most recent call last):\n'
                  b'RuntimeError: owned_census_fixture_command_failed\n'
                  b'AssertionError: known_record_changed\n'
                  b'ValueError: nats_census_incomplete_or_invalid\n'
                  b'RuntimeError: identifier_shaped_secret123\n'
                  b'RuntimeError: raw credential material must not appear\n')
        result = SimpleNamespace(returncode=1, stdout=b'', stderr=stderr)
        with patch.object(integration.fixture.subprocess, 'run', return_value=result):
            with self.assertRaises(RuntimeError) as raised:
                integration.fixture.call(['docker', 'run', 'synthetic'])
        self.assertEqual(str(raised.exception),
            'owned_census_fixture_command_failed: RuntimeError: owned_census_fixture_command_failed; '
            'AssertionError: known_record_changed; ValueError: nats_census_incomplete_or_invalid; RuntimeError')
        self.assertNotIn('raw credential material', str(raised.exception))
        self.assertNotIn('identifier_shaped_secret123', str(raised.exception))

    def test_failed_before_outer_creation_still_removes_exact_owned_volume(self):
        operation = 'voice-nats-rollout-fixture-' + '1' * 16
        label = 'voice.rollout.census=' + operation
        volume = {'Name': operation, 'Labels': {'voice.rollout.census': operation},
                  'Mountpoint': '/var/lib/docker/volumes/' + operation + '/_data'}

        def docker(args):
            if args[:3] == ['docker', 'info', '--format']:
                return 'linux'
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


if __name__ == '__main__':
    unittest.main()
