"""Behavioral tests for the planner-driven local preflight foundation."""
import tempfile
import subprocess
import json
import unittest
from pathlib import Path
from unittest.mock import patch

import local_preflight


class LocalPreflightTests(unittest.TestCase):
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
