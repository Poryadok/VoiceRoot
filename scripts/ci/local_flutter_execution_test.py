"""A Flutter execution must account for its command results and source drift."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

import local_flutter_execution as execution


class LocalFlutterExecutionTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / 'repo'
        self.root.mkdir()
        subprocess.run(['git', 'init', '-q', str(self.root)], check=True)
        self.git('config', 'user.name', 'Test')
        self.git('config', 'user.email', 'test@example.invalid')
        (self.root / 'src/frontend/test').mkdir(parents=True)
        (self.root / 'src/frontend/test/example_test.dart').write_text('test source')
        self.git('add', '.')
        self.git('commit', '-qm', 'test')
        self.head = self.git('rev-parse', 'HEAD').stdout.decode().strip()
        self.output = Path(self.tmp.name) / 'run'

    def git(self, *args):
        return subprocess.run(['git', '-C', str(self.root), *args], check=True, capture_output=True)

    def fake_flutter(self, command, cwd=None, stdout=None, stderr=None, **kwargs):
        if '--version' in command:
            return subprocess.CompletedProcess(command, 0, stdout=b'{"frameworkRevision":"test"}')
        if 'test' in command:
            self.test_env = kwargs.get('env', {})
            path = str(Path(cwd) / (command[-1] if command[-1].startswith('test/') else 'test/example_test.dart'))
            rows = [{'type':'start'}, {'type':'suite','suite':{'id':1,'path':path}},
                    {'type':'allSuites','count':1},
                    {'type':'testStart','test':{'id':2,'suiteID':1,'name':'example','metadata':{'skip':False}}},
                    {'type':'testDone','testID':2,'result':'success','skipped':getattr(self, 'skipped', False),'hidden':False},
                    {'type':'done','success':True}]
            stdout.write(('\n'.join(json.dumps(row) for row in rows) + '\n').encode())
        return subprocess.CompletedProcess(command, 0)

    def test_full_test_inventory_and_command_results_are_recorded(self):
        with mock.patch.object(execution, '_invoke', side_effect=self.fake_flutter):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['tests']['passed'], 1)
        self.assertEqual(result['tests']['skipped'], [])
        self.assertEqual(len(result['commands']), 4)
        self.assertIn('--no-pub', result['commands'][2]['argv'])
        self.assertEqual(result['status'], 'incomplete')
        self.assertTrue(result['unbound_inputs'])

    def test_failed_command_and_source_mutation_cannot_report_success(self):
        def fail(command, **kwargs):
            if 'analyze' in command:
                return subprocess.CompletedProcess(command, 2)
            return self.fake_flutter(command, **kwargs)
        with mock.patch.object(execution, '_invoke', side_effect=fail):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['status'], 'failed')
        self.assertEqual(result['commands'][-1]['exit_code'], 2)

    def test_source_change_during_command_is_rejected(self):
        def mutate(command, **kwargs):
            result = self.fake_flutter(command, **kwargs)
            if 'analyze' in command:
                (Path(kwargs['cwd']) / 'test/example_test.dart').write_text('changed')
            return result
        with mock.patch.object(execution, '_invoke', side_effect=mutate):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['status'], 'failed')
        self.assertIn('snapshot changed', result['error'])

    def test_new_test_source_during_command_is_rejected(self):
        def mutate(command, **kwargs):
            result = self.fake_flutter(command, **kwargs)
            if 'analyze' in command:
                (Path(kwargs['cwd']) / 'test/injected_test.dart').write_text('injected')
            return result
        with mock.patch.object(execution, '_invoke', side_effect=mutate):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['status'], 'failed')
        self.assertIn('unexpected', result['error'])

    def test_known_golden_diffs_are_outputs_but_added_test_sources_are_rejected(self):
        def golden_outputs(command, **kwargs):
            result = self.fake_flutter(command, **kwargs)
            if 'test' in command:
                failures = Path(kwargs['cwd']) / 'test/failures'
                failures.mkdir()
                for suffix in ('isolatedDiff', 'maskedDiff', 'masterImage', 'testImage'):
                    (failures / ('e2e_key_backup_h_' + suffix + '.png')).write_bytes(b'generated')
            return result
        with mock.patch.object(execution, '_invoke', side_effect=golden_outputs):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(result['tests']['passed'], 1)
        def injected_source(command, **kwargs):
            result = golden_outputs(command, **kwargs)
            if 'test' in command:
                (Path(kwargs['cwd']) / 'test/failures/injected_test.dart').write_text('injected')
            return result
        with mock.patch.object(execution, '_invoke', side_effect=injected_source):
            result = execution.run(self.root, self.head, self.output.parent / 'run-injected', 'flutter')
        self.assertEqual(result['status'], 'failed')
        self.assertIn('unexpected', result['error'])

    def test_dependency_cache_change_during_command_is_rejected(self):
        def mutate(command, **kwargs):
            result = self.fake_flutter(command, **kwargs)
            if 'analyze' in command:
                (Path(kwargs['env']['PUB_CACHE']) / 'injected.dart').write_text('changed')
            return result
        with mock.patch.object(execution, '_invoke', side_effect=mutate):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['status'], 'failed')
        self.assertIn('dependency cache changed', result['error'])

    def test_skipped_case_is_recorded_and_fails(self):
        self.skipped = True
        with mock.patch.object(execution, '_invoke', side_effect=self.fake_flutter):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['status'], 'failed')
        self.assertEqual(result['tests']['skipped'][0]['name'], 'example')

    def test_foreign_capture_and_voice_environment_are_not_inherited(self):
        with mock.patch.dict(os.environ, {'VOICE_UI_CAPTURE_DIR': 'foreign',
                                         'UIV3_CAPTURE_DIR': 'foreign'}):
            with mock.patch.object(execution, '_invoke', side_effect=self.fake_flutter):
                execution.run(self.root, self.head, self.output, 'flutter')
        self.assertNotIn('VOICE_UI_CAPTURE_DIR', self.test_env)
        self.assertNotIn('UIV3_CAPTURE_DIR', self.test_env)
        self.assertEqual(self.test_env['VOICE_MUSIC_CAPTURE_DIR'], str(self.output / 'music-captures'))

    def test_dependency_directory_digest_changes_for_bytes_and_inventory(self):
        cache = Path(self.tmp.name) / 'cache'
        cache.mkdir()
        (cache / 'package').mkdir()
        item = cache / 'package' / 'source.dart'
        item.write_bytes(b'one')
        first = execution.directory_digest(cache)
        item.write_bytes(b'two')
        self.assertNotEqual(first, execution.directory_digest(cache))
        item.write_bytes(b'one')
        (cache / 'package' / 'another.dart').write_bytes(b'new')
        self.assertNotEqual(first, execution.directory_digest(cache))

    def test_discovery_separates_browser_only_and_rejects_unknown_teston(self):
        test_dir = self.root / 'src/frontend/test'
        (test_dir / 'browser_test.dart').write_text("@TestOn('browser')\nlibrary;\n")
        profiles = execution.classify_suites(test_dir, ['example_test.dart', 'browser_test.dart'])
        self.assertEqual(profiles, {'vm': ['example_test.dart'], 'chrome': ['browser_test.dart']})
        (test_dir / 'unknown_test.dart').write_text("@TestOn('some-future-platform')\nlibrary;\n")
        with self.assertRaisesRegex(ValueError, 'unsupported TestOn'):
            execution.classify_suites(test_dir, ['unknown_test.dart'])
        (test_dir / 'unknown_test.dart').write_text("// @TestOn('browser')\nvoid main() {}\n")
        with self.assertRaisesRegex(ValueError, 'unsupported TestOn'):
            execution.classify_suites(test_dir, ['unknown_test.dart'])

    def test_browser_only_selection_runs_in_chrome(self):
        (self.root / 'src/frontend/test/browser_test.dart').write_text("@TestOn('browser')\nlibrary;\n")
        with mock.patch.object(execution, '_invoke', side_effect=self.fake_flutter):
            result = execution.run(self.root, self.head, self.output, 'flutter', ['browser_test.dart'])
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(result['tests']['passed'], 1)
        command = result['commands'][-1]['argv']
        self.assertIn('chrome', command)
        self.assertIn('test/browser_test.dart', command)
        self.assertNotIn('--concurrency=1', command)

    def test_full_inventory_runs_both_platforms_before_rejecting_skips(self):
        (self.root / 'src/frontend/test/browser_test.dart').write_text("@TestOn('browser')\nlibrary;\n")
        self.skipped = True
        with mock.patch.object(execution, '_invoke', side_effect=self.fake_flutter):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['status'], 'failed')
        self.assertTrue(result['full_inventory'])
        self.assertEqual(result['tests']['tests'], 2)
        self.assertEqual(len(result['tests']['skipped']), 2)
        self.assertEqual(set(result['tests']['event_sha256']), {'test-vm', 'test-chrome'})
        self.assertIn('chrome', result['commands'][-1]['argv'])

    def test_failed_chrome_does_not_accept_passing_vm_result(self):
        (self.root / 'src/frontend/test/browser_test.dart').write_text("@TestOn('browser')\nlibrary;\n")
        def fail_browser(command, **kwargs):
            if 'chrome' in command:
                return subprocess.CompletedProcess(command, 2)
            return self.fake_flutter(command, **kwargs)
        with mock.patch.object(execution, '_invoke', side_effect=fail_browser):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['status'], 'failed')
        self.assertEqual(result['tests']['passed'], 1)
        self.assertEqual(result['commands'][-1]['exit_code'], 2)


if __name__ == '__main__':
    unittest.main()
