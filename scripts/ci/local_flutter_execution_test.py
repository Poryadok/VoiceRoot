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
        (self.root / '.github/ci').mkdir(parents=True)
        (self.root / '.github/ci/e2e-features.yml').write_text(
            'smoke_flutter: [test/example_test.dart]\n'
            'full_flutter: [test/example_test.dart, test/other_test.dart]\n'
            'a1_flutter_profile_handoff: [test/a1_test.dart]\n', encoding='utf-8')
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

    def test_full_inventory_runs_browser_suites_only_in_chrome(self):
        browser = self.root / 'src/frontend/test/browser_test.dart'
        browser.write_text("@TestOn('browser')\nlibrary;\n", encoding='utf-8')
        with mock.patch.object(execution, '_invoke', side_effect=self.fake_flutter):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        commands = [item['argv'] for item in result['commands'] if 'test' in item['argv']]
        self.assertEqual(result['status'], 'incomplete')
        self.assertTrue(result['full_inventory'])
        self.assertEqual(result['tests']['passed'], 2)
        self.assertEqual(len(commands), 2)
        self.assertIn('test/example_test.dart', commands[0])
        self.assertNotIn('test/browser_test.dart', commands[0])
        self.assertIn('test/browser_test.dart', commands[1])
        self.assertIn('chrome', commands[1])

    def test_large_inventory_is_batched_without_losing_or_repeating_suites(self):
        suites = ['suite_' + str(index).zfill(4) + '_' + 'x' * 60 + '_test.dart'
                  for index in range(300)]
        batches = execution.suite_batches(r'C:\Flutter SDK\bin\flutter.bat', suites, 'vm')
        self.assertGreater(len(batches), 1)
        self.assertEqual([suite for _, _, batch in batches for suite in batch], suites)
        self.assertEqual(len({label for label, _, _ in batches}), len(batches))
        for _, command, batch in batches:
            self.assertLessEqual(len(subprocess.list2cmdline(command).encode('utf-16-le')) // 2, 6000)
            self.assertEqual([value for value in command if value.startswith('test/')],
                             ['test/' + suite for suite in batch])
        with self.assertRaises(ValueError):
            execution.suite_batches('flutter', ['x' * 6000 + '_test.dart'], 'vm')

    def test_multiple_batches_aggregate_events_and_fail_on_later_error(self):
        other = 'another_' + 'x' * 45 + '_test.dart'
        (self.root / 'src/frontend/test' / other).write_text('test source')
        with mock.patch.object(execution, 'FLUTTER_COMMAND_LIMIT', 135):
            with mock.patch.object(execution, '_invoke', side_effect=self.fake_flutter):
                result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(result['tests']['passed'], 2)
        self.assertEqual(len(result['tests']['suites']), 2)
        self.assertEqual(set(result['tests']['event_sha256']), {'test-vm-001', 'test-vm-002'})

        def fail_second(command, **kwargs):
            if 'test' in command and 'test/example_test.dart' in command:
                return subprocess.CompletedProcess(command, 2)
            return self.fake_flutter(command, **kwargs)
        with mock.patch.object(execution, 'FLUTTER_COMMAND_LIMIT', 135):
            with mock.patch.object(execution, '_invoke', side_effect=fail_second):
                failed = execution.run(self.root, self.head, self.output.parent / 'failed', 'flutter')
        self.assertEqual(failed['status'], 'failed')
        self.assertIn('test-vm-002 failed', failed['error'])

    def test_accessibility_capture_is_enabled_and_requires_both_owned_images(self):
        suite = 'accessibility_settings_capture_test.dart'
        (self.root / 'src/frontend/test' / suite).write_text('capture test')

        def run_with_captures(command, **kwargs):
            result = self.fake_flutter(command, **kwargs)
            if 'test' in command:
                self.assertIn('--dart-define=VOICE_ACCESSIBILITY_CAPTURE=true', command)
                directory = Path(kwargs['cwd']).parents[1] / 'tmp/anton122-accessibility'
                directory.mkdir(parents=True)
                for orientation in ('H', 'V'):
                    (directory / ('accessibility-' + orientation + '.png')).write_bytes(b'\x89PNG\r\n\x1a\nfixture')
            return result

        with mock.patch.object(execution, '_invoke', side_effect=run_with_captures):
            result = execution.run(self.root, self.head, self.output, 'flutter', selected=[suite])
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(len(result['accessibility_captures']), 2)
        self.assertFalse((self.root / 'tmp/anton122-accessibility').exists())

        self.output = Path(self.tmp.name) / 'missing-captures'
        with mock.patch.object(execution, '_invoke', side_effect=self.fake_flutter):
            missing = execution.run(self.root, self.head, self.output, 'flutter', selected=[suite])
        self.assertEqual(missing['status'], 'failed')
        self.assertNotIn('accessibility_captures', missing)

    def test_accessibility_capture_is_validated_in_numbered_batch(self):
        suite = 'accessibility_settings_capture_test.dart'
        other = 'z' * 45 + '_test.dart'
        for name in (suite, other):
            (self.root / 'src/frontend/test' / name).write_text('test source')
        def invoke(command, **kwargs):
            result = self.fake_flutter(command, **kwargs)
            if 'test/' + suite in command:
                self.assertIn('--dart-define=VOICE_ACCESSIBILITY_CAPTURE=true', command)
                directory = Path(kwargs['cwd']).parents[1] / 'tmp/anton122-accessibility'
                directory.mkdir(parents=True)
                for orientation in ('H', 'V'):
                    (directory / ('accessibility-' + orientation + '.png')).write_bytes(b'\x89PNG\r\n\x1a\nfixture')
            return result
        with mock.patch.object(execution, 'FLUTTER_COMMAND_LIMIT', 160):
            with mock.patch.object(execution, '_invoke', side_effect=invoke):
                result = execution.run(self.root, self.head, self.output, 'flutter', selected=[suite, other])
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(len(result['accessibility_captures']), 2)
        self.assertEqual(set(result['tests']['event_sha256']), {'test-vm-001', 'test-vm-002'})

    def test_failed_command_and_source_mutation_cannot_report_success(self):
        def fail(command, **kwargs):
            if 'analyze' in command:
                return subprocess.CompletedProcess(command, 2)
            return self.fake_flutter(command, **kwargs)
        with mock.patch.object(execution, '_invoke', side_effect=fail):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['status'], 'failed')
        self.assertEqual(result['commands'][-1]['exit_code'], 2)

    def test_flutter_invocations_use_the_bounded_diagnostic_supervisor(self):
        completed = subprocess.CompletedProcess(['flutter', '--version'], 0, b'{}', b'')
        with mock.patch.object(execution.supervision, 'run_diagnostic', return_value=completed) as run:
            self.assertIs(execution._invoke(['flutter', '--version'], cwd=self.root,
                                            capture_output=True, check=False), completed)
        self.assertEqual(run.call_args.kwargs['timeout'], execution.FLUTTER_COMMAND_TIMEOUT)
        self.assertTrue(run.call_args.kwargs['capture_output'])

    def test_supervisor_timeout_is_reported_as_a_failed_diagnostic(self):
        def timeout(command, **kwargs):
            if 'analyze' in command:
                raise subprocess.TimeoutExpired(command, 1)
            return self.fake_flutter(command, **kwargs)
        with mock.patch.object(execution, '_invoke', side_effect=timeout):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['status'], 'failed')
        self.assertIn('timed out', result['error'])

    def test_supervisor_refusal_and_cleanup_error_fail_the_diagnostic(self):
        errors = [execution.supervision.SupervisionError('Linux is unsupported'),
                  ExceptionGroup('cleanup failed', [ValueError('unverified cleanup')])]
        for index, failure in enumerate(errors):
            with self.subTest(failure=index):
                def refuse(command, **kwargs):
                    if 'analyze' in command:
                        raise failure
                    return self.fake_flutter(command, **kwargs)
                with mock.patch.object(execution, '_invoke', side_effect=refuse):
                    result = execution.run(self.root, self.head, self.output.parent / ('failure-' + str(index)), 'flutter')
                self.assertEqual(result['status'], 'failed')
                self.assertIn('unsupported' if index == 0 else 'cleanup failed', result['error'])

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
        self.assertEqual(result['skip_classification'], [{
            'suite': 'test/example_test.dart', 'name': 'example',
            'catalog_sections': ['smoke_flutter', 'full_flutter']}])

    def test_skip_catalog_association_is_not_a_waiver_and_rejects_foreign_paths(self):
        frontend = self.root / 'src/frontend/test'
        skipped = [{'suite': str(frontend / 'other_test.dart'), 'name': 'live case'},
                   {'suite': str(frontend / 'uncatalogued_test.dart'), 'name': 'unknown case'}]
        self.assertEqual(execution.classify_skips(self.root, skipped), [
            {'suite': 'test/other_test.dart', 'name': 'live case', 'catalog_sections': ['full_flutter']},
            {'suite': 'test/uncatalogued_test.dart', 'name': 'unknown case', 'catalog_sections': []}])
        with self.assertRaisesRegex(ValueError, 'outside the source snapshot'):
            execution.classify_skips(self.root, [{'suite': str(self.root / 'foreign.dart'), 'name': 'case'}])
        (self.root / '.github/ci/e2e-features.yml').write_text('smoke_flutter: [broken\n', encoding='utf-8')
        with self.assertRaisesRegex(ValueError, 'invalid source E2E catalog'):
            execution.classify_skips(self.root, skipped)

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
