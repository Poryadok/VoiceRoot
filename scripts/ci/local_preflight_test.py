"""Behavioral tests for the planner-driven local preflight foundation."""
import tempfile
import subprocess
import json
import unittest
from pathlib import Path
from unittest.mock import patch

import local_preflight


class LocalPreflightTests(unittest.TestCase):
    def chat_inputs(self):
        return {'helper': str(self.root / 'creator.exe'), 'helper_sha256': 'a' * 64,
                'postgres_archive': str(self.root / 'postgres.tar'), 'postgres_archive_sha256': 'b' * 64,
                'postgres_image_id': 'sha256:' + 'c' * 64}

    def chat_result(self):
        inputs = self.chat_inputs()
        return {'schema': 'voice-local-chat-private-diagnostic-v1', 'status': 'incomplete', 'source': self.source,
                'diagnostic': {'status': 'incomplete', 'source': self.source, 'exit_code': 0,
                               'events_sha256': 'd' * 64,
                               'tests': {'packages': ['voice/backend/chat/internal/store'], 'passed': 1, 'skipped': []}},
                'inputs': {'creator_sha256': inputs['helper_sha256'], 'postgres_archive_sha256': inputs['postgres_archive_sha256'],
                           'postgres_image_id': inputs['postgres_image_id']},
                'ownership_journal': 'run/tmp/ci-local/voiceci-fixture/ownership.json', 'slot': 0}

    def test_opt_in_chat_runs_only_when_affected_and_cannot_satisfy_matrix(self):
        plan = {'source': self.source, 'components': {'go_integration': ['chat']},
                'checks': {'backend-go-integration-pr': ['chat']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_chat_orchestration, 'run', return_value=self.chat_result()) as run:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter',
                                         chat_inputs=self.chat_inputs())
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['backend-go-integration-pr']['status'], 'not_run')
        self.assertEqual(report['chat_private_report'], 'chat-private/diagnostic.json')
        values = self.chat_inputs()
        run.assert_called_once_with(self.root, self.source['head'], self.output / 'chat-private',
                                    values['helper'], values['helper_sha256'], values['postgres_archive'],
                                    values['postgres_archive_sha256'], values['postgres_image_id'], 'go')

    def test_selected_chat_does_not_run_without_explicit_inputs(self):
        plan = {'source': self.source, 'components': {'go_integration': ['chat']},
                'checks': {'backend-go-integration-pr': ['chat']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_chat_orchestration, 'run') as run:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        run.assert_not_called()
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['backend-go-integration-pr']['status'], 'not_run')

    def test_chat_inputs_refuse_unselected_or_malformed_before_execution(self):
        for plan, inputs in (({'source': self.source, 'components': {'go_integration': ['bot']},
                               'checks': {'backend-go-integration-pr': ['bot']}}, self.chat_inputs()),
                             ({'source': self.source, 'components': {'go_integration': ['chat']},
                               'checks': {'backend-go-integration-pr': ['chat']}}, {'helper': 'partial'})):
            with self.subTest(inputs=inputs), tempfile.TemporaryDirectory() as tmp:
                output = Path(tmp) / 'result'
                with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                     patch.object(local_preflight.local_chat_orchestration, 'run') as run:
                    report = local_preflight.run(self.root, 'c' * 40, self.source['head'], output, 'flutter', chat_inputs=inputs)
                self.assertEqual(report['status'], 'failed')
                run.assert_not_called()

    def test_chat_mismatch_skip_or_cleanup_uncertainty_cannot_qualify(self):
        plan = {'source': self.source, 'components': {'go_integration': ['chat']},
                'checks': {'backend-go-integration-pr': ['chat']}}
        for mutation in ('source', 'skip', 'image', 'cleanup'):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as tmp:
                result = self.chat_result()
                if mutation == 'source': result['source'] = {'head': 'different'}
                if mutation == 'skip': result['diagnostic']['tests']['skipped'] = ['required']
                if mutation == 'image': result['inputs']['postgres_image_id'] = 'sha256:' + 'd' * 64
                failure = local_preflight.OwnershipError('cleanup uncertain') if mutation == 'cleanup' else None
                with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                     patch.object(local_preflight.local_chat_orchestration, 'run', side_effect=failure, return_value=result):
                    report = local_preflight.run(self.root, 'c' * 40, self.source['head'], Path(tmp) / 'result',
                                                 'flutter', chat_inputs=self.chat_inputs())
                self.assertEqual(report['status'], 'failed')
                self.assertEqual(report['checks']['backend-go-integration-pr']['status'], 'not_run')

    def test_chat_combined_execution_and_cleanup_failure_is_not_swallowed(self):
        plan = {'source': self.source, 'components': {'go_integration': ['chat']},
                'checks': {'backend-go-integration-pr': ['chat']}}
        failure = BaseExceptionGroup('uncertain', [KeyboardInterrupt(), local_preflight.OwnershipError('cleanup')])
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_chat_orchestration, 'run', side_effect=failure):
            with self.assertRaises(BaseExceptionGroup):
                local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter',
                                    chat_inputs=self.chat_inputs())
        self.assertFalse((self.output / 'preflight.json').exists())

    def test_ci_routing_subset_does_not_satisfy_script_matrix(self):
        plan = {'source': self.source, 'checks': {'ci-script-tests': ['CI change']}}
        diagnostic = {'schema': 'voice-local-ci-routing-diagnostic-v1', 'status': 'incomplete',
                      'source': self.source, 'commands': [{'name': 'gate', 'tests': 8}, {'name': 'router', 'tests': 4}]}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_ci_routing, 'run', return_value=diagnostic) as route:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        route.assert_called_once_with(self.root, self.source, self.output / 'ci-routing')
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['ci-script-tests']['status'], 'not_run')
        self.assertEqual(report['ci_routing_report'], 'ci-routing/diagnostic.json')

    def test_ci_routing_supervision_refusal_fails_closed(self):
        plan = {'source': self.source, 'checks': {'ci-script-tests': ['CI change']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_ci_routing, 'run', side_effect=local_preflight.SupervisionError('unsupported')):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertEqual(report['checks']['ci-script-tests']['status'], 'not_run')

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / 'checkout'
        self.root.mkdir()
        self.output = Path(self.temp.name) / 'report'
        self.source = {'head': 'a' * 40, 'tree': 'b' * 40}
        self.plan = {'source': self.source, 'checks': {'flutter': ['changed frontend'],
                     'flutter-unit-tests': ['changed frontend'], 'backend-auth': ['shared contract']}}

    def run_with(self, plan, flutter_result=None, final_source=None):
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=final_source or self.source), \
             patch.object(local_preflight.local_flutter_execution, 'run', return_value=flutter_result) as flutter:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        return report, flutter

    def test_selected_flutter_runs_but_unimplemented_checks_never_pass(self):
        result = {'status': 'incomplete', 'source': self.source, 'full_inventory': True,
                  'tests': {'skipped': [], 'passed': 1}}
        report, flutter = self.run_with(self.plan, result)
        flutter.assert_called_once()
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['backend-auth']['status'], 'not_run')
        self.assertEqual(report['checks']['flutter-unit-tests']['status'], 'diagnostic')
        self.assertTrue((self.output / 'preflight.json').is_file())

    def test_unselected_flutter_does_not_run(self):
        report, flutter = self.run_with({'source': self.source, 'checks': {'backend-auth': ['auth']}})
        flutter.assert_not_called()
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['backend-auth']['status'], 'not_run')

    def test_selected_current_diff_is_source_bound_and_stays_diagnostic(self):
        plan = {'source': self.source, 'checks': {'current-diff-contract': ['every pull request']}}
        diagnostic = {'status': 'diagnostic', 'source': self.source, 'base': 'c' * 40}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_current_diff, 'run', return_value=diagnostic) as current:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        current.assert_called_once_with(self.root, self.source, 'c' * 40, self.output / 'current-diff')
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['current-diff-contract']['status'], 'diagnostic')

    def test_current_diff_failure_or_wrong_base_fails_preflight(self):
        plan = {'source': self.source, 'checks': {'current-diff-contract': ['every pull request']}}
        diagnostic = {'status': 'diagnostic', 'source': self.source, 'base': 'd' * 40}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_current_diff, 'run', return_value=diagnostic):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertEqual(report['checks']['current-diff-contract']['status'], 'not_run')

    def test_current_diff_refusal_fails_preflight(self):
        plan = {'source': self.source, 'checks': {'current-diff-contract': ['every pull request']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_current_diff, 'run', side_effect=ValueError('contract refused')):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertIn('contract refused', report['error'])

    def test_selected_protobuf_runs_but_cannot_pass_preflight(self):
        plan = {'source': self.source, 'checks': {'protobuf': ['contract changed']}}
        diagnostic = {'status': 'incomplete', 'source': self.source, 'base': 'c' * 40,
                      'commands': [{'name': name, 'exit_code': 0} for name in
                                   ('buf-lint', 'buf-format', 'buf-breaking', 'go-codegen-selftest',
                                    'go-codegen', 'auth-proto-sync', 'auth-compile', 'r23-contract')],
                      'r23': {'passed': 1, 'skipped': []}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_proto_execution, 'run', return_value=diagnostic) as proto:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output,
                                         'flutter', 'feature/example', 'go-executable', 'buf-executable')
        proto.assert_called_once_with(self.root, self.source, 'c' * 40, self.output / 'protobuf',
                                      'go-executable', 'buf-executable')
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['protobuf']['status'], 'diagnostic')

    def test_failed_or_mismatched_protobuf_diagnostic_fails_preflight(self):
        plan = {'source': self.source, 'checks': {'protobuf': ['contract changed']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_proto_execution, 'run', return_value={'status': 'failed'}):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertEqual(report['checks']['protobuf']['status'], 'not_run')

    def test_selected_go_services_run_without_claiming_pkg_or_preflight_pass(self):
        plan = {'source': self.source, 'components': {'go': ['chat', 'voice']},
                'checks': {'backend-go': ['affected service'], 'backend-go-pkg': ['shared package']}}
        def service_result(name):
            return {'acquisition': {'status': 'incomplete', 'source': self.source,
                                    'module_dir': 'src/backend/' + name, 'archive_sha256': 'f' * 64},
                    'transfer': {'status': 'incomplete', 'archive_sha256': 'f' * 64},
                    'source_archive_sha256': 'e' * 64, 'slot': 0,
                    'ownership_journal': name + '/tmp/ci-local/voiceci-fixture/ownership.json',
                    'inventory': {'status': 'incomplete', 'packages_with_tests': ['voice/backend/' + name],
                                  'packages_without_tests': []},
                    'diagnostic': {'status': 'incomplete', 'exit_code': 0,
                                   'tests': {'packages': ['voice/backend/' + name], 'passed': 1, 'skipped': []}}}
        diagnostic = {'status': 'incomplete', 'source': self.source,
                      'image': {'reference': local_preflight.local_go_plan.GO_IMAGE, 'id': 'sha256:' + 'a' * 64},
                      'services': {'chat': service_result('chat'), 'voice': service_result('voice')}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_go_plan, 'run', return_value=diagnostic) as go:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output,
                                         'flutter', 'feature/example', 'go-executable')
        go.assert_called_once_with(self.root, self.source, ['chat', 'voice'], self.output / 'go', 'go-executable')
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['backend-go']['status'], 'diagnostic')
        self.assertEqual(report['checks']['backend-go-pkg']['status'], 'not_run')

    def test_go_diagnostic_must_cover_every_planned_service(self):
        plan = {'source': self.source, 'components': {'go': ['chat', 'voice']},
                'checks': {'backend-go': ['affected service']}}
        diagnostic = {'status': 'incomplete', 'source': self.source, 'services': {'chat': {}}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_go_plan, 'run', return_value=diagnostic):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertIn('did not match', report['error'])

    def test_missing_docker_fails_selected_go_preflight(self):
        plan = {'source': self.source, 'components': {'go': ['chat']},
                'checks': {'backend-go': ['affected service']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_go_plan, 'run',
                          side_effect=local_preflight.OwnershipError('Docker unavailable')):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertEqual(report['checks']['backend-go']['status'], 'not_run')
        self.assertIn('Docker unavailable', report['error'])

    def test_source_change_invalidates_run(self):
        result = {'status': 'incomplete', 'source': self.source, 'full_inventory': True,
                  'tests': {'skipped': [], 'passed': 1}}
        report, _ = self.run_with(self.plan, result, {'head': 'a' * 40, 'tree': 'd' * 40})
        self.assertEqual(report['status'], 'failed')
        self.assertIn('source changed', report['error'])

    def test_failed_flutter_diagnostic_cannot_succeed(self):
        result = {'status': 'failed', 'source': self.source, 'error': 'required test skipped'}
        report, _ = self.run_with(self.plan, result)
        self.assertEqual(report['status'], 'failed')

    def test_flutter_diagnostic_must_match_planned_source(self):
        result = {'status': 'incomplete', 'source': {'head': 'different'}, 'full_inventory': True,
                  'tests': {'skipped': [], 'passed': 1}}
        report, _ = self.run_with(self.plan, result)
        self.assertEqual(report['status'], 'failed')
        self.assertIn('does not match', report['error'])

    def test_flutter_skips_cannot_be_diagnostic_success(self):
        result = {'status': 'incomplete', 'source': self.source, 'full_inventory': True,
                  'tests': {'skipped': ['example'], 'passed': 1}}
        report, _ = self.run_with(self.plan, result)
        self.assertEqual(report['status'], 'failed')

    def test_missing_test_counts_are_not_a_completed_diagnostic(self):
        result = {'status': 'incomplete', 'source': self.source, 'full_inventory': True,
                  'tests': {'skipped': []}}
        report, _ = self.run_with(self.plan, result)
        self.assertEqual(report['status'], 'failed')

    def test_output_in_checkout_is_rejected(self):
        with self.assertRaises(ValueError):
            local_preflight.run(self.root, 'c' * 40, self.source['head'], self.root / 'report', 'flutter')

    def test_planner_failure_is_recorded_as_failed_preflight(self):
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan',
                          side_effect=subprocess.CalledProcessError(1, ['bash'], stderr='jq is required\n')):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertIsNone(report['plan'])
        self.assertIn('jq is required', report['error'])
        self.assertEqual(json.loads((self.output / 'preflight.json').read_text(encoding='utf-8')), report)

    def test_head_ref_is_forwarded_to_shared_planner(self):
        plan = {'source': self.source, 'checks': {'voice-history-audit': ['release branch']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan) as planner, \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source):
            local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter', 'codex/voice-master-release-x')
        planner.assert_called_once_with(self.root, 'c' * 40, self.source['head'], 'codex/voice-master-release-x')

    def test_real_git_source_and_planner_preserve_dirty_identity(self):
        def git(*args):
            return subprocess.check_output(['git', '-C', str(self.root), *args], stderr=subprocess.PIPE)
        git('init', '-q')
        git('config', 'user.name', 'Test')
        git('config', 'user.email', 'test@example.invalid')
        (self.root / '.github/ci').mkdir(parents=True)
        (self.root / 'scripts/ci').mkdir(parents=True)
        (self.root / 'scripts/ci/current-diff-contract.py').write_bytes(
            (Path(__file__).parent / 'current-diff-contract.py').read_bytes())
        (self.root / '.github/ci/path-filters.yml').write_text('code:\n  - "src/**"\ndocs:\n  - "docs/**"\n')
        (self.root / 'docs').mkdir()
        (self.root / 'docs/readme.md').write_text('before\n')
        git('add', '.')
        git('commit', '-qm', 'base')
        head = git('rev-parse', 'HEAD').decode().strip()
        (self.root / 'docs/readme.md').write_text('after\n')
        matrix = {'go_services': [], 'docker_go_services': [], 'integration_go_services': [],
                  'run_go': False, 'run_pkg': False}
        with patch.object(local_preflight.affected_check_plan, 'resolve_go_matrix', return_value=matrix), \
             patch.object(local_preflight.local_flutter_execution, 'run') as flutter:
            report = local_preflight.run(self.root, head, head, self.output, 'flutter')
        flutter.assert_not_called()
        self.assertEqual(report['status'], 'incomplete')
        self.assertIn('docs/readme.md', report['plan']['changed_paths'])
        self.assertNotEqual(report['plan']['source']['tree'], git('rev-parse', 'HEAD^{tree}').decode().strip())

    def test_planner_and_flutter_runner_share_real_git_snapshot(self):
        def git(*args):
            return subprocess.check_output(['git', '-C', str(self.root), *args], stderr=subprocess.PIPE)
        git('init', '-q')
        git('config', 'user.name', 'Test')
        git('config', 'user.email', 'test@example.invalid')
        (self.root / '.github/ci').mkdir(parents=True)
        (self.root / 'scripts/ci').mkdir(parents=True)
        (self.root / 'scripts/ci/current-diff-contract.py').write_bytes(
            (Path(__file__).parent / 'current-diff-contract.py').read_bytes())
        (self.root / '.github/ci/path-filters.yml').write_text('code:\n  - "src/**"\nfrontend:\n  - "src/frontend/**"\n')
        test_dir = self.root / 'src/frontend/test'
        test_dir.mkdir(parents=True)
        (test_dir / 'example_test.dart').write_text('before\n')
        git('add', '.')
        git('commit', '-qm', 'base')
        head = git('rev-parse', 'HEAD').decode().strip()
        (test_dir / 'example_test.dart').write_text('after\n')
        matrix = {'go_services': [], 'docker_go_services': [], 'integration_go_services': [],
                  'run_go': False, 'run_pkg': False}
        def invoke(command, cwd=None, stdout=None, **kwargs):
            if '--version' in command:
                return subprocess.CompletedProcess(command, 0, stdout=b'{"frameworkRevision":"fixture"}')
            if 'test' in command:
                path = str(Path(cwd) / 'test/example_test.dart')
                events = [{'type': 'start'}, {'type': 'suite', 'suite': {'id': 1, 'path': path}},
                          {'type': 'allSuites', 'count': 1},
                          {'type': 'testStart', 'test': {'id': 2, 'suiteID': 1, 'name': 'example', 'metadata': {'skip': False}}},
                          {'type': 'testDone', 'testID': 2, 'result': 'success', 'skipped': False, 'hidden': False},
                          {'type': 'done', 'success': True}]
                stdout.write(('\n'.join(json.dumps(event) for event in events) + '\n').encode())
            return subprocess.CompletedProcess(command, 0)
        with patch.object(local_preflight.affected_check_plan, 'resolve_go_matrix', return_value=matrix), \
             patch.object(local_preflight.local_flutter_execution, '_invoke', side_effect=invoke):
            report = local_preflight.run(self.root, head, head, self.output, 'flutter')
        diagnostic = json.loads((self.output / 'flutter/diagnostic.json').read_text(encoding='utf-8'))
        self.assertEqual(diagnostic['source'], report['plan']['source'])
        self.assertEqual(diagnostic['tests']['passed'], 1)
        self.assertEqual(report['checks']['flutter-unit-tests']['status'], 'diagnostic')
        self.assertEqual(report['status'], 'incomplete')


if __name__ == '__main__':
    unittest.main()
