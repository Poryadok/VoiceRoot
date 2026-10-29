"""Required Go integration cases must execute, not merely be selected."""
import importlib.util
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('receipts', ROOT / 'scripts/ci/go_integration_receipts.py')
receipts = importlib.util.module_from_spec(spec)
spec.loader.exec_module(receipts)


class GoIntegrationReceiptTests(unittest.TestCase):
    def test_story_lfp_inventory_covers_all_current_package_tests(self):
        inventory = json.loads((ROOT / 'scripts/ci/story_lfp_required.json').read_text(encoding='utf-8'))
        cases = inventory['story']
        self.assertEqual(len(cases), len({tuple(case) for case in cases}))
        discovered = set()
        for name in ('lfpconsentstore', 'lfpconsenttransport'):
            package = 'voice/backend/story/internal/' + name
            for path in (ROOT / 'src/backend/story/internal' / name).glob('*_test.go'):
                source = path.read_text(encoding='utf-8')
                self.assertNotRegex(source, r'(?m)^//go:build ', 'review tagged Story LFP tests explicitly')
                for test in re.findall(r'(?m)^func (Test\w+)\(t \*testing\.T\)', source):
                    discovered.add((package, test))
        self.assertTrue(discovered)
        self.assertEqual({tuple(case) for case in cases}, discovered)

    def test_user_association_inventory_covers_all_current_package_tests(self):
        inventory = json.loads((ROOT / 'scripts/ci/user_association_required.json').read_text(encoding='utf-8'))
        cases = inventory['user']
        package = 'voice/backend/user/internal/identityassociation'
        discovered = set()
        for path in (ROOT / 'src/backend/user/internal/identityassociation').glob('*_test.go'):
            source = path.read_text(encoding='utf-8')
            self.assertNotRegex(source, r'(?m)^//go:build ', 'review tagged User association tests explicitly')
            discovered.update((package, test) for test in
                              re.findall(r'(?m)^func (Test\w+)\(t \*testing\.T\)', source))
        self.assertTrue(discovered)
        self.assertEqual(len(cases), len({tuple(case) for case in cases}))
        self.assertEqual({tuple(case) for case in cases}, discovered)

    def test_required_case_must_run_and_pass_exactly_once(self):
        required = [('voice/backend/chat/internal/store', 'TestStoreIntegration')]
        events = [{'Package': required[0][0], 'Test': required[0][1], 'Action': action}
                  for action in ('run', 'pass')]
        events.append({'Package': required[0][0], 'Action': 'pass'})
        self.assertEqual(receipts.verify_required(events, required, 0), [])
        for altered in ([], events[:1], events[:2],
                        [events[0], dict(events[1], Action='skip'), events[2]],
                        [events[0], dict(events[1], Action='fail'), events[2]], events + events):
            with self.subTest(events=altered):
                self.assertTrue(receipts.verify_required(altered, required, 0))
        self.assertTrue(receipts.verify_required(events, required, 1))

    def test_no_skip_scope_rejects_skipped_subtests_and_packages(self):
        package = 'voice/backend/story/internal/lfpconsentstore'
        events = [{'Package': package, 'Test': 'TestRequired', 'Action': 'run'},
                  {'Package': package, 'Test': 'TestRequired', 'Action': 'pass'},
                  {'Package': package, 'Action': 'pass'}]
        self.assertEqual(receipts.verify_no_skips(events, [package]), [])
        for skipped in ({'Package': package, 'Test': 'TestOther/subcase', 'Action': 'skip'},
                        {'Package': package, 'Action': 'skip'}):
            self.assertTrue(receipts.verify_no_skips(events + [skipped], [package]))
        self.assertEqual(receipts.verify_no_skips(events + [
            {'Package': 'voice/backend/other', 'Test': 'TestOptional', 'Action': 'skip'}], [package]), [])
        with self.assertRaises(ValueError):
            receipts.verify_no_skips(events, [])

    def test_package_identity_matters_and_empty_inventory_is_invalid(self):
        required = [('voice/backend/chat/internal/store', 'TestStoreIntegration')]
        events = [{'Package': 'voice/backend/chat/other', 'Test': required[0][1], 'Action': action}
                  for action in ('run', 'pass')]
        self.assertTrue(receipts.verify_required(events, required, 0))
        with self.assertRaises(ValueError):
            receipts.verify_required(events, [], 0)
        with self.assertRaises(ValueError):
            receipts.verify_required(events, required + required, 0)

    def test_event_stream_rejects_malformed_or_truncated_input(self):
        line = json.dumps({'Package': 'voice/backend/chat', 'Test': 'TestCase', 'Action': 'pass'})
        self.assertEqual(receipts.read_events(line + '\n')[0]['Action'], 'pass')
        for text in ('', line, line + '\n{bad json}\n', '[]\n'):
            with self.subTest(text=text), self.assertRaises(ValueError):
                receipts.read_events(text)

    def test_summary_counts_named_cases_and_preserves_skips(self):
        events = [
            {'Package': 'voice/backend/chat/store', 'Test': 'TestA', 'Action': 'run'},
            {'Package': 'voice/backend/chat/store', 'Test': 'TestA', 'Action': 'pass'},
            {'Package': 'voice/backend/chat/store', 'Test': 'TestB', 'Action': 'run'},
            {'Package': 'voice/backend/chat/store', 'Test': 'TestB', 'Action': 'skip'},
            {'Package': 'voice/backend/chat/store', 'Action': 'pass'},
        ]
        summary = receipts.summarize_events(events)
        self.assertEqual(summary['counts'], {'pass': 1, 'skip': 1, 'fail': 0})
        self.assertEqual(summary['skipped'], [['voice/backend/chat/store', 'TestB']])
        self.assertEqual(summary['packages'], {'voice/backend/chat/store': 'pass'})

    def test_cli_rejects_skipped_required_case(self):
        package = 'voice/backend/analytics/internal/store'
        name = 'TestClickHouseInsertBatch'
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'events.jsonl'
            def run(action):
                events = [{'Package': package, 'Test': name, 'Action': 'run'},
                          {'Package': package, 'Test': name, 'Action': action},
                          {'Package': package, 'Action': 'pass'}]
                path.write_text(''.join(json.dumps(e) + '\n' for e in events), encoding='utf-8')
                return subprocess.run([sys.executable, str(ROOT / 'scripts/ci/go_integration_receipts.py'),
                                       '--events', str(path), '--package', package, '--test', name,
                                       '--exit-code', '0'],
                                      capture_output=True, text=True)
            self.assertEqual(run('pass').returncode, 0)
            self.assertNotEqual(run('skip').returncode, 0)

    def test_cli_writes_source_bound_diagnostic_summary_on_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'events.jsonl'
            out = Path(directory) / 'summary.json'
            events = [{'Package': 'voice/backend/chat/store', 'Test': 'TestA', 'Action': 'run'},
                      {'Package': 'voice/backend/chat/store', 'Test': 'TestA', 'Action': 'fail'},
                      {'Package': 'voice/backend/chat/store', 'Action': 'fail'}]
            path.write_text(''.join(json.dumps(e) + '\n' for e in events), encoding='utf-8')
            result = subprocess.run([sys.executable, str(ROOT / 'scripts/ci/go_integration_receipts.py'),
                '--events', str(path), '--exit-code', '1', '--summary', str(out), '--service', 'chat',
                '--candidate', 'a' * 40, '--run', '123', '--attempt', '1'], capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            summary = json.loads(out.read_text(encoding='utf-8'))
            self.assertEqual(summary['identity']['candidate'], 'a' * 40)
            self.assertEqual(summary['counts']['fail'], 1)
            self.assertEqual(summary['exit_code'], 1)

    def test_real_go_json_distinguishes_pass_and_skip(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'go.mod').write_text('module example.org/ci-fixture\n\ngo 1.20\n', encoding='utf-8')
            (root / 'case_test.go').write_text(
                'package fixture\nimport "testing"\n'
                'func TestRuns(t *testing.T) {}\n'
                'func TestSkips(t *testing.T) { t.Skip("fixture") }\n', encoding='utf-8')
            env = dict(os.environ, GOWORK='off', GOFLAGS='', GOTOOLCHAIN='local')
            output = subprocess.check_output(['go', 'test', '-json', '-count=1', './...'],
                                             cwd=root, env=env, text=True)
            events = receipts.read_events(output)
            self.assertEqual(receipts.verify_required(events, [('example.org/ci-fixture', 'TestRuns')], 0), [])
            self.assertTrue(receipts.verify_required(events, [('example.org/ci-fixture', 'TestSkips')], 0))

    def test_matrix_requires_each_selected_service_at_exact_candidate(self):
        identity = {'candidate': 'a' * 40, 'run': '123', 'attempt': '2'}
        required = {'chat': [('voice/backend/chat/store', 'TestIntegration')]}
        events = [{'Package': 'voice/backend/chat/store', 'Test': 'TestIntegration', 'Action': a}
                  for a in ('run', 'pass')]
        events.append({'Package': 'voice/backend/chat/store', 'Action': 'pass'})
        receipt = {'service': 'chat', 'identity': identity, 'exit_code': 0, 'events': events}
        self.assertEqual(receipts.verify_matrix(['chat'], required, [receipt], identity), [])
        for submitted in ([], [receipt, receipt],
                          [dict(receipt, identity=dict(identity, candidate='b' * 40))],
                          [dict(receipt, identity=dict(identity, attempt='1'))],
                          [dict(receipt, exit_code=1)], [dict(receipt, events=events[:1])]):
            with self.subTest(submitted=submitted):
                self.assertTrue(receipts.verify_matrix(['chat'], required, submitted, identity))
        self.assertTrue(receipts.verify_matrix(['chat'], {}, [receipt], identity))
        wrong_package = {'chat': [('voice/backend/other/store', 'TestIntegration')]}
        wrong_events = [dict(event, Package='voice/backend/other/store') for event in events]
        self.assertTrue(receipts.verify_matrix(['chat'], wrong_package,
                                               [dict(receipt, events=wrong_events)], identity))
        with self.assertRaises(ValueError):
            receipts.verify_matrix(['../chat'], required, [receipt], identity)
        with self.assertRaises(ValueError):
            receipts.verify_matrix(['chat'], required, [receipt], dict(identity, candidate='0' * 40))

    def test_receipt_requires_exact_selected_case_inventory(self):
        identity = {'candidate': 'a' * 40, 'run': '123', 'attempt': '2'}
        inventory = {'chat': [['voice/backend/chat/store', 'TestIntegration']]}
        events = [{'Package': 'voice/backend/chat/store', 'Test': 'TestIntegration', 'Action': a}
                  for a in ('run', 'pass')]
        events.append({'Package': 'voice/backend/chat/store', 'Action': 'pass'})
        receipt = receipts.make_receipt('chat', identity, inventory, events, 0)
        self.assertEqual(receipts.verify_matrix(['chat'], inventory, [receipt], identity), [])
        unrelated = {'Package': 'voice/backend/chat/other', 'Test': 'TestOther', 'Action': 'pass',
                     'Output': 'unrelated log text'}
        filtered = receipts.make_receipt('chat', identity, inventory, events + [unrelated], 0)
        self.assertNotIn(unrelated, filtered['events'])
        for altered in ([], events[:1]):
            with self.subTest(events=altered), self.assertRaises(ValueError):
                receipts.make_receipt('chat', identity, inventory, altered, 0)
        with self.assertRaises(ValueError):
            receipts.make_receipt('chat', identity, inventory, events, 1)

    def test_matrix_cli_binds_receipts_and_rejects_missing_or_skipped_cases(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            inventory = {'chat': [['voice/backend/chat/store', 'TestStore']],
                         'user': [['voice/backend/user/store', 'TestStore']]}
            inventory_file = root / 'inventory.json'
            inventory_file.write_text(json.dumps(inventory), encoding='utf-8')
            script = str(ROOT / 'scripts/ci/go_integration_matrix.py')
            common = ['--inventory', str(inventory_file), '--candidate', 'a' * 40,
                      '--run', '123', '--attempt', '1']
            files = []
            for service in inventory:
                package, name = inventory[service][0]
                events = [{'Package': package, 'Test': name, 'Action': action}
                          for action in ('run', 'pass')]
                events.append({'Package': package, 'Action': 'pass'})
                path = root / (service + '.jsonl')
                path.write_text(''.join(json.dumps(event) + '\n' for event in events), encoding='utf-8')
                receipt = root / (service + '.json')
                args = [sys.executable, script, 'produce', *common, '--service', service,
                        '--events', str(path), '--exit-code', '0', '--output', str(receipt)]
                self.assertEqual(subprocess.run(args, capture_output=True).returncode, 0)
                files.append((receipt, path))
            verify = [sys.executable, script, 'verify', *common, '--services', '["chat","user"]']
            both = [arg for receipt, events in files
                    for arg in ('--receipt', str(receipt), '--events', str(events))]
            self.assertEqual(subprocess.run(verify + both, capture_output=True).returncode, 0)
            strict = [sys.executable, script, 'produce', *common, '--service', 'chat',
                      '--events', str(files[0][1]), '--exit-code', '0', '--reject-skips',
                      '--output', str(root / 'strict.json')]
            self.assertEqual(subprocess.run(strict, capture_output=True).returncode, 0)
            strict_verify = [sys.executable, script, 'verify', *common, '--services', '["chat"]',
                             '--require-no-skips', '--events', str(files[0][1]), '--receipt']
            self.assertEqual(subprocess.run(strict_verify + [str(root / 'strict.json')],
                                            capture_output=True).returncode, 0)
            self.assertNotEqual(subprocess.run(strict_verify + [str(files[0][0])],
                                               capture_output=True).returncode, 0)
            self.assertEqual(json.loads(files[0][0].read_text(encoding='utf-8'))['schema'],
                             'voice-go-integration-receipt-v1')
            self.assertNotEqual(subprocess.run(verify + both[:4], capture_output=True).returncode, 0)
            self.assertNotEqual(subprocess.run(verify + both + both[:4], capture_output=True).returncode, 0)
            self.assertNotEqual(subprocess.run(verify + ['--receipt', str(files[0][0])],
                                               capture_output=True).returncode, 0)
            different_attempt = verify.copy()
            different_attempt[different_attempt.index('--attempt') + 1] = '2'
            self.assertNotEqual(subprocess.run(different_attempt + both, capture_output=True).returncode, 0)
            changed = json.loads(files[0][0].read_text(encoding='utf-8'))
            del changed['schema']
            files[0][0].write_text(json.dumps(changed), encoding='utf-8')
            self.assertNotEqual(subprocess.run(verify + both, capture_output=True).returncode, 0)
            changed['schema'] = 'voice-go-integration-receipt-v1'
            changed['identity']['candidate'] = 'b' * 40
            files[0][0].write_text(json.dumps(changed), encoding='utf-8')
            self.assertNotEqual(subprocess.run(verify + both, capture_output=True).returncode, 0)
            changed['identity']['candidate'] = 'a' * 40
            files[0][0].write_text(json.dumps(changed), encoding='utf-8')
            original = files[0][1].read_text(encoding='utf-8')
            files[0][1].write_text(original + '{}\n', encoding='utf-8')
            self.assertNotEqual(subprocess.run(verify + both, capture_output=True).returncode, 0)
            skipped = original.replace('"pass"', '"skip"', 1)
            files[0][1].write_text(skipped, encoding='utf-8')
            changed['event_sha256'] = hashlib.sha256(skipped.encode('utf-8')).hexdigest()
            files[0][0].write_text(json.dumps(changed), encoding='utf-8')
            self.assertNotEqual(subprocess.run(verify + both, capture_output=True).returncode, 0)
            files[0][1].write_text(original, encoding='utf-8')
            bad = root / 'chat.jsonl'
            bad.write_text(bad.read_text(encoding='utf-8').replace('"pass"', '"skip"', 1), encoding='utf-8')
            failed = subprocess.run([sys.executable, script, 'produce', *common, '--service', 'chat',
                                     '--events', str(bad), '--exit-code', '0',
                                     '--output', str(root / 'invalid.json')], capture_output=True)
            self.assertNotEqual(failed.returncode, 0)
            self.assertFalse((root / 'invalid.json').exists())
            bad.write_text(original + json.dumps({'Package': 'voice/backend/chat/store',
                                                  'Test': 'TestOther', 'Action': 'skip'}) + '\n',
                           encoding='utf-8')
            failed = subprocess.run([sys.executable, script, 'produce', *common, '--service', 'chat',
                                     '--events', str(bad), '--exit-code', '0', '--reject-skips',
                                     '--output', str(root / 'invalid-skip.json')], capture_output=True)
            self.assertNotEqual(failed.returncode, 0)
            self.assertFalse((root / 'invalid-skip.json').exists())


if __name__ == '__main__':
    unittest.main()
